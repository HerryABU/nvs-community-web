package aihub

import (
	"nvs-server/models"
	ai "nvs-server/models/ai"
	"testing"
	"time"
)

// ===== (原 ai_free_quota_test.go) =====

func TestAiFreeQuotaDefaultOff(t *testing.T) {
	setupAiTestDB(t)
	// 默认关闭：任何用户都不可走免费
	if IsAiFreeQuotaEnabled() {
		t.Fatal("免费额度默认应为关闭")
	}
	if ok, _, _ := CheckAiFreeQuota(1, 100); ok {
		t.Fatal("关闭状态下不应判定为可免费")
	}
	// 开启后：默认每日 5 次
	if err := SetAiFreeQuotaEnabled(true); err != nil {
		t.Fatalf("开启失败: %v", err)
	}
	cfg := GetAiFreeQuotaConfig()
	if cfg.CallsPerDay != 5 {
		t.Fatalf("默认每日免费次数应为 5，实际 %d", cfg.CallsPerDay)
	}
	if ok, callsLeft, _ := CheckAiFreeQuota(1, 100); !ok || callsLeft != 5 {
		t.Fatalf("开启后应可免费且剩余 5 次，实际 ok=%v left=%d", ok, callsLeft)
	}
}

func TestAiFreeQuotaConsumeAndExhaust(t *testing.T) {
	setupAiTestDB(t)
	if err := SetAiFreeQuotaEnabled(true); err != nil {
		t.Fatalf("开启失败: %v", err)
	}
	if err := SetAiFreeQuotaConfig(AiFreeQuotaConfig{CallsPerDay: 2, MaxTokensPerDay: 0}); err != nil {
		t.Fatalf("配置失败: %v", err)
	}
	// 第 1、2 次：可免费，记用量
	for i := 0; i < 2; i++ {
		if ok, _, _ := CheckAiFreeQuota(7, 50); !ok {
			t.Fatalf("第 %d 次应可免费", i+1)
		}
		if err := RecordAiFreeUsage(7, 1, 50); err != nil {
			t.Fatalf("记用量失败: %v", err)
		}
	}
	// 第 3 次：次数用尽 → 不可免费
	if ok, _, _ := CheckAiFreeQuota(7, 50); ok {
		t.Fatal("次数用尽后不应可免费")
	}
	// 换用户不受影响
	if ok, callsLeft, _ := CheckAiFreeQuota(8, 50); !ok || callsLeft != 2 {
		t.Fatalf("其他用户应仍有 2 次，实际 ok=%v left=%d", ok, callsLeft)
	}
}

func TestAiFreeQuotaTokenBudget(t *testing.T) {
	setupAiTestDB(t)
	if err := SetAiFreeQuotaEnabled(true); err != nil {
		t.Fatalf("开启失败: %v", err)
	}
	// tokens 预算 100/日，单次预估 60：第一次可，第二次（60+60>100）不可
	if err := SetAiFreeQuotaConfig(AiFreeQuotaConfig{CallsPerDay: -1, MaxTokensPerDay: 100}); err != nil {
		t.Fatalf("配置失败: %v", err)
	}
	if ok, _, tokensLeft := CheckAiFreeQuota(9, 60); !ok || tokensLeft != 100 {
		t.Fatalf("首次应可免费且 tokens 剩 100，实际 ok=%v left=%d", ok, tokensLeft)
	}
	_ = RecordAiFreeUsage(9, 1, 60)
	if ok, _, _ := CheckAiFreeQuota(9, 60); ok {
		t.Fatal("tokens 预算不足时不应可免费")
	}
	if ok, _, _ := CheckAiFreeQuota(9, 40); !ok {
		t.Fatal("tokens 预算内应可免费")
	}
}

func TestAiFreeQuotaOverride(t *testing.T) {
	setupAiTestDB(t)
	if err := SetAiFreeQuotaEnabled(true); err != nil {
		t.Fatalf("开启失败: %v", err)
	}
	// 按人覆盖：禁止免费（0 次）
	if err := SetUserAiFreeQuotaOverride(10, 0, 0, "测试封禁免费"); err != nil {
		t.Fatalf("覆盖失败: %v", err)
	}
	if ok, _, _ := CheckAiFreeQuota(10, 10); ok {
		t.Fatal("覆盖为 0 次后不应可免费")
	}
	// 覆盖为不限次数
	if err := SetUserAiFreeQuotaOverride(10, -1, 0, ""); err != nil {
		t.Fatalf("覆盖失败: %v", err)
	}
	if ok, callsLeft, _ := CheckAiFreeQuota(10, 10); !ok || callsLeft != -1 {
		t.Fatalf("覆盖为不限后应可免费，实际 ok=%v left=%d", ok, callsLeft)
	}
	// 删除覆盖 → 恢复平台默认（5 次）
	if err := DeleteUserAiFreeQuotaOverride(10); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if ok, callsLeft, _ := CheckAiFreeQuota(10, 10); !ok || callsLeft != 5 {
		t.Fatalf("删除覆盖后应恢复默认 5 次，实际 ok=%v left=%d", ok, callsLeft)
	}
}

