package models

import (
	"time"

	"gorm.io/gorm"
)

type Novel struct {
	ID              uint    `gorm:"primaryKey" json:"id"`
	AuthorID        uint    `gorm:"not null;index;index:idx_novels_author_status,priority:1" json:"author_id"`
	Title           string  `gorm:"size:256;not null" json:"title"`
	Category        string  `gorm:"size:64;default:其他;index" json:"category"`
	Tags            string  `gorm:"type:json" json:"tags"` // JSON 数组，代码中保证默认 "[]"（MySQL JSON 不支持 DEFAULT）
	Summary         string  `gorm:"type:text" json:"summary"`
	CoverURL        string  `gorm:"size:512;default:''" json:"cover_url"`
	PricePerChapter float64 `gorm:"type:decimal(10,2);default:0.00" json:"price_per_chapter"`
	Status          string  `gorm:"size:32;default:draft;index:idx_novels_author_status,priority:2" json:"status"`
	SourceType      string  `gorm:"size:32;default:original" json:"source_type"`  // original / reprint
	CreationMethod  string  `gorm:"size:32;default:human" json:"creation_method"` // human / ai / human_ai_assisted
	// Mode 叙事形态：prose=散文体（默认）/ script=话本气泡体（对话为主，按「角色：台词」解析渲染）
	Mode          string         `gorm:"size:12;not null;default:prose;index" json:"mode"`
	WallEnabled   bool           `gorm:"not null;default:false" json:"wall_enabled"` // 作者隔离开关：true=外人需登录才能查看本作品
	TotalWords    int            `gorm:"default:0" json:"total_words"`
	TotalChapters int            `gorm:"default:0" json:"total_chapters"`
	ViewCount     int            `gorm:"default:0" json:"view_count"`
	BanReason     string         `gorm:"type:text" json:"ban_reason"`                    // 封禁原因（可能很长）
	DeletedBy     string         `gorm:"size:32;default:''" json:"deleted_by,omitempty"` // user/admin
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	Author        *User           `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
	Categories    []NovelCategory `gorm:"foreignKey:NovelID" json:"-"`
	CategoryNames []string        `gorm:"-" json:"categories,omitempty"`
}

// 章节发布状态（多人协作：draft → pending → published / rejected）
const (
	ChapterStatusDraft     = "draft"
	ChapterStatusPending   = "pending"
	ChapterStatusPublished = "published"
	ChapterStatusRejected  = "rejected"
)

// NovelCategory 作品多分类中间表
type NovelCategory struct {
	ID       uint   `gorm:"primaryKey" json:"-"`
	NovelID  uint   `gorm:"not null;uniqueIndex:uk_novel_category;index:idx_novel_category_novel" json:"novel_id"`
	Category string `gorm:"size:64;not null;uniqueIndex:uk_novel_category;index:idx_novel_category_cat" json:"category"`
}

func (NovelCategory) TableName() string {
	return "novel_categories"
}

func (Novel) TableName() string {
	return "novels"
}

// CreateNovel 创建作品。Tags 为 JSON 字段（MySQL 不允许 DEFAULT），此处兜底为合法空数组
func CreateNovel(novel *Novel) error {
	if novel.Tags == "" {
		novel.Tags = "[]"
	}
	return DB.Create(novel).Error
}

func GetNovelByID(id uint) (*Novel, error) {
	var novel Novel
	err := DB.Preload("Author").First(&novel, id).Error
	if err != nil {
		return nil, err
	}
	novel.CategoryNames = GetNovelCategoryNames(novel.ID)
	return &novel, nil
}

func GetNovels(category, search string, page, pageSize int) ([]Novel, int64, error) {
	var novels []Novel
	var total int64

	query := DB.Model(&Novel{}).Where("status = ?", "published")

	if category != "" {
		// 支持多分类：通过子查询匹配 category 字段或 novel_categories 表
		query = query.Where(
			"category = ? OR id IN (SELECT novel_id FROM novel_categories WHERE category = ?)",
			category, category,
		)
	}

	if search != "" {
		like := "%" + search + "%"
		query = query.Joins("LEFT JOIN users ON users.id = novels.author_id").
			Where("(novels.title LIKE ? OR novels.summary LIKE ? OR novels.tags LIKE ? OR users.nickname LIKE ? OR users.username LIKE ?)", like, like, like, like, like)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Preload("Author").Order("novels.updated_at DESC").Offset(offset).Limit(pageSize).Find(&novels).Error; err != nil {
		return nil, 0, err
	}

	// 填充每个作品的分类列表
	for i := range novels {
		novels[i].CategoryNames = GetNovelCategoryNames(novels[i].ID)
	}

	return novels, total, nil
}

// GetNovelsSorted 支持排序的获取作品列表
func GetNovelsSorted(category, search, sortBy string, page, pageSize int) ([]Novel, int64, error) {
	var novels []Novel
	var total int64

	query := DB.Model(&Novel{}).Where("status = ?", "published")

	if category != "" {
		query = query.Where(
			"category = ? OR id IN (SELECT novel_id FROM novel_categories WHERE category = ?)",
			category, category,
		)
	}

	if search != "" {
		like := "%" + search + "%"
		query = query.Joins("LEFT JOIN users ON users.id = novels.author_id").
			Where("(novels.title LIKE ? OR novels.summary LIKE ? OR novels.tags LIKE ? OR users.nickname LIKE ? OR users.username LIKE ?)", like, like, like, like, like)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderClause := "novels.updated_at DESC"
	switch sortBy {
	case "created_at":
		orderClause = "novels.created_at DESC"
	case "updated_at":
		orderClause = "novels.updated_at DESC"
	case "featured":
		orderClause = "novels.total_chapters DESC, novels.updated_at DESC"
	}

	offset := (page - 1) * pageSize
	if err := query.Preload("Author").Order(orderClause).Offset(offset).Limit(pageSize).Find(&novels).Error; err != nil {
		return nil, 0, err
	}

	for i := range novels {
		novels[i].CategoryNames = GetNovelCategoryNames(novels[i].ID)
	}

	return novels, total, nil
}

// GetNovelCategoryNames 获取作品的分类名列表
func GetNovelCategoryNames(novelID uint) []string {
	var cats []NovelCategory
	DB.Where("novel_id = ?", novelID).Find(&cats)
	var result []string
	for _, c := range cats {
		result = append(result, c.Category)
	}
	// 不强制加"其他"——如果没有分类就返回空，让前端自行处理
	return result
}

func GetNovelsByAuthor(authorID uint, page, pageSize int) ([]Novel, int64, error) {
	var novels []Novel
	var total int64

	query := DB.Model(&Novel{}).Where("author_id = ?", authorID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("updated_at DESC").Offset(offset).Limit(pageSize).Find(&novels).Error; err != nil {
		return nil, 0, err
	}

	// 填充每个作品的分类列表
	for i := range novels {
		novels[i].CategoryNames = GetNovelCategoryNames(novels[i].ID)
	}

	return novels, total, nil
}

func UpdateNovel(novel *Novel) error {
	return DB.Save(novel).Error
}

func DeleteNovel(id uint) error {
	return DB.Delete(&Novel{}, id).Error
}

// Chapter 章节模型
type Chapter struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	NovelID          uint           `gorm:"not null;uniqueIndex:uk_novel_chapter;index" json:"novel_id"`
	ChapterNumber    int            `gorm:"not null;uniqueIndex:uk_novel_chapter" json:"chapter_number"`
	Title            string         `gorm:"size:256;not null" json:"title"`
	Content          string         `gorm:"-" json:"content,omitempty"`
	ContentPath      string         `gorm:"size:512;not null" json:"content_path"`
	ContentHash      string         `gorm:"size:64;default:''" json:"content_hash"`
	ContentSignature string         `gorm:"size:64;default:''" json:"content_signature"` // HMAC-SHA256 作者签名
	WordCount        int            `gorm:"default:0" json:"word_count"`
	Status           string         `gorm:"size:32;default:published;index" json:"status"`  // draft/pending/published/rejected
	// Mode 本章叙事形态：空=继承作品 / prose 散文体 / script 话本气泡体 / mixed 散文与话本混排
	Mode string `gorm:"size:8;default:''" json:"mode"`
	AuthorID         uint           `gorm:"not null;default:0;index" json:"author_id"`      // 本章作者（多人协作署名）
	ReviewNote       string         `gorm:"size:256;default:''" json:"review_note"`         // 驳回/审核意见
	BanReason        string         `gorm:"type:text" json:"ban_reason"`                    // 176号令封禁原因（封禁时写入）
	DeletedBy        string         `gorm:"size:32;default:''" json:"deleted_by,omitempty"` // user=用户自删 / admin=管理员封禁删除 / 空=未删除或旧数据
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"` // 合规留存：软删除标记

	AuthorName string `gorm:"-" json:"author_name,omitempty"` // 本章作者展示名（运行时填充）
	// 版本控制（server/chaptervcs）：当前分支上的最新版本号，供前端下次保存时带 base_version
	Version int `gorm:"-" json:"version,omitempty"`
	Branch  string `gorm:"-" json:"branch,omitempty"`
}

