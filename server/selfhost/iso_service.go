package selfhost

// 路径隔离服务（GWh 三种代理类型的 NVS 版本）：
// pass-through / strip / isolated。由管理员显式声明服务名，未声明的路径不接管。

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
	"nvs-server/models"
	isoconfig "nvs-server/selfhost/isolation/config"
	isoengine "nvs-server/selfhost/isolation/engine"
	"nvs-server/selfhost/target"
)

type IsoService struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Name            string    `gorm:"size:64;not null;uniqueIndex" json:"name"`
	Target          string    `gorm:"size:512;not null" json:"target"`
	Mode            string    `gorm:"size:16;not null;default:strip" json:"mode"`
	Timeout         int       `gorm:"default:10" json:"timeout"`
	Enabled         bool      `gorm:"default:true;index" json:"enabled"`
	Description     string    `gorm:"size:512;default:''" json:"description"`
	CookieHat       string    `gorm:"size:32;default:''" json:"cookie_hat"`
	HTMLRewrite     bool      `gorm:"default:false" json:"html_rewrite"`
	KnownAbsPrefix  string    `gorm:"size:1024;default:''" json:"known_abs_prefixes"`
	HostMode        string    `gorm:"size:16;default:keep" json:"host_mode"`
	NoFullCompat    bool      `gorm:"default:false" json:"no_full_compat"`
	MaxRewriteBytes int64     `gorm:"default:0" json:"max_rewrite_bytes"`
	DropForeign     bool      `gorm:"default:false" json:"drop_foreign_cookies"`
	PassThroughFix  bool      `gorm:"default:false" json:"passthrough_location_prefix"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (IsoService) TableName() string { return "selfhost_iso_services" }

func MigrateIsoServices(db *gorm.DB) error { return db.AutoMigrate(&IsoService{}) }

var isoNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

func normalizeIsoService(in *IsoService) error {
	in.Name = strings.ToLower(strings.TrimSpace(strings.Trim(in.Name, "/")))
	if !isoNameRe.MatchString(in.Name) {
		return errors.New("服务名只能使用小写字母、数字、连字符和下划线")
	}
	if reservedProxyPrefixes[in.Name] {
		return errors.New("该服务名属于平台保留路径")
	}
	mode, ok := isoconfig.ParseMode(in.Mode)
	if !ok {
		return errors.New("模式只能是 pass-through / strip / isolated")
	}
	in.Mode = mode.Canonical()
	in.Target = strings.TrimSpace(in.Target)
	if _, err := target.ParseBackend(in.Target); err != nil {
		return errors.New("目标地址必须是完整 http/https URL（主机:端口）")
	}
	if in.Timeout < 1 || in.Timeout > 120 {
		in.Timeout = 10
	}
	if _, ok := isoconfig.ParseHostMode(in.HostMode); !ok {
		return errors.New("Host 模式只能是 keep / rewrite")
	}
	return nil
}

func ListIsoServices() ([]IsoService, error) {
	var list []IsoService
	err := models.DB.Order("name ASC").Find(&list).Error
	return list, err
}

func SaveIsoService(in IsoService) (*IsoService, error) {
	if err := normalizeIsoService(&in); err != nil {
		return nil, err
	}
	var old IsoService
	notFound := models.DB.Where("name = ?", in.Name).First(&old).Error != nil
	if !notFound {
		in.ID, in.CreatedAt = old.ID, old.CreatedAt
		if err := models.DB.Save(&in).Error; err != nil {
			return nil, err
		}
	} else if err := models.DB.Create(&in).Error; err != nil {
		return nil, err
	}
	ReloadIsoServices()
	return &in, nil
}

func ToggleIsoService(id uint, enabled bool) error {
	if err := models.DB.Model(&IsoService{}).Where("id = ?", id).Update("enabled", enabled).Error; err != nil {
		return err
	}
	ReloadIsoServices()
	return nil
}

func DeleteIsoService(id uint) error {
	if err := models.DB.Delete(&IsoService{}, id).Error; err != nil {
		return err
	}
	ReloadIsoServices()
	return nil
}

// 完整 GWh 隔离引擎：配置来自 selfhost_iso_services 表，经 config.Table 快照热加载。
// 三种模式 pass-through / strip / isolated 的全部语义（路径/Cookie/Location/响应体改写、
// 熔断/指标/审计/跳数守卫）都由移植过来的 isolation/engine 实现。
var isoEngine = isoengine.New(isoengine.Options{})

func isoEngineTable(list []IsoService) *isoconfig.Table {
	tbl := &isoconfig.Table{Version: 1, Source: "selfhost_iso_services", LoadedAt: time.Now()}
	for i := range list {
		s := &list[i]
		mode, ok := isoconfig.ParseMode(s.Mode)
		if !ok {
			continue
		}
		hostMode, _ := isoconfig.ParseHostMode(s.HostMode)
		var abs []string
		for _, p := range strings.Split(s.KnownAbsPrefix, ",") {
			if p = strings.TrimSpace(p); p != "" {
				abs = append(abs, p)
			}
		}
		maxBytes := s.MaxRewriteBytes
		if maxBytes <= 0 {
			maxBytes = isoconfig.DefaultMaxRewriteBytes // GWh 配置归一化默认 2MB
		}
		svc := &isoconfig.Service{
			Name: s.Name, Prefix: "/" + s.Name, Target: s.Target,
			Mode: mode, CookieHat: s.CookieHat, HTMLRewrite: s.HTMLRewrite,
			KnownAbsPrefix: abs, Timeout: s.Timeout, HostMode: hostMode,
			NoFullCompat: s.NoFullCompat, MaxRewriteBytes: maxBytes,
			DropForeign: s.DropForeign, PassThroughFix: s.PassThroughFix,
			Enabled: s.Enabled,
		}
		if parsed, err := target.ParseBackend(s.Target); err == nil {
			svc.Backend = parsed
			svc.Scheme, svc.Host, svc.Root = parsed.Scheme, parsed.Host, parsed.Root
		}
		tbl.Services = append(tbl.Services, svc)
	}
	return tbl
}

func ReloadIsoServices() {
	list, err := ListIsoServices()
	if err != nil {
		return
	}
	isoEngine.Reload(isoEngineTable(list), "db")
}

// IsoEngine 返回完整隔离引擎实例（供管理接口与代理入口共用）。
func IsoEngine() *isoengine.Engine { return isoEngine }
