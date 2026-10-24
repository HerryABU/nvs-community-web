package groups

import (
	"regexp"
	"strings"
	"time"

	"nvs-server/models"
)

// ==================== 群组 IM 能力：未读计数 + @提醒 ====================
// 未读口径：以「最后阅读时间」为准，统计该时间之后发布的已过审帖子数（不含自己发的）。
// 之所以不用「最后阅读帖子 ID」：帖子可能被删/被拒，时间戳更稳；帖子 ID 仅作为辅助记录。
//
// @提醒：解析正文里的 @用户名（2~32 位汉字/字母/数字/下划线），
// 仅通知「该群正常成员」，不通知自己，不通知非成员（避免跨群骚扰/探测）。

// GroupReadState 群组阅读状态（每人每群一条）
type GroupReadState struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	GroupID        uint      `gorm:"not null;uniqueIndex:idx_group_read" json:"group_id"`
	UserID         uint      `gorm:"not null;uniqueIndex:idx_group_read" json:"user_id"`
	LastReadAt     time.Time `json:"last_read_at"`
	LastReadPostID uint      `gorm:"not null;default:0" json:"last_read_post_id"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (GroupReadState) TableName() string { return "group_read_states" }

// MarkGroupRead 标记已读到「现在」（或指定帖子时间）
func MarkGroupRead(groupID, userID, postID uint) error {
	if groupID == 0 || userID == 0 {
		return nil
	}
	now := time.Now()
	var st GroupReadState
	err := models.DB.Where("group_id = ? AND user_id = ?", groupID, userID).First(&st).Error
	if err != nil {
		rec := &GroupReadState{
			GroupID: groupID, UserID: userID,
			LastReadAt: now, LastReadPostID: postID, UpdatedAt: now,
		}
		if cerr := models.DB.Create(rec).Error; cerr != nil {
			return cerr
		}
		return models.DB.Model(&GroupReadState{}).Where("id = ?", rec.ID).
			Updates(map[string]interface{}{"last_read_at": now, "last_read_post_id": postID}).Error
	}
	return models.DB.Model(&GroupReadState{}).Where("id = ?", st.ID).
		Updates(map[string]interface{}{
			"last_read_at": now, "last_read_post_id": postID, "updated_at": now,
		}).Error
}

// UnreadCount 单群未读数（已发布、非本人、晚于最后阅读时间）
func UnreadCount(groupID, userID uint) int64 {
	if groupID == 0 || userID == 0 {
		return 0
	}
	var st GroupReadState
	if err := models.DB.Where("group_id = ? AND user_id = ?", groupID, userID).First(&st).Error; err != nil {
		// 从未读过：以「入群时间」为基准，避免一进群就看到历史全量未读
		var m GroupMember
		if merr := models.DB.Where("group_id = ? AND user_id = ?", groupID, userID).First(&m).Error; merr != nil {
			return 0
		}
		return countUnreadSince(groupID, userID, m.JoinedAt)
	}
	return countUnreadSince(groupID, userID, st.LastReadAt)
}

func countUnreadSince(groupID, userID uint, since time.Time) int64 {
	var n int64
	models.DB.Model(&GroupPost{}).
		Where("group_id = ? AND status = ? AND created_at > ? AND user_id <> ?",
			groupID, GroupPostPublished, since, userID).
		Count(&n)
	return n
}

// UnreadCounts 批量未读（列表页避免 N+1）：按群分组各查一次
func UnreadCounts(userID uint, groupIDs []uint) map[uint]int64 {
	out := map[uint]int64{}
	if userID == 0 || len(groupIDs) == 0 {
		return out
	}
	var states []GroupReadState
	models.DB.Where("user_id = ? AND group_id IN ?", userID, groupIDs).Find(&states)
	stateOf := map[uint]GroupReadState{}
	for _, s := range states {
		stateOf[s.GroupID] = s
	}
	// 入群时间（未读基准兜底）
	var members []GroupMember
	models.DB.Where("user_id = ? AND group_id IN ?", userID, groupIDs).Find(&members)
	joinedOf := map[uint]time.Time{}
	for _, m := range members {
		joinedOf[m.GroupID] = m.JoinedAt
	}
	for _, gid := range groupIDs {
		since, ok := joinedOf[gid]
		if st, has := stateOf[gid]; has {
			since, ok = st.LastReadAt, true
		}
		if !ok {
			continue
		}
		out[gid] = countUnreadSince(gid, userID, since)
	}
	return out
}

// @提及解析
var mentionRe = regexp.MustCompile(`@([\p{Han}A-Za-z0-9_]{2,32})`)

// ParseMentions 提取 @用户名（去重、保持顺序）
func ParseMentions(content string) []string {
	if !strings.Contains(content, "@") {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range mentionRe.FindAllStringSubmatch(content, -1) {
		name := strings.TrimSpace(m[1])
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}
