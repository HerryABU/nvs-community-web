package aigc

// ==================== 出口管道：合成 → 角标 → 编码 → 写 AIGC 字段 ====================
//
// 顺序不可交换（需求 §11）：写元数据必须是最后一步，之后不能有任何重编码。
//
// 这里的函数**不碰数据库、不碰 HTTP**：ProduceID 由调用方生成传进来，
// 写库也在调用方做。这样单测可以直接拿内存里的图片跑完整链路。

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"strings"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp" // 注册 WebP 解码器（编码仍走 jpeg/png）
)

// ExportRequest 一次导出的入参（全部来自前端参数，不含图片二进制）
type ExportRequest struct {
	Source []byte // 原图字节
	Mime   string // 原图 MIME（image/jpeg / image/png / image/webp）
	// SourceIsAI false = 用户上传的普通图片：跳过标识写入（FR-7.2）
	SourceIsAI bool

	// 裁剪区域（像素，相对原图）；任一项 <=0 表示不裁剪
	CropX, CropY, CropW, CropH int
	// 输出尺寸；<=0 表示保持裁剪后原尺寸
	TargetW, TargetH int
	// 输出格式 jpeg / png / webp（webp 无标准库编码器，会退回 jpeg 并在结果里说明）
	Format string

	// WaiveLabel 申请不含显式标识的版本（隐式字段仍无条件写入）
	WaiveLabel bool
	// Prompt 非空则作为「业务附加信息」写进独立命名空间
	Prompt string

	// ProduceID 内容编号（调用方生成，必须唯一）
	ProduceID string
	// ModelName 仅用于 ReservedCode1 复合格式，**不进法定字段语义**
	ModelName string
}

// ExportResult 管道输出
type ExportResult struct {
	Data []byte
	Mime string
	Ext  string

	Width, Height int
	FontSize      int // 角标实际字号（0 = 没画）
	Labeled       bool

	Fields         AIGCFields
	PromptAttached bool
	// SkippedXMP 原图里已有合规 AIGC 字段，按「只保留一份」跳过写入
	SkippedXMP bool
	// Note 需要告诉调用方的降级说明（如 webp 退回 jpeg）
	Note string
}

// ErrTooLarge 输出像素超过上限
var ErrTooLarge = errors.New("输出尺寸超出本站上限")

// ErrBadCrop 裁剪区域非法
var ErrBadCrop = errors.New("裁剪区域超出原图范围")

