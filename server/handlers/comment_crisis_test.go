package handlers

import (
	"fmt"
	"testing"
	"time"

	"nvs-server/aisafety"
	"nvs-server/models"
	"nvs-server/security"
	sentimentstore "nvs-server/sentiment/store"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupCrisisTestDB 独立内存库（唯一名）。
//
// 刻意不用 setupHandlerTestDB 的 file::memory:?cache=shared ——
// 那是全包共享的同名库，新加 DB 测试会污染既有测试（单跑过、全量偶发挂）。
func setupCrisisTestDB(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:crisis_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移测试库失败: %v", err)
	}
	if err := aisafety.Migrate(db); err != nil {
		t.Fatalf("迁移安全闸表失败: %v", err)
	}
	// 舆情子库用独立 DB 句柄（绝对保护模式要调它的裁决）
	sentimentstore.SetDB(db)
	if err := sentimentstore.Migrate(db); err != nil {
		t.Fatalf("迁移舆情表失败: %v", err)
	}
	models.DB = db
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
}

// TestMinorCrisisSelfHarmSlang 自残黑话 → 自动折叠 + 观察期 + 报站长 + 抚慰私信 + 热线
func TestMinorCrisisSelfHarmSlang(t *testing.T) {
	setupCrisisTestDB(t)
	seedTestUser(t, 1, "victim", "reader")
	seedTestUser(t, 2, "boss", "admin")

	raw := "今天真的很难受，我不想活了，浮木算了"
	cm := &models.Comment{UserID: 1, NovelID: 100, Content: raw}
	if err := models.DB.Create(cm).Error; err != nil {
		t.Fatalf("创建评论失败: %v", err)
	}

	c, _ := asUser("reader", 1)
	card := handleMinorCrisis(c, cm, raw)
	if card == nil || !card.Triggered {
		t.Fatal("自残黑话应触发危机防护")
	}
	if card.Hotline == "" {
		t.Error("危机卡片应带心理危机干预热线")
	}
	if card.Comfort == "" {
		t.Error("危机卡片应带抚慰文案")
	}
	if card.Action != "intercept" {
		t.Errorf("黑话命中的处置动作应为 intercept，实际 %q", card.Action)
	}

	// 评论应被自动折叠 + 标记
	var got models.Comment
	if err := models.DB.First(&got, cm.ID).Error; err != nil {
		t.Fatalf("读取评论失败: %v", err)
	}
	if got.ModerationStatus != "folded" {
		t.Errorf("命中黑话的评论应自动折叠，实际 %q", got.ModerationStatus)
	}
	if !got.FlaggedMinor {
		t.Error("命中黑话的评论应置 flagged_minor")
	}

	// 作者应进入高危观察期（绝对保护模式据此生效）
	var u models.User
	if err := models.DB.First(&u, 1).Error; err != nil {
		t.Fatalf("读取用户失败: %v", err)
	}
	if !u.RiskYouthMental {
		t.Error("作者应被标记为高危观察期")
	}
	if u.RiskYouthMentalUntil == nil || !u.RiskYouthMentalUntil.After(time.Now()) {
		t.Error("高危观察期应有未来的截止时间")
	}

	// 站长应收到告警（人在回路：最后的生死按钮由活人按）
	var alert models.Notification
	if err := models.DB.Where("user_id = ? AND type = ?", 2, "youth_crisis").First(&alert).Error; err != nil {
		t.Error("站长应收到心理危机告警")
	}
	// 作者应收到抚慰私信（系统发出，不是别人在盯着他看）
	var comfort models.Notification
	if err := models.DB.Where("user_id = ? AND type = ?", 1, "crisis_comfort").First(&comfort).Error; err != nil {
		t.Fatal("作者应收到抚慰私信")
	}
	if comfort.Content == "" {
		t.Error("抚慰私信不应为空")
	}
	if comfort.ActorID != 0 {
		t.Errorf("抚慰私信应以系统身份发出，actor_id 应为 0，实际 %d", comfort.ActorID)
	}
	// 危机事件留痕（供《高危用户心理干预报告》）
	var ev models.CrisisEvent
	if err := models.DB.Where("user_id = ?", 1).First(&ev).Error; err != nil {
		t.Fatal("应写入危机事件留痕")
	}
	if ev.Source != "keyword" {
		t.Errorf("事件来源应为 keyword，实际 %q", ev.Source)
	}
	if ev.Action != "intercept" {
		t.Errorf("事件动作应为 intercept，实际 %q", ev.Action)
	}
}

// TestMinorCrisisNormalCommentNotTriggered 普通评论不误伤
func TestMinorCrisisNormalCommentNotTriggered(t *testing.T) {
	setupCrisisTestDB(t)
	seedTestUser(t, 1, "normal", "reader")
	seedTestUser(t, 2, "boss", "admin")

	raw := "这本书写得太好看了"
	cm := &models.Comment{UserID: 1, NovelID: 100, Content: raw}
	if err := models.DB.Create(cm).Error; err != nil {
		t.Fatalf("创建评论失败: %v", err)
	}

	c, _ := asUser("reader", 1)
	if card := handleMinorCrisis(c, cm, raw); card != nil {
		t.Fatalf("普通评论不应触发危机防护，实际 %+v", card)
	}

	var u models.User
	models.DB.First(&u, 1)
	if u.RiskYouthMental {
		t.Error("普通评论不应让用户进入观察期")
	}
	var n int64
	models.DB.Model(&models.Notification{}).Where("type = ?", "crisis_comfort").Count(&n)
	if n != 0 {
		t.Errorf("普通评论不应发出抚慰私信，实际 %d 条", n)
	}
}

