// Package vendor 厂商 / 模型版本解析工具（嵌套子包，非上帝类）。
//
// 模型 ID 约定（管理员可维护前缀）：前缀-版本-亚版本，例如 deepseek-v3.1-turbo
//   - 前缀 prefix   = deepseek   → 对应 AiVendor.Prefix（厂商主名字）
//   - 版本 version  = v3.1       → 中间段（同一厂商的代际，如 v2 / v3 / v3.1）
//   - 亚版本 sub     = turbo      → 末尾段（同版本下的微调/渠道变体）
//
// 同 (厂商, 版本) 下比较亚版本「谁最新」仅用于管理端展示（newest 标记），
// 用户侧不可见亚版本噪声，只看到厂商主名字（+ 版本）。
package vendor

import (
	"regexp"
	"strconv"
	"strings"
)

// ParsedModelID 解析后的模型标识结构
type ParsedModelID struct {
	Raw        string `json:"raw"`
	Prefix     string `json:"prefix"`
	Version    string `json:"version"`
	SubVersion string `json:"sub_version"`
}

var dateLike = regexp.MustCompile(`(\d{4})[-/_]?(\d{2})[-/_]?(\d{2})`)

// ParseModelID 解析「前缀-版本-亚版本」结构。
// 以 '-'（兼容 '/”_' 归一）切分：第一段=前缀，第二段=版本，其余合并=亚版本。
// 仅单段时整串作前缀；无版本/亚版本则留空。
func ParseModelID(raw string) ParsedModelID {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.Trim(s, "-")
	parts := strings.Split(s, "-")
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			clean = append(clean, p)
		}
	}
	out := ParsedModelID{Raw: raw}
	if len(clean) >= 1 {
		out.Prefix = clean[0]
	}
	if len(clean) >= 2 {
		out.Version = clean[1]
	}
	if len(clean) >= 3 {
		out.SubVersion = strings.Join(clean[2:], "-")
	}
	return out
}

// 亚版本新颖度词表（越高越新；仅作同版本下的次级排序参考）
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
		// 归一为绝对日期值（两位数年份按 2000+ 处理），量级远高于其他规则
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
