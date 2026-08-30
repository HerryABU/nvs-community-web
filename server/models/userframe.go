package models

import (
	"time"

	"gorm.io/gorm"
)

// UserFrame 用户自定义UI模板（用于美化小说渲染页面）
// 模板可调用平台格式API获取小说数据，支持沙盒内按钮等控件
type UserFrame struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	UserID      uint   `gorm:"not null;index" json:"user_id"`
	NovelID     *uint  `gorm:"index" json:"novel_id"`              // 关联作品（可选，为空则为全局模板）
	Name        string `gorm:"size:128;not null" json:"name"`      // 模板名称
	Description string `gorm:"size:512" json:"description"`        // 描述
	FilePath    string `gorm:"size:512;not null" json:"file_path"` // 文件系统路径
	ThumbURL    string `gorm:"size:512" json:"thumb_url"`          // 缩略图URL
	IsActive    bool   `gorm:"default:true" json:"is_active"`      // 是否启用
	IsPublic    bool   `gorm:"default:false" json:"is_public"`     // 是否公开分享
	Tags        string `gorm:"type:text" json:"tags"`              // JSON 标签数组
	Version     int    `gorm:"default:1" json:"version"`           // 版本号
	// 模板专属字段
	HasControls  bool   `gorm:"default:false" json:"has_controls"`              // 是否包含按钮/控件
	UsesNovelAPI bool   `gorm:"default:false" json:"uses_novel_api"`            // 是否调用平台小说API
	SandboxLevel string `gorm:"size:16;default:strict" json:"sandbox_level"`    // strict(只读) / interactive(含控件)
	FrameType    string `gorm:"size:16;default:reader;index" json:"frame_type"` // reader(阅读模版) / author(作者展现模版)
	// 审查与运行模式
	ReviewStatus string         `gorm:"size:16;default:approved;index" json:"review_status"` // approved(通过) / pending(待复核) / rejected(驳回)
	ReviewNote   string         `gorm:"size:512" json:"review_note"`                         // 管理员复核意见
	ScanWarnings string         `gorm:"type:text" json:"scan_warnings"`                      // 自动扫描告警（JSON 数组）
	RunMode      string         `gorm:"size:16;default:sandbox;index" json:"run_mode"`       // sandbox(沙盒) / inline(非沙盒直接注入，仅限本人页面)
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"` // 合规留存：软删除标记

	User  *User  `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Novel *Novel `gorm:"foreignKey:NovelID" json:"novel,omitempty"`
}

func (UserFrame) TableName() string { return "user_frames" }

// UserHTML 用户自定义扩展HTML（ZIP上传，含HTML+CSS+JS+WASM等资源）
// 解压后作为独立沙盒应用运行
type UserHTML struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	UserID         uint   `gorm:"not null;index" json:"user_id"`
	NovelID        *uint  `gorm:"index" json:"novel_id"`                         // 关联作品（可选）
	Name           string `gorm:"size:128;not null" json:"name"`                 // 扩展名称
	Description    string `gorm:"size:512" json:"description"`                   // 描述
	ExtractDir     string `gorm:"size:512;not null" json:"extract_dir"`          // ZIP解压后的目录路径
	EntryFile      string `gorm:"size:256;not null" json:"entry_file"`           // 入口HTML文件名（如 index.html）
	FileCount      int    `gorm:"default:0" json:"file_count"`                   // 解压后文件数
	TotalSize      int64  `gorm:"default:0" json:"total_size"`                   // 解压后总大小（字节）
	FilePath       string `gorm:"size:512;not null;default:''" json:"file_path"` // 旧字段兼容
	ZipPath        string `gorm:"size:512" json:"zip_path"`                      // 原始ZIP文件路径
	Port           int    `gorm:"default:0" json:"port"`                         // 分配的代理端口（0=未分配）
	ProjectName    string `gorm:"size:64;index" json:"project_name"`             // 用户命名的项目名（代理URL中使用，隐藏端口）
	IsActive       bool   `gorm:"default:true" json:"is_active"`                 // 是否启用
	AllowWasm      bool   `gorm:"default:false" json:"allow_wasm"`               // 是否允许加载WASM
	IsPublic       bool   `gorm:"default:false" json:"is_public"`                // 是否在广场展示
	IsDownloadable bool   `gorm:"default:false" json:"is_downloadable"`          // 是否允许下载ZIP
	ThumbURL       string `gorm:"size:512" json:"thumb_url"`                     // 缩略图URL
	// 审查与运行模式
	// review_status 语义：
	//   pending   预检发现中/低危告警，已解压但待管理员复核（复核通过才可公开）
	//   quarantine 预检命中高危，ZIP 原样封存、不解压，等管理员处置（放行=补解压，驳回=退回）
	//   approved  审核通过（可公开）
	//   rejected  驳回（不公开；quarantine 转 rejected 时可选择删除封存件）
	ReviewStatus string `gorm:"size:16;default:approved;index" json:"review_status"` // approved / pending / quarantine / rejected
	ReviewNote   string `gorm:"size:512" json:"review_note"`                         // 管理员复核意见
	ScanWarnings string `gorm:"type:text" json:"scan_warnings"`                      // 自动扫描告警（JSON 数组）
	RunMode      string `gorm:"size:16;default:sandbox;index" json:"run_mode"`       // sandbox / inline
	// 封存态（预检未通过 → 不解压，只留原始 ZIP）
	// ⚠️ is_extracted 列默认值必须是 false：GORM struct Create 会把 false 当零值忽略、
	// 套用列默认值。若默认为 true，封存件会被错误地标成「已解压」，放行时跳过补解压、
	// 详情页也会走错分支。存量（新增列前已解压的）扩展由 BackfillUserHTMLExtracted 回填。
	IsExtracted     bool           `gorm:"default:false" json:"is_extracted"`            // 是否已解压（false = 仅封存原始 ZIP）
	QuarantinePath  string         `gorm:"size:512;default:''" json:"quarantine_path"`   // 封存的原始 ZIP 路径
	ScanBlocked     bool           `gorm:"default:false" json:"scan_blocked"`            // 预检是否命中高危（阻断级）
	ScanBlockReason string         `gorm:"size:512;default:''" json:"scan_block_reason"` // 阻断原因（命中的文件+规则）
	ReviewedBy      uint           `gorm:"default:0" json:"reviewed_by"`                 // 复核管理员 ID
	ReviewedAt      *time.Time     `json:"reviewed_at"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"` // 合规留存：软删除标记

	User  *User  `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Novel *Novel `gorm:"foreignKey:NovelID" json:"novel,omitempty"`
}

func (UserHTML) TableName() string { return "user_htmls" }

// BackfillUserHTMLExtracted 存量回填（幂等）：is_extracted 是后加的列，默认 false，
// 需把「新增列之前就已经解压落盘的扩展」标记为已解压，否则它们会被误判成封存件，
// 管理员详情页读不到入口文件、放行时还会触发一次多余的重解压。
// 判定依据：extract_dir 非空即视为已解压（封存件的 extract_dir 恒为空串）。
func BackfillUserHTMLExtracted() {
	_ = DB.Model(&UserHTML{}).
		Where("is_extracted = ? AND extract_dir IS NOT NULL AND extract_dir <> ?", false, "").
		Update("is_extracted", true).Error
}
