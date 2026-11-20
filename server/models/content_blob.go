package models

import "time"

// ContentBlob 正文的数据库副本。
//
// ref_type 取 "chapter" / "blog"，ref_id 对应各自主键；
// 混合模式下文件仍是主存储、这里是镜像，纯数据库模式下这里就是唯一存储。
// 有了这层，切换存储模式只需改读取路径，不需要改小说/博客的业务模型。
type ContentBlob struct {
	RefType     string    `gorm:"size:32;primaryKey" json:"ref_type"`
	RefID       uint      `gorm:"primaryKey" json:"ref_id"`
	Content     string    `gorm:"type:text;not null" json:"-"`
	ContentHash string    `gorm:"size:64" json:"content_hash"`
	UpdatedAt   time.Time `json:"-"`
}

func (ContentBlob) TableName() string { return "content_blobs" }
