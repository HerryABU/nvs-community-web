package groups

import (
	"testing"
	"time"

	"nvs-server/models"
)

// TestParseMentions @提及解析：去重、忽略过短/非法、最多取前若干个由调用方限制
func TestParseMentions(t *testing.T) {
	got := ParseMentions("你好 @张三 看下 @li_si 和 @张三 还有 @a 以及邮箱 a@b.com")
	if len(got) != 2 || got[0] != "张三" || got[1] != "li_si" {
		t.Fatalf("@解析结果异常：%#v", got)
	}
	if ParseMentions("没有提及的内容") != nil {
		t.Fatalf("无 @ 时应返回 nil")
	}
}

// TestUnreadAndMarkRead 未读口径：他人已发布帖子计未读；自己发的不计；标记已读后归零
func TestUnreadAndMarkRead(t *testing.T) {
	setupGroupTestDB(t)
	owner := mkUser(t, "r_owner", models.RoleAuthor)
	member := mkUser(t, "r_member", models.RoleReader)

	g, err := CreateGroup(owner.ID, owner, GroupInput{Name: "未读测试群"})
	if err != nil {
		t.Fatalf("建群失败: %v", err)
	}
	if _, err := JoinGroup(g, member, ""); err != nil {
		t.Fatalf("加入失败: %v", err)
	}

	// 群主发一帖 → 成员未读应为 1
	if _, err := CreateGroupPost(g, owner, "第一帖", "内容内容", false); err != nil {
		t.Fatalf("发帖失败: %v", err)
	}
	if n := UnreadCount(g.ID, member.ID); n != 1 {
		t.Fatalf("成员未读数应为 1，得到 %d", n)
	}
	// 自己发的帖子不应计入自己的未读
	if _, err := CreateGroupPost(g, member, "成员自己发的", "内容内容", false); err != nil {
		t.Fatalf("发帖失败: %v", err)
	}
	if n := UnreadCount(g.ID, member.ID); n != 1 {
		t.Fatalf("自己发的帖不应计未读，得到 %d", n)
	}
	// 批量接口与单群一致
	if got := UnreadCounts(member.ID, []uint{g.ID})[g.ID]; got != 1 {
		t.Fatalf("批量未读应为 1，得到 %d", got)
	}

	// 标记已读 → 归零
	if err := MarkGroupRead(g.ID, member.ID, 0); err != nil {
		t.Fatalf("标记已读失败: %v", err)
	}
	if n := UnreadCount(g.ID, member.ID); n != 0 {
		t.Fatalf("标记已读后未读应为 0，得到 %d", n)
	}

	// 群主再发一帖 → 又变 1
	if _, err := CreateGroupPost(g, owner, "第二帖", "内容内容", false); err != nil {
		t.Fatalf("发帖失败: %v", err)
	}
	if n := UnreadCount(g.ID, member.ID); n != 1 {
		t.Fatalf("新帖后未读应为 1，得到 %d", n)
	}

	// 非成员无未读
	stranger := mkUser(t, "r_stranger", models.RoleReader)
	if n := UnreadCount(g.ID, stranger.ID); n != 0 {
		t.Fatalf("非成员未读应为 0，得到 %d", n)
	}
}

// TestUnreadUsesPostIDNotTime 回归：未读必须按帖子 ID 判定，不能被时间精度/时钟回拨吃掉
//
// 历史缺陷：只按 `created_at > last_read_at` 统计。用户「标记已读」与别人「发帖」落在同一时刻时，
// 该帖被永久算作已读（线上概率低但真实；测试里同秒必然命中，正是本次 push 被拦的原因）。
// 现在 last_read_post_id 优先，这里把 last_read_at 人为推到未来，模拟最坏情况。
func TestUnreadUsesPostIDNotTime(t *testing.T) {
	setupGroupTestDB(t)
	owner := mkUser(t, "id_owner", models.RoleAuthor)
	member := mkUser(t, "id_member", models.RoleReader)

	g, err := CreateGroup(owner.ID, owner, GroupInput{Name: "ID口径测试群"})
	if err != nil {
		t.Fatalf("建群失败: %v", err)
	}
	if _, err := JoinGroup(g, member, ""); err != nil {
		t.Fatalf("加入失败: %v", err)
	}
	first, err := CreateGroupPost(g, owner, "第一帖", "内容内容", false)
	if err != nil {
		t.Fatalf("发帖失败: %v", err)
	}

	// 读到现在：应记住 last_read_post_id = 第一帖 ID
	if err := MarkGroupRead(g.ID, member.ID, 0); err != nil {
		t.Fatalf("标记已读失败: %v", err)
	}
	var st GroupReadState
	if err := models.DB.Where("group_id = ? AND user_id = ?", g.ID, member.ID).First(&st).Error; err != nil {
		t.Fatalf("读取状态失败: %v", err)
	}
	if st.LastReadPostID != first.ID {
		t.Fatalf("标记已读应记住最新帖 ID %d，得到 %d", first.ID, st.LastReadPostID)
	}

	// 最坏情况：last_read_at 比之后的所有帖子都晚（同刻/时钟回拨/跨机时钟漂移）
	models.DB.Model(&GroupReadState{}).Where("id = ?", st.ID).
		Update("last_read_at", time.Now().Add(time.Hour))

	second, err := CreateGroupPost(g, owner, "第二帖", "内容内容", false)
	if err != nil {
		t.Fatalf("发帖失败: %v", err)
	}
	if n := UnreadCount(g.ID, member.ID); n != 1 {
		t.Fatalf("时间被推到未来时，未读仍应为 1（ID 口径），得到 %d", n)
	}
	if got := UnreadCounts(member.ID, []uint{g.ID})[g.ID]; got != 1 {
		t.Fatalf("批量口径应一致为 1，得到 %d", got)
	}

	// 再读到第二帖 → 归零；自己发帖不计
	if err := MarkGroupRead(g.ID, member.ID, second.ID); err != nil {
		t.Fatalf("标记已读失败: %v", err)
	}
	if n := UnreadCount(g.ID, member.ID); n != 0 {
		t.Fatalf("读完后未读应为 0，得到 %d", n)
	}
}
