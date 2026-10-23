package models

import (
	"testing"
	"time"
)

// TestPublishAndLikeMedia 作品广场：只允许成功作品发布、取消发布、点赞计数
func TestPublishAndLikeMedia(t *testing.T) {
	db := setupAiTestDB(t)
	old := DB
	DB = db
	defer func() { DB = old }()

	u := User{Username: "gallery_u", Email: "g@example.com", PasswordHash: "x"}
	if err := DB.Create(&u).Error; err != nil {
		t.Fatalf("建用户失败：%v", err)
	}
	task := &AiMediaTask{
		UserID: u.ID, Kind: AiKindImage, ModelID: 0, ModelName: "测试模型",
		Prompt: "雨夜霓虹下的旧书店", Status: AiMediaSucceeded,
		Result: `{"images":["https://example.com/a.png"]}`, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := DB.Create(task).Error; err != nil {
		t.Fatalf("建任务失败：%v", err)
	}

	// 发布：标题留空自动取提示词
	if err := PublishAiMedia(task.ID, u.ID, true, "", "国风"); err != nil {
		t.Fatalf("发布失败：%v", err)
	}
	list, total, err := ListPublicMedia(AiKindImage, "国风", 1, 10)
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("公开列表异常：total=%d len=%d err=%v", total, len(list), err)
	}
	if list[0].Title == "" || list[0].Style != "国风" {
		t.Fatalf("标题/风格未写入：%q/%q", list[0].Title, list[0].Style)
	}

	// 点赞
	if err := LikeAiMedia(task.ID); err != nil {
		t.Fatalf("点赞失败：%v", err)
	}
	var after AiMediaTask
	DB.Where("id = ?", task.ID).First(&after)
	if after.Likes != 1 {
		t.Fatalf("点赞数应为 1，得到 %d", after.Likes)
	}

	// 非作者不能发布
	other := User{Username: "gallery_v", Email: "g2@example.com", PasswordHash: "x"}
	DB.Create(&other)
	if err := PublishAiMedia(task.ID, other.ID, true, "劫持", ""); err == nil {
		t.Fatalf("非作者发布应被拒绝")
	}

	// 失败任务不可发布
	failed := &AiMediaTask{UserID: u.ID, Kind: AiKindImage, Prompt: "x", Status: AiMediaFailed, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	DB.Create(failed)
	if err := PublishAiMedia(failed.ID, u.ID, true, "", ""); err == nil {
		t.Fatalf("失败任务不应可发布")
	}

	// 取消发布后公开列表应为空
	if err := PublishAiMedia(task.ID, u.ID, false, "", "国风"); err != nil {
		t.Fatalf("取消发布失败：%v", err)
	}
	if _, total2, _ := ListPublicMedia(AiKindImage, "", 1, 10); total2 != 0 {
		t.Fatalf("取消发布后仍可见：%d", total2)
	}
}
