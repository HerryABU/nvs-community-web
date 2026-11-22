package selfhost

// 四层 TCP 端口映射：监听一个端口并转发到另一个 主机:端口。
// 仅管理员可配置；与路径级代理互不干扰。

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/selfhost/portmap"
	"strconv"
)

type PortMap struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ListenPort  int       `gorm:"not null;uniqueIndex" json:"listen_port"`
	TargetHost  string    `gorm:"size:255;not null;default:127.0.0.1" json:"target_host"`
	TargetPort  int       `gorm:"not null" json:"target_port"`
	Enabled     bool      `gorm:"default:true;index" json:"enabled"`
	Description string    `gorm:"size:512;default:''" json:"description"`
	LastError   string    `gorm:"size:512;default:''" json:"last_error"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (PortMap) TableName() string { return "selfhost_port_maps" }

func MigratePortMaps(db *gorm.DB) error { return db.AutoMigrate(&PortMap{}) }

func validatePortMap(in *PortMap) error {
	if in.ListenPort < 1 || in.ListenPort > 65535 || in.TargetPort < 1 || in.TargetPort > 65535 {
		return errors.New("端口必须在 1-65535 之间")
	}
	in.TargetHost = strings.TrimSpace(in.TargetHost)
	if in.TargetHost == "" {
		return errors.New("目标主机不能为空")
	}
	selfPort, _ := strconv.Atoi(config.ServerPort)
	if in.ListenPort == selfPort && selfPort > 0 {
		return errors.New("监听端口不能与平台自身端口冲突")
	}
	// 127.0.0.1 同端口映射必然绕回自己，直接拒绝。
	local := in.TargetHost == "127.0.0.1" || strings.EqualFold(in.TargetHost, "localhost") || in.TargetHost == "::1"
	if local && in.ListenPort == in.TargetPort {
		return errors.New("监听端口与目标端口不能相同（会形成自环）")
	}
	in.Description = strings.TrimSpace(in.Description)
	return nil
}

func ListPortMaps() ([]PortMap, error) {
	var list []PortMap
	err := models.DB.Order("listen_port ASC").Find(&list).Error
	return list, err
}

func SavePortMap(in PortMap) (*PortMap, error) {
	if err := validatePortMap(&in); err != nil {
		return nil, err
	}
	var old PortMap
	notFound := models.DB.Where("listen_port = ?", in.ListenPort).First(&old).Error != nil
	if !notFound {
		in.ID = old.ID
		in.CreatedAt = old.CreatedAt
		in.LastError = ""
		if err := models.DB.Save(&in).Error; err != nil {
			return nil, err
		}
	} else if err := models.DB.Create(&in).Error; err != nil {
		return nil, err
	}
	ReloadPortMaps()
	return &in, nil
}

func TogglePortMap(id uint, enabled bool) error {
	if err := models.DB.Model(&PortMap{}).Where("id = ?", id).Updates(map[string]interface{}{"enabled": enabled, "last_error": ""}).Error; err != nil {
		return err
	}
	ReloadPortMaps()
	return nil
}

func DeletePortMap(id uint) error {
	if err := models.DB.Delete(&PortMap{}, id).Error; err != nil {
		return err
	}
	ReloadPortMaps()
	return nil
}

var portMapManager = portmap.New()

func StartPortMaps() { ReloadPortMaps() }

func ReloadPortMaps() {
	list, err := ListPortMaps()
	if err != nil {
		return
	}
	endpoints := make([]portmap.Endpoint, 0, len(list))
	for _, p := range list {
		if !p.Enabled {
			continue
		}
		endpoints = append(endpoints, portmap.Endpoint{ID: p.ID, ListenPort: p.ListenPort, Target: p.TargetHost + ":" + strconv.Itoa(p.TargetPort)})
	}
	portMapManager.Reload(endpoints, func(id uint, err error) {
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		models.DB.Model(&PortMap{}).Where("id = ?", id).Update("last_error", msg)
	})
}

func PortMapRunning(id uint) bool { return portMapManager.Running(id) }
