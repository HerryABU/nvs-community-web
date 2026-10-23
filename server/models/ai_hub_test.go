package models

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAiHubTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:ai_hub_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	DB = db
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestAiChatSessionFlow(t *testing.T) {
	setupAiHubTestDB(t)
	s, err := CreateAiChatSession(1, 2, "写作教练", 3)
	if err != nil {
		t.Fatalf("建会话失败: %v", err)
	}
	// 首条用户消息定标题
	if _, e := AppendAiChatMessage(s.ID, "user", "帮我看下第三章的开头", 0, 0, 0, 0); e != nil {
		t.Fatalf("落消息失败: %v", e)
	}
	TouchAiChatSession(s.ID, "帮我看下第三章的开头")
	got, _ := GetAiChatSession(s.ID, 1)
	if got.Title != "帮我看下第三章的开头" {
		t.Errorf("首条消息应定标题，实际 %q", got.Title)
	}
	// 助手消息带成本
	if _, e := AppendAiChatMessage(s.ID, "assistant", "开头的问题是…", 100, 200, 3, 9); e != nil {
		t.Fatalf("落助手消息失败: %v", e)
	}
	msgs, _ := ListAiChatMessages(s.ID, 50)
	if len(msgs) != 2 || msgs[1].Cost != 3 || msgs[1].CallLogID != 9 {
		t.Errorf("消息列表异常: %+v", msgs)
	}
	// 归属校验：他人会话不可见
	if _, e := GetAiChatSession(s.ID, 999); e == nil {
		t.Errorf("他人会话应不可见")
	}
	// 列表与删除
	list, _ := ListAiChatSessions(1, 10)
	if len(list) != 1 {
		t.Errorf("会话列表应 1 个，实际 %d", len(list))
	}
	if e := DeleteAiChatSession(s.ID, 1); e != nil {
		t.Fatalf("删除会话失败: %v", e)
	}
	if n, _ := CountAiChatMessages(s.ID); n != 0 {
		t.Errorf("删除会话应连带清消息，剩余 %d", n)
	}
}

func TestAiAgentSeedAndCRUD(t *testing.T) {
	setupAiHubTestDB(t)
	SeedDefaultAiAgents()
	agents, err := ListAiAgents(false)
	if err != nil || len(agents) != 4 {
		t.Fatalf("应种子 4 个内置智能体，实际 %d err=%v", len(agents), err)
	}
	// 重复种子不叠加
	SeedDefaultAiAgents()
	agents2, _ := ListAiAgents(false)
	if len(agents2) != 4 {
		t.Errorf("重复种子不应叠加，实际 %d", len(agents2))
	}
	// 禁用过滤
	_ = UpdateAiAgentFields(agents[0].ID, map[string]interface{}{"enabled": false})
	onlyEnabled, _ := ListAiAgents(true)
	if len(onlyEnabled) != 3 {
		t.Errorf("启用列表应 3 个，实际 %d", len(onlyEnabled))
	}
	// 新建 + 空名拒绝
	if e := CreateAiAgent(&AiAgent{Name: "  ", SystemPrompt: "x"}); e == nil {
		t.Errorf("空名称应被拒绝")
	}
	a := &AiAgent{Name: "自定义助手", SystemPrompt: "你是测试助手", Enabled: true}
	if e := CreateAiAgent(a); e != nil {
		t.Fatalf("创建失败: %v", e)
	}
	// 更新与删除
	if e := UpdateAiAgentFields(a.ID, map[string]interface{}{"description": "测试用"}); e != nil {
		t.Fatalf("更新失败: %v", e)
	}
	got, _ := GetAiAgent(a.ID)
	if got.Description != "测试用" {
		t.Errorf("更新未生效: %q", got.Description)
	}
	if e := DeleteAiAgent(a.ID); e != nil {
		t.Fatalf("删除失败: %v", e)
	}
}

