package models

import (
	"strings"
	"testing"
)

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
