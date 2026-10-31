package aigc

// ==================== 显式标识（角标）烧录 ====================
//
// 国标要求：可感知的文字标识，位于边或角，**文字高度 ≥ 最短边 5%**，文案须同时含
// 「人工智能/AI」与「生成/合成」要素。
//
// 字体随二进制 go:embed 打进包里（部署环境不可控，不能指望系统字体），
// 并做 sync.Map 缓存 —— 每次请求重新解析 TTF 会慢一个数量级（NFR-1）。
//
// 内嵌字体是 Noto Sans SC 的**子集**（SIL OFL 1.1，见 assets/FONT-LICENSE.txt），
// 只保留角标可能用到的字符，12 KB；站长若要从后台换字体，可上传自有 TTF 覆盖。

import (
	_ "embed"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

//go:embed assets/LabelSansSC.ttf
var builtinFontData []byte

var (
	fontLoadOnce sync.Once
	fontParsed   *truetype.Font
	fontParseErr error
	// fontCustomPath 站长上传的字体（存在则优先，避免子集缺字）
	fontCustomPath string
	customOnce     sync.Once
	customFont     *truetype.Font
	// faceCache 字号 → font.Face（truetype.NewFace 每次会重新解析度量，缓存掉）
	faceCache sync.Map
)

// SetCustomFontPath 指定自定义字体文件（空字符串 = 用内置子集）。
// 由 handlers 在启动时按配置注入。
func SetCustomFontPath(p string) {
	fontCustomPath = p
	faceCache = sync.Map{}
	customOnce = sync.Once{}
	customFont = nil
}

func parseFont(data []byte, from string) (*truetype.Font, error) {
	f, err := truetype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("字体解析失败（%s）：%w", from, err)
	}
	return f, nil
}

func activeFont() (*truetype.Font, error) {
	if fontCustomPath != "" {
		customOnce.Do(func() {
			b, err := os.ReadFile(fontCustomPath)
			if err != nil {
				customFont = nil // 读不到就回退内置，不要让整条链路挂掉
				return
			}
			if f, err := parseFont(b, filepath.Base(fontCustomPath)); err == nil {
				customFont = f
			}
		})
		if customFont != nil {
			return customFont, nil
		}
	}
	fontLoadOnce.Do(func() {
		fontParsed, fontParseErr = parseFont(builtinFontData, "内置子集")
	})
	return fontParsed, fontParseErr
}

// FaceForSize 取指定字号的 font.Face（带缓存）
func FaceForSize(size float64) (font.Face, error) {
	f, err := activeFont()
	if err != nil {
		return nil, err
	}
	key := int(math.Round(size * 4)) // 0.25px 粒度足够，避免浮点 key 爆炸
	if v, ok := faceCache.Load(key); ok {
		return v.(font.Face), nil
	}
	face := truetype.NewFace(f, &truetype.Options{
		Size:    size,
		DPI:     72, // DPI=72 时 Size 就是像素高度，和国标「文字高度」口径一致
		Hinting: font.HintingFull,
	})
	faceCache.Store(key, face)
	return face, nil
}

// ErrFontUnavailable 字体不可用（内置子集损坏，属打包事故）
var ErrFontUnavailable = errors.New("标识字体不可用")

// LabelMetrics 角标绘制结果（写库 + 后台核验用）
type LabelMetrics struct {
	FontSize int // 实际字号（像素）
	Text     string
	Width    int // 图片宽
	Height   int // 图片高
}

// DrawLabel 在图上烧录显式角标，返回新图与度量信息。
// 入参 img 不会被修改（gg 会新建画布）。
func DrawLabel(img image.Image, cfg Config) (image.Image, LabelMetrics, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, LabelMetrics{}, errors.New("图片尺寸非法")
	}

	shortSide := math.Min(float64(w), float64(h))
	pct := cfg.FontSizePct
	if pct < 0.05 {
		pct = 0.05 // 国标下限，配置层已兜一次，这里再兜一次
	}
	fontSize := math.Max(14, float64(int(math.Round(shortSide*pct))))
	face, err := FaceForSize(fontSize)
	if err != nil {
		return nil, LabelMetrics{}, ErrFontUnavailable
	}

	dc := gg.NewContextForImage(img)
	dc.SetFontFace(face)

	text := cfg.LabelText
	if text == "" {
		text = "AI生成"
	}
	margin := fontSize * 0.8
	var ax, ay, anchorX, anchorY float64
	switch cfg.Position {
	case "bottom-left":
		ax, ay, anchorX, anchorY = margin, float64(h)-margin, 0, 1
	case "top-right":
		ax, ay, anchorX, anchorY = float64(w)-margin, margin, 1, 0
	case "top-left":
		ax, ay, anchorX, anchorY = margin, margin, 0, 0
	default: // bottom-right
		ax, ay, anchorX, anchorY = float64(w)-margin, float64(h)-margin, 1, 1
	}

	// 先用深色描一遍再压浅色正文：纯白字压在浅色图上会看不见（尤其是白底图）
	shadow := cfg.ShadowOpacity
	if shadow <= 0 {
		shadow = 0.3
	}
	dc.SetColor(color.RGBA{0, 0, 0, uint8(shadow * 255)})
	dc.DrawStringAnchored(text, ax+1.2, ay+1.2, anchorX, anchorY)

	r, g, bl := hexToRGB(cfg.TextColor)
	alpha := cfg.TextOpacity
	if alpha <= 0 {
		alpha = 0.85
	}
	dc.SetColor(color.RGBA{r, g, bl, uint8(alpha * 255)})
	dc.DrawStringAnchored(text, ax, ay, anchorX, anchorY)

	return dc.Image(), LabelMetrics{
		FontSize: int(math.Round(fontSize)),
		Text:     text,
		Width:    w,
		Height:   h,
	}, nil
}

func hexToRGB(s string) (uint8, uint8, uint8) {
	if len(s) == 7 && s[0] == '#' {
		if v, err := strconv.ParseUint(s[1:], 16, 32); err == nil {
			return uint8(v >> 16), uint8(v >> 8), uint8(v)
		}
	}
	return 255, 255, 255
}

// LabelFontHeight 用字体度量给出「实际字高」，用于自我核验是否 ≥ 短边 5%。
// gg 的 Size 是 em 尺寸，汉字实际墨迹高度会略小于它 —— 核验时按度量算更严谨。
func LabelFontHeight(size float64) (float64, error) {
	face, err := FaceForSize(size)
	if err != nil {
		return 0, ErrFontUnavailable
	}
	m := face.Metrics()
	// Ascent/Descent 是 26.6 定点数
	return float64(m.Ascent+m.Descent) / 64.0, nil
}
