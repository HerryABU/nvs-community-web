package ai

import (
	"encoding/json"
	"errors"
	"nvs-server/models"
	"strings"
	"time"
)

// ==================== 用户投稿智能体（提交 → 站长审核 → 广场上架） ====================
//
// 与 AiAgent 的分工：
//   AiAgent       —— **已上架**的智能体（广场里能看到、能点进去对话）
//   AgentSubmission —— **投稿单**：用户提交、等站长审、被拒留痕
//
// 为什么要分两张表而不是给 AiAgent 加个 status：
//   1) 广场查询只要「已上架」，不想每次都带 status 过滤条件，容易漏；
//   2) 被拒的稿子要留档（用户可以改了重投），不能和正式内容混在一起被误上架；
//   3) 审核通过时才落 AiAgent，AiAgent 的 AutoMigrate/索引不受投稿流程影响。

const agentSubmitConfigKey = "agent_submit_config"

// AgentSubmitConfig 投稿开关（站长设置）
type AgentSubmitConfig struct {
	Enabled    bool `json:"enabled"`     // 是否开放用户投稿
	NeedReview bool `json:"need_review"` // 投稿是否需要站长审核（关闭则提交即上架）
	// MaxPending 每人最多几份待审稿（防刷；0 = 不限）
	MaxPending int `json:"max_pending"`
	// MinLevel 最低等级要求（0 = 不限）
	MinLevel int `json:"min_level"`
}

// DefaultAgentSubmitConfig 默认：开放投稿、需要审核、每人最多 3 份待审
func DefaultAgentSubmitConfig() AgentSubmitConfig {
	return AgentSubmitConfig{Enabled: true, NeedReview: true, MaxPending: 3}
}

// GetAgentSubmitConfig 读取投稿开关（配置写坏时回退默认，避免投稿入口 500）
func GetAgentSubmitConfig() AgentSubmitConfig {
	raw := strings.TrimSpace(models.GetPlatformConfig(agentSubmitConfigKey))
	if raw == "" {
		return DefaultAgentSubmitConfig()
	}
	var c AgentSubmitConfig
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return DefaultAgentSubmitConfig()
	}
	if c.MaxPending < 0 {
		c.MaxPending = 0
	}
	return c
}

