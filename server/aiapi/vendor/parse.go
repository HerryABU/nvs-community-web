// Package vendor 厂商 / 模型版本解析工具（嵌套子包，非上帝类）。
//
// 模型 ID 的真实命名并不规整，各家上游各写各的。管理员需要的其实是把一串
// 模型名拆成**可读的四段**，例如：
//
//	deepseek-v4.1-flash-1017
//	  ├ family   deepseek   ← 模型大名（= 厂商/家族）
//	  ├ version  v4.1       ← 版本号（代际）
//	  ├ variant  flash      ← 类型名称（同代下的定位/档位）
//	  └ build    1017       ← 子号（构建号、日期号、迭代号）
//
// 为什么要单独拆 variant 和 build：老实现把「第三段之后」全部塞进 sub_version，
// 于是 `flash-1017` 糊成一坨，既没法按「类型」筛选，也没法按「子号」排序。
//
// 常见尾缀（variant 词表，按档位语义归组，非穷举；上游会不断新增）：
//
//	轻量/低价：mini, lite, flash, air, small, nano, tiny, fast
//	主力/标准：chat, instruct, base, default, standard, turbo
//	增强/高价：pro, plus, max, ultra, large, heavy, advanced, opus, sonnet
//	能力开关：thinking, reasoning, think, r1（推理）, vision（多模态）,
//	         audio（语音）, image（图像）, code（代码）, tool（工具调用）
//	时效标记：preview（预览版，随时变）, latest（滚动最新）, exp/experimental（实验）,
//	         beta, rc, stable, deprecated
//	计费变体：free（免费额度专用）, paid
//
// 尾部数字（build）通常是：构建号（1017）、发布日（0301 / 0613）、
// 或紧凑日期（20250101 / 20240806）。它们不改变模型能力，只用于区分快照。
package vendor

import (
	"regexp"
	"strconv"
	"strings"
)

// ParsedModelID 解析后的模型标识结构（四段式）
type ParsedModelID struct {
	Raw        string `json:"raw"`
	Prefix     string `json:"prefix"`      // 模型大名 / 家族（deepseek）
	Version    string `json:"version"`     // 版本号（v4.1）
	Variant    string `json:"variant"`     // 类型名称（flash / pro / thinking）
	Build      string `json:"build"`       // 子号（1017 / 20250101）
	SubVersion string `json:"sub_version"` // 兼容旧字段：variant + build 的合并形态
}

var dateLike = regexp.MustCompile(`(\d{4})[-/_]?(\d{2})[-/_]?(\d{2})`)

// versionLike 版本号：v4 / v4.1 / 4 / 4o / 3.5 / o1 里的数字形态
var versionLike = regexp.MustCompile(`^(v\d+(\.\d+)*|\d{1,3}(\.\d+)*|\d+o)$`)

// sizeLike 参数量/上下文量：7b / 70b / 72b / 128k / 1m —— 归入 build（不是版本）
var sizeLike = regexp.MustCompile(`^\d+(\.\d+)?[bkm]$`)

// buildLike 子号：纯数字（1017/0301）或紧凑日期（20250101）
var buildLike = regexp.MustCompile(`^\d{3,8}$`)

// familyWithVersion 家族名里内嵌版本：qwen2.5 / glm4 / yi1
var familyWithVersion = regexp.MustCompile(`^([A-Za-z][A-Za-z.\-]{0,}?)(\d+(\.\d+)*)$`)

// variantWords 类型名称词表（小写）。命中即视为 variant。
var variantWords = map[string]bool{
	// 轻量 / 低价
	"mini": true, "lite": true, "flash": true, "air": true, "small": true,
	"nano": true, "tiny": true, "fast": true, "speed": true, "eco": true,
	// 主力 / 标准
	"chat": true, "instruct": true, "base": true, "default": true,
	"standard": true, "turbo": true, "general": true,
	// 增强 / 高价
	"pro": true, "plus": true, "max": true, "ultra": true, "large": true,
	"heavy": true, "advanced": true, "opus": true, "sonnet": true, "haiku": true,
	"premier": true, "premium": true,
	// 能力开关
	"thinking": true, "reasoning": true, "reasoner": true, "think": true, "r1": true,
	"vision": true, "vl": true, "audio": true, "image": true, "code": true,
	"coder": true, "tool": true, "tools": true,
	// 时效标记
	"preview": true, "latest": true, "exp": true, "experimental": true,
	"beta": true, "rc": true, "stable": true, "deprecated": true,
	// 计费变体
	"free": true, "paid": true,
}

