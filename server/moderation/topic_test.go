package moderation

import (
	"fmt"
	"testing"
	"time"

	"nvs-server/models"
	sentimentstore "nvs-server/sentiment/store"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// 独立命名内存库：`file::memory:?cache=shared` 是同名共享库，
// 同包多测试会互相污染（本项目踩过，见 MEMORY）。
func setupTopicDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:moderation_topic_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	if err := db.AutoMigrate(&models.Chapter{}, &models.Novel{}); err != nil {
		t.Fatalf("迁移业务表失败: %v", err)
	}
	if err := sentimentstore.Migrate(db); err != nil {
		t.Fatalf("迁移舆情表失败: %v", err)
	}
	models.DB = db
	sentimentstore.SetDB(db)
	return db
}

// AI 标注落库：标签正确归类、置信度与复核状态按门槛分流。
func TestApplyAIAssessment(t *testing.T) {
	setupTopicDB(t)

	// 高置信：auto
	if err := sentimentstore.ApplyAIAssessment(sentimentstore.ContentNovel, 101, []string{"二次元", "时政"}, 85); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	ct, err := sentimentstore.TopicOf(sentimentstore.ContentNovel, 101)
	if err != nil || ct == nil {
		t.Fatalf("读不到标注: %v", err)
	}
	if ct.Source != sentimentstore.SourceAI {
		t.Fatalf("来源应为 ai，得到 %s", ct.Source)
	}
	if ct.ReviewStatus != sentimentstore.ReviewAuto {
		t.Fatalf("高置信应为 auto，得到 %s", ct.ReviewStatus)
	}
	// 二次元 + 时政 → 亚文化×政治批判 → 高风险（产品口径）
	if ct.Risk != "high" {
		t.Fatalf("组合话题应为 high，得到 %s（groups=%s）", ct.Risk, ct.Groups)
	}

	// 低置信：needs_review（守卫据此只标注、不拦截）
	if err := sentimentstore.ApplyAIAssessment(sentimentstore.ContentNovel, 102, []string{"时政"}, 30); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	low, _ := sentimentstore.TopicOf(sentimentstore.ContentNovel, 102)
	if low == nil || low.ReviewStatus != sentimentstore.ReviewNeedsReview {
		t.Fatalf("低置信应为 needs_review，得到 %+v", low)
	}
}

// 人工标注优先：AI 不得覆盖人改过的标签。
func TestManualTopicWins(t *testing.T) {
	setupTopicDB(t)

	// 先由人改：标为「玄幻」（无争议话题），策略 off
	db := models.DB
	if err := db.Create(&sentimentstore.ContentTopic{
		ContentType: sentimentstore.ContentNovel, ContentID: 201,
		Tags: `["玄幻"]`, Groups: `[]`, BaseRisk: "low", Risk: "low",
		Source: "manual", Confidence: 100, ReviewStatus: sentimentstore.ReviewConfirmed,
		Policy:    sentimentstore.PolicyOff,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("写人工标注失败: %v", err)
	}

	// AI 再来写「政治批判」——必须被忽略
	if err := sentimentstore.ApplyAIAssessment(sentimentstore.ContentNovel, 201, []string{"政治批判"}, 95); err != nil {
		t.Fatalf("AI 写入不应报错（应静默跳过）: %v", err)
	}
	ct, _ := sentimentstore.TopicOf(sentimentstore.ContentNovel, 201)
	if ct == nil {
		t.Fatalf("标注丢失")
	}
	if ct.Source != "manual" {
		t.Fatalf("来源被 AI 改了: %s", ct.Source)
	}
	if ct.Policy != sentimentstore.PolicyOff {
		t.Fatalf("策略被 AI 改了: %s", ct.Policy)
	}
	if ct.Risk == "high" {
		t.Fatalf("风险被 AI 覆盖了: %s", ct.Risk)
	}
}

// 章节 → 所属作品：评论区挂在作品下，标注必须落到作品上。
func TestTopicRefChapterToNovel(t *testing.T) {
	setupTopicDB(t)
	db := models.DB
	if err := db.Create(&models.Novel{Title: "测试作品", AuthorID: 1}).Error; err != nil {
		t.Fatalf("建作品失败: %v", err)
	}
	var novel models.Novel
	db.First(&novel)
	if err := db.Create(&models.Chapter{NovelID: novel.ID, Title: "第一章"}).Error; err != nil {
		t.Fatalf("建章节失败: %v", err)
	}
	var ch models.Chapter
	db.First(&ch)

	ct, id := topicRef(ScopeChapter, ch.ID)
	if ct != sentimentstore.ContentNovel || id != novel.ID {
		t.Fatalf("章节应映射到作品: got=(%s,%d) want=(%s,%d)", ct, id, sentimentstore.ContentNovel, novel.ID)
	}

	// 论坛/评论本身不作为话题载体
	if ct, _ := topicRef(ScopeComment, 1); ct != "" {
		t.Fatalf("评论不应作为话题载体，得到 %s", ct)
	}
}
