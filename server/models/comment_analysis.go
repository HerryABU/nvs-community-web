package models

import (
	"strings"
)

// ─────────────────────────────────────────────────────────────
// 评论区分析：@ 提及候选 + 评论网（社交图谱）分析
// ─────────────────────────────────────────────────────────────

// MentionCandidate @ 提及联想候选用户
type MentionCandidate struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}

// GetMentionCandidates 返回 @ 提及联想候选。
//   - scope="global"：全站用户（username/nickname LIKE q），不限作品
//   - scope="participants"（默认）：仅本作品/博客下出现过的评论者（按 target_type/target_id 限定）
//
// limit 控制返回条数（默认 20）。q 为空时返回热门候选（按出现先后）。
func GetMentionCandidates(targetType string, targetID uint, scope, q string, limit int) ([]MentionCandidate, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var users []User
	var err error
	if scope == "global" {
		q2 := DB.Model(&User{})
		if q != "" {
			q2 = q2.Where("username LIKE ? OR nickname LIKE ?", "%"+q+"%", "%"+q+"%")
		}
		err = q2.Order("id DESC").Limit(limit).Find(&users).Error
	} else {
		// 本作品参与者：先取去重评论者 user_id，再按 q 过滤用户名
		var uids []uint
		base := DB.Model(&Comment{})
		if targetType == "blog" {
			base = base.Where("blog_id = ?", targetID)
		} else {
			base = base.Where("novel_id = ?", targetID)
		}
		base.Select("DISTINCT user_id").Scan(&uids)
		if len(uids) == 0 {
			return []MentionCandidate{}, nil
		}
		q2 := DB.Where("id IN ?", uids)
		if q != "" {
			q2 = q2.Where("username LIKE ? OR nickname LIKE ?", "%"+q+"%", "%"+q+"%")
		}
		err = q2.Order("id DESC").Limit(limit).Find(&users).Error
	}
	if err != nil {
		return nil, err
	}
	out := make([]MentionCandidate, 0, len(users))
	for _, u := range users {
		out = append(out, MentionCandidate{ID: u.ID, Username: u.Username, Nickname: u.Nickname})
	}
	return out, nil
}

// ───────────── 评论网分析（社交图谱） ─────────────

// CommentNetwork 评论网分析结果
type CommentNetwork struct {
	TargetType       string         `json:"target_type"`
	TargetID         uint           `json:"target_id"`
	TotalComments    int            `json:"total_comments"`
	CommenterCount   int            `json:"commenter_count"`
	InteractionCount int            `json:"interaction_count"` // 有效互动边数量（A 回复 B，A≠B）
	Nodes            []NetworkNode  `json:"nodes"`
	Edges            []NetworkEdge  `json:"edges"`   // 详细边（谁对哪条评论回复了谁，含时间估计），按时间倒序
	TopPairs         []NetworkPair  `json:"top_pairs"` // 按互动频次聚合的(A→B)对，用于关系图边与榜单
}

type NetworkNode struct {
	UserID      uint   `json:"user_id"`
	Username    string `json:"username"`
	Nickname    string `json:"nickname"`
	CommentCount int   `json:"comment_count"`
	InDegree    int    `json:"in_degree"`  // 收到的回复数
	OutDegree   int    `json:"out_degree"` // 发出的回复数
}

type NetworkEdge struct {
	FromUserID   uint   `json:"from_user_id"`
	FromName     string `json:"from_name"`
	ToUserID     uint   `json:"to_user_id"`
	ToName       string `json:"to_name"`
	CommentID    uint   `json:"comment_id"`    // 本条回复评论 id
	ParentCommentID uint `json:"parent_comment_id"` // 显式父评论 id
	CommentExcerpt string `json:"comment_excerpt"` // 本条回复评论正文（A 说的话）
	ParentExcerpt string `json:"parent_excerpt"`
	CreatedMicro int64  `json:"created_micro"`
	// 时间估计：若回复者显式父评论不是被回复者「最新的一条」，则估计其实际想回复的是被回复者更早前的最新评论
	EstimatedTargetID       uint   `json:"estimated_target_id"`
	EstimatedTargetExcerpt  string `json:"estimated_target_excerpt"`
	IsEstimated             bool   `json:"is_estimated"`
}

