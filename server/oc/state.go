package oc

// ==================== 角色状态与养成 ====================
// 借鉴「小团子/咔伴」的养成玩法：互动会改变 OC（情绪、好感、精力），
// 每一轮发言既被状态影响，也反过来改变状态，形成「创造-互动-回馈」的闭环。
//
// 实现：AI 输出第一行约定为 [情绪:词]（与话本气泡的表情标记同源），
// 解析出情绪词后按情绪极性调整 mood，按是否被回应调整 affinity/energy。

import (
	"regexp"
	"strings"
	"time"

	"nvs-server/models"
)

var emoteTagRe = regexp.MustCompile(`^\s*\[情绪[:：]\s*([^\[\]]{1,8})\s*\]\s*`)

// EmotePolarity 常见情绪词的极性（正/负/中性），用于调整状态
var emotePolarity = map[string]int{
	"笑": 6, "微笑": 4, "开心": 8, "大笑": 9, "喜悦": 8, "感动": 6, "害羞": 3, "爱": 8,
	"平静": 0, "思考": 0, "疑惑": -1, "惊讶": 1, "认真": 1,
	"哭": -8, "怒": -9, "生气": -9, "难过": -7, "怕": -6, "累": -5, "无语": -3, "冷": -3,
}

// ParseEmotion 解析输出首行的 [情绪:词]，返回（情绪词, 去掉标记后的正文）
func ParseEmotion(content string) (string, string) {
	m := emoteTagRe.FindStringSubmatch(content)
	if len(m) != 2 {
		return "", content
	}
	return strings.TrimSpace(m[1]), strings.TrimSpace(content[len(m[0]):])
}

// StateDelta 状态变化量
type StateDelta struct {
	Mood     int `json:"mood"`
	Affinity int `json:"affinity"`
	Energy   int `json:"energy"`
}

// ApplyTurnToState 一轮发言后更新角色状态（并剥离情绪标记用于展示）
func ApplyTurnToState(characterID uint, emotion string, repliedByOther bool) StateDelta {
	var c Character
	if err := models.DB.Where("id = ?", characterID).First(&c).Error; err != nil {
		return StateDelta{}
	}
	d := StateDelta{}
	if p, ok := emotePolarity[emotion]; ok {
		d.Mood = p / 2
		if d.Mood == 0 && p != 0 {
			d.Mood = sign(p)
		}
	} else {
		d.Mood = 1 // 未识别情绪：轻微正向（有互动）
	}
	if repliedByOther {
		d.Affinity = 2
	}
	d.Energy = -1 // 发言消耗精力

	patch := map[string]interface{}{"updated_at": time.Now()}
	if d.Mood != 0 {
		patch["mood"] = clamp01(c.Mood+d.Mood, 0)
	}
	if d.Affinity != 0 {
		patch["affinity"] = clamp01(c.Affinity+d.Affinity, 0)
	}
	if d.Energy != 0 {
		patch["energy"] = clamp01(c.Energy+d.Energy, 0)
	}
	_ = models.DB.Model(&Character{}).Where("id = ?", characterID).Updates(patch).Error
	return d
}

func sign(v int) int {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}

// EmoteAvatar 按情绪取立绘/表情（小团子的「立绘动作互动」）
func EmoteAvatar(c Character, emotion string) string {
	if c.EmoteMap == "" || emotion == "" {
		return c.AvatarURL
	}
	// 轻量解析：不引 JSON 依赖，按 "k=v" 逐项匹配
	for _, part := range strings.FieldsFunc(c.EmoteMap, func(r rune) bool { return r == ',' || r == '\n' }) {
		kv := strings.SplitN(strings.TrimSpace(part), ":", 2)
		if len(kv) != 2 {
			kv = strings.SplitN(strings.TrimSpace(part), "=", 2)
		}
		if len(kv) != 2 {
			continue
		}
		if strings.TrimSpace(strings.Trim(kv[0], `"'`)) == emotion {
			return strings.TrimSpace(strings.Trim(kv[1], `"'`))
		}
	}
	return c.AvatarURL
}

// EmotionOf 从一条发言内容里取情绪（供前端展示气泡表情/立绘）
func EmotionOf(content string) string {
	e, _ := ParseEmotion(content)
	return e
}
