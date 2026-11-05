package aiops

import (
	"encoding/json"
	"errors"
	"nvs-server/models"
	"strings"
)

// ==================== AI 工作台左栏自定义菜单（站长设置） ====================
//
// 参考站左栏底部有「查看更多 / 自定义应用 / 添加菜单」。按用户界定：
// **这是站长设置的，不是用户自己加的**；挂站内面板或任意 URL 都可以，
// 因为是站长自己录的，所以不走「URL 白名单审核」。
//
// 但有一件事仍然必须做：**拒绝 javascript: / data: 这类伪协议**。
// 那不是「审核」，而是基本的正确性——这类 URL 写进 href 就是站内 XSS，
// 站长多半是被粘贴了恶意串，平台有义务挡住。
//
// 存储：platform_configs.ai_nav_items（JSON 数组），整体保存、原子生效，
// 不建表是因为条目数量少（≤50）且没有按用户维度扩展的需求。

const aiNavItemsKey = "ai_nav_items"

// AiNavKinds 合法类型
//
//	panel —— 站内面板（target 为 AI 工作台 tab key，或站内路径 /xxx）
//	link  —— 任意外部 URL（http/https）
var AiNavKinds = []string{"panel", "link"}

// AiNavItem 一条自定义菜单
type AiNavItem struct {
	Key     string `json:"key"`     // 唯一标识（前端 :key，新建时后端补 UUID 风格串）
	Title   string `json:"title"`   // 显示名
	Emoji   string `json:"emoji"`   // 图标（emoji，站内没有图标库依赖）
	Kind    string `json:"kind"`    // panel / link
	Target  string `json:"target"`  // panel: tab key 或 /path；link: http(s) URL
	Group   string `json:"group"`   // 分组名，空 = 「自定义应用」
	NewTab  bool   `json:"new_tab"` // 是否新标签打开（link 建议开）
	Enabled bool   `json:"enabled"` // 是否显示
}

// GetAiNavItems 读取自定义菜单（坏配置返回空列表，不让左栏整体挂掉）
func GetAiNavItems() []AiNavItem {
	raw := strings.TrimSpace(models.GetPlatformConfig(aiNavItemsKey))
	if raw == "" {
		return nil
	}
	var list []AiNavItem
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil
	}
	// 输出前再过一遍校验：历史配置可能存了今天才算非法的 URL
	out := make([]AiNavItem, 0, len(list))
	for _, it := range list {
		if err := validateAiNavItem(&it); err == nil && it.Enabled {
			out = append(out, it)
		}
	}
	return out
}

// GetAllAiNavItems 管理端读取（含未启用项，方便站长编辑）
func GetAllAiNavItems() []AiNavItem {
	raw := strings.TrimSpace(models.GetPlatformConfig(aiNavItemsKey))
	if raw == "" {
		return nil
	}
	var list []AiNavItem
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil
	}
	return list
}

// SetAiNavItems 整体保存（逐条校验；任何一条非法就整体拒绝，避免半套菜单）
func SetAiNavItems(list []AiNavItem) error {
	if len(list) > 50 {
		return errors.New("自定义菜单最多 50 条")
	}
	seen := map[string]bool{}
	for i := range list {
		if err := validateAiNavItem(&list[i]); err != nil {
			return err
		}
		// key 去重：重复 key 会让前端 v-for 渲染异常
		if seen[list[i].Key] {
			list[i].Key = list[i].Key + "-" + itoaSmall(i)
		}
		seen[list[i].Key] = true
	}
	if len(list) == 0 {
		return models.SetPlatformConfig(aiNavItemsKey, "[]")
	}
	b, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return models.SetPlatformConfig(aiNavItemsKey, string(b))
}

// validateAiNavItem 逐条校验并就地补默认值（Key/Group/Enabled）
func validateAiNavItem(it *AiNavItem) error {
	it.Title = strings.TrimSpace(it.Title)
	it.Target = strings.TrimSpace(it.Target)
	it.Group = strings.TrimSpace(it.Group)
	it.Kind = strings.ToLower(strings.TrimSpace(it.Kind))

	if it.Title == "" {
		return errors.New("菜单名不能为空")
	}
	if len([]rune(it.Title)) > 16 {
		return errors.New("菜单名最多 16 个字：" + it.Title)
	}
	if it.Target == "" {
		return errors.New("菜单「" + it.Title + "」缺少跳转目标")
	}
	if it.Kind == "" {
		// 未指定时按形态推断：以 / 或 http 开头但不像 tab key 的当链接
		if strings.HasPrefix(it.Target, "http://") || strings.HasPrefix(it.Target, "https://") {
			it.Kind = "link"
		} else {
			it.Kind = "panel"
		}
	}
	switch it.Kind {
	case "panel":
		// 站内：允许 tab key（无斜杠）或站内绝对路径
		if strings.Contains(it.Target, "://") {
			return errors.New("「" + it.Title + "」是站内面板，目标不能是完整 URL")
		}
	case "link":
		if !strings.HasPrefix(it.Target, "http://") && !strings.HasPrefix(it.Target, "https://") {
			return errors.New("「" + it.Title + "」的外部链接必须以 http:// 或 https:// 开头")
		}
	default:
		return errors.New("菜单类型只能是 panel（站内）或 link（外部链接）")
	}
	// 伪协议拦截：javascript: / data: / vbscript: / file: 一律拒绝
	low := strings.ToLower(it.Target)
	for _, bad := range []string{"javascript:", "data:", "vbscript:", "file:"} {
		if strings.HasPrefix(low, bad) {
			return errors.New("「" + it.Title + "」的目标协议不被允许（只为防 XSS，与审核无关）")
		}
	}
	if it.Group == "" {
		it.Group = "自定义应用"
	}
	if len([]rune(it.Group)) > 16 {
		return errors.New("分组名最多 16 个字")
	}
	if it.Key == "" {
		it.Key = "nav-" + itoaSmall(len(it.Target)) + "-" + sanitizeKey(it.Target)
	}
	it.Emoji = strings.TrimSpace(it.Emoji)
	if it.Emoji == "" {
		it.Emoji = "🔗"
	}
	return nil
}

// sanitizeKey 把目标串压成短 key（只为稳定渲染，不参与鉴权）
func sanitizeKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == '/' || r == '-' || r == '_' || r == '.' {
			b.WriteRune('-')
		}
		if b.Len() >= 24 {
			break
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "item"
	}
	return out
}

// itoaSmall 极小整数转串（避免为两处拼接引入 strconv）
func itoaSmall(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
