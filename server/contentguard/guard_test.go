package contentguard

import (
	"strings"
	"testing"

	"nvs-server/security"
)

func init() {
	// 注入测试违禁词（与运行环境的词表无关，保证用例确定性）
	security.SetDynamicBadWords([]security.DynamicBadWord{
		{ID: 999001, Name: "测试赌博词", Category: "赌博",
			MatchType: "literal", Pattern: "赌博", Action: "block", Scope: "all"},
		{ID: 999002, Name: "测试赌博平台", Category: "赌博",
			MatchType: "literal", Pattern: "澳门线上娱乐", Action: "block", Scope: "all"},
	})
}

func TestDecodeLayers_Base64(t *testing.T) {
	res := security.DecodeLayers("6LWM5Y2a", 5)
	if res.Depth != 1 || res.Layers[0] != "base64" {
		t.Fatalf("expected 1 layer base64, got %+v", res)
	}
	if res.Text != "赌博" {
		t.Fatalf("expected 赌博, got %q", res.Text)
	}
}

func TestDecodeLayers_Multi(t *testing.T) {
	res := security.DecodeLayers("NkxXTTVZMmE=", 5)
	if res.Depth < 2 {
		t.Fatalf("expected >=2 layers, got %+v", res)
	}
	if res.Text != "赌博" {
		t.Fatalf("expected 赌博, got %q", res.Text)
	}
}

func TestDecodeLayers_Unicode(t *testing.T) {
	res := security.DecodeLayers(`\u8d4c\u535a`, 5)
	if res.Depth < 1 || res.Text != "赌博" {
		t.Fatalf("expected 赌博, got %+v", res)
	}
}

func TestDecodeLayers_URL(t *testing.T) {
	res := security.DecodeLayers("%E8%B5%8C%E5%8D%9A", 5)
	if res.Depth < 1 || res.Text != "赌博" {
		t.Fatalf("expected 赌博, got %+v", res)
	}
}

func TestLooksEncoded_Normal(t *testing.T) {
	if security.LooksEncoded("今天天气不错，适合散步。") {
		t.Fatal("normal Chinese text should not look encoded")
	}
}

func TestCaesarShift(t *testing.T) {
	if got := security.CaesarShift("hello", 13); got != "uryyb" {
		t.Fatalf("rot13 failed: %q", got)
	}
	best, score := security.TryCaesar("uryyb")
	if best != "hello" || score <= security.ReadabilityScore("uryyb") {
		t.Fatalf("TryCaesar failed: best=%q score=%d", best, score)
	}
}

func TestGuardText_EncodedBadWord_Block(t *testing.T) {
	d := GuardText("6LWM5Y2a", "comment")
	if d.Action != "block" {
		t.Fatalf("expected block, got %+v", d)
	}
	if d.Decoded != "赌博" {
		t.Fatalf("decoded=%q", d.Decoded)
	}
}

func TestGuardText_PII_Review(t *testing.T) {
	d := GuardText("我的邮箱是 test@example.com，手机号 13812345678", "comment")
	if d.Action != "review" {
		t.Fatalf("expected review, got %+v", d)
	}
	if len(d.PIITypes) == 0 {
		t.Fatal("expected pii types")
	}
}

func TestGuardText_APIKey(t *testing.T) {
	d := GuardText("api_key=sk-abcdefghijklmnop123456", "comment")
	if len(d.PIITypes) == 0 {
		t.Fatalf("expected apikey detection, got %+v", d)
	}
}

func TestGuardText_RiskyLink(t *testing.T) {
	d := GuardText("快看这个 https://bit.ly/abc123 有福利", "comment")
	if len(d.RiskyLinks) == 0 {
		t.Fatalf("expected risky link, got %+v", d)
	}
	d2 := GuardText("下载地址 http://192.168.1.1/x.exe", "comment")
	if len(d2.RiskyLinks) == 0 {
		t.Fatalf("expected ip link risky, got %+v", d2)
	}
}

func TestGuardText_Normal(t *testing.T) {
	d := GuardText("这本书写得真好，人物刻画很细腻，推荐大家阅读。", "comment")
	if d.Action != "allow" {
		t.Fatalf("expected allow, got %+v", d)
	}
}

func TestDecodeLayers_EmbeddedBase64(t *testing.T) {
	res := security.DecodeLayers("内部渠道 5r6z6Zeo57q/5LiK5aix5LmQ 快来注册", 5)
	if !strings.Contains(res.Text, "澳门线上娱乐") {
		t.Fatalf("expected embedded base64 decoded, got %q", res.Text)
	}
}

func TestGuardText_EmbeddedEncodedBadWord_Block(t *testing.T) {
	d := GuardText("内部渠道 5r6z6Zeo57q/5LiK5aix5LmQ 快来注册", "forum")
	if d.Action != "block" {
		t.Fatalf("expected block, got %+v", d)
	}
}

func TestGuardText_SuspiciousMultiLayer(t *testing.T) {
	// 多层编码 + 无法还原 → block
	d := GuardText("NkxXTTVZMmE=", "comment")
	if d.Action != "block" {
		t.Fatalf("expected block for multi-layer, got %+v", d)
	}
}
