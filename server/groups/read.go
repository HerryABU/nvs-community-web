package groups

import (
	"regexp"
	"strings"
	"time"

	"nvs-server/models"
)

// ==================== 群组 IM 能力：未读计数 + @提醒 ====================
// 未读口径（双轨，ID 优先）：
//   1) 有 last_read_post_id 时按 **ID** 统计：id > 已读到的帖子 ID，且未被拒/未删。
//      ID 自增单调，不受时间精度影响——这一点很关键：早期只按 `created_at > last_read_at`，
//      用户「标记已读」与别人「发帖」落在同一时刻（或系统时钟回拨）时，那条帖子会被永久算作已读，
//      用户再也看不到未读提醒（线上概率低但真实存在，测试里同秒必然命中）。
//   2) last_read_post_id = 0（老数据 / 手动传 0 且群内无帖）时回退到时间基准。
// 删除/审核不通过只会让计数偏小（该帖本就不该出现），不会导致漏读，因此 ID 口径是安全的。
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
	// postID=0 表示「读到现在」：同时记下当前最大已发布帖 ID，
	// 让后续未读判定走 ID 口径（不受时间精度/时钟回拨影响）
	if postID == 0 {
		var maxID uint
		models.DB.Model(&GroupPost{}).
			Where("group_id = ? AND status = ?", groupID, GroupPostPublished).
			Select("COALESCE(MAX(id), 0)").Scan(&maxID)
		postID = maxID
	}
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
	if st.LastReadPostID > 0 {
		return countUnreadAfterID(groupID, userID, st.LastReadPostID)
	}
	return countUnreadSince(groupID, userID, st.LastReadAt)
}

// countUnreadAfterID 主口径：比已读到的帖子更新、且已过审、非本人
func countUnreadAfterID(groupID, userID, afterID uint) int64 {
	var n int64
	models.DB.Model(&GroupPost{}).
		Where("group_id = ? AND status = ? AND id > ? AND user_id <> ?",
			groupID, GroupPostPublished, afterID, userID).
		Count(&n)
	return n
}

// countUnreadSince 时间基准（兜底口径）。
// 用 `>=` 而不是 `>`：Windows 的 time.Now() 粒度约 15ms（部分系统 0.5ms~15.6ms），
// 「加入群组」与「别人发帖」可以取到**完全相同**的时间戳，用 `>` 会永久漏掉那条未读。
// 代价是可能多算 1 条（入群前同刻的帖 / 标记已读前同刻的帖），用户点开即清零——
// 宁可多算一条，也不能让人永远看不到未读。
func countUnreadSince(groupID, userID uint, since time.Time) int64 {
	var n int64
	models.DB.Model(&GroupPost{}).
		Where("group_id = ? AND status = ? AND created_at >= ? AND user_id <> ?",
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
		if st, has := stateOf[gid]; has {
			// 与单群口径保持一致：有 ID 走 ID，否则回退时间
			if st.LastReadPostID > 0 {
				out[gid] = countUnreadAfterID(gid, userID, st.LastReadPostID)
			} else {
				out[gid] = countUnreadSince(gid, userID, st.LastReadAt)
			}
			continue
		}
		// 从未读过：以入群时间为基准
		if since, ok := joinedOf[gid]; ok {
			out[gid] = countUnreadSince(gid, userID, since)
		}
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
