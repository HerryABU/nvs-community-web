package models

import (
	"encoding/json"
	"strings"
	"time"
)

// ==================== 评论区策略（楼加密的连带范围）===================
//
// 「楼加密」这个功能本身有两种截然不同的语义，用户感受天差地别：
//
//	A. 密楼 + 连带评论区
//	   楼未解锁时，楼正文显示 🔒 占位，且该楼下的全部楼内回复一并不返回。
//	   语义是「这整块对话是一次私密的定向发言」，外人连读都不许读。
//	   适合：定向回复作者本人、小范围私密讨论。
//
//	B. 密楼 + 评论区照常（默认）
//	   楼未解锁时，只把楼正文本身收起来；楼内回复（评论区）照常可见可读。
//	   语义是「我只锁住我这一条发言的内容，底下大伙的讨论照旧公开」。
//   适合：绝大多数场景 —— 加密的是「我说了什么」，不是「大家聊了什么」。
//
// 现实里 B 才是常态诉求：楼加密是「入口限制」（防止未成年人/陌生人看到我的私密内容），
// 而不是「把这一整串对话从世界上抹掉」。如果连带把别人的回复也藏了，
// 会出现「回复者本人说的话，作者自己却看不见」这种荒唐局面 ——
// 作者设密码是为了藏自己的内容，结果连别人对他的回应一起藏了。
//
// 所以这里做成**管理员可选**：两种都保留，站长按站点调性决定。
// 默认 B（评论区不连带加密），与「楼加密 ≠ 评论区加密」的产品直觉一致。

// CommentPolicy 评论区策略配置
type CommentPolicy struct {
	// LockHidesReplies 楼加密时是否连带隐藏楼内回复。
	//   false（默认）= 只锁楼正文，楼内回复照常可见（评论区不加密）
	//   true          = 连楼内回复一起藏（整块对话私密）
	//
	// 注意：这是「保留旧行为」的开关，不是新增行为 —— 老站点原本就是 true 语义，
	// 但那是从「把内容全藏起来」的思路推出来的默认值，不是用户明确表达的意愿。
	// 这里改成显式可选，默认 false。
	LockHidesReplies bool `json:"lock_hides_replies"`

	// LockedFloorCanReply 未解锁的楼是否允许回复。
	//   false（默认）= 未解锁时该楼只读（不提示密码，因为回帖区本来就公开）
	//   true          = 未解锁也能回复（回复不携带密码校验）
	//
	// 与 LockHidesReplies 的关系：前者管「能不能读」，这里管「能不能写」。
	// 两者独立：选 B 语义（回复可见）时，未解锁用户看到的是别人的回复，
	// 但没看过楼正文，让 ta 参与讨论会变成盲回，故默认禁止回复更合理。
	LockedFloorCanReply bool `json:"locked_floor_can_reply"`

	UpdatedAt time.Time `json:"updated_at"`
}

const commentPolicyCfgKey = "comment_policy"

// DefaultCommentPolicy 默认：楼加密只锁楼正文，楼内回复（评论区）照常可见；
// 未解锁的楼只读，不允许盲回。
func DefaultCommentPolicy() CommentPolicy {
	return CommentPolicy{
		LockHidesReplies:    false,
		LockedFloorCanReply: false,
	}
}

// LoadCommentPolicy 读取评论区策略（缺失或损坏时回退默认值）
func LoadCommentPolicy() CommentPolicy {
	out := DefaultCommentPolicy()
	raw := GetPlatformConfig(commentPolicyCfgKey)
	if strings.TrimSpace(raw) == "" {
		return out
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return DefaultCommentPolicy()
	}
	return out
}

// SaveCommentPolicy 保存评论区策略
func SaveCommentPolicy(p CommentPolicy) error {
	p.UpdatedAt = time.Now()
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return SetPlatformConfig(commentPolicyCfgKey, string(b))
}

// LockHidesFloorReplies 当前策略下，楼加密是否连带隐藏楼内回复。
// 单独包一层是为了给「策略没读到」的场景一个安全的默认（不隐藏）。
func LockHidesFloorReplies() bool {
	return LoadCommentPolicy().LockHidesReplies
}
