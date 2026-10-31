package aigc

// Config 出口管道需要的配置（从 models.AigcConfig 映射而来）。
// 故意不复用 models 里的结构：aigc 包保持零业务依赖，单测里可以随便造配置。
type Config struct {
	// Enabled 总开关：关掉后不画角标、不写字段（私有部署自担责任时用）
	Enabled bool
	// LabelText 角标文案
	LabelText string
	// Position bottom-right / bottom-left / top-right / top-left
	Position string
	// FontSizePct 字号占最短边比例（≥0.05）
	FontSizePct float64
	// TextColor #RRGGBB
	TextColor string
	// TextOpacity 0~1
	TextOpacity float64
	// ShadowOpacity 0~1
	ShadowOpacity float64
	// ContentProducer 生成合成服务提供者名称 / TC260 编码
	ContentProducer string
	// ReservedIncludeModel 是否把模型标识写进 ReservedCode1 复合格式
	ReservedIncludeModel bool
	// MaxOutputPixels 像素上限
	MaxOutputPixels int
}

// DefaultConfig 缺省值（与 models.DefaultAigcConfig 保持一致）
func DefaultConfig() Config {
	return Config{
		Enabled:         true,
		LabelText:       "AI生成",
		Position:        "bottom-right",
		FontSizePct:     0.05,
		TextColor:       "#FFFFFF",
		TextOpacity:     0.85,
		ShadowOpacity:   0.30,
		ContentProducer: "本站",
		MaxOutputPixels: 40_000_000,
	}
}