type NetworkPair struct {
	FromUserID uint   `json:"from_user_id"`
	FromName   string `json:"from_name"`
	ToUserID   uint   `json:"to_user_id"`
	ToName     string `json:"to_name"`
	Count      int    `json:"count"`
	// 高频互怼/刷评预警：单向互动次数超过阈值标记可疑（配合 AI 审核的轻量规则）
	Suspicious bool `json:"suspicious"`
}

// 高频互动可疑阈值（单向 A→B 超过该次数视为「疑似刷评/抱团」）
const SuspiciousPairThreshold = 30

func commentSnippet(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	runeCount := 0
	for i := range s {
		runeCount++
		if runeCount > 40 {
			return s[:i] + "…"
		}
	}
	return s
}

func effMicro(c Comment) int64 {
	if c.CreatedMicro > 0 {
		return c.CreatedMicro
	}
	return c.CreatedAt.UnixMicro()
}

// GetCommentNetwork 计算某作品/博客的评论社交图谱。
// filterUserID>0 时只保留与该评论者相关的边（下钻到「某评论者」）。
// 时间估计：对每条 A→B 的回复，取 B 在回复时刻前的最新评论作为「估计目标」，
// 若该估计目标 != 显式父评论，则标记 is_estimated=true（即 A 大概率是在顺着 B 最新轨迹留言）。
func GetCommentNetwork(targetType string, targetID, filterUserID uint) (*CommentNetwork, error) {
	var comments []Comment
	base := DB.Model(&Comment{})
	if targetType == "blog" {
		base = base.Where("blog_id = ?", targetID)
	} else {
		base = base.Where("novel_id = ?", targetID)
	}
	// 仅取未删除评论（与用户可见范围一致）；上限 3000 防止超大作品 OOM
	err := base.Order("created_at ASC").Limit(3000).Find(&comments).Error
	if err != nil {
		return nil, err
	}

	byID := make(map[uint]Comment, len(comments))
	// 每位评论者的评论（按微秒倒序），用于时间估计取「回复时刻前的最新」
	byUser := map[uint][]Comment{}
	for _, c := range comments {
		byID[c.ID] = c
		byUser[c.UserID] = append(byUser[c.UserID], c)
	}
	for uid, list := range byUser {
		// 按微秒倒序
		for i := 0; i < len(list); i++ {
			for j := i + 1; j < len(list); j++ {
				if effMicro(list[j]) > effMicro(list[i]) {
					list[i], list[j] = list[j], list[i]
				}
			}
		}
		byUser[uid] = list
	}

	// 用户名映射（评论者 + 被回复者）
	needUID := map[uint]bool{}
	for _, c := range comments {
		needUID[c.UserID] = true
	}
	nameOf := map[uint]struct{ username, nickname string }{}
	if len(needUID) > 0 {
		ids := make([]uint, 0, len(needUID))
		for id := range needUID {
			ids = append(ids, id)
		}
		var users []User
		DB.Where("id IN ?", ids).Find(&users)
		for _, u := range users {
			nameOf[u.ID] = struct{ username, nickname string }{u.Username, u.Nickname}
		}
	}

	displayName := func(uid uint) string {
		if n, ok := nameOf[uid]; ok {
			if n.nickname != "" {
				return n.nickname
			}
			return n.username
		}
		return "用户" + itoa(uid)
	}

	type agg struct {
		in, out int
	}
	nodeStat := map[uint]*agg{}
	getStat := func(uid uint) *agg {
		if nodeStat[uid] == nil {
			nodeStat[uid] = &agg{}
		}
		return nodeStat[uid]
	}

	var edges []NetworkEdge
	pairCount := map[string]int{} // "from->to" -> count
	for _, c := range comments {
		if c.ParentID == nil || *c.ParentID == 0 {
			continue // 顶层评论（开新楼）不是对用户的回复
		}
		parent, ok := byID[*c.ParentID]
		if !ok {
			continue // 父评论已删除：不构成社交边
		}
		from, to := c.UserID, parent.UserID
		if from == to {
			continue // 自己回复自己不算社交互动
		}
		fs, ts := getStat(from), getStat(to)
		fs.out++
		ts.in++

		// 时间估计：取 to 在 c 时刻前的最新评论
		var estID uint
		var estExcerpt string
		isEst := false
		for _, cm := range byUser[to] {
			if effMicro(cm) < effMicro(c) {
				estID = cm.ID
				estExcerpt = commentSnippet(cm.Content)
				isEst = (cm.ID != parent.ID)
				break
			}
		}
		if estID == 0 { // 没有更早的：估计目标即显式父评论
			estID = parent.ID
			estExcerpt = commentSnippet(parent.Content)
			isEst = false
		}

		edges = append(edges, NetworkEdge{
			FromUserID:             from,
			FromName:               displayName(from),
			ToUserID:               to,
			ToName:                 displayName(to),
			CommentID:              c.ID,
			ParentCommentID:        parent.ID,
			CommentExcerpt:         commentSnippet(c.Content),
			ParentExcerpt:          commentSnippet(parent.Content),
			CreatedMicro:           effMicro(c),
			EstimatedTargetID:      estID,
			EstimatedTargetExcerpt: estExcerpt,
			IsEstimated:            isEst,
		})
		pairCount[itoa(from)+"->"+itoa(to)]++
	}

	// 过滤到某评论者（下钻）
	if filterUserID > 0 {
		filtered := edges[:0]
		for _, e := range edges {
			if e.FromUserID == filterUserID || e.ToUserID == filterUserID {
				filtered = append(filtered, e)
			}
		}
		edges = filtered
		// 重算节点统计（仅相关用户）
		nodeStat = map[uint]*agg{}
		for _, e := range edges {
			getStat(e.FromUserID).out++
			getStat(e.ToUserID).in++
		}
	}

	// 节点
	nodes := make([]NetworkNode, 0, len(nodeStat))
	for uid, st := range nodeStat {
		nodes = append(nodes, NetworkNode{
			UserID:       uid,
			Username:     nameOf[uid].username,
			Nickname:     nameOf[uid].nickname,
			CommentCount: len(byUser[uid]),
			InDegree:     st.in,
			OutDegree:    st.out,
		})
	}

	// 聚合对（用于关系图边 + 榜单）
	type pairKey struct{ from, to uint }
	pairMap := map[pairKey]int{}
	for k, n := range pairCount {
		parts := strings.SplitN(k, "->", 2)
		pairMap[pairKey{atoi(parts[0]), atoi(parts[1])}] = n
	}
	pairs := make([]NetworkPair, 0, len(pairMap))
	for k, n := range pairMap {
		pairs = append(pairs, NetworkPair{
			FromUserID: k.from,
			FromName:   displayName(k.from),
			ToUserID:   k.to,
			ToName:     displayName(k.to),
			Count:      n,
			Suspicious: n >= SuspiciousPairThreshold,
		})
	}
	// 按互动次数倒序，取前 30
	sortPairsDesc(pairs)
	if len(pairs) > 30 {
		pairs = pairs[:30]
	}

	// 详细边按时间倒序（最新互动在前），过量截断保护
	sortEdgesDesc(edges)
	if len(edges) > 1000 {
		edges = edges[:1000]
	}

	// 评论者计数：参与评论的总人数
	commenterCount := len(byUser)
	totalComments := len(comments)

	return &CommentNetwork{
		TargetType:       targetType,
		TargetID:         targetID,
		TotalComments:    totalComments,
		CommenterCount:   commenterCount,
		InteractionCount: len(edges),
		Nodes:            nodes,
		Edges:            edges,
		TopPairs:         pairs,
	}, nil
}

func itoa(u uint) string {
	if u == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for u > 0 {
		i--
		buf[i] = byte('0' + u%10)
		u /= 10
	}
	return string(buf[i:])
}

func atoi(s string) uint {
	var n uint
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			n = n*10 + uint(s[i]-'0')
		}
	}
	return n
}

func sortPairsDesc(p []NetworkPair) {
	for i := 0; i < len(p); i++ {
		for j := i + 1; j < len(p); j++ {
			if p[j].Count > p[i].Count {
				p[i], p[j] = p[j], p[i]
			}
		}
	}
}

func sortEdgesDesc(e []NetworkEdge) {
	for i := 0; i < len(e); i++ {
		for j := i + 1; j < len(e); j++ {
			if e[j].CreatedMicro > e[i].CreatedMicro {
				e[i], e[j] = e[j], e[i]
			}
		}
	}
}
