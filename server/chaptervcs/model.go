package chaptervcs

// ==================== 章节版本控制（git 语义的轻量实现） ====================
// 为什么不用真实 git：章节正文现存在「主笔目录/{章节号}.html」，引入 go-git 会与既有文件存储双写、
// 并带来较重的二进制依赖（Windows 部署/运维成本）。这里自建一条 git 语义的版本链：
//   commit  = 一次章节保存（父版本 + 作者 + 提交说明 + 内容快照 + 哈希）
//   branch  = 一条独立的版本链（默认 main），协作者可各开分支写作避免互相覆盖
//   head    = 某分支上版本号最大的提交
//   merge   = 三方合并（base/ours/theirs），无冲突自动合，冲突保留并交给作者确认
//   revert  = 以历史版本内容再提交一次（不删历史，可追溯）
//
// 快照落盘在作品目录 .versions/{章节号}/{分支}/{版本号}.md，DB 只存元数据 + 哈希，
// 正文不进数据库，避免历史膨胀拖垮 SQLite。

import (
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	// DefaultBranch 默认分支（正文对外可见的那条链）
	DefaultBranch = "main"
	// MaxBranchLen 分支名长度上限
	MaxBranchLen = 32
	// MaxMessageLen 提交说明长度上限
	MaxMessageLen = 200
	// MaxSnapshotsPerChapter 单章节快照上限（超出时最老的快照文件被清理，元数据保留）
	MaxSnapshotsPerChapter = 200
)

// ChapterVersion 一次章节提交（append-only，删除只清快照不删元数据）
type ChapterVersion struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	ChapterID  uint   `gorm:"not null;index:idx_ch_ver" json:"chapter_id"`
	NovelID    uint   `gorm:"not null;index" json:"novel_id"`
	ChapterNum int    `gorm:"not null;index:idx_ch_ver" json:"chapter_num"`
	Branch     string `gorm:"size:32;not null;default:main;index:idx_ch_ver" json:"branch"`
	Version    int    `gorm:"not null;index:idx_ch_ver" json:"version"`

	// commit 图：ParentVer 为主父（0 = 首次提交），MergeFrom/MergeFromVer 记录第二父（合并提交）
	ParentVer    int    `gorm:"not null;default:0" json:"parent_ver"`
	MergeFrom    string `gorm:"size:32;default:''" json:"merge_from,omitempty"`
	MergeFromVer int    `gorm:"not null;default:0" json:"merge_from_ver,omitempty"`

	Title       string `gorm:"size:256" json:"title"`
	ContentPath string `gorm:"size:512" json:"-"` // 快照路径不外泄
	ContentHash string `gorm:"size:64;not null" json:"content_hash"`
	WordCount   int    `gorm:"not null;default:0" json:"word_count"`
	Message     string `gorm:"size:256" json:"message"`
	AuthorID    uint   `gorm:"not null;index" json:"author_id"`

	// Force 标记「冲突后以我的为准」，供审计回溯
	Force bool `gorm:"not null;default:false" json:"force"`
	// Deleted 作者删除了该历史快照（元数据保留，正文已清理）
	Deleted bool `gorm:"not null;default:false" json:"deleted"`

	CreatedAt time.Time `json:"created_at"`

	// 运行时字段（不入库）
	AuthorName string `gorm:"-" json:"author_name,omitempty"`
	IsHead     bool   `gorm:"-" json:"is_head,omitempty"`
}

func (ChapterVersion) TableName() string { return "chapter_versions" }

// ChapterBranch 分支指针（显式记录 head，避免每次 max(version) 扫描）
type ChapterBranch struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ChapterID  uint      `gorm:"not null;uniqueIndex:idx_branch" json:"chapter_id"`
	Name       string    `gorm:"size:32;not null;uniqueIndex:idx_branch" json:"name"`
	HeadVer    int       `gorm:"not null;default:0" json:"head_ver"`
	FromVer    int       `gorm:"not null;default:0" json:"from_ver"`    // 开分支时基于哪个版本
	FromBranch string    `gorm:"size:32;default:''" json:"from_branch"` // 开分支时的源分支
	AuthorID   uint      `gorm:"not null" json:"author_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (ChapterBranch) TableName() string { return "chapter_branches" }

var branchRe = regexp.MustCompile(`^[A-Za-z0-9_\-\.]{1,32}$`)

// NormalizeBranch 分支名归一化（非法字符替换为 -，空值回落 main）
func NormalizeBranch(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return DefaultBranch
	}
	if !branchRe.MatchString(name) {
		var b strings.Builder
		for _, r := range name {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-', r == '.':
				b.WriteRune(r)
			default:
				b.WriteRune('-')
			}
		}
		name = b.String()
	}
	if len(name) > MaxBranchLen {
		name = name[:MaxBranchLen]
	}
	if name == "" {
		return DefaultBranch
	}
	return name
}

// Migrate 建表（独立功能包，由 main 在主导航迁移后调用）
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&ChapterVersion{}, &ChapterBranch{}); err != nil {
		return err
	}
	return nil
}