// Export 执行完整出口管道
func Export(req ExportRequest, cfg Config) (*ExportResult, error) {
	if len(req.Source) == 0 {
		return nil, errors.New("原图为空")
	}
	if cfg.MaxOutputPixels <= 0 {
		cfg.MaxOutputPixels = 40_000_000
	}

	// ── 0. 先用 DecodeConfig 探尺寸，别为了知道多大就把几百 MB 的图解开 ──
	srcCfg, srcFormat, err := image.DecodeConfig(bytes.NewReader(req.Source))
	if err != nil {
		return nil, fmt.Errorf("无法识别的图片格式：%w", err)
	}
	if srcCfg.Width*srcCfg.Height > cfg.MaxOutputPixels*4 {
		return nil, ErrTooLarge
	}

	img, err := imaging.Decode(bytes.NewReader(req.Source), imaging.AutoOrientation(true))
	if err != nil {
		return nil, fmt.Errorf("图片解码失败：%w", err)
	}
	origMime := mimeOfFormat(srcFormat, req.Mime)

	// ── 1. 裁剪 ──
	if req.CropW > 0 && req.CropH > 0 {
		b := img.Bounds()
		x, y := req.CropX, req.CropY
		// 允许前端传浮点取整误差：夹到合法区间，而不是直接报错让用户白试一次
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		if x >= b.Dx() || y >= b.Dy() {
			return nil, ErrBadCrop
		}
		w, h := req.CropW, req.CropH
		if x+w > b.Dx() {
			w = b.Dx() - x
		}
		if y+h > b.Dy() {
			h = b.Dy() - y
		}
		if w < 8 || h < 8 {
			return nil, ErrBadCrop
		}
		img = imaging.Crop(img, image.Rect(x, y, x+w, y+h))
	}

	// ── 2. 缩放到目标尺寸 ──
	if req.TargetW > 0 && req.TargetH > 0 {
		if req.TargetW*req.TargetH > cfg.MaxOutputPixels {
			return nil, ErrTooLarge
		}
		if img.Bounds().Dx() != req.TargetW || img.Bounds().Dy() != req.TargetH {
			img = imaging.Resize(img, req.TargetW, req.TargetH, imaging.Lanczos)
		}
	}

	out := &ExportResult{
		Width:  img.Bounds().Dx(),
		Height: img.Bounds().Dy(),
	}

	// ── 3. 烧显式角标（豁免 / 非 AI 图跳过） ──
	if cfg.Enabled && req.SourceIsAI && !req.WaiveLabel {
		labeled, lm, lerr := DrawLabel(img, cfg)
		if lerr != nil {
			return nil, lerr
		}
		img = labeled
		out.FontSize = lm.FontSize
		out.Labeled = true
	}

	// ── 4. 编码 ──
	format := normalizeFormat(req.Format)
	mime, ext, note := encoderFor(format)
	if note != "" {
		out.Note = note
	}
	encoded, err := encodeImage(img, ext)
	if err != nil {
		return nil, err
	}
	out.Data = encoded
	out.Mime = mime
	out.Ext = ext

	// ── 5. 写隐式字段（最后一步、无条件执行；用户上传图不写） ──
	if !cfg.Enabled || !req.SourceIsAI {
		out.Fields = AIGCFields{}
		return out, nil
	}
	if req.ProduceID == "" {
		return nil, errors.New("缺少 ProduceID")
	}

	// 幂等：原图已带合规 AIGC 字段就不再写第二份（国标：只保留一份）
	if HasAIGC(ExtractXMP(req.Source, origMime)) {
		out.SkippedXMP = true
		return out, nil
	}

	fields := AIGCFields{
		Label:             "1",
		ContentProducer:   cfg.ContentProducer,
		ProduceID:         req.ProduceID,
		ReservedCode1:     reservedCode1(req.ProduceID, cfg.ContentProducer, req.ModelName, cfg.ReservedIncludeModel),
		ContentPropagator: "",
		PropagateID:       "",
		ReservedCode2:     "",
	}
	packet := BuildXMP(fields, req.Prompt)
	final, werr := InjectXMP(out.Data, out.Mime, packet)
	if werr != nil {
		return nil, fmt.Errorf("写入 AIGC 元数据失败：%w", werr)
	}
	out.Data = final
	out.Fields = fields
	out.PromptAttached = strings.TrimSpace(req.Prompt) != ""
	return out, nil
}

// reservedCode1 生成侧预留位：默认写完整性摘要；站长要求随文件带模型标识时用复合格式。
//
// 国标没有「模型」字段，所以模型名只能塞进预留位，且明确告知站长这是扩展用法。
func reservedCode1(produceID, producer, model string, includeModel bool) string {
	sum := sha256.Sum256([]byte(produceID + producer))
	h := hex.EncodeToString(sum[:])[:32]
	if includeModel && strings.TrimSpace(model) != "" {
		return "h=" + h + ";m=" + sanitizeModelTag(model)
	}
	return h
}

// sanitizeModelTag 复合段里的模型标识只能保留安全字符，避免破坏 `;` 分隔与 XMP 语法
func sanitizeModelTag(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.' || r == '/':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('_')
		}
		if b.Len() >= 48 {
			break
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

func normalizeFormat(f string) string {
	switch strings.ToLower(strings.TrimSpace(f)) {
	case "png":
		return "png"
	case "webp", "image/webp":
		return "webp"
	case "jpg", "jpeg", "image/jpeg", "":
		return "jpeg"
	default:
		return "jpeg"
	}
}

// encoderFor 返回实际可用的编码器；WebP 没有标准库编码器，退回 JPEG 并说明
func encoderFor(format string) (mime, ext, note string) {
	switch format {
	case "png":
		return "image/png", "png", ""
	case "webp":
		return "image/jpeg", "jpg", "Go 标准库没有 WebP 编码器，本次已按 JPEG 输出（元数据照样写入）"
	default:
		return "image/jpeg", "jpg", ""
	}
}

func encodeImage(img image.Image, ext string) ([]byte, error) {
	var buf bytes.Buffer
	switch ext {
	case "png":
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
	default:
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func mimeOfFormat(format, hint string) string {
	switch strings.ToLower(format) {
	case "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	case "gif":
		return "image/gif"
	}
	if hint != "" {
		return hint
	}
	return "image/jpeg"
}
