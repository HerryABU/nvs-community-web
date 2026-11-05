package aihub

// ==================== 渠道账户凭据（加密入库） ====================
//
// 渠道监控里那套「渠道地址 + 系统访问令牌 nap_ + 用户 ID + 模型密钥 sk_」以前只能每次手输。
// 站长要求存下来，之后直接点「拉取」。但 nap_ 权限很大（能查账户、部分站点还能改配置），
// 所以：
//   · 只存**密文**（AES-256-GCM，utils.EncryptField），**不存明文、不回传前端**；
//   · 前端只看得到尾号（…a1b2）与是否已配置；
//   · **不用 JWT 包装**：JWT 是「签名」不是「加密」，payload 人人可解；而且它天然带过期，
//     存这种长期凭据只会逼人把有效期设成几年，反而更危险。要可逆就用对称加密。
//
// 指纹（sha256）只用于同渠道判重与「同一把令牌换了没」，不可逆。

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/utils"
)

// ErrNoChannelAccount 该渠道还没有保存过凭据
var ErrNoChannelAccount = errors.New("该渠道还没有保存凭据")

// AiChannelAccount 一条渠道（base_url）的账户凭据
type AiChannelAccount struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// BaseURL 归一化后的站点根（与 AiChannelKey 同口径，便于互相对照）
	BaseURL string `gorm:"size:255;not null;uniqueIndex" json:"base_url"`
	Label   string `gorm:"size:64;default:''" json:"label"`
	// UserID New API 的数字用户 ID（供 New-Api-User 头用）；不是机密，但一起存方便
	UserID string `gorm:"size:32;default:''" json:"user_id"`

	// AccessTokenEnc nap_ 系统访问令牌密文（权限大，重点保护）
	AccessTokenEnc string `gorm:"type:text" json:"-"`
	AccessTokenFP  string `gorm:"size:64;default:''" json:"-"`
	// AccessTokenTail 尾 4 位，仅用于前端显示「已保存 …a1b2」
	AccessTokenTail string `gorm:"size:8;default:''" json:"access_token_tail"`

	// ModelKeyEnc sk_ 模型密钥密文（探模型列表 / 查单 Key 用量用）
	ModelKeyEnc  string `gorm:"type:text" json:"-"`
	ModelKeyFP   string `gorm:"size:64;default:''" json:"-"`
	ModelKeyTail string `gorm:"size:8;default:''" json:"model_key_tail"`

	Enabled bool `gorm:"not null;default:true" json:"enabled"`

	// 最近一次探测快照：省得每次打开弹窗都打上游（也少暴露一次令牌）
	LastCheckedAt *time.Time `json:"last_checked_at"`
	LastOK        bool       `gorm:"not null;default:false" json:"last_ok"`
	LastModels    int        `gorm:"not null;default:0" json:"last_models"`
	LastBalance   float64    `gorm:"not null;default:0" json:"last_balance"`
	LastUsed      float64    `gorm:"not null;default:0" json:"last_used"`
	LastCurrency  string     `gorm:"size:8;default:''" json:"last_currency"`
	LastError     string     `gorm:"size:255;default:''" json:"last_error"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AiChannelAccount) TableName() string { return "ai_channel_accounts" }

// tail4 取明文尾 4 位（令牌太短就不给尾号，免得把整串露出来）
func tail4(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 8 {
		return ""
	}
	return s[len(s)-4:]
}

// SaveAiChannelAccount 保存/更新一条渠道凭据。
// 传空字符串表示「这一项不动」；传 "-" 表示清空该项。
func SaveAiChannelAccount(baseURL, label, userID, accessToken, modelKey string) (*AiChannelAccount, error) {
	base := normalizeAccountBase(baseURL)
	if base == "" {
		return nil, fmt.Errorf("渠道地址为空")
	}
	var acc AiChannelAccount
	err := models.DB.Where("base_url = ?", base).First(&acc).Error
	if err != nil {
		acc = AiChannelAccount{BaseURL: base, Enabled: true}
	}

	accessToken = strings.TrimSpace(accessToken)
	modelKey = strings.TrimSpace(modelKey)

	// 清空语义：显式传 "-"
	if accessToken == "-" {
		acc.AccessTokenEnc, acc.AccessTokenFP, acc.AccessTokenTail = "", "", ""
	} else if accessToken != "" {
		enc, e := utils.EncryptField(accessToken)
		if e != nil {
			return nil, fmt.Errorf("令牌加密失败：%w", e)
		}
		acc.AccessTokenEnc = enc
		acc.AccessTokenFP = fingerprintOf(accessToken)
		acc.AccessTokenTail = tail4(accessToken)
	}
	if modelKey == "-" {
		acc.ModelKeyEnc, acc.ModelKeyFP, acc.ModelKeyTail = "", "", ""
	} else if modelKey != "" {
		enc, e := utils.EncryptField(modelKey)
		if e != nil {
			return nil, fmt.Errorf("模型密钥加密失败：%w", e)
		}
		acc.ModelKeyEnc = enc
		acc.ModelKeyFP = fingerprintOf(modelKey)
		acc.ModelKeyTail = tail4(modelKey)
	}
	if userID = strings.TrimSpace(userID); userID != "" && userID != "-" {
		acc.UserID = userID
	} else if userID == "-" {
		acc.UserID = ""
	}
	if label = strings.TrimSpace(label); label != "" {
		acc.Label = label
	}
	acc.UpdatedAt = time.Now()

	if acc.ID == 0 {
		if err := models.DB.Create(&acc).Error; err != nil {
			return nil, err
		}
	} else {
		// 用 map 写：密文字段可能被显式清空（零值），Create/Updates 的结构体写法会跳过
		if err := models.DB.Model(&AiChannelAccount{}).Where("id = ?", acc.ID).Updates(map[string]interface{}{
			"label":             acc.Label,
			"user_id":           acc.UserID,
			"access_token_enc":  acc.AccessTokenEnc,
			"access_token_fp":   acc.AccessTokenFP,
			"access_token_tail": acc.AccessTokenTail,
			"model_key_enc":     acc.ModelKeyEnc,
			"model_key_fp":      acc.ModelKeyFP,
			"model_key_tail":    acc.ModelKeyTail,
			"updated_at":        acc.UpdatedAt,
		}).Error; err != nil {
			return nil, err
		}
	}
	return &acc, nil
}

func fingerprintOf(plain string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(plain)))
	return hex.EncodeToString(sum[:])
}

// GetAiChannelAccount 按渠道地址取凭据
func GetAiChannelAccount(baseURL string) (*AiChannelAccount, error) {
	base := normalizeAccountBase(baseURL)
	if base == "" {
		return nil, ErrNoChannelAccount
	}
	var acc AiChannelAccount
	if err := models.DB.Where("base_url = ?", base).First(&acc).Error; err != nil {
		return nil, ErrNoChannelAccount
	}
	return &acc, nil
}

// ListAiChannelAccounts 全部渠道凭据（后台列表用，密文不外泄）
func ListAiChannelAccounts() []AiChannelAccount {
	var list []AiChannelAccount
	models.DB.Order("updated_at DESC").Find(&list)
	return list
}

// DeleteAiChannelAccount 删除凭据
func DeleteAiChannelAccount(id uint) error {
	return models.DB.Delete(&AiChannelAccount{}, id).Error
}

// DecryptAccessToken 解出 nap_ 明文。调用方必须「用后即焚」，不得写日志、不得回传前端。
func (a *AiChannelAccount) DecryptAccessToken() (string, error) {
	if a == nil || a.AccessTokenEnc == "" {
		return "", fmt.Errorf("该渠道没有保存系统访问令牌")
	}
	return utils.DecryptField(a.AccessTokenEnc)
}

// DecryptModelKey 解出 sk_ 明文（同上：不落日志）
func (a *AiChannelAccount) DecryptModelKey() (string, error) {
	if a == nil || a.ModelKeyEnc == "" {
		return "", fmt.Errorf("该渠道没有保存模型密钥")
	}
	return utils.DecryptField(a.ModelKeyEnc)
}

// HasAccessToken / HasModelKey 前端据此显示「已保存 / 未保存」
func (a *AiChannelAccount) HasAccessToken() bool { return a != nil && a.AccessTokenEnc != "" }
func (a *AiChannelAccount) HasModelKey() bool    { return a != nil && a.ModelKeyEnc != "" }

// RecordAiChannelProbe 记录一次探测结果（异步/同步都行，失败不影响主流程）
func RecordAiChannelProbe(baseURL string, ok bool, modelCount int, balance, used float64, currency, errText string) {
	base := normalizeAccountBase(baseURL)
	if base == "" {
		return
	}
	now := time.Now()
	upd := map[string]interface{}{
		"last_checked_at": now,
		"last_ok":         ok,
		"last_models":     modelCount,
		"last_error":      clipRunes(errText, 200),
		"updated_at":      now,
	}
	if balance > 0 || used > 0 {
		upd["last_balance"] = balance
		upd["last_used"] = used
		upd["last_currency"] = currency
	}
	_ = models.DB.Model(&AiChannelAccount{}).Where("base_url = ?", base).Updates(upd).Error
}

// clipRunes 截断到 n 个字符（与 models.truncateRunes 区分：那是另一个文件里的同名函数）
func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// normalizeAccountBase 把「站长粘进来的地址」还原成站点根。
//
// 为什么这里重复一份、不复用 aiclient.NormalizeBaseURL：
// models 是被依赖方，反向 import 客户端包会把依赖方向搞乱。语义与它保持一致：
// 剥掉 /api/user/self、/v1/models、/models、/v1 这类「具体接口」后缀。
// 站长的真实填法（实测）经常是 https://api.hcnsec.cn/api/user/self 或 …/v1。
func normalizeAccountBase(u string) string {
	s := strings.ToLower(strings.TrimSpace(u))
	s = strings.TrimRight(s, "/")
	// 循环剥：/v1/models 需要剥两次才到底
	for i := 0; i < 4; i++ {
		before := s
		for _, suf := range []string{"/api/user/self", "/api/user", "/v1/models", "/models", "/v1"} {
			if strings.HasSuffix(s, suf) {
				s = strings.TrimRight(strings.TrimSuffix(s, suf), "/")
				break
			}
		}
		if s == before {
			break
		}
	}
	return s
}
