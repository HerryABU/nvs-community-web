package aigc

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func solidJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{30, 40, 60, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func solidPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{200, 210, 220, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// 七字段必须一字不差地往返
func TestXMPRoundTrip(t *testing.T) {
	f := AIGCFields{
		Label:           "1",
		ContentProducer: "示例站点 <测试&Co>",
		ProduceID:       "0b9a1c1e-1111-2222-3333-444455556666",
		ReservedCode1:   "abcdef0123456789abcdef0123456789",
	}
	packet := BuildXMP(f, "")
	got := ReadAIGCFields(packet)
	if got.Label != "1" {
		t.Fatalf("Label = %q", got.Label)
	}
	if got.ContentProducer != f.ContentProducer {
		t.Fatalf("ContentProducer 往返失真：%q", got.ContentProducer)
	}
	if got.ProduceID != f.ProduceID {
		t.Fatalf("ProduceID = %q", got.ProduceID)
	}
	if got.ContentPropagator != "" || got.PropagateID != "" || got.ReservedCode2 != "" {
		t.Fatalf("传播侧字段应为空：%+v", got)
	}
	if !HasAIGC(packet) {
		t.Fatal("HasAIGC 应判为已标识")
	}
}

// XML 特殊字符必须被转义，否则核验方解析会直接失败
func TestXMPEscapesXML(t *testing.T) {
	packet := string(BuildXMP(AIGCFields{Label: "1", ProduceID: "x", ContentProducer: `a"b&c<d`}, ""))
	if strings.Contains(packet, `a"b`) {
		t.Fatalf("属性里的双引号没转义：%s", packet)
	}
	if !strings.Contains(packet, "&amp;") || !strings.Contains(packet, "&lt;") {
		t.Fatalf("XML 实体缺失：%s", packet)
	}
}

// JPEG：注入后仍能被正常解码，且只保留一份 xap 段（国标「仅保留一份」）
func TestInjectJPEGSingleXMP(t *testing.T) {
	src := solidJPEG(t, 64, 64)
	packet := BuildXMP(AIGCFields{Label: "1", ProduceID: "pid-1", ContentProducer: "站点"}, "")

	out, err := InjectXMP(src, "image/jpeg", packet)
	if err != nil {
		t.Fatalf("inject: %v", err)
	}
	if n := bytes.Count(out, []byte(xmpApp1Header)); n != 1 {
		t.Fatalf("xap 段数量 = %d，应为 1", n)
	}
	if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("注入后 JPEG 解不开：%v", err)
	}
	if got := ReadAIGCFields(ExtractXMP(out, "image/jpeg")); got.ProduceID != "pid-1" {
		t.Fatalf("读回的 ProduceID = %q", got.ProduceID)
	}

	// 再注入一次：应替换而不是追加
	out2, err := InjectXMP(out, "image/jpeg", BuildXMP(AIGCFields{Label: "1", ProduceID: "pid-2", ContentProducer: "站点"}, ""))
	if err != nil {
		t.Fatalf("inject2: %v", err)
	}
	if n := bytes.Count(out2, []byte(xmpApp1Header)); n != 1 {
		t.Fatalf("二次注入后 xap 段数量 = %d，应为 1", n)
	}
	if got := ReadAIGCFields(ExtractXMP(out2, "image/jpeg")); got.ProduceID != "pid-2" {
		t.Fatalf("二次注入应覆盖，读回 %q", got.ProduceID)
	}
}

// PNG：iTXt 的 CRC 必须正确，否则解码器会报错
func TestInjectPNGKeepsImageValid(t *testing.T) {
	src := solidPNG(t, 40, 30)
	packet := BuildXMP(AIGCFields{Label: "1", ProduceID: "png-1", ContentProducer: "站点"}, "")
	out, err := InjectXMP(src, "image/png", packet)
	if err != nil {
		t.Fatalf("inject png: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("注入后 PNG 解不开（CRC 错？）：%v", err)
	}
	if img.Bounds().Dx() != 40 || img.Bounds().Dy() != 30 {
		t.Fatalf("尺寸被改了：%v", img.Bounds())
	}
	if got := ReadAIGCFields(ExtractXMP(out, "image/png")); got.ProduceID != "png-1" {
		t.Fatalf("读回失败：%+v", got)
	}
}

// 提示词必须落在独立命名空间，且不污染法定七字段
func TestXMPSeparatesPromptNamespace(t *testing.T) {
	packet := BuildXMP(AIGCFields{Label: "1", ProduceID: "p", ContentProducer: "s"}, "一只赛博猫")
	s := string(packet)
	if !strings.Contains(s, PromptNamespace) {
		t.Fatal("提示词命名空间缺失")
	}
	if !strings.Contains(s, "一只赛博猫") {
		t.Fatal("提示词内容缺失")
	}
	// 法定描述块里不应出现提示词
	firstBlock := s[:strings.Index(s, PromptNamespace)]
	if strings.Contains(firstBlock, "一只赛博猫") {
		t.Fatal("提示词混进了法定字段块")
	}
}

// 空 ProduceID 不算已标识（避免半成品被当成合规）
func TestHasAIGCFalseOnEmptyProduceID(t *testing.T) {
	packet := []byte(`<rdf:Description AIGC:Label="1" AIGC:ProduceID=""/>`)
	if HasAIGC(packet) {
		t.Fatal("空 ProduceID 不应判为已标识")
	}
	if HasAIGC(nil) {
		t.Fatal("空 packet 不应判为已标识")
	}
}
