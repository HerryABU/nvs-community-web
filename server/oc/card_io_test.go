package oc

import (
	"encoding/json"
	"strings"
	"testing"

	"nvs-server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

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