func (Chapter) TableName() string {
	return "chapters"
}

func CreateChapter(chapter *Chapter) error {
	return DB.Create(chapter).Error
}

func GetChapterByID(id uint) (*Chapter, error) {
	var chapter Chapter
	err := DB.First(&chapter, id).Error
	if err != nil {
		return nil, err
	}
	return &chapter, nil
}

func GetChapterByNovelAndNumber(novelID uint, chapterNumber int) (*Chapter, error) {
	var chapter Chapter
	err := DB.Where("novel_id = ? AND chapter_number = ?", novelID, chapterNumber).First(&chapter).Error
	if err != nil {
		return nil, err
	}
	return &chapter, nil
}

func GetChaptersByNovel(novelID uint) ([]Chapter, error) {
	var chapters []Chapter
	err := DB.Where("novel_id = ?", novelID).Order("chapter_number ASC").Find(&chapters).Error
	if err != nil {
		return nil, err
	}
	return chapters, nil
}

// GetMaxChapterNumber 返回某作品已有的最大章节序号（含软删除章节）。
// 关键：必须包含软删除记录，否则软删除某章后新建章节会复用其编号，造成「串号」。
func GetMaxChapterNumber(novelID uint) (int, error) {
	var maxNum int
	err := DB.Unscoped().Model(&Chapter{}).Where("novel_id = ?", novelID).
		Select("COALESCE(MAX(chapter_number), 0)").Scan(&maxNum).Error
	return maxNum, err
}