func TestAiMediaImageCharge(t *testing.T) {
	setupAiHubTestDB(t)
	m := &AiModel{Name: "绘画模型", Kind: AiKindImage, ModelID: "img", BaseURL: "http://x", Enabled: true, PriceCall: 5}
	if err := DB.Create(m).Error; err != nil {
		t.Fatalf("建模型失败: %v", err)
	}
	// 类型筛选：chat 列表不含绘画模型
	chatModels, _ := ListAiModelsByKind(AiKindChat, true)
	if len(chatModels) != 0 {
		t.Errorf("chat 列表不应含绘画模型，实际 %d", len(chatModels))
	}
	imgModels, _ := ListAiModelsByKind(AiKindImage, true)
	if len(imgModels) != 1 {
		t.Fatalf("image 列表应 1 个，实际 %d", len(imgModels))
	}
	// 空 kind（旧数据）按 chat 归一
	_ = DB.Create(&AiModel{Name: "旧模型", ModelID: "old", BaseURL: "http://x", Enabled: true}).Error
	chatModels2, _ := ListAiModelsByKind("", true)
	if len(chatModels2) != 1 {
		t.Errorf("空 kind 旧数据应按 chat 纳入，实际 %d", len(chatModels2))
	}
	if NormalizeAiKind("bogus") != AiKindChat {
		t.Errorf("非法 kind 应归一为 chat")
	}

	// 任务扣款：充 10 → 扣 5 → 退 5
	if _, e := AppendTokenLedger(1, TokenTypeGrant, 10, "admin", 1, "", ""); e != nil {
		t.Fatalf("增发失败: %v", e)
	}
	task, err := CreateAiMediaTask(1, m, AiKindImage, "一只猫", AiMediaParams{Size: "1024x1024", N: 1})
	if err != nil {
		t.Fatalf("建任务失败: %v", err)
	}
	if task.Status != AiMediaPending {
		t.Fatalf("初始状态应 pending，实际 %s", task.Status)
	}
	nb, cerr := ChargeAiMediaTask(task.ID)
	if cerr != nil || nb != 5 {
		t.Fatalf("扣款后期望余额 5，实际 nb=%d err=%v", nb, cerr)
	}
	got, _ := GetAiMediaTask(task.ID, 1)
	if got.Status != AiMediaSucceeded || got.Cost != 5 {
		t.Errorf("任务状态异常: %+v", got)
	}
	// 重复扣款安全（幂等）
	if nb2, _ := ChargeAiMediaTask(task.ID); nb2 != 5 {
		t.Errorf("重复扣款应幂等返回 5，实际 %d", nb2)
	}
	// 流水：consume -5 一笔
	var n int64
	DB.Model(&TokenLedger{}).Where("user_id = ? AND type = ?", 1, TokenTypeConsume).Count(&n)
	if n != 1 {
		t.Errorf("应 1 笔消费流水，实际 %d", n)
	}
	// 退款回滚
	rb, rerr := RefundAiMediaTask(task.ID, "测试退款")
	if rerr != nil || rb != 10 {
		t.Fatalf("退款后期望余额 10，实际 rb=%d err=%v", rb, rerr)
	}
	got, _ = GetAiMediaTask(task.ID, 1)
	if got.Status != AiMediaFailed {
		t.Errorf("退款后状态应 failed，实际 %s", got.Status)
	}
	// 归属校验
	if _, e := GetAiMediaTask(task.ID, 999); e == nil {
		t.Errorf("他人任务应不可见")
	}
	// 历史分页
	list, total, _ := ListAiMediaTasks(1, AiKindImage, 1, 20)
	if total != 1 || len(list) != 1 {
		t.Errorf("历史异常: total=%d len=%d", total, len(list))
	}
	// free 模型不碰账本
	free := &AiModel{Name: "免费绘画", Kind: AiKindImage, ModelID: "img2", BaseURL: "http://x", Enabled: true, PriceCall: 0}
	_ = DB.Create(free).Error
	ft, _ := CreateAiMediaTask(2, free, AiKindImage, "测试", AiMediaParams{})
	if nb3, e := ChargeAiMediaTask(ft.ID); e != nil || nb3 != 0 {
		t.Errorf("免费任务应返回 0，实际 nb=%d err=%v", nb3, e)
	}
}
