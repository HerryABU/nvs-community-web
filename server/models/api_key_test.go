package models

// API Key 模型纯函数单元测试（不依赖 DB）

import (
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	full, prefix, hash, err := GenerateAPIKey("reader")
	if err != nil {
		t.Fatal(err)
	}
	if len(full) != len("nvs_rdr_")+32 {
		t.Fatalf("Key 长度异常: %d", len(full))
	}
	if prefix[:len("nvs_rdr_")] != "nvs_rdr_" {
		t.Fatalf("前缀错误: %s", prefix)
	}
	if HashAPIKey(full) != hash {
		t.Fatal("哈希不一致")
	}
	// 不同角色前缀
	if p, _, _, _ := GenerateAPIKey("author"); p[:len("nvs_aut_")] != "nvs_aut_" {
		t.Fatal("author 前缀错误")
	}
	if p, _, _, _ := GenerateAPIKey("admin"); p[:len("nvs_adm_")] != "nvs_adm_" {
		t.Fatal("admin 前缀错误")
	}
	if p, _, _, _ := GenerateFederatedKey(); p[:len("nvs_fed_")] != "nvs_fed_" {
		t.Fatal("federated 前缀错误")
	}
}

func TestSanitizeQuotaConfig(t *testing.T) {
	def := DefaultAPIRateLimitConfig()
	// 零值补全
	cfg := APIRateLimitConfig{}
	cfg = sanitizeQuotaConfig(cfg, def)
	if cfg.Admin.Req != def.Admin.Req || cfg.Reader.Req != def.Reader.Req || cfg.Bot.Req != def.Bot.Req {
		t.Fatal("零值应回退默认配额")
	}
	// 超上限夹取
	cfg.Reader.Req = 99999999
	cfg = sanitizeQuotaConfig(cfg, def)
	if cfg.Reader.Req > 1000000 {
		t.Fatal("配额应被夹取到上限内")
	}
}

func TestRoleQuotaFor(t *testing.T) {
	cfg := DefaultAPIRateLimitConfig()
	if cfg.RoleQuotaFor("admin").Req != cfg.Admin.Req {
		t.Fatal("admin 配额错误")
	}
	if cfg.RoleQuotaFor("author").Req != cfg.Author.Req {
		t.Fatal("author 配额错误")
	}
	if cfg.RoleQuotaFor("reader").Req != cfg.Reader.Req {
		t.Fatal("reader 配额错误")
	}
	if cfg.RoleQuotaFor("unknown").Req != cfg.Reader.Req {
		t.Fatal("未知角色应回退 reader 配额")
	}
}

func TestMatchAllowedIP(t *testing.T) {
	// 空白名单 = 放行
	if !MatchAllowedIP("", "1.2.3.4") {
		t.Fatal("空白名单应放行")
	}
	// 精确 IP
	if !MatchAllowedIP("1.2.3.4, 5.6.7.8", "5.6.7.8") {
		t.Fatal("应命中精确 IP")
	}
	if MatchAllowedIP("1.2.3.4", "9.9.9.9") {
		t.Fatal("不应命中未列入 IP")
	}
	// CIDR
	if !MatchAllowedIP("10.0.0.0/8", "10.1.2.3") {
		t.Fatal("应命中 CIDR 网段")
	}
	if MatchAllowedIP("10.0.0.0/8", "11.1.2.3") {
		t.Fatal("不应命中网段外 IP")
	}
}
