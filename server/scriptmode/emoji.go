package scriptmode

// ==================== 气泡内表情 ====================
// 语法：行内写 [笑] / [哭] 这类标记，渲染为对应表情字符。
// 实现要点：只在「已转义文本」上做替换，且不认识的词原样保留——绝不因此引入未转义内容。

import (
	"html"
	"regexp"
	"strings"
	"sync"
)

// emotes 内置表情词典（键为中文短词，值为表情字符）
var emotes = map[string]string{
	"笑": "😄", "微笑": "🙂", "大笑": "😂", "偷笑": "😏", "苦笑": "😅",
	"哭": "😢", "大哭": "😭", "泪": "🥲", "哭笑": "🤣",
	"怒": "😠", "生气": "😡", "无语": "😑", "汗": "😓", "惊": "😲",
	"疑": "🤔", "问": "❓", "叹": "😮‍💨", "羞": "😳", "怕": "😨",
	"喜": "😊", "爱": "😍", "冷": "😒", "困": "🥱", "晕": "😵",
	"赞": "👍", "踩": "👎", "鼓掌": "👏", "抱": "🤗", "心": "❤️",
}

// EmoteKeys 供编辑器提示可用表情
func EmoteKeys() []string {
	keys := make([]string, 0, len(emotes))
	for k := range emotes {
		keys = append(keys, k)
	}
	return keys
}

var emoteRe = regexp.MustCompile(`\[([^\[\]\s]{1,8})\]`)

// customEmotes 用户自定义表情（key → 图片 URL），由 oc 包在启动与写操作后注册。
// 只接受站内 /uploads 或 http(s)，其它一律丢弃（防止 javascript:/data: 混进来）。
var (
	customMu     sync.RWMutex
	customEmotes = map[string]string{}
)

// RegisterCustomEmotes 覆盖式注册（每次全量替换，避免残留已删除的键）
func RegisterCustomEmotes(m map[string]string) {
	clean := make(map[string]string, len(m))
	for k, v := range m {
		if k == "" || v == "" {
			continue
		}
		low := strings.ToLower(v)
		if strings.HasPrefix(v, "/uploads/") || strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
			clean[k] = v
		}
	}
	customMu.Lock()
	customEmotes = clean
	customMu.Unlock()
}

// CustomEmoteKeys 自定义表情键（编辑器提示用）
func CustomEmoteKeys() []string {
	customMu.RLock()
	defer customMu.RUnlock()
	out := make([]string, 0, len(customEmotes))
	for k := range customEmotes {
		out = append(out, k)
	}
	return out
}

func lookupCustom(key string) (string, bool) {
	customMu.RLock()
	defer customMu.RUnlock()
	v, ok := customEmotes[key]
	return v, ok
}

// RenderEmotes 把 [笑] 形式的表情标记替换为表情字符（不认识的原样保留）
func RenderEmotes(s string) string {
	return emoteRe.ReplaceAllStringFunc(s, func(m string) string {
		key := m[1 : len(m)-1]
		if v, ok := emotes[key]; ok {
			return v
		}
		return m
	})
}

// emoteSpan 渲染为带类名的 span，便于前端放大/加样式
func emoteSpan(s string) string {
	return emoteRe.ReplaceAllStringFunc(s, func(m string) string {
		key := m[1 : len(m)-1]
		if u, ok := lookupCustom(key); ok {
			// 图片：alt 用原始标记，属性值做 HTML 转义（URL 已在注册时白名单校验）
			return `<img class="sb-emote-img" src="` + html.EscapeString(u) +
				`" alt="` + html.EscapeString(m) + `" loading="lazy">`
		}
		if v, ok := emotes[key]; ok {
			return `<span class="sb-emote" title="[` + key + `]">` + v + `</span>`
		}
		return m
	})
}

// HasEmote 文本里是否含表情标记（编辑器提示用）
func HasEmote(s string) bool {
	return strings.Contains(s, "[") && emoteRe.MatchString(s)
}
