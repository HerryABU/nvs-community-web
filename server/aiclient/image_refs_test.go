package aiclient

import "testing"

// TestParseDataURL 参考图只接受 data URL（拒绝外链，防 SSRF）且限图片类型
func TestParseDataURL(t *testing.T) {
	if _, err := ParseDataURL("https://evil.example.com/a.png"); err == nil {
		t.Fatalf("外链参考图应被拒绝")
	}
	ok := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	ref, err := ParseDataURL(ok)
	if err != nil {
		t.Fatalf("合法 data URL 解析失败：%v", err)
	}
	if ref.Mime != "image/png" || len(ref.Data) == 0 {
		t.Fatalf("解析结果异常：%+v", ref)
	}
	if _, err := ParseDataURL("data:text/html;base64,PHNjcmlwdD4="); err == nil {
		t.Fatalf("非图片类型应被拒绝")
	}
	if _, err := ParseDataURL("data:image/png;base64,!!!notbase64"); err == nil {
		t.Fatalf("非法 base64 应被拒绝")
	}
	// 数量上限
	refs := []string{ok, ok, ok, ok}
	if _, err := ImageRefsFromDataURLs(refs); err == nil {
		t.Fatalf("超过 %d 张应被拒绝", MaxRefImages)
	}
}