// ParseModelID 解析模型 ID 为「家族 - 版本 - 类型 - 子号」四段。
//
// 策略（顺序很重要）：
//  1. 归一分隔符（_ / → -），去掉尾部 :free / :beta 这类冒号后缀（归入 variant）；
//  2. 逐段扫描：命中 variantWords → variant；形如版本号 → version；
//     纯数字/日期 → build；其余首段 → family，后续非数字 → 依次补到 family（连字符厂商名，
//     如 text-embedding-3-large）；
//  3. 兜底：仍有剩余段则并入 build（保持信息不丢）。
func ParseModelID(raw string) ParsedModelID {
	s := strings.TrimSpace(raw)
	lowerAll := strings.ToLower(s)

	// 冒号后缀（OpenRouter 风格 :free / :online）单独摘出，视为 variant 补充
	colonSuffix := ""
	if i := strings.LastIndex(s, ":"); i > 0 {
		colonSuffix = strings.ToLower(strings.TrimSpace(s[i+1:]))
		s = s[:i]
	}

	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.Trim(s, "-")
	parts := strings.Split(s, "-")
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			clean = append(clean, p)
		}
	}

	out := ParsedModelID{Raw: raw}
	if len(clean) == 0 {
		return out
	}

	// 日期合并：2024 / 08 / 06 拼回 2024-08-06（否则会被拆成版本和子号）
	merged := make([]string, 0, len(clean))
	for i := 0; i < len(clean); i++ {
		if i+2 < len(clean) {
			y, e1 := strconv.Atoi(clean[i])
			_, e2 := strconv.Atoi(clean[i+1])
			_, e3 := strconv.Atoi(clean[i+2])
			if e1 == nil && e2 == nil && e3 == nil && len(clean[i]) == 4 && (y >= 1990 && y <= 2099) {
				merged = append(merged, clean[i]+"-"+clean[i+1]+"-"+clean[i+2])
				i += 2
				continue
			}
		}
		merged = append(merged, clean[i])
	}

	family := make([]string, 0, 3)
	version := make([]string, 0, 2)
	variant := make([]string, 0, 2)
	build := make([]string, 0, 2)

	// 1) 家族：吃掉所有「不是版本/不是参数量/不是类型词」的段（支持 text-embedding 这种多段家族）
	i := 0
	for i < len(merged) {
		lp := strings.ToLower(merged[i])
		if versionLike.MatchString(lp) || sizeLike.MatchString(lp) || variantWords[lp] {
			break
		}
		family = append(family, merged[i])
		i++
	}

	// 1.5) 家族名内嵌版本（qwen2.5 → qwen + 2.5）
	if len(family) == 1 && len(family[0]) >= 3 {
		if m := familyWithVersion.FindStringSubmatch(family[0]); m != nil && len(m[1]) >= 2 {
			family = []string{m[1]}
			version = append(version, m[2])
		}
	}

	// 2) 版本：连续版本号合并（claude-3-5 → 3-5）
	for i < len(merged) {
		lp := strings.ToLower(merged[i])
		// 参数量（72b/128k）不是版本，交给下面的 build
		if sizeLike.MatchString(lp) || !versionLike.MatchString(lp) {
			break
		}
		version = append(version, merged[i])
		i++
	}

	// 3) 余下：类型词 → variant；参数量/纯数字 → build；都不像则优先补 variant
	tailStart := i // 记住版本之后的起点：sub_version 要按**原始顺序**回放，保持向后兼容
	for ; i < len(merged); i++ {
		t := merged[i]
		lp := strings.ToLower(t)
		switch {
		case variantWords[lp]:
			variant = append(variant, t)
		case sizeLike.MatchString(lp), buildLike.MatchString(lp), dateLike.MatchString(t):
			build = append(build, t)
		case isNumeric(t) && len(variant) > 0:
			build = append(build, t)
		default:
			if len(variant) == 0 {
				variant = append(variant, t)
			} else {
				build = append(build, t)
			}
		}
	}

	if colonSuffix != "" {
		variant = append(variant, colonSuffix)
	}

	out.Prefix = strings.Join(family, "-")
	out.Version = strings.Join(version, "-")
	out.Variant = strings.Join(variant, "-")
	out.Build = strings.Join(build, "-")
	// 兼容旧字段：版本之后的原始尾段（保持原有书写顺序，不因 variant/build 重排而变）
	out.SubVersion = strings.Trim(strings.Join(merged[tailStart:], "-"), "-")

	out.Raw = raw
	if out.Prefix == "" {
		out.Prefix = strings.ToLower(lowerAll)
	}
	return out
}

// isNumeric 纯数字判断（用于「已出现类型词后的尾部数字视为子号」）
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ─────────────────────────────────────────────────────────────
// 亚版本新颖度（同版本下「谁更新」的排序参考；管理端展示用）
// ─────────────────────────────────────────────────────────────

// subTier 亚版本档位词表（越高越新/越强；仅作排序参考）
var subTier = map[string]int{
	"max": 60, "pro": 55, "turbo": 50, "plus": 48, "ultra": 46,
	"mini": 30, "nano": 28, "lite": 26, "base": 20, "light": 22,
}

// SubVersionFreshness 返回亚版本「新颖度」得分，越高越新。
// 优先级：可解析日期（YYYYMMDD / YYMMDD / MMDD）→ 数值最大；
// 否则取尾部数字（小版本号）；再否则按词表档位；都没有给中性值。
func SubVersionFreshness(sub string) int {
	s := strings.ToLower(strings.TrimSpace(sub))
	if s == "" {
		return 0
	}
	// 1) 日期型（如 0801 / 20240101 / 0115）
	if m := dateLike.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		if y < 100 {
			y += 2000
		}
		return y*10000 + mo*100 + d + 1_000_000
	}
	// 2) 尾部纯数字（小版本号，如 -0125 / -0706）
	if n, err := strconv.Atoi(s); err == nil {
		return 500_000 + n
	}
	// 3) 含数字后缀（如 turbo-0125）
	if idx := strings.LastIndexAny(s, "-_"); idx >= 0 {
		if n, err := strconv.Atoi(s[idx+1:]); err == nil {
			return 500_000 + n
		}
	}
	// 4) 词表档位
	if t, ok := subTier[s]; ok {
		return t
	}
	// 5) 含词表片段
	for k, v := range subTier {
		if strings.Contains(s, k) {
			return v
		}
	}
	return 10 // 中性兜底
}

// NewestSubVersion 在一组亚版本里挑出「最新」的那个（并列取字典序较大者）。
func NewestSubVersion(subs []string) string {
	best := ""
	bestScore := -1
	for _, s := range subs {
		if s == "" {
			continue
		}
		score := SubVersionFreshness(s)
		if score > bestScore || (score == bestScore && s > best) {
			best = s
			bestScore = score
		}
	}
	return best
}