// SetAgentSubmitConfig 保存投稿开关
func SetAgentSubmitConfig(c AgentSubmitConfig) error {
	if c.MaxPending < 0 {
		return errors.New("待审上限不能为负数")
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return models.SetPlatformConfig(agentSubmitConfigKey, string(b))
}

// AgentSubmission 投稿单
type AgentSubmission struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	UserID       uint   `gorm:"not null;index" json:"user_id"`
	UserName     string `gorm:"size:64;default:''" json:"user_name"` // 冗余：审核列表要显示作者名
	Name         string `gorm:"size:64;not null" json:"name"`
	Emoji        string `gorm:"size:8;default:''" json:"emoji"`
	Description  string `gorm:"size:256;default:''" json:"description"`
	Greeting     string `gorm:"size:256;default:''" json:"greeting"`
	Category     string `gorm:"size:32;default:''" json:"category"`
	SystemPrompt string `gorm:"type:text" json:"system_prompt"` // 投稿的核心内容：特殊提示词
	// Status: pending 待审 / approved 已通过 / rejected 已拒
	Status     string     `gorm:"size:16;not null;default:'pending';index" json:"status"`
	ReviewNote string     `gorm:"size:256;default:''" json:"review_note"`
	ReviewerID uint       `gorm:"not null;default:0" json:"reviewer_id"`
	ReviewedAt *time.Time `json:"reviewed_at"`
	// AgentID 通过后对应的广场智能体 id（改稿重审时用于更新原条目）
	AgentID   uint      `gorm:"not null;default:0" json:"agent_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AgentSubmission) TableName() string { return "agent_submissions" }

// 投稿状态
const (
	AgentSubPending  = "pending"
	AgentSubApproved = "approved"
	AgentSubRejected = "rejected"
)

// CreateAgentSubmission 建投稿单（做字段白名单式清洗与长度校验）
func CreateAgentSubmission(userID uint, userName string, in AgentSubmission) (*AgentSubmission, error) {
	name := strings.TrimSpace(in.Name)
	prompt := strings.TrimSpace(in.SystemPrompt)
	if name == "" {
		return nil, errors.New("请填写智能体名称")
	}
	if len([]rune(name)) > 32 {
		return nil, errors.New("名称最多 32 个字")
	}
	if prompt == "" {
		return nil, errors.New("请填写提示词（这是智能体的核心）")
	}
	if len([]rune(prompt)) > 8000 {
		return nil, errors.New("提示词最多 8000 字")
	}
	if len([]rune(in.Description)) > 200 {
		return nil, errors.New("简介最多 200 个字")
	}
	if len([]rune(in.Greeting)) > 200 {
		return nil, errors.New("开场白最多 200 个字")
	}

	cfg := GetAgentSubmitConfig()
	if !cfg.Enabled {
		return nil, errors.New("本站暂未开放智能体投稿")
	}
	if cfg.MaxPending > 0 {
		var n int64
		models.DB.Model(&AgentSubmission{}).
			Where("user_id = ? AND status = ?", userID, AgentSubPending).Count(&n)
		if n >= int64(cfg.MaxPending) {
			return nil, errors.New("你的待审投稿已达上限，请等站长审完再投")
		}
	}

	sub := &AgentSubmission{
		UserID: userID, UserName: strings.TrimSpace(userName),
		Name: name, Emoji: strings.TrimSpace(in.Emoji),
		Description:  strings.TrimSpace(in.Description),
		Greeting:     strings.TrimSpace(in.Greeting),
		Category:     strings.TrimSpace(in.Category),
		SystemPrompt: prompt,
		Status:       AgentSubPending,
	}
	if err := models.DB.Create(sub).Error; err != nil {
		return nil, err
	}
	return sub, nil
}

// ApproveAgentSubmission 审核通过：落一个广场智能体（或更新已有的），并标记通过。
//
// needReview=false 时用户提交即调它（免审直上），所以它可以独立被调用。
func ApproveAgentSubmission(sub *AgentSubmission, reviewerID uint, note string) error {
	if sub == nil {
		return errors.New("投稿不存在")
	}
	if sub.Status == AgentSubApproved {
		return errors.New("该投稿已通过，无需重复审核")
	}

	now := time.Now()
	agent := &AiAgent{
		Name: sub.Name, Emoji: sub.Emoji, Description: sub.Description,
		Greeting: sub.Greeting, Category: sub.Category,
		SystemPrompt: sub.SystemPrompt,
		Enabled:      true,
		SubmittedBy:  sub.UserID,
	}
	var err error
	if sub.AgentID > 0 {
		// 改稿重审：更新原条目，避免广场里出现两个同名智能体
		fields := map[string]interface{}{
			"name": agent.Name, "emoji": agent.Emoji, "description": agent.Description,
			"greeting": agent.Greeting, "category": agent.Category,
			"system_prompt": agent.SystemPrompt, "enabled": true,
		}
		err = models.DB.Model(&AiAgent{}).Where("id = ?", sub.AgentID).Updates(fields).Error
		if err == nil {
			agent.ID = sub.AgentID
		}
	} else {
		err = models.DB.Create(agent).Error
	}
	if err != nil {
		return err
	}

	upd := map[string]interface{}{
		"status": AgentSubApproved, "review_note": strings.TrimSpace(note),
		"reviewer_id": reviewerID, "reviewed_at": &now, "agent_id": agent.ID,
	}
	if err := models.DB.Model(&AgentSubmission{}).Where("id = ?", sub.ID).Updates(upd).Error; err != nil {
		return err
	}
	// 同步内存对象：调用方常拿着同一个结构体连着做判断，
	// 只更新库不回写会让「重复审核」检查失效（真踩过：广场出现两个同名条目）。
	sub.Status = AgentSubApproved
	sub.ReviewNote = strings.TrimSpace(note)
	sub.ReviewerID = reviewerID
	sub.ReviewedAt = &now
	sub.AgentID = agent.ID
	return nil
}

// RejectAgentSubmission 审核拒绝（留痕，用户可改后重投）
func RejectAgentSubmission(sub *AgentSubmission, reviewerID uint, note string) error {
	if sub == nil {
		return errors.New("投稿不存在")
	}
	if sub.Status == AgentSubRejected {
		return errors.New("该投稿已被拒绝")
	}
	now := time.Now()
	if err := models.DB.Model(&AgentSubmission{}).Where("id = ?", sub.ID).Updates(map[string]interface{}{
		"status": AgentSubRejected, "review_note": strings.TrimSpace(note),
		"reviewer_id": reviewerID, "reviewed_at": &now,
	}).Error; err != nil {
		return err
	}
	sub.Status = AgentSubRejected
	sub.ReviewNote = strings.TrimSpace(note)
	sub.ReviewerID = reviewerID
	sub.ReviewedAt = &now
	return nil
}

// ListAgentSubmissions 审核列表（status 空 = 全部，按新到旧）
func ListAgentSubmissions(status string, page, size int) ([]AgentSubmission, int64) {
	q := models.DB.Model(&AgentSubmission{})
	if s := strings.TrimSpace(status); s != "" {
		q = q.Where("status = ?", s)
	}
	var total int64
	q.Count(&total)
	var list []AgentSubmission
	q.Order("id DESC").Limit(size).Offset((page - 1) * size).Find(&list)
	return list, total
}

// CountPendingAgentSubmissions 待审数量（后台红点用）
func CountPendingAgentSubmissions() int64 {
	var n int64
	models.DB.Model(&AgentSubmission{}).Where("status = ?", AgentSubPending).Count(&n)
	return n
}
