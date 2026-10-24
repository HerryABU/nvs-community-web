package scriptmode

// ==================== 气泡内表情 ====================
// 语法：行内写 [笑] / [哭] 这类标记，渲染为对应表情字符。
// 实现要点：只在「已转义文本」上做替换，且不认识的词原样保留——绝不因此引入未转义内容。

import (
	"regexp"
	"strings"
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

var emoteRe = regexp.MustCompile(`\[([^\[\]\s]{1,6})\]`)

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
