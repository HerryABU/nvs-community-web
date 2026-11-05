package oc

import (
	"encoding/json"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"nvs-server/config"
	"nvs-server/models"
	social "nvs-server/models/social"
	"os"
	"strings"
	"testing"
)

// ===== (原 oc_test.go) =====

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
	u := &models.User{Username: name, Email: name + "@example.com", PasswordHash: "x", Role: social.RoleReader}
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

// ===== 合并自 card_io_test.go =====

func setupCardDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:card_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	models.DB = db
	if err := db.AutoMigrate(&Character{}, &WorldBook{}, &WorldEntry{}, &Emote{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_ = db.Exec("DELETE FROM oc_characters").Error
	_ = db.Exec("DELETE FROM oc_world_books").Error
	_ = db.Exec("DELETE FROM oc_world_entries").Error
	_ = db.Exec("DELETE FROM oc_emotes").Error
}

// TestImportCardV2 酒馆 V2 卡片导入：标准字段 + extensions.nvs + character_book
func TestImportCardV2(t *testing.T) {
	raw := []byte(`{
		"spec":"chara_card_v2","spec_version":"2.0",
		"data":{
			"name":"林墨","description":"沉默寡言的书店老板","personality":"冷淡",
			"scenario":"雨夜的书店","first_mes":"「书要关门了。」","mes_example":"<START>",
			"creator_notes":"测试用","system_prompt":"保持冷淡","post_history_instructions":"",
			"alternate_greetings":["「还没走？」"],"tags":["都市","治愈"],
			"extensions":{
				"nvs":{"appearance":"灰毛衣","speech":"短句，句末常带省略号"},
				"character_book":{"name":"书店世界","entries":{
					"0":{"key":["书店","雨"],"content":"书店在巷子深处。","insertion_order":10,"probability":100}
				}}
			}
		}
	}`)
	res, err := ParseCard(raw)
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != "林墨" || res.Persona != "沉默寡言的书店老板" {
		t.Fatalf("基础字段解析错: %+v", res)
	}
	if res.Appearance != "灰毛衣" || res.Speech == "" {
		t.Fatalf("扩展字段解析错: %+v", res)
	}
	if len(res.Greetings) != 1 || res.Tags != "都市,治愈" {
		t.Fatalf("开场/标签解析错: %+v", res)
	}
	if res.Book == nil || len(res.Book.Entries) != 1 {
		t.Fatalf("世界书应被解析: %+v", res.Book)
	}
	if res.Book.Entries[0].Content == "" || len(res.Book.Entries[0].Keys) != 2 {
		t.Fatalf("世界书条目解析错: %+v", res.Book.Entries[0])
	}
}

// TestImportCardV1 扁平 V1 卡片
func TestImportCardV1(t *testing.T) {
	raw := []byte(`{"name":"阿七","description":"街头歌手","personality":"热情","first_mes":"来听一首？"}`)
	res, err := ParseCard(raw)
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != "阿七" || res.Persona != "街头歌手" {
		t.Fatalf("V1 解析错: %+v", res)
	}
}

// TestImportCardInvalid 非法输入必须明确报错，不能静默建一个空角色
func TestImportCardInvalid(t *testing.T) {
	if _, err := ParseCard([]byte("not json")); err == nil {
		t.Fatal("非 JSON 应报错")
	}
	if _, err := ParseCard([]byte(`{"data":{"description":"x"}}`)); err == nil {
		t.Fatal("缺 name 应报错")
	}
	if _, err := ParseCard(nil); err == nil {
		t.Fatal("空输入应报错")
	}
}

// TestExportCardRoundTrip 导出→再解析，关键字段不丢
func TestExportCardRoundTrip(t *testing.T) {
	setupCardDB(t)
	ch := &Character{
		UserID: 1, Name: "苏晚", Persona: "冷静的侦探", Appearance: "长风衣",
		Speech: "语速慢", Scenario: "雾港", FirstMessage: "你迟到了。",
		MesExample: "示例", CreatorNotes: "备注", SystemPrompt: "系统",
		Tags: "悬疑,推理", Greetings: `["你来晚了。"]`,
		Mood: 70, Affinity: 20, Energy: 80, EmoteMap: `{"笑":"/uploads/a.png"}`,
		Visibility: "private", Review: "approved", Status: "active",
	}
	if err := models.DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	card, err := ExportCard(ch)
	if err != nil {
		t.Fatal(err)
	}
	if card.Spec != "chara_card_v2" || card.Data.Name != "苏晚" {
		t.Fatalf("导出结构错: %+v", card)
	}
	if card.Data.Extensions == nil {
		t.Fatal("必须带 extensions.nvs（否则自家字段不可逆）")
	}
	// 往返：把导出的 JSON 再解析
	b, _ := json.Marshal(card)
	back, err := ParseCard(b)
	if err != nil {
		t.Fatal(err)
	}
	if back.Name != ch.Name || back.Persona != ch.Persona {
		t.Fatalf("往返丢字段: %+v", back)
	}
	if back.Appearance != ch.Appearance || back.Speech != ch.Speech {
		t.Fatalf("扩展字段往返丢失: appearance=%q speech=%q", back.Appearance, back.Speech)
	}
	if len(back.Greetings) != 1 {
		t.Fatalf("备用开场往返丢失: %+v", back.Greetings)
	}
}

// TestEmoteValidation 表情键/URL 校验（防 XSS 与语法冲突）
func TestEmoteValidation(t *testing.T) {
	if err := ValidateEmoteKey("笑"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateEmoteKey("带[括号"); err == nil {
		t.Fatal("含方括号应拒绝")
	}
	if err := ValidateEmoteKey(""); err == nil {
		t.Fatal("空键应拒绝")
	}
	if err := ValidateEmoteKey("一二三四五六七八九"); err == nil {
		t.Fatal("超长应拒绝")
	}
	if err := ValidateEmoteURL("/uploads/2026/01/a.png"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateEmoteURL("https://x.com/a.png"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"javascript:alert(1)", "data:image/png;base64,xx", "blob:x", "ftp://x/a.png"} {
		if err := ValidateEmoteURL(bad); err == nil {
			t.Fatalf("应拒绝: %s", bad)
		}
	}
}

// TestCreateEmoteKeyConflict 键全局唯一：别人占了就明确报错，不静默覆盖
func TestCreateEmoteKeyConflict(t *testing.T) {
	setupCardDB(t)
	if _, err := CreateEmote(1, "小猫", "/uploads/a.png", ""); err != nil {
		t.Fatal(err)
	}
	_, err := CreateEmote(2, "小猫", "/uploads/b.png", "")
	if err == nil || !strings.Contains(err.Error(), "占用") {
		t.Fatalf("键冲突应明确拒绝: %v", err)
	}
	// 同一用户重复添加也提示
	if _, err := CreateEmote(1, "小猫", "/uploads/a.png", ""); err == nil {
		t.Fatal("同一用户重复键也应拒绝")
	}
	// 删除后可重建
	list, _ := ListMyEmotes(1)
	if len(list) != 1 {
		t.Fatalf("应有一条: %d", len(list))
	}
	if err := DeleteEmote(1, list[0].ID); err != nil {
		t.Fatal(err)
	}
	if e, err := CreateEmote(2, "小猫", "/uploads/b.png", ""); err != nil || e.Key != "小猫" {
		t.Fatalf("删除后应可重建: %v", err)
	}
}
