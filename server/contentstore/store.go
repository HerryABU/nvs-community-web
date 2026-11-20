// Package contentstore 把「正文存文件还是存库」收成一个边界。
//
// 两种模式（存在 platform_configs.storage_mode）：
//   - hybrid（默认）：文件为主，DB 镜像一份，读文件时若文件丢了仍能兜底；
//   - database（纯数据库）：只写 DB，不落文件，适合高性能机器/无共享盘部署。
//
// 迁移时由 dbadmin 按用户分批回填，迁移期间 maintenance 标志会挡住所有写入。
package contentstore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"nvs-server/models"
)

const (
	ModeHybrid   = "hybrid"
	ModeDatabase = "database"
	configKey    = "storage_mode"

	RefChapter = "chapter"
	RefBlog    = "blog"
)

// Mode 当前存储模式；缺省与非法值一律按混合模式处理（保持老部署行为不变）
func Mode() string {
	m := strings.ToLower(strings.TrimSpace(models.GetPlatformConfig(configKey)))
	if m == ModeDatabase {
		return ModeDatabase
	}
	return ModeHybrid
}

// IsDatabase 纯数据库模式
func IsDatabase() bool { return Mode() == ModeDatabase }

func hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// Put 写正文。混合模式文件+库都写；纯数据库模式只写库。
func Put(refType string, refID uint, path, content string) error {
	if models.DB == nil {
		return errors.New("数据库不可用")
	}
	if !IsDatabase() && path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	row := models.ContentBlob{RefType: refType, RefID: refID, Content: content, ContentHash: hash(content)}
	return models.DB.Save(&row).Error
}

// Get 读正文。纯数据库模式只读库；混合模式优先文件、文件缺失再回退库。
func Get(refType string, refID uint, path string) (string, error) {
	if !IsDatabase() && path != "" {
		if b, err := os.ReadFile(path); err == nil {
			return string(b), nil
		}
	}
	if models.DB == nil {
		return "", errors.New("数据库不可用")
	}
	var row models.ContentBlob
	if err := models.DB.Where("ref_type = ? AND ref_id = ?", refType, refID).First(&row).Error; err != nil {
		return "", err
	}
	return row.Content, nil
}

// LoadDB 只读库内副本（迁移/校验用，不受当前模式影响）
func LoadDB(refType string, refID uint) (string, bool) {
	if models.DB == nil {
		return "", false
	}
	var row models.ContentBlob
	if err := models.DB.Where("ref_type = ? AND ref_id = ?", refType, refID).First(&row).Error; err != nil {
		return "", false
	}
	return row.Content, true
}

// Remove 删除库内副本（文件由业务层自行处理，软删除历史不动文件）
func Remove(refType string, refID uint) {
	if models.DB == nil {
		return
	}
	models.DB.Where("ref_type = ? AND ref_id = ?", refType, refID).Delete(&models.ContentBlob{})
}