// TestMinorCrisisReport 报告应汇总事件并写明「最后由站长决定」
func TestMinorCrisisReport(t *testing.T) {
	setupCrisisTestDB(t)
	seedTestUser(t, 1, "victim", "reader")

	raw := "轻生"
	cm := &models.Comment{UserID: 1, NovelID: 100, Content: raw}
	models.DB.Create(cm)
	c, _ := asUser("reader", 1)
	handleMinorCrisis(c, cm, raw)

	report, err := BuildCrisisReport(1)
	if err != nil {
		t.Fatalf("生成报告失败: %v", err)
	}
	for _, want := range []string{"高危用户心理干预报告", "victim", "内置黑话词表", "站长"} {
		if !contains(report, want) {
			t.Errorf("报告应包含 %q，实际内容：\n%s", want, report)
		}
	}
}

// TestShieldHighRiskTargets 绝对保护模式：指向高危观察期用户的敌意评论先折叠
func TestShieldHighRiskTargets(t *testing.T) {
	setupCrisisTestDB(t)
	seedTestUser(t, 1, "vulnerable", "reader")
	seedTestUser(t, 3, "attacker", "reader")

	// 让 1 号进入高危观察期
	until := time.Now().Add(72 * time.Hour)
	if err := models.DB.Model(&models.User{}).Where("id = ?", 1).Updates(map[string]any{
		"risk_youth_mental":       true,
		"risk_youth_mental_until": until,
	}).Error; err != nil {
		t.Fatalf("预置观察期失败: %v", err)
	}

	// 刷屏/出征行为（@ 5 人以上）会被舆情守卫判为 block → 绝对保护模式应折叠
	hostile := &models.Comment{UserID: 3, NovelID: 100, Content: "@a1 @a2 @a3 @a4 @a5 你太矫情了"}
	shielded := shieldHighRiskTargets(hostile, 1, nil)
	if len(shielded) == 0 {
		t.Fatal("指向高危观察期用户的敌意评论应被绝对保护模式拦下")
	}
	if hostile.ModerationStatus != "folded" {
		t.Errorf("被保护拦下的评论应标记 folded，实际 %q", hostile.ModerationStatus)
	}
	if shielded[0] != 1 {
		t.Errorf("应保护用户 1，实际 %v", shielded)
	}

	// 非高危用户不受此保护
	seedTestUser(t, 5, "safe", "reader")
	other := &models.Comment{UserID: 3, NovelID: 100, Content: "@a1 @a2 @a3 @a4 @a5 你太矫情了"}
	if got := shieldHighRiskTargets(other, 5, nil); len(got) != 0 {
		t.Error("非高危观察期用户不应触发绝对保护模式")
	}
}

// TestHeldCommentsHiddenFromPublic 被折叠的评论对普通读者不可见，对取证视图可见
func TestHeldCommentsHiddenFromPublic(t *testing.T) {
	setupCrisisTestDB(t)
	seedTestUser(t, 1, "author", "reader")

	// 一条正常评论 + 一条被折叠的评论
	normal := &models.Comment{UserID: 1, NovelID: 200, Content: "正常评论", FloorNo: 1}
	models.DB.Create(normal)
	held := &models.Comment{UserID: 1, NovelID: 200, Content: "被折叠的评论", FloorNo: 2, ModerationStatus: "folded"}
	models.DB.Create(held)

	pg, err := models.GetCommentsByNovel(200, 0, 1, 20, false, "new")
	if err != nil {
		t.Fatalf("查询评论失败: %v", err)
	}
	if pg.Total != 1 || pg.FloorTotal != 1 {
		t.Errorf("普通读者只应看到 1 条评论，实际 total=%d floor=%d", pg.Total, pg.FloorTotal)
	}
	for _, c := range pg.List {
		if c.ID == held.ID {
			t.Error("被折叠的评论不应出现在普通读者的列表里")
		}
	}

	// 取证视图（管理员/警方）仍能看到
	pgStaff, err := models.GetCommentsByNovel(200, 0, 1, 20, true, "new")
	if err != nil {
		t.Fatalf("查询取证视图失败: %v", err)
	}
	if pgStaff.Total != 2 {
		t.Errorf("取证视图应看到 2 条评论，实际 %d", pgStaff.Total)
	}
}

// TestDistressSignals 低落语义粗筛：能挑出绝望表达，不误伤普通抱怨
func TestDistressSignals(t *testing.T) {
	positive := []string{
		"今天天气真好，可惜我再也看不到了",
		"撑着不住了，好累",
		"算了解脱了吧，反正没意义",
	}
	for _, s := range positive {
		if len(security.ScanDistressSignals(s)) == 0 {
			t.Errorf("应识别出低落信号：%q", s)
		}
	}
	negative := []string{
		"这本书写得太好看了",
		"今天加班到十点，累死了",
		"跳楼价真的很划算",
	}
	for _, s := range negative {
		if len(security.ScanDistressSignals(s)) != 0 {
			t.Errorf("不应误判为低落信号：%q", s)
		}
	}
}

// contains 简易子串包含（避免为一个小工具引入 strings 之外的依赖）
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
