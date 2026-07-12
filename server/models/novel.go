package models

import (
	"time"
)

type Novel struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	AuthorID        uint      `gorm:"not null;index" json:"author_id"`
	Title           string    `gorm:"size:256;not null" json:"title"`
	Category        string    `gorm:"size:64;default:其他;index" json:"category"`
	Tags            string    `gorm:"type:json" json:"tags"`
	Summary         string    `gorm:"type:text" json:"summary"`
	CoverURL        string    `gorm:"size:512;default:''" json:"cover_url"`
	PricePerChapter float64   `gorm:"type:decimal(10,2);default:0.00" json:"price_per_chapter"`
	Status          string    `gorm:"size:16;default:draft;index" json:"status"`
	SourceType      string    `gorm:"size:16;default:original" json:"source_type"`        // original / reprint
	CreationMethod  string    `gorm:"size:16;default:human" json:"creation_method"`       // human / ai / human_ai_assisted
	TotalWords      int       `gorm:"default:0" json:"total_words"`
	TotalChapters   int       `gorm:"default:0" json:"total_chapters"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	// 关联
	Author       *User           `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
	Categories   []NovelCategory `gorm:"foreignKey:NovelID" json:"-"`
	CategoryNames []string        `gorm:"-" json:"categories,omitempty"`
}

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

func CreateNovel(novel *Novel) error {
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
			Where("(novels.title LIKE ? OR novels.summary LIKE ? OR novels.tags LIKE ? OR users.nickname LIKE ?)", like, like, like, like)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Preload("Author").Order("updated_at DESC").Offset(offset).Limit(pageSize).Find(&novels).Error; err != nil {
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
			Where("(novels.title LIKE ? OR novels.summary LIKE ? OR novels.tags LIKE ? OR users.nickname LIKE ?)", like, like, like, like)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderClause := "updated_at DESC"
	switch sortBy {
	case "created_at":
		orderClause = "created_at DESC"
	case "updated_at":
		orderClause = "updated_at DESC"
	case "featured":
		orderClause = "total_chapters DESC, updated_at DESC"
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
	ID            uint      `gorm:"primaryKey" json:"id"`
	NovelID       uint      `gorm:"not null;uniqueIndex:uk_novel_chapter;index" json:"novel_id"`
	ChapterNumber int       `gorm:"not null;uniqueIndex:uk_novel_chapter" json:"chapter_number"`
	Title         string    `gorm:"size:256;not null" json:"title"`
	Content       string    `gorm:"-" json:"content,omitempty"`
	ContentPath   string    `gorm:"size:512;not null" json:"content_path"`
	ContentHash      string `gorm:"size:64;default:''" json:"content_hash"`
	ContentSignature string `gorm:"size:64;default:''" json:"content_signature"` // HMAC-SHA256 作者签名
	WordCount        int    `gorm:"default:0" json:"word_count"`
	Status        string    `gorm:"size:16;default:draft" json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (Chapter) TableName() string {
	return "chapters"
}

func CreateChapter(chapter *Chapter) error {
	return DB.Create(chapter).Error
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

func GetMaxChapterNumber(novelID uint) (int, error) {
	var maxNum int
	err := DB.Model(&Chapter{}).Where("novel_id = ?", novelID).
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