func TestAiCallLogFreeSettlement(t *testing.T) {
	setupAiTestDB(t)
	if err := SetAiFreeQuotaEnabled(true); err != nil {
		t.Fatalf("开启失败: %v", err)
	}
	m := seedAiModel(t, 2, 4, 1024)
	// 免费调用：结算后 cost=0、不产生代币流水
	freeLog, err := ai.CreateAiCallLog(1, m, "continue", "", 0, 1000, 500, true)
	if err != nil {
		t.Fatalf("创建免费调用失败: %v", err)
	}
	nb, err := ai.SettleAiCallLog(freeLog.ID, 1000, 500)
	if err != nil {
		t.Fatalf("结算失败: %v", err)
	}
	if nb != 0 {
		t.Fatalf("免费调用不应改变余额，实际 %d", nb)
	}
	var log ai.AiCallLog
	if err := models.DB.Where("id = ?", freeLog.ID).First(&log).Error; err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if log.Status != ai.AiCallSettled || log.Cost != 0 {
		t.Fatalf("免费调用应 settled 且 cost=0，实际 %s/%d", log.Status, log.Cost)
	}
	// 付费调用：正常结算扣款（模型价 2/4 → 1000 入 = 2，500 出 = 2，共 4）
	payLog, err := ai.CreateAiCallLog(2, m, "continue", "", 0, 1000, 500, false)
	if err != nil {
		t.Fatalf("创建付费调用失败: %v", err)
	}
	models.AppendTokenLedger(2, models.TokenTypeRechargeCard, 10, "", 0, "测试充值", "")
	nb, err = ai.SettleAiCallLog(payLog.ID, 1000, 500)
	if err != nil {
		t.Fatalf("付费结算失败: %v", err)
	}
	if nb != 6 {
		t.Fatalf("付费调用后余额应为 6，实际 %d", nb)
	}
}

// ===== 合并自 ai_gallery_test.go =====

// TestPublishAndLikeMedia 作品广场：只允许成功作品发布、取消发布、点赞计数
func TestPublishAndLikeMedia(t *testing.T) {
	db := setupAiTestDB(t)
	old := models.DB
	models.DB = db
	defer func() { models.DB = old }()

	u := models.User{Username: "gallery_u", Email: "g@example.com", PasswordHash: "x"}
	if err := models.DB.Create(&u).Error; err != nil {
		t.Fatalf("建用户失败：%v", err)
	}
	task := &ai.AiMediaTask{
		UserID: u.ID, Kind: ai.AiKindImage, ModelID: 0, ModelName: "测试模型",
		Prompt: "雨夜霓虹下的旧书店", Status: ai.AiMediaSucceeded,
		Result: `{"images":["https://example.com/a.png"]}`, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := models.DB.Create(task).Error; err != nil {
		t.Fatalf("建任务失败：%v", err)
	}

	// 发布：标题留空自动取提示词
	if err := PublishAiMedia(task.ID, u.ID, true, "", "国风"); err != nil {
		t.Fatalf("发布失败：%v", err)
	}
	list, total, err := ListPublicMedia(ai.AiKindImage, "国风", 1, 10)
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
	var after ai.AiMediaTask
	models.DB.Where("id = ?", task.ID).First(&after)
	if after.Likes != 1 {
		t.Fatalf("点赞数应为 1，得到 %d", after.Likes)
	}

	// 非作者不能发布
	other := models.User{Username: "gallery_v", Email: "g2@example.com", PasswordHash: "x"}
	models.DB.Create(&other)
	if err := PublishAiMedia(task.ID, other.ID, true, "劫持", ""); err == nil {
		t.Fatalf("非作者发布应被拒绝")
	}

	// 失败任务不可发布
	failed := &ai.AiMediaTask{UserID: u.ID, Kind: ai.AiKindImage, Prompt: "x", Status: ai.AiMediaFailed, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	models.DB.Create(failed)
	if err := PublishAiMedia(failed.ID, u.ID, true, "", ""); err == nil {
		t.Fatalf("失败任务不应可发布")
	}

	// 取消发布后公开列表应为空
	if err := PublishAiMedia(task.ID, u.ID, false, "", "国风"); err != nil {
		t.Fatalf("取消发布失败：%v", err)
	}
	if _, total2, _ := ListPublicMedia(ai.AiKindImage, "", 1, 10); total2 != 0 {
		t.Fatalf("取消发布后仍可见：%d", total2)
	}
}