func UpdateChapter(chapter *Chapter) error {
	return DB.Save(chapter).Error
}

func DeleteChapter(id uint) error {
	return DB.Delete(&Chapter{}, id).Error
}

func GetChapterCountByNovel(novelID uint) (int64, error) {
	var count int64
	err := DB.Model(&Chapter{}).Where("novel_id = ?", novelID).Count(&count).Error
	return count, err
}

// UpdateNovelStats 更新作品的章节数和总字数统计
func UpdateNovelStats(novelID uint) error {
	count, err := GetChapterCountByNovel(novelID)
	if err != nil {
		return err
	}
	totalWords, err := GetTotalWordsByNovel(novelID)
	if err != nil {
		return err
	}
	return DB.Model(&Novel{}).Where("id = ?", novelID).Updates(map[string]interface{}{
		"total_chapters": count,
		"total_words":    totalWords,
	}).Error
}

func GetTotalWordsByNovel(novelID uint) (int, error) {
	var total int
	err := DB.Model(&Chapter{}).Where("novel_id = ?", novelID).
		Select("COALESCE(SUM(word_count), 0)").Scan(&total).Error
	return total, err
}

// RecomputeAllNovelStats 批量重算所有作品的章节数与总字数统计（从 chapters 表聚合写回 novels）。
// 用于修复因历史导入 / 迁移 / 早期版本未回填，导致 novels.total_chapters / total_words
// 与真实章节不一致的脏数据。含软删除作品一并重算，保证统计口径一致。
// 返回：处理作品数、实际发生变更数。
func RecomputeAllNovelStats() (processed int, changed int, err error) {
	// 1) 聚合：每个作品的章节数 + 总字数（chapters 默认已过滤软删除行）
	type agg struct {
		NovelID   uint `gorm:"column:novel_id"`
		ChapCount int  `gorm:"column:chap_count"`
		WordSum   int  `gorm:"column:word_sum"`
	}
	var aggs []agg
	if err = DB.Model(&Chapter{}).
		Select("novel_id, COUNT(*) as chap_count, COALESCE(SUM(word_count), 0) as word_sum").
		Group("novel_id").Scan(&aggs).Error; err != nil {
		return 0, 0, err
	}
	aggMap := make(map[uint]agg, len(aggs))
	for _, a := range aggs {
		aggMap[a.NovelID] = a
	}

	// 2) 遍历全部作品（含软删除），写回统计；用 map Updates 精确写列，规避 GORM 零值坑
	var novels []Novel
	if err = DB.Unscoped().Find(&novels).Error; err != nil {
		return 0, 0, err
	}
	for i := range novels {
		n := &novels[i]
		tc, tw := 0, 0
		if a, ok := aggMap[n.ID]; ok {
			tc, tw = a.ChapCount, a.WordSum
		}
		if n.TotalChapters != tc || n.TotalWords != tw {
			if err = DB.Model(&Novel{}).Where("id = ?", n.ID).
				Updates(map[string]interface{}{
					"total_chapters": tc,
					"total_words":    tw,
				}).Error; err != nil {
				return processed, changed, err
			}
			changed++
		}
		processed++
	}
	return processed, changed, nil
}
