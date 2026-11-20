package models

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// NovelIndex 小说章节清单（index.json）的数据库副本。
//
// 文件系统里那份 index.json 是给 Nginx 静态直出的；本表是同一份 JSON 的库内镜像，
// 用途：
//   - 纯数据库存储模式下不再依赖文件，读者端可以直接从 API 取到清单；
//   - 文件与库不一致时可比对 Hash 快速发现；
//   - 迁移/备份时不必扫描小说目录。
//
// Data 存的是**原样 JSON 字节**，避免文件与库各自 marshal 出不同格式。
type NovelIndex struct {
	NovelID   uint      `gorm:"primaryKey" json:"novel_id"`
	Data      string    `gorm:"type:text;not null" json:"data"`
	Hash      string    `gorm:"size:64;index" json:"hash"`
	UpdatedAt time.Time `json:"-"`
}

func (NovelIndex) TableName() string { return "novel_indexes" }

// SaveNovelIndex 写入/覆盖库内副本（内容寻址 Hash 便于一致性校验）
func SaveNovelIndex(novelID uint, data []byte) error {
	if DB == nil {
		return nil
	}
	sum := sha256.Sum256(data)
	row := NovelIndex{NovelID: novelID, Data: string(data), Hash: hex.EncodeToString(sum[:])}
	return DB.Save(&row).Error
}

// GetNovelIndex 读取库内副本；ok=false 表示尚未生成（老数据可现场重建）
func GetNovelIndex(novelID uint) ([]byte, bool) {
	if DB == nil {
		return nil, false
	}
	var row NovelIndex
	if err := DB.Where("novel_id = ?", novelID).First(&row).Error; err != nil {
		return nil, false
	}
	return []byte(row.Data), true
}

// DeleteNovelIndex 删除整本小说时同步清理
func DeleteNovelIndex(novelID uint) {
	if DB == nil {
		return
	}
	DB.Where("novel_id = ?", novelID).Delete(&NovelIndex{})
}
