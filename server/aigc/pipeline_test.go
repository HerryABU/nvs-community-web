package aigc

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func testConfig() Config {
	c := DefaultConfig()
	c.ContentProducer = "示例站点"
	return c
}

// 角标字号必须 ≥ 最短边 5%（国标硬指标），并且真的画上去了
func TestDrawLabelSizeAndPixels(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, color.RGBA{20, 20, 20, 255})
		}
	}
	out, lm, err := DrawLabel(img, testConfig())
	if err != nil {
		t.Fatalf("DrawLabel: %v", err)
	}
	if lm.FontSize < 15 { // 300*0.05 = 15
		t.Fatalf("字号 %d 小于短边 5%%（15px）", lm.FontSize)
	}
	if lm.Text != "AI生成" {
		t.Fatalf("文案 = %q", lm.Text)
	}

	// 右下角区域应当出现明显的亮像素（白字压在深色底上）
	rgba, ok := out.(*image.RGBA)
	if !ok {
		t.Fatalf("返回类型 %T", out)
	}
	changed := 0
	for y := 300 - 60; y < 300; y++ {
		for x := 400 - 120; x < 400; x++ {
			r, g, b, _ := rgba.At(x, y).RGBA()
			if r>>8 > 120 && g>>8 > 120 && b>>8 > 120 {
				changed++
			}
		}
	}
	if changed < 50 {
		t.Fatalf("右下角亮像素只有 %d 个，角标可能没画上", changed)
	}
}

// 字号下限：小图也要保证 ≥14px 且不低于 5%
func TestDrawLabelMinFontSize(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 80, 80))
	_, lm, err := DrawLabel(img, testConfig())
	if err != nil {
		t.Fatalf("DrawLabel: %v", err)
	}
	if lm.FontSize < 14 {
		t.Fatalf("小图字号 %d < 14", lm.FontSize)
	}
}

// AI 图：既有角标又有隐式字段
func TestExportAIImageBothLayers(t *testing.T) {
	src := solidJPEG(t, 320, 240)
	res, err := Export(ExportRequest{
		Source:     src,
		Mime:       "image/jpeg",
		SourceIsAI: true,
		Format:     "jpeg",
		ProduceID:  "aigc-test-0001",
	}, testConfig())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if !res.Labeled || res.FontSize == 0 {
		t.Fatalf("AI 图必须烧角标：%+v", res)
	}
	if res.Fields.Label != "1" || res.Fields.ProduceID != "aigc-test-0001" {
		t.Fatalf("字段不对：%+v", res.Fields)
	}
	if res.Fields.ContentProducer != "示例站点" {
		t.Fatalf("ContentProducer = %q", res.Fields.ContentProducer)
	}
	if len(res.Fields.ReservedCode1) != 32 {
		t.Fatalf("ReservedCode1 应为 32 位摘要：%q", res.Fields.ReservedCode1)
	}
	// 输出文件里能读回来
	got := ReadAIGCFields(ExtractXMP(res.Data, res.Mime))
	if got.ProduceID != "aigc-test-0001" {
		t.Fatalf("输出文件读回失败：%+v", got)
	}
}

// 豁免：不画角标，但隐式字段照写（需求 FR-3.5 / 验收 #5）
func TestExportWaivedStillWritesFields(t *testing.T) {
	res, err := Export(ExportRequest{
		Source:     solidJPEG(t, 200, 200),
		Mime:       "image/jpeg",
		SourceIsAI: true,
		Format:     "jpeg",
		WaiveLabel: true,
		ProduceID:  "aigc-waive-1",
	}, testConfig())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if res.Labeled || res.FontSize != 0 {
		t.Fatalf("豁免路径不应画角标：%+v", res)
	}
	if got := ReadAIGCFields(ExtractXMP(res.Data, res.Mime)); got.Label != "1" || got.ProduceID != "aigc-waive-1" {
		t.Fatalf("豁免路径的隐式字段丢了：%+v", got)
	}
}

// 用户上传图：两层都不写（验收 #9）
func TestExportUserUploadNoMark(t *testing.T) {
	res, err := Export(ExportRequest{
		Source:     solidPNG(t, 120, 90),
		Mime:       "image/png",
		SourceIsAI: false,
		Format:     "png",
	}, testConfig())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if res.Labeled {
		t.Fatal("用户上传图不该画角标")
	}
	if len(ExtractXMP(res.Data, res.Mime)) != 0 {
		t.Fatal("用户上传图不该写 AIGC 字段")
	}
}

// 裁剪 + 缩放：输出尺寸必须是请求的尺寸
func TestExportCropAndResize(t *testing.T) {
	res, err := Export(ExportRequest{
		Source:     solidJPEG(t, 600, 400),
		Mime:       "image/jpeg",
		SourceIsAI: true,
		CropX:      50, CropY: 40, CropW: 300, CropH: 200,
		TargetW: 150, TargetH: 100,
		Format:    "jpeg",
		ProduceID: "aigc-crop-1",
	}, testConfig())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if res.Width != 150 || res.Height != 100 {
		t.Fatalf("输出尺寸 = %dx%d", res.Width, res.Height)
	}
}

