package oc

import (
	"os"
	"strings"
	"testing"

	"nvs-server/config"
	"nvs-server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupOCDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:oc_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("主迁移失败: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("OC 表迁移失败: %v", err)
	}
	old := models.DB
	models.DB = db
	dir := t.TempDir()
	oldDir := config.NovelDataDir
	config.NovelDataDir = dir
	t.Cleanup(func() {
		models.DB = old
		config.NovelDataDir = oldDir
		_ = os.RemoveAll(dir)
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func mkOCUser(t *testing.T, name string) *models.User {
	t.Helper()
	u := &models.User{Username: name, Email: name + "@example.com", PasswordHash: "x", Role: models.RoleReader}
	if err := models.DB.Create(u).Error; err != nil {
		t.Fatalf("建用户失败: %v", err)
	}
	return u
}

// TestCharacterCRUD 角色卡创建/公开待审/软删除留证
func TestCharacterCRUD(t *testing.T) {
	setupOCDB(t)
	u := mkOCUser(t, "oc_u1")

	c, err := CreateCharacter(u.ID, CharacterInput{
		Name: "青崖", Persona: "冷淡的守夜人", Speech: "短句、少形容词",
		Scenario: "雨夜的城门", FirstMessage: "「这么晚，进城做什么？」",
		MesExample: "{{user}}：你是谁？\n{{char}}：守夜的。",
		Visibility: VisPrivate,
	})
	if err != nil || c.Review != ReviewApproved {
		t.Fatalf("创建私有角色卡失败: %+v err=%v", c, err)
	}
	// 转公开 → 待审核
	if _, err := UpdateCharacter(u.ID, c.ID, CharacterInput{
		Name: "青崖", Persona: "冷淡的守夜人", Visibility: VisPublic,
	}); err != nil {
		t.Fatal(err)
	}
	var got Character
	_ = models.DB.Where("id = ?", c.ID).First(&got).Error
	if got.Review != ReviewPending {
		t.Fatalf("转公开应回到待审: %+v", got)
	}
	// 他人不可读未过审的公开卡
	other := mkOCUser(t, "oc_u2")
	if _, err := GetCharacter(c.ID, other.ID, false); err == nil {
		t.Fatal("未过审的公开卡不应可读")
	}
	// 审核通过
	if err := ReviewCharacter(c.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if cc, err := GetCharacter(c.ID, other.ID, false); err != nil || cc.Name != "青崖" {
		t.Fatalf("过审后应可读: %+v err=%v", cc, err)
	}
	// 软删除留证
	if err := DeleteCharacter(u.ID, c.ID); err != nil {
		t.Fatal(err)
	}
	var after Character
	_ = models.DB.Where("id = ?", c.ID).First(&after).Error
	if after.Status != "deleted" {
		t.Fatalf("删除应留证: %+v", after)
	}
}

// TestLorebookActivation 世界书按键触发（酒馆式：不整本塞上下文）
func TestLorebookActivation(t *testing.T) {
	setupOCDB(t)
	u := mkOCUser(t, "oc_lore")
	book, err := CreateWorldBook(u.ID, 0, "铁堡设定", "要塞城市", VisPrivate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SaveEntry(u.ID, book.ID, 0, EntryPlace, "铁堡", "铁堡扼守峡谷，由铁卫驻守。", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveEntry(u.ID, book.ID, 0, EntryFaction, "黑鸦", "黑鸦是一群走私者。", 20); err != nil {
		t.Fatal(err)
	}
	// 常驻条目
	if _, err := SaveEntry(u.ID, book.ID, 0, EntryRule, "夜间戒严", "入夜后城门关闭。", 5); err != nil {
		t.Fatal(err)
	}
	var constantEntry WorldEntry
	_ = models.DB.Where("book_id = ? AND key = ?", book.ID, "夜间戒严").First(&constantEntry).Error
	_ = models.DB.Model(&WorldEntry{}).Where("id = ?", constantEntry.ID).Update("constant", true).Error

	entries, _ := ListEntries(book.ID)
	recent := []Turn{{Seq: 1, Content: "我们明天去铁堡看看。"}}
	act := Activate(book, entries, recent)
	if act.Total == 0 {
		t.Fatal("应至少命中条目")
	}
	before, after, _ := RenderLore(act)
	all := before + after
	if !strings.Contains(all, "铁堡") {
		t.Fatalf("未注入铁堡条目: %q", all)
	}
	if strings.Contains(all, "黑鸦") {
		t.Fatalf("未提及的黑鸦不应注入: %q", all)
	}
	if !strings.Contains(all, "夜间戒严") {
		t.Fatalf("常驻条目应始终注入: %q", all)
	}
}

// TestLorebookGroupAndBudget 包含组只取其一 + token 预算截断
func TestLorebookGroupAndBudget(t *testing.T) {
	book := &WorldBook{TokenBudget: 200}
	e1 := WorldEntry{ID: 1, Key: "龙", Content: "龙族古老而骄傲，占据北境群山。", Group: "race", InsertionOrder: 10, Enabled: true}
	e2 := WorldEntry{ID: 2, Key: "龙", Content: "另一条龙族设定，不该同时出现。", Group: "race", InsertionOrder: 20, Enabled: true}
	e3 := WorldEntry{ID: 3, Key: "王", Content: "国王年迈。", InsertionOrder: 30, Enabled: true}
	act := Activate(book, []WorldEntry{e1, e2, e3}, []Turn{{Content: "龙与王"}})
	if act.Total < 2 {
		t.Fatalf("应命中 2 条（组取其一 + 王）: %+v", act)
	}
	var hasE2 bool
	for _, l := range append(append([]ActivatedEntry{}, act.Before...), act.After...) {
		if l.Entry.ID == 2 {
			hasE2 = true
		}
	}
	if hasE2 {
		t.Fatal("同组的两条不应同时激活")
	}
}

// TestEmotionAndState 情绪标记解析 + 状态养成
func TestEmotionAndState(t *testing.T) {
	setupOCDB(t)
	u := mkOCUser(t, "oc_state")
	c, err := CreateCharacter(u.ID, CharacterInput{Name: "小灯", Persona: "活泼", Mood: 50, Energy: 80})
	if err != nil {
		t.Fatal(err)
	}
	emotion, body := ParseEmotion("[情绪:笑] 今天天气真好。")
	if emotion != "笑" || strings.Contains(body, "情绪") {
		t.Fatalf("情绪解析错误: %q / %q", emotion, body)
	}
	d := ApplyTurnToState(c.ID, emotion, true)
	if d.Mood <= 0 {
		t.Fatalf("正向情绪应提升 mood: %+v", d)
	}
	var got Character
	_ = models.DB.Where("id = ?", c.ID).First(&got).Error
	if got.Mood <= 50 {
		t.Fatalf("状态未变化: %+v", got)
	}
	if got.Affinity != 2 {
		t.Fatalf("好感应提升: %+v", got)
	}
	// 负向情绪
	_ = ApplyTurnToState(c.ID, "怒", false)
	_ = models.DB.Where("id = ?", c.ID).First(&got).Error
	if got.Mood >= 60 {
		t.Fatalf("负向情绪应降低 mood: %+v", got)
	}
}

// TestPublishFlowToNovel 对话流 → 话本章节（含设定集与来源追溯）
func TestPublishFlowToNovel(t *testing.T) {
	setupOCDB(t)
	u := mkOCUser(t, "oc_pub")
	f := &Flow{GroupID: 1, StarterID: u.ID, Schedule: ScheduleRoundRobin, Status: FlowRunning, MaxTurns: 10}
	if err := models.DB.Create(f).Error; err != nil {
		t.Fatal(err)
	}
	// 群配置：启用 OC
	if _, err := SaveGroupSetting(1, GroupSetting{Enabled: true, Mode: ModeMixed}); err != nil {
		t.Fatal(err)
	}
	char, _ := CreateCharacter(u.ID, CharacterInput{Name: "阿澈", Persona: "沉默", Visibility: VisPrivate})
	turns := []Turn{
		{FlowID: f.ID, GroupID: 1, Seq: 1, CharacterID: char.ID, Content: "你来了。", Review: ReviewApproved},
		{FlowID: f.ID, GroupID: 1, Seq: 2, CharacterID: char.ID, Content: "坐吧。", Review: ReviewApproved},
	}
	for i := range turns {
		if err := models.DB.Create(&turns[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	res, err := PublishFlowToNovel(PublishInput{
		FlowID: f.ID, UserID: u.ID, Title: "雨夜对话",
		Style: StyleDialogue, IncludeSettings: true, PublishNow: true,
	})
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	if res.ChapterCnt != 2 || res.NovelID == 0 {
		t.Fatalf("发布结果异常: %+v", res)
	}
	var chs []models.Chapter
	_ = models.DB.Where("novel_id = ?", res.NovelID).Order("chapter_number ASC").Find(&chs).Error
	if len(chs) != 2 {
		t.Fatalf("章节数不对: %d", len(chs))
	}
	if chs[0].Title != "设定集" || !strings.Contains(chs[0].Title, "设定") {
		t.Fatalf("首个章节应是设定集: %+v", chs[0])
	}
	if chs[1].Mode != "script" {
		t.Fatalf("对话体应落成话本形态: %+v", chs[1])
	}
	if chs[1].SourceFlowID != f.ID {
		t.Fatalf("来源追溯缺失: %+v", chs[1])
	}
	// 正文应写成「角色：台词」
	data, err := os.ReadFile(chs[1].ContentPath)
	if err != nil {
		t.Fatalf("正文文件缺失: %v", err)
	}
	if !strings.Contains(string(data), "阿澈：你来了。") {
		t.Fatalf("话本正文格式不对: %q", string(data))
	}
}

// TestModerationWindow 窗口审核：多条一起审，命中即标记并暂停
func TestModerationWindow(t *testing.T) {
	setupOCDB(t)
	u := mkOCUser(t, "oc_mod")
	f := &Flow{GroupID: 2, StarterID: u.ID, Status: FlowRunning, MaxTurns: 50, TurnCount: 3}
	if err := models.DB.Create(f).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := SaveGroupSetting(2, GroupSetting{
		Enabled: true, AuditLevel: AuditStrict, AuditTiming: AuditPost, WindowSize: 3,
	}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		_ = models.DB.Create(&Turn{FlowID: f.ID, GroupID: 2, Seq: i, Content: "普通的一句台词。", Review: ReviewApproved}).Error
	}
	w, err := AuditWindow(f.ID, 3)
	if err != nil {
		t.Fatalf("窗口审核失败: %v", err)
	}
	if w == nil || w.TurnCount != 3 {
		t.Fatalf("窗口范围不对: %+v", w)
	}
	if w.Result != "pass" {
		t.Fatalf("正常内容应通过: %+v", w)
	}
	var tws []Turn
	_ = models.DB.Where("flow_id = ?", f.ID).Find(&tws).Error
	for _, tt := range tws {
		if tt.WindowID == 0 {
			t.Fatalf("窗口 ID 未回写: %+v", tt)
		}
	}
}
