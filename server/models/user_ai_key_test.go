package models

import "testing"

func TestUserAiKeyCrud(t *testing.T) {
	setupAiTestDB(t)
	// 首个自动默认
	k1, err := CreateUserAiKey(1, "我的 DeepSeek", "https://api.deepseek.com/v1", "sk-test-123456", "deepseek-chat")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if !k1.IsDefault {
		t.Fatal("首个密钥应自动设为默认")
	}
	plain, derr := k1.DecryptApiKey()
	if derr != nil || plain != "sk-test-123456" {
		t.Fatalf("解密往返失败: plain=%q err=%v", plain, derr)
	}
	// 列表不回传密钥本体
	list, lerr := ListUserAiKeys(1)
	if lerr != nil || len(list) != 1 || list[0].KeyEnc == "" {
		t.Fatalf("列表异常: %+v err=%v", list, lerr)
	}
	// 越权读取：返回 nil
	other, _ := GetUserAiKey(k1.ID, 2)
	if other != nil {
		t.Fatal("越权读取应返回 nil")
	}
	// 第二个非默认
	k2, _ := CreateUserAiKey(1, "备用", "https://api.openai.com/v1", "sk-test-2", "")
	if k2.IsDefault {
		t.Fatal("第二个密钥不应为默认")
	}
	// 设默认：事务内先清零再置位
	if err := SetDefaultUserAiKey(k2.ID, 1); err != nil {
		t.Fatalf("设默认失败: %v", err)
	}
	reload, _ := GetUserAiKey(k1.ID, 1)
	if reload.IsDefault {
		t.Fatal("原默认应被取消")
	}
	// ResolveUserAiKey：不指定 → 默认；指定 → 指定
	r, _ := ResolveUserAiKey(1, 0)
	if r == nil || r.ID != k2.ID {
		t.Fatalf("应解析到默认密钥 k2，实际 %+v", r)
	}
	r2, _ := ResolveUserAiKey(1, k1.ID)
	if r2 == nil || r2.ID != k1.ID {
		t.Fatalf("应解析到指定密钥 k1，实际 %+v", r2)
	}
	// 删除默认后：剩余第一个提升为默认
	if err := DeleteUserAiKey(k2.ID, 1); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	r3, _ := ResolveUserAiKey(1, 0)
	if r3 == nil || r3.ID != k1.ID {
		t.Fatalf("删除默认后应提升 k1，实际 %+v", r3)
	}
}

func TestUserAiKeyLimit(t *testing.T) {
	setupAiTestDB(t)
	for i := 0; i < MaxUserAiKeys; i++ {
		if _, err := CreateUserAiKey(1, "k", "https://x.com/v1", "sk-abc", ""); err != nil {
			t.Fatalf("第 %d 个创建失败: %v", i+1, err)
		}
	}
	if _, err := CreateUserAiKey(1, "k", "https://x.com/v1", "sk-abc", ""); err == nil {
		t.Fatal("超过上限应报错")
	}
}