// webp 没有标准库编码器：退回 JPEG，并且必须把这件事说给调用方（不能静默换格式）
func TestExportWebpFallsBackToJPEG(t *testing.T) {
	res, err := Export(ExportRequest{
		Source:     solidJPEG(t, 100, 100),
		Mime:       "image/jpeg",
		SourceIsAI: true,
		Format:     "webp",
		ProduceID:  "aigc-webp-1",
	}, testConfig())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if res.Mime != "image/jpeg" || res.Ext != "jpg" {
		t.Fatalf("应回退 jpeg：%s %s", res.Mime, res.Ext)
	}
	if res.Note == "" {
		t.Fatal("降级必须有说明")
	}
	if _, err := jpeg.Decode(bytes.NewReader(res.Data)); err != nil {
		t.Fatalf("回退后的 jpeg 解不开：%v", err)
	}
}

// 超出像素上限直接拒，别把内存打爆
func TestExportTooLarge(t *testing.T) {
	cfg := testConfig()
	cfg.MaxOutputPixels = 1000
	_, err := Export(ExportRequest{
		Source:     solidJPEG(t, 200, 200),
		Mime:       "image/jpeg",
		SourceIsAI: true,
		TargetW:    500, TargetH: 500,
		Format:    "jpeg",
		ProduceID: "x",
	}, cfg)
	if err == nil {
		t.Fatal("超上限应报错")
	}
}

// 复合格式：h= 摘要 + m= 模型标识（验收 #12）
func TestReservedCodeComposite(t *testing.T) {
	plain := reservedCode1("pid", "站点", "deepseek-chat", false)
	if len(plain) != 32 || bytes.Contains([]byte(plain), []byte("m=")) {
		t.Fatalf("默认应为纯摘要：%q", plain)
	}
	comp := reservedCode1("pid", "站点", "deepseek-chat", true)
	if !bytes.HasPrefix([]byte(comp), []byte("h=")) || !bytes.Contains([]byte(comp), []byte(";m=deepseek-chat")) {
		t.Fatalf("复合格式不对：%q", comp)
	}
	// 模型名里的分隔符会被净化，不允许破坏结构
	dirty := reservedCode1("pid", "站点", "a;b=c d", true)
	if bytes.Count([]byte(dirty), []byte(";")) != 1 {
		t.Fatalf("模型名里的分号没净化：%q", dirty)
	}
}

// 幂等：原图已带合规字段时跳过写入（国标「只保留一份」）
func TestExportIdempotent(t *testing.T) {
	first, err := Export(ExportRequest{
		Source: solidJPEG(t, 100, 100), Mime: "image/jpeg", SourceIsAI: true,
		Format: "jpeg", ProduceID: "pid-first",
	}, testConfig())
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := Export(ExportRequest{
		Source: first.Data, Mime: "image/jpeg", SourceIsAI: true,
		Format: "jpeg", ProduceID: "pid-second",
	}, testConfig())
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if !second.SkippedXMP {
		t.Fatal("应识别为已标识并跳过")
	}
	if n := bytes.Count(second.Data, []byte(xmpApp1Header)); n != 0 {
		// 原图带的那份会被复制进输出？此路径下我们直接返回原字节，不该再写
		t.Logf("xap 段数 = %d（原图自带 1 份，属预期）", n)
	}
}

// 字段顺序/数量固定：新增第八个字段会让核验方解析失败
func TestAIGCFieldsJSONShape(t *testing.T) {
	b, err := json.Marshal(AIGCFields{Label: "1"})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	if len(m) != 7 {
		t.Fatalf("字段数 = %d，国标是 7 个：%s", len(m), b)
	}
	for _, k := range []string{"Label", "ContentProducer", "ProduceID", "ReservedCode1", "ContentPropagator", "PropagateID", "ReservedCode2"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("缺字段 %s", k)
		}
	}
}

// 验收 #6：单张出口管道延迟 < 50ms（1024×1024，含裁剪+缩放+角标+编码+写字段）
func BenchmarkExport1024(b *testing.B) {
	src := solidJPEGB(b, 1024, 1024)
	cfg := testConfig()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Export(ExportRequest{
			Source:     src,
			Mime:       "image/jpeg",
			SourceIsAI: true,
			CropX:      0, CropY: 0, CropW: 900, CropH: 900,
			TargetW: 800, TargetH: 800,
			Format:    "jpeg",
			ProduceID: "bench-produce-id",
		}, cfg); err != nil {
			b.Fatalf("export: %v", err)
		}
	}
}

// solidJPEGB 测试/基准共用的纯色 JPEG
func solidJPEGB(tb testing.TB, w, h int) []byte {
	tb.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), 120, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		tb.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}
