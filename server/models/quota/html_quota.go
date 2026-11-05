package quota

import (
	"encoding/json"
	"fmt"
	"nvs-server/models"
	softdel "nvs-server/models/softdel"
	"strings"
	"time"
)

// ==================== HTML / WASM 托管额度 ====================
//
// 站长可对「每人能托管多少项目、占多少空间」设限，超额需要申请空间。
// 为什么放在 platform_configs 而不是 .env：这类运营参数需要**即时生效**，
// 而 .env 改了要重启；同时站长会在后台反复调，落库更合适（与访问设置同构）。
//
// 两级配置：
//   1) 全局默认（platform_configs.html_quota）
//   2) 按人覆盖（html_quota_overrides）—— 站长给某个作者单独放宽/收紧

// HTMLQuotaSetting HTML/WASM 托管额度（全局默认）
type HTMLQuotaSetting struct {
	Enabled        bool      `json:"enabled"`           // 总开关；关闭后不限量
	MaxProjects    int       `json:"max_projects"`      // 每人最多项目数（0=不限）
	MaxTotalSizeMB int       `json:"max_total_size_mb"` // 每人累计总大小上限（MB，0=不限）
	RequireApply   bool      `json:"require_apply"`     // 超额时提示「向站长申请空间」
	UpdatedAt      time.Time `json:"updated_at"`
}

const htmlQuotaCfgKey = "html_quota"

// DefaultHTMLQuotaSetting 默认：启用，每人 10 个项目、200MB。
// 取值刻意保守——托管的是可执行前端产物，默认不该无限量。
func DefaultHTMLQuotaSetting() HTMLQuotaSetting {
	return HTMLQuotaSetting{
		Enabled: true, MaxProjects: 10, MaxTotalSizeMB: 200, RequireApply: true,
	}
}

// LoadHTMLQuotaSetting 读取全局额度配置（缺失或损坏时回退默认值）
func LoadHTMLQuotaSetting() HTMLQuotaSetting {
	c := DefaultHTMLQuotaSetting()
	raw := models.GetPlatformConfig(htmlQuotaCfgKey)
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return DefaultHTMLQuotaSetting()
		}
	}
	if c.MaxProjects < 0 {
		c.MaxProjects = 0
	}
	if c.MaxTotalSizeMB < 0 {
		c.MaxTotalSizeMB = 0
	}
	return c
}

// SaveHTMLQuotaSetting 保存全局额度配置
func SaveHTMLQuotaSetting(c HTMLQuotaSetting) error {
	c.UpdatedAt = time.Now()
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return models.SetPlatformConfig(htmlQuotaCfgKey, string(b))
}

// HTMLQuotaOverride 按人覆盖（0 表示「跟随全局」，-1 表示「不限」）
type HTMLQuotaOverride struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"uniqueIndex;not null" json:"user_id"`
	MaxProjects    int       `gorm:"default:0" json:"max_projects"`
	MaxTotalSizeMB int       `gorm:"default:0" json:"max_total_size_mb"`
	Note           string    `gorm:"size:255" json:"note"` // 申请理由 / 站长备注
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (HTMLQuotaOverride) TableName() string { return "html_quota_overrides" }

// EffectiveHTMLQuota 计算某用户的实际额度：
// 有覆盖则用覆盖值（>0 生效；0 表示该维度跟随全局；-1 表示不限），否则用全局值。
func EffectiveHTMLQuota(userID uint) (maxProjects, maxTotalMB int, enabled bool) {
	g := LoadHTMLQuotaSetting()
	if !g.Enabled {
		return 0, 0, false
	}
	maxProjects, maxTotalMB = g.MaxProjects, g.MaxTotalSizeMB
	var ov HTMLQuotaOverride
	if err := models.DB.Where("user_id = ?", userID).First(&ov).Error; err == nil {
		if ov.MaxProjects != 0 {
			if ov.MaxProjects < 0 {
				maxProjects = 0 // -1 = 不限
			} else {
				maxProjects = ov.MaxProjects
			}
		}
		if ov.MaxTotalSizeMB != 0 {
			if ov.MaxTotalSizeMB < 0 {
				maxTotalMB = 0
			} else {
				maxTotalMB = ov.MaxTotalSizeMB
			}
		}
	}
	return maxProjects, maxTotalMB, true
}

// HTMLUsage 统计某用户已用额度（项目数 + 累计字节）
func HTMLUsage(userID uint) (count int, totalBytes int64) {
	var n int64
	_ = models.DB.Model(&softdel.UserHTML{}).Where("user_id = ?", userID).Count(&n).Error
	var sum int64
	_ = models.DB.Model(&softdel.UserHTML{}).Where("user_id = ?", userID).
		Select("COALESCE(SUM(total_size),0)").Scan(&sum).Error
	return int(n), sum
}

// HTMLQuotaError 额度不足时返回的错误（文案可直接展示给用户）
type HTMLQuotaError struct {
	Reason string
}

func (e *HTMLQuotaError) Error() string { return e.Reason }

// CheckHTMLQuota 校验上传是否还在额度内。addBytes 为新项目解压后的预计大小。
// 调用方捕获 *HTMLQuotaError 时按 403 返回并原样展示 Reason。
func CheckHTMLQuota(userID uint, addBytes int64) error {
	maxProjects, maxMB, enabled := EffectiveHTMLQuota(userID)
	if !enabled {
		return nil
	}
	count, used := HTMLUsage(userID)
	g := LoadHTMLQuotaSetting()
	tail := ""
	if g.RequireApply {
		tail = "；如需更多空间，请向站长提交申请"
	}
	if maxProjects > 0 && count >= maxProjects {
		return &HTMLQuotaError{Reason: fmt.Sprintf(
			"已达到项目数上限（%d 个，当前 %d 个）%s", maxProjects, count, tail)}
	}
	if maxMB > 0 {
		limit := int64(maxMB) * 1024 * 1024
		if used+addBytes > limit {
			return &HTMLQuotaError{Reason: fmt.Sprintf(
				"已达到空间上限（%d MB，已用 %.1f MB，本次需 %.1f MB）%s",
				maxMB, float64(used)/1048576, float64(addBytes)/1048576, tail)}
		}
	}
	return nil
}

// HTMLQuotaSnapshot 给前端展示的额度快照
func HTMLQuotaSnapshot(userID uint) map[string]interface{} {
	maxProjects, maxMB, enabled := EffectiveHTMLQuota(userID)
	count, used := HTMLUsage(userID)
	return map[string]interface{}{
		"enabled":           enabled,
		"max_projects":      maxProjects,
		"max_total_size_mb": maxMB,
		"used_projects":     count,
		"used_bytes":        used,
		"used_mb":           float64(used) / 1048576,
	}
}
