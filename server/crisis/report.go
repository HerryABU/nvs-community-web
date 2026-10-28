package crisis

import (
	"fmt"
	"strings"

	"nvs-server/models"
)

// BuildCrisisReport 生成《高危用户心理干预报告》。
//
// AI 负责把散落的信号聚合成一份人能读懂的材料；
// 但「是否报警」「是否以官方身份发关怀邮件」必须由站长人工决定 —— 报告末尾会写明这一点。
func BuildCrisisReport(userID uint) (string, error) {
	u, err := models.GetUserByID(userID)
	if err != nil {
		return "", err
	}
	events, _ := models.ListCrisisEventsByUser(userID)

	var b strings.Builder
	b.WriteString("# 高危用户心理干预报告\n\n")
	fmt.Fprintf(&b, "- 用户：%s（#%d）\n", u.Username, u.ID)
	if strings.TrimSpace(u.Nickname) != "" {
		fmt.Fprintf(&b, "- 昵称：%s\n", u.Nickname)
	}
	fmt.Fprintf(&b, "- 未成年标记：%s\n", boolLabel(IsMinorUser(u)))
	if u.RiskYouthMentalUntil != nil {
		fmt.Fprintf(&b, "- 观察期至：%s\n", u.RiskYouthMentalUntil.Format("2006-01-02 15:04"))
	} else {
		b.WriteString("- 观察期至：未设置\n")
	}
	fmt.Fprintf(&b, "- 危机事件总数：%d\n", len(events))

	// 事件按来源统计（关键词 vs AI 语义）
	kw, ai := 0, 0
	for _, e := range events {
		if e.Source == "ai" {
			ai++
		} else {
			kw++
		}
	}
	fmt.Fprintf(&b, "- 其中：关键词命中 %d 次 / AI 语义判危 %d 次\n\n", kw, ai)

	b.WriteString("## 事件时间线（新 → 旧）\n\n")
	if len(events) == 0 {
		b.WriteString("_暂无危机事件记录。_\n")
	}
	for _, e := range events {
		fmt.Fprintf(&b, "### %s · %s\n\n", e.CreatedAt.Format("2006-01-02 15:04"), CrisisActionLabel(e.Action))
		fmt.Fprintf(&b, "- 来源：%s\n", CrisisSourceLabel(e.Source))
		fmt.Fprintf(&b, "- 内容类型：%s #%d\n", e.ContentType, e.ContentID)
		fmt.Fprintf(&b, "- 片段：%s\n", e.Snippet)
		if strings.TrimSpace(e.Reason) != "" {
			fmt.Fprintf(&b, "- 判据：%s\n", e.Reason)
		}
		b.WriteString("\n")
	}

	b.WriteString("---\n\n")
	b.WriteString("本报告由 AI 安全系统自动生成：识别、留痕与汇总已完成。\n")
	b.WriteString("**最后的处置动作（报警 / 以官方身份发送关怀邮件 / 人工回访）必须由站长决定**，")
	b.WriteString("AI 不会替你按下那个按钮。\n")
	return b.String(), nil
}

// CrisisSourceLabel 事件来源的中文标签。
func CrisisSourceLabel(source string) string {
	switch source {
	case "ai":
		return "AI 语义分析"
	case "keyword":
		return "内置黑话词表"
	default:
		return source
	}
}

// boolLabel 布尔值的中文标签。
func boolLabel(v bool) string {
	if v {
		return "是"
	}
	return "否"
}
