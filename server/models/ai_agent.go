package models

import (
	"strings"
	"time"

	"nvs-server/utils"
)

// ==================== AI 智能体预设（§5.4） ====================
// 智能体 = 系统提示词预设（人设）：用户点卡即带预设开会话。
// 内置默认种子（首次初始化写入），站长可在后台增删改。

// AiAgent 智能体预设
type AiAgent struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Name         string    `gorm:"size:64;not null" json:"name"`
	Emoji        string    `gorm:"size:8;default:''" json:"emoji"`
	Description  string    `gorm:"size:256;default:''" json:"description"`
	SystemPrompt string    `gorm:"type:text" json:"system_prompt"`
	Greeting     string    `gorm:"size:256;default:''" json:"greeting"` // 开场白
	Category     string    `gorm:"size:32;default:'';index" json:"category"` // 分类（写作/编程/绘画/翻译…）
	UsedCount    int64     `gorm:"not null;default:0" json:"used_count"`      // 使用次数（市场排序用）
	// 上游类型：空/llm = 平台模型池；coze = 扣子（Coze）官方智能体
	Upstream  string `gorm:"size:16;not null;default:''" json:"upstream"`
	BotID     string `gorm:"size:64;default:''" json:"bot_id"`       // Coze bot_id
	CozeBase  string `gorm:"size:128;default:''" json:"coze_base"`   // api.coze.cn / api.coze.com
	TokenEnc  string `gorm:"type:text" json:"-"`                     // Coze PAT（加密存储，只写不读）
	Builtin      bool      `gorm:"not null;default:false" json:"builtin"` // 内置种子（禁止删除标识，仅提示）
	Enabled      bool      `gorm:"not null;default:true" json:"enabled"`
	SortOrder    int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (AiAgent) TableName() string { return "ai_agents" }

// ListAiAgents 智能体列表（管理员全部 / 用户仅启用）
func ListAiAgents(onlyEnabled bool) ([]AiAgent, error) {
	var list []AiAgent
	q := DB.Order("sort_order ASC, id ASC")
	if onlyEnabled {
		q = q.Where("enabled = ?", true)
	}
	err := q.Find(&list).Error
	return list, err
}

// GetAiAgent 取智能体
func GetAiAgent(id uint) (*AiAgent, error) {
	var a AiAgent
	if err := DB.Where("id = ?", id).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

// CreateAiAgent 创建智能体
func CreateAiAgent(a *AiAgent) error {
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" {
		return errAiAgentNameEmpty
	}
	now := time.Now()
	a.CreatedAt = now
	a.UpdatedAt = now
	return DB.Create(a).Error
}

var errAiAgentNameEmpty = &aiAgentError{"名称不能为空"}

type aiAgentError struct{ msg string }

func (e *aiAgentError) Error() string { return e.msg }

// UpdateAiAgentFields map 精确更新（零值安全）
func UpdateAiAgentFields(id uint, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now()
	return DB.Model(&AiAgent{}).Where("id = ?", id).Updates(fields).Error
}

// DeleteAiAgent 删除智能体
func DeleteAiAgent(id uint) error {
	return DB.Where("id = ?", id).Delete(&AiAgent{}).Error
}

// SeedDefaultAiAgents 首次初始化内置智能体（仅当表为空时）
func SeedDefaultAiAgents() {
	var n int64
	DB.Model(&AiAgent{}).Count(&n)
	if n > 0 {
		return
	}
	defaults := []AiAgent{
		{
			Name: "写作教练", Emoji: "✍️", SortOrder: 1, Builtin: true,
			Description:  "帮你打磨结构、对白与节奏的小说创作顾问",
			SystemPrompt: "你是一位经验丰富的中文小说写作教练，熟悉类型文学的叙事结构、人物弧光与节奏把控。请用直接、具体、可操作的建议回应用户的创作问题，避免空泛的鼓励。",
			Greeting:     "把你想聊的情节、人物或卡文点告诉我，我们一起拆解。",
		},
		{
			Name: "世界观设定师", Emoji: "🌍", SortOrder: 2, Builtin: true,
			Description:  "构建自洽的世界观：力量体系、社会结构、历史脉络",
			SystemPrompt: "你是一位擅长世界观构建的设定师。请帮助用户把模糊的灵感发展为逻辑自洽、有细节、可延展的设定体系，并主动指出设定间的潜在冲突。",
			Greeting:     "想先从力量体系、社会结构还是历史脉络开始？",
		},
		{
			Name: "毒舌评论家", Emoji: "🎭", SortOrder: 3, Builtin: true,
			Description:  "以苛刻但专业的视角审阅你的文字",
			SystemPrompt: "你是一位言辞犀利但专业严谨的文学评论家。请指出用户文本中的具体问题（节奏、视角、信息密度、陈词滥调），给出修改示范，但保持建设性。",
			Greeting:     "把文字贴上来吧，我会毫不留情地挑刺——但附上解法。",
		},
		{
			Name: "资料考据助手", Emoji: "📚", SortOrder: 4, Builtin: true,
			Description:  "查证历史、科学与文化细节，标注不确定处",
			SystemPrompt: "你是一位严谨的资料考据助手。回答时区分「确证事实」与「存疑推测」，对不确定的内容明确标注，并提示用户需要进一步核验的方向。",
			Greeting:     "需要考据什么？历史、科学还是文化习俗？",
		},
	}
	for i := range defaults {
		_ = DB.Create(&defaults[i])
	}
}

// TouchAiAgent 累加使用次数（对话成功后调用）
func TouchAiAgent(id uint) {
	if id == 0 {
		return
	}
	DB.Model(&AiAgent{}).Where("id = ?", id).UpdateColumn("used_count", DB.Raw("used_count + 1"))
}

// ListAiAgentCategories 去重分类（仅启用）
func ListAiAgentCategories() []string {
	var cats []string
	DB.Model(&AiAgent{}).Where("enabled = ? AND category <> ''", true).
		Distinct().Order("category ASC").Pluck("category", &cats)
	return cats
}

// IsCoze 是否为扣子智能体
func (a *AiAgent) IsCoze() bool { return strings.EqualFold(strings.TrimSpace(a.Upstream), "coze") }

// SetCozeToken 加密保存 Coze 访问令牌（留空表示不修改）
func (a *AiAgent) SetCozeToken(plain string) error {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return nil
	}
	enc, err := utils.EncryptField(plain)
	if err != nil {
		return err
	}
	return DB.Model(&AiAgent{}).Where("id = ?", a.ID).UpdateColumn("token_enc", enc).Error
}

// DecryptCozeToken 解密 Coze 令牌
func (a *AiAgent) DecryptCozeToken() (string, error) {
	if strings.TrimSpace(a.TokenEnc) == "" {
		return "", nil
	}
	return utils.DecryptField(a.TokenEnc)
}
