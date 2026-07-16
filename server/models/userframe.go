package models

import "time"

// UserFrame 用户自定义UI模板（用于美化小说渲染页面）
// 模板可调用平台格式API获取小说数据，支持沙盒内按钮等控件
type UserFrame struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	NovelID     *uint     `gorm:"index" json:"novel_id"`                 // 关联作品（可选，为空则为全局模板）
	Name        string    `gorm:"size:128;not null" json:"name"`         // 模板名称
	Description string    `gorm:"size:512" json:"description"`           // 描述
	FilePath    string    `gorm:"size:512;not null" json:"file_path"`    // 文件系统路径
	ThumbURL    string    `gorm:"size:512" json:"thumb_url"`            // 缩略图URL
	IsActive    bool      `gorm:"default:true" json:"is_active"`         // 是否启用
	IsPublic    bool      `gorm:"default:false" json:"is_public"`        // 是否公开分享
	Tags        string    `gorm:"type:text" json:"tags"`                // JSON 标签数组
	Version     int       `gorm:"default:1" json:"version"`              // 版本号
	// 模板专属字段
	HasControls bool   `gorm:"default:false" json:"has_controls"`        // 是否包含按钮/控件
	UsesNovelAPI bool  `gorm:"default:false" json:"uses_novel_api"`      // 是否调用平台小说API
	SandboxLevel string `gorm:"size:16;default:strict" json:"sandbox_level"` // strict(只读) / interactive(含控件)
	FrameType    string `gorm:"size:16;default:reader;index" json:"frame_type"` // reader(阅读模版) / author(作者展现模版)
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	User  *User  `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Novel *Novel `gorm:"foreignKey:NovelID" json:"novel,omitempty"`
}

func (UserFrame) TableName() string { return "user_frames" }

// UserHTML 用户自定义扩展HTML（ZIP上传，含HTML+CSS+JS+WASM等资源）
// 解压后作为独立沙盒应用运行
type UserHTML struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	NovelID     *uint     `gorm:"index" json:"novel_id"`                 // 关联作品（可选）
	Name        string    `gorm:"size:128;not null" json:"name"`         // 扩展名称
	Description string    `gorm:"size:512" json:"description"`           // 描述
	ExtractDir  string    `gorm:"size:512;not null" json:"extract_dir"`  // ZIP解压后的目录路径
	EntryFile   string    `gorm:"size:256;not null" json:"entry_file"`   // 入口HTML文件名（如 index.html）
	FileCount   int       `gorm:"default:0" json:"file_count"`           // 解压后文件数
	TotalSize   int64     `gorm:"default:0" json:"total_size"`           // 解压后总大小（字节）
	FilePath    string    `gorm:"size:512;not null;default:''" json:"file_path"` // 旧字段兼容
	ZipPath     string    `gorm:"size:512" json:"zip_path"`             // 原始ZIP文件路径
	Port        int       `gorm:"default:0" json:"port"`               // 分配的代理端口（0=未分配）
	ProjectName string    `gorm:"size:64;index" json:"project_name"`    // 用户命名的项目名（代理URL中使用，隐藏端口）
	IsActive    bool      `gorm:"default:true" json:"is_active"`         // 是否启用
	AllowWasm      bool   `gorm:"default:false" json:"allow_wasm"`          // 是否允许加载WASM
	IsPublic       bool   `gorm:"default:false" json:"is_public"`           // 是否在广场展示
	IsDownloadable bool   `gorm:"default:false" json:"is_downloadable"`    // 是否允许下载ZIP
	ThumbURL       string `gorm:"size:512" json:"thumb_url"`              // 缩略图URL
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	User  *User  `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Novel *Novel `gorm:"foreignKey:NovelID" json:"novel,omitempty"`
}

func (UserHTML) TableName() string { return "user_htmls" }