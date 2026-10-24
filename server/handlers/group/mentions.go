package grouphandler

import (
	"fmt"
	"strings"

	"nvs-server/groups"
	"nvs-server/models"
)

// ==================== 群组 @提醒 ====================
// 规则（与 IM 一致的最小实现）：
//   - 只解析正文里的 @用户名（2~32 位汉字/字母/数字/下划线），最多 10 个；
//   - 只通知「该群正常成员」，非成员不通知（避免跨群骚扰与用户名探测）；
//   - 不通知自己，同一帖对同一人只通知一次（正文去重已在 ParseMentions 做）；
//   - 通知落站内通知（type=group_mention），跳转到群详情并带 post 参数便于定位。

const maxMentionsPerPost = 10

// notifyGroupMentions 扫描内容中的 @提及并推送站内通知
func notifyGroupMentions(g *groups.Group, content string, actorID, postID uint) {
	if g == nil || actorID == 0 || postID == 0 {
		return
	}
	names := groups.ParseMentions(content)
	if len(names) == 0 {
		return
	}
	if len(names) > maxMentionsPerPost {
		names = names[:maxMentionsPerPost]
	}

	// 只在该群成员范围内解析（避免 @ 站外用户）
	var members []groups.GroupMember
	models.DB.Where("group_id = ? AND status = ?", g.ID, groups.GroupMemberActive).
		Limit(500).Find(&members)
	if len(members) == 0 {
		return
	}
	memberIDs := make([]uint, 0, len(members))
	for _, m := range members {
		memberIDs = append(memberIDs, m.UserID)
	}
	var users []models.User
	models.DB.Select("id, username, nickname").Where("id IN ?", memberIDs).Find(&users)

	want := map[string]bool{}
	for _, n := range names {
		want[strings.ToLower(n)] = true
	}
	actorName := ""
	if actor, err := models.GetUserByID(actorID); err == nil && actor != nil {
		actorName = displayNameOf(actor)
	}
	excerpt := strings.TrimSpace(content)
	if len([]rune(excerpt)) > 80 {
		excerpt = string([]rune(excerpt)[:80]) + "…"
	}

	notified := 0
	for _, u := range users {
		if u.ID == actorID {
			continue
		}
		if !want[strings.ToLower(u.Username)] && !want[strings.ToLower(u.Nickname)] {
			continue
		}
		link := fmt.Sprintf("/group/%d?post=%d", g.ID, postID)
		title := fmt.Sprintf("%s 在「%s」提到了你", actorName, g.Name)
		_ = models.PushNotification(actorID, u.ID, "group_mention", title, excerpt, link)
		notified++
		if notified >= maxMentionsPerPost {
			break
		}
	}
}
