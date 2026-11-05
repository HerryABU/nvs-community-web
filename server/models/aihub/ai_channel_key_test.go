package aihub

import (
	"fmt"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"nvs-server/models"
	ai "nvs-server/models/ai"
	"strings"
	"testing"
	"time"
)

// ===== (原 ai_channel_key_test.go) =====

func setupChannelKeyTestDB(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:chankey_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存测试库失败: %v", err)
	}
	if err := db.AutoMigrate(&ai.AiModel{}, &AiChannelKey{}, &AiModelKey{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	old := models.DB
	models.DB = db
	t.Cleanup(func() { models.DB = old })
}

const testChannel = "https://api.example.com/v1"

// 同一把密钥在同一渠道下不应重复入库（否则轮询权重会被悄悄放大）
func TestAddAiChannelKey_RejectsDuplicate(t *testing.T) {
	setupChannelKeyTestDB(t)
	if _, err := AddAiChannelKey(testChannel, "卡1", "sk-abcdefgh12345678", "", 1); err != nil {
		t.Fatalf("首次添加应成功: %v", err)
	}
	if _, err := AddAiChannelKey(testChannel, "卡1-重复", "sk-abcdefgh12345678", "", 1); err == nil {
		t.Fatalf("同一把密钥重复添加应被拒绝")
	}
	// 尾斜杠/大小写不同但指向同一渠道，也要判为重复
	if _, err := AddAiChannelKey("HTTPS://API.example.com/v1/", "卡1-别名", "sk-abcdefgh12345678", "", 1); err == nil {
		t.Fatalf("渠道地址归一后应判为同一渠道")
	}
	// 不同渠道可以存同一把（不同站点各记一份是合理的）
	if _, err := AddAiChannelKey("https://other.example.com/v1", "卡2", "sk-abcdefgh12345678", "", 1); err != nil {
		t.Fatalf("不同渠道应允许相同密钥: %v", err)
	}
}

// 连败达到阈值自动摘除，之后的轮询不再选中它
func TestPickAiChannelKey_AutoDisableAfterFails(t *testing.T) {
	setupChannelKeyTestDB(t)
	k, err := AddAiChannelKey(testChannel, "坏卡", "sk-broken-00000001", "", 1)
	if err != nil {
		t.Fatalf("添加失败: %v", err)
	}
	for i := 0; i < MaxFailBeforeDisable; i++ {
		ReportAiChannelKey(k.ID, false)
	}
	var got AiChannelKey
	if err := models.DB.Where("id = ?", k.ID).First(&got).Error; err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if got.Enabled {
		t.Fatalf("连败 %d 次后应自动停用", MaxFailBeforeDisable)
	}
	if got.FailTotal != int64(MaxFailBeforeDisable) {
		t.Fatalf("累计失败数应为 %d，实际 %d", MaxFailBeforeDisable, got.FailTotal)
	}
	if p := PickAiChannelKey(testChannel); p != nil {
		t.Fatalf("池内无可用卡密时应返回 nil，实际取到 id=%d", p.ID)
	}
	// 站长手动勾回启用：连续失败计数应清零，累计数保留
	enabled := true
	if err := UpdateAiChannelKey(k.ID, nil, nil, &enabled); err != nil {
		t.Fatalf("启用失败: %v", err)
	}
	if p := PickAiChannelKey(testChannel); p == nil || p.ID != k.ID {
		t.Fatalf("启用后应能重新被选中")
	}
	if err := models.DB.Where("id = ?", k.ID).First(&got).Error; err == nil {
		if got.FailCount != 0 {
			t.Fatalf("重新启用后连续失败计数应清零，实际 %d", got.FailCount)
		}
		if got.FailTotal != int64(MaxFailBeforeDisable) {
			t.Fatalf("累计失败数不应被清零，实际 %d", got.FailTotal)
		}
	}
}

// 成功上报会把连续失败清零（避免偶发失败累积到熔断）
func TestReportAiChannelKey_SuccessResetsFailCount(t *testing.T) {
	setupChannelKeyTestDB(t)
	k, _ := AddAiChannelKey(testChannel, "卡A", "sk-good-0000000001", "", 1)
	ReportAiChannelKey(k.ID, false)
	ReportAiChannelKey(k.ID, true)
	var got AiChannelKey
	models.DB.Where("id = ?", k.ID).First(&got)
	if got.FailCount != 0 {
		t.Fatalf("成功后连续失败计数应归零，实际 %d", got.FailCount)
	}
	if got.SuccessTotal != 1 || got.FailTotal != 1 {
		t.Fatalf("累计计数应各为 1，实际 success=%d fail=%d", got.SuccessTotal, got.FailTotal)
	}
}

// 取密钥顺序：渠道池优先于模型自带密钥（老配置只作兜底）
func TestAcquireModelKey_PrefersChannelPool(t *testing.T) {
	setupChannelKeyTestDB(t)
	model := &ai.AiModel{Name: "m1", Kind: "chat", ModelID: "deepseek-chat", BaseURL: testChannel, Enabled: true}
	if err := ai.CreateAiModel(model, "sk-model-legacy-0001"); err != nil {
		t.Fatalf("建模型失败: %v", err)
	}
	// 只有模型密钥时，取到的是 model 来源
	lease, err := AcquireModelKey(model)
	if err != nil || lease.Source != "model" {
		t.Fatalf("无渠道卡密时应回退模型密钥，实际 source=%v err=%v", lease, err)
	}

	if _, err := AddAiChannelKey(testChannel, "渠道卡", "sk-channel-00000009", "", 1); err != nil {
		t.Fatalf("加渠道卡密失败: %v", err)
	}
	lease, err = AcquireModelKey(model)
	if err != nil {
		t.Fatalf("取密钥失败: %v", err)
	}
	if lease.Source != "channel_pool" {
		t.Fatalf("有渠道卡密时应优先用渠道池，实际 source=%s", lease.Source)
	}
	if lease.Key != "sk-channel-00000009" {
		t.Fatalf("取到的密钥不对: %s", lease.Key)
	}
	if lease.ChannelKeyID == 0 {
		t.Fatalf("应带回渠道卡密 id，便于上报成功/失败")
	}
}

// 迁移：把按模型存的密钥并入渠道池，且重复执行不会重复插入
func TestMigrateModelKeysToChannels_Idempotent(t *testing.T) {
	setupChannelKeyTestDB(t)
	m1 := &ai.AiModel{Name: "m1", Kind: "chat", ModelID: "a", BaseURL: testChannel, Enabled: true}
	m2 := &ai.AiModel{Name: "m2", Kind: "chat", ModelID: "b", BaseURL: testChannel, Enabled: true}
	if err := ai.CreateAiModel(m1, "sk-shared-key-000001"); err != nil {
		t.Fatalf("建模型失败: %v", err)
	}
	// 第二个模型用同一把 key（同一渠道下重复是常态）
	if err := ai.CreateAiModel(m2, "sk-shared-key-000001"); err != nil {
		t.Fatalf("建模型失败: %v", err)
	}
	// 再给 m1 加一把模型池密钥（新 key）
	if _, err := AddAiModelKey(m1.ID, "池内卡", "sk-pool-key-00000002", 1); err != nil {
		t.Fatalf("加模型密钥失败: %v", err)
	}

	n, err := MigrateModelKeysToChannels()
	if err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	if n != 2 {
		t.Fatalf("应新增 2 把（同渠道同 key 去重后），实际 %d", n)
	}
	list, _ := ListAiChannelKeys(testChannel)
	if len(list) != 2 {
		t.Fatalf("渠道池应有 2 把，实际 %d", len(list))
	}

	// 再跑一次：不应新增
	n2, err := MigrateModelKeysToChannels()
	if err != nil {
		t.Fatalf("第二次迁移失败: %v", err)
	}
	if n2 != 0 {
		t.Fatalf("迁移应幂等，第二次不应新增，实际 %d", n2)
	}
	if list2, _ := ListAiChannelKeys(testChannel); len(list2) != 2 {
		t.Fatalf("重复迁移后数量应不变，实际 %d", len(list2))
	}
}

// 统计按渠道聚合，一次查出所有渠道的卡密数量（避免 N+1）
func TestGetChannelKeyStats(t *testing.T) {
	setupChannelKeyTestDB(t)
	a, _ := AddAiChannelKey(testChannel, "a1", "sk-aaaaaaaaaa0001", "", 1)
	AddAiChannelKey(testChannel, "a2", "sk-aaaaaaaaaa0002", "", 1)
	AddAiChannelKey("https://b.example.com/v1", "b1", "sk-bbbbbbbbbb0001", "", 1)
	disabled := false
	UpdateAiChannelKey(a.ID, nil, nil, &disabled)

	stats := GetChannelKeyStats()
	if s := stats[testChannel]; s.Total != 2 || s.Enabled != 1 || s.Disabled != 1 {
		t.Fatalf("渠道 A 统计不对: %+v", s)
	}
	if s := stats["https://b.example.com/v1"]; s.Total != 1 || s.Enabled != 1 {
		t.Fatalf("渠道 B 统计不对: %+v", s)
	}
}

// ===== 合并自 ai_channel_account_test.go =====

// 加密入库的底线：库里不能出现明文，但必须能解回来
func TestChannelAccountEncryptedAtRest(t *testing.T) {
	setupAigcTestDB(t)

	const (
		napToken = "nap_VERY_SECRET_TOKEN_9f8e7d6c5b4a"
		skKey    = "sk-model-key-abcdef1234567890"
	)
	acc, err := SaveAiChannelAccount("https://api.example.com/v1", "示例", "56347", napToken, skKey)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if strings.Contains(acc.AccessTokenEnc, "VERY_SECRET") || strings.Contains(acc.AccessTokenEnc, "nap_") {
		t.Fatal("密文里出现了明文片段")
	}
	if strings.Contains(acc.ModelKeyEnc, "abcdef1234567890") {
		t.Fatal("模型密钥密文里出现了明文片段")
	}
	if acc.AccessTokenTail != "5b4a" {
		t.Fatalf("尾号应只保留 4 位：%q", acc.AccessTokenTail)
	}

	// 解回来必须一字不差
	got, err := acc.DecryptAccessToken()
	if err != nil || got != napToken {
		t.Fatalf("解密失败：%q %v", got, err)
	}
	mk, err := acc.DecryptModelKey()
	if err != nil || mk != skKey {
		t.Fatalf("模型密钥解密失败：%q %v", mk, err)
	}

	// 归一化：/v1 后缀要被剥成站点根
	if acc.BaseURL != "https://api.example.com" {
		t.Fatalf("base_url 未归一化：%q", acc.BaseURL)
	}
}

// 同一渠道重复保存 = 更新，不新增；留空表示不改
func TestChannelAccountUpsertAndPartialUpdate(t *testing.T) {
	setupAigcTestDB(t)

	if _, err := SaveAiChannelAccount("https://a.example.com", "第一次", "1", "nap_first", "sk_first"); err != nil {
		t.Fatalf("first: %v", err)
	}
	// 只改用户 ID，令牌留空
	acc2, err := SaveAiChannelAccount("https://a.example.com", "", "56347", "", "")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if acc2.UserID != "56347" {
		t.Fatalf("user_id 没更新：%q", acc2.UserID)
	}
	if acc2.AccessTokenEnc == "" {
		t.Fatal("留空不应清掉已存的令牌")
	}
	if t2, _ := acc2.DecryptAccessToken(); t2 != "nap_first" {
		t.Fatalf("令牌被意外改了：%q", t2)
	}

	list := ListAiChannelAccounts()
	if len(list) != 1 {
		t.Fatalf("重复保存应更新同一条，实际 %d 条", len(list))
	}

	// 传 "-" 显式清空
	acc3, err := SaveAiChannelAccount("https://a.example.com", "", "", "-", "")
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if acc3.HasAccessToken() {
		t.Fatalf("令牌应被清空：%q", acc3.AccessTokenEnc)
	}
	if acc3.AccessTokenTail != "" {
		t.Fatalf("清空后不应留尾号：%q", acc3.AccessTokenTail)
	}
}

// 没保存过的渠道返回明确错误（前端据此显示「未保存」）
func TestChannelAccountNotFound(t *testing.T) {
	setupAigcTestDB(t)
	if _, err := GetAiChannelAccount("https://none.example.com"); err == nil {
		t.Fatal("不存在的渠道应返回错误")
	}
}
