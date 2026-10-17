package models

import (
	"testing"
	"time"
)

// 造一个带指定时间的评论（直接落库，绕开 resolveFloor，仅用于分析测试）
func seedComment(t *testing.T, userID, novelID uint, parentID *uint, created time.Time, micro int64) uint {
	t.Helper()
	c := &Comment{
		UserID:       userID,
		NovelID:      novelID,
		ChapterNumber: 0,
		Content:      "test",
		ParentID:     parentID,
		CreatedAt:    created,
		CreatedMicro: micro,
	}
	if err := DB.Create(c).Error; err != nil {
		t.Fatalf("seed comment failed: %v", err)
	}
	return c.ID
}

func TestParseMentions(t *testing.T) {
	setupTestDB(t)
	// 两个用户：alice / 罗小黑
	alice := User{Username: "alice", Email: "alice@example.com", Nickname: "爱丽丝"}
	bob := User{Username: "luoheimao", Email: "luoheimao@example.com", Nickname: "罗小黑"}
	if err := DB.Create(&alice).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&bob).Error; err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		content string
		want    []string // 期望命中的 username（按顺序去重）
	}{
		{"hi @alice how are you", []string{"alice"}},
		{"@alice @luoheimao 你们好", []string{"alice", "luoheimao"}},
		{"重复 @alice @alice 一次", []string{"alice"}},
		{"邮箱 a@b.com 不应触发", nil}, // @ 前非空白，不当作提及
		{"@ghost 不存在的用户", nil},    // 用户不存在 → 忽略
		// 用昵称 @罗小黑（luoheimao 的 nickname）匹配；Name 取正文中字面匹配到的文本，
		// 供前端正则替换 @罗小黑 → 链接；链接目标用 user_id，昵称解析照常生效。
		{"@罗小黑 中文昵称", []string{"罗小黑"}},
	}
	for _, c := range cases {
		got := ParseMentions(c.content)
		if len(got) != len(c.want) {
			t.Errorf("ParseMentions(%q) = %d 条，期望 %d 条 (got=%v)", c.content, len(got), len(c.want), got)
			continue
		}
		for i, w := range c.want {
			if got[i].Name != w {
				t.Errorf("ParseMentions(%q)[%d].Name = %q，期望 %q", c.content, i, got[i].Name, w)
			}
		}
	}
}

func TestGetCommentNetwork_TimeEstimationAndDegrees(t *testing.T) {
	setupTestDB(t)
	novelID := uint(9001)
	base := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

	// 用户 A、B、C
	a := User{Username: "netA", Email: "neta@example.com"}
	b := User{Username: "netB", Email: "netb@example.com"}
	c := User{Username: "netC", Email: "netc@example.com"}
	for _, u := range []*User{&a, &b, &c} {
		if err := DB.Create(u).Error; err != nil {
			t.Fatal(err)
		}
	}

	// A 开楼（顶层）
	floorA := seedComment(t, a.ID, novelID, nil, base.Add(0), base.Add(0).UnixMicro())

	// B 回复 A 的楼（t+1）
	bReply := seedComment(t, b.ID, novelID, &floorA, base.Add(1*time.Minute), base.Add(1*time.Minute).UnixMicro())

	// B 在 t+2 又发了一条独立评论（顶层）
	b2 := seedComment(t, b.ID, novelID, nil, base.Add(2*time.Minute), base.Add(2*time.Minute).UnixMicro())

	// A 在 t+3 回复「B 的 b2」（显式 parent=b2），但 B 此时最新的是 b2 本身 → 估计==显式，不标记
	seedComment(t, a.ID, novelID, &b2, base.Add(3*time.Minute), base.Add(3*time.Minute).UnixMicro())

	// A 在 t+5 回复「B 的 bReply(t+1)」，但 B 最新的是 b2(t+2)（早于 t+5）→ 估计目标应为 b2，is_estimated=true
	edge := seedComment(t, a.ID, novelID, &bReply, base.Add(5*time.Minute), base.Add(5*time.Minute).UnixMicro())

	net, err := GetCommentNetwork("novel", novelID, 0)
	if err != nil {
		t.Fatalf("GetCommentNetwork error: %v", err)
	}
	if net.InteractionCount < 3 {
		t.Errorf("interaction count = %d，期望 >=3", net.InteractionCount)
	}

	// 找到 A→B 且 parent==bReply 的那条边，验证时间估计
	var est *NetworkEdge
	for i := range net.Edges {
		e := net.Edges[i]
		if e.FromUserID == a.ID && e.ToUserID == b.ID && e.ParentCommentID == bReply {
			est = &net.Edges[i]
		}
	}
	if est == nil {
		t.Fatalf("未找到 A→B(parent=bReply) 的边")
	}
	if !est.IsEstimated {
		t.Errorf("期望 is_estimated=true（B 最新评论为 b2 而非显式父 bReply），实际 false")
	}
	if est.EstimatedTargetID != b2 {
		t.Errorf("估计目标应为 b2(%d)，实际 %d", b2, est.EstimatedTargetID)
	}

	// C 节点 degree 校验：netA out>=2, netB in>=2
	var nodeA, nodeB *NetworkNode
	for i := range net.Nodes {
		if net.Nodes[i].UserID == a.ID {
			nodeA = &net.Nodes[i]
		}
		if net.Nodes[i].UserID == b.ID {
			nodeB = &net.Nodes[i]
		}
	}
	if nodeA == nil || nodeB == nil {
		t.Fatalf("缺少 A/B 节点")
	}
	if nodeA.OutDegree < 2 {
		t.Errorf("A out_degree=%d，期望>=2", nodeA.OutDegree)
	}
	if nodeB.InDegree < 2 {
		t.Errorf("B in_degree=%d，期望>=2", nodeB.InDegree)
	}
	_ = edge
}
