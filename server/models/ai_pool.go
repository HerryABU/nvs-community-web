package models

import (
	"strings"
	"time"

	"nvs-server/utils"
)

// ==================== AI 模型池（§5.3 计费底座） ====================
// AiModel 描述一个 OpenAI 兼容上游：base_url + api_key（AES-256-GCM 加密落库）+
// 每 1K tokens 的代币单价（0 = 免费）。管理端 CRUD，用户端只读可用列表。
//
// 零值纪律：enabled=false / price=0 均为合法业务值，更新一律走 map 精确写，
// 不用 struct Save（避免零值被 GORM 跳过或误判）。

// AiModel AI 上游模型（OpenAI 兼容）
type AiModel struct {
	ID          uint    `gorm:"primaryKey" json:"id"`
	Name        string  `gorm:"size:64;not null" json:"name"`                    // 展示名（如「默认模型」）
	Kind        string  `gorm:"size:16;not null;default:chat;index" json:"kind"` // chat/image/video
	ModelID     string  `gorm:"size:128;not null" json:"model_id"`               // 上游模型标识（如 gpt-4o-mini）
	BaseURL     string  `gorm:"size:256;not null" json:"base_url"`               // 上游地址（不含 /chat/completions）
	ApiKeyEnc   string  `gorm:"type:text" json:"-"`                              // api_key 密文（EncryptField），不回传前端
	PriceIn     int64   `gorm:"not null;default:0" json:"price_in"`              // 每 1K 输入 tokens 代币价（0=免费，chat 用）
	PriceOut    int64   `gorm:"not null;default:0" json:"price_out"`             // 每 1K 输出 tokens 代币价（0=免费，chat 用）
	PriceCall   int64   `gorm:"not null;default:0" json:"price_call"`            // 每次调用代币价（image/video 用，0=免费）
	MaxTokens   int     `gorm:"not null;default:2048" json:"max_tokens"`         // 默认最大输出 tokens
	Temperature float64 `gorm:"not null;default:0.7" json:"temperature"`         // 默认温度
	Enabled     bool    `gorm:"not null;default:true" json:"enabled"`            // 是否启用
	Priority    int     `gorm:"not null;default:0" json:"priority"`              // 优先级（大者优先，备用池预留）
	// ApiKind 上游协议族：openai（默认 /v1/images/generations 等）/ mj（Midjourney 代理）/ gemini（generateContent）
	ApiKind string `gorm:"size:16;not null;default:''" json:"api_kind"`
	// ProviderHint 上游归属：domestic（境内）/ overseas（境外）/ 空（未标注）→ 用于合规提示
	ProviderHint string `gorm:"size:16;not null;default:''" json:"provider_hint"`
	// Vendor 厂商（OpenAI/Anthropic/Google/DeepSeek/阿里/字节/Midjourney…）→ 前端「全部厂商」筛选
	Vendor string `gorm:"size:32;not null;default:'';index" json:"vendor"`
	// Thinking 思考型模型（DeepSeek-R1 / GLM / Qwen thinking 等）：
	// 它们的正文可能整段落在 reasoning 字段里，content 为空。
	// 二值标记只用于「给站长一个解释」与前端展示提示，不影响调用逻辑
	//（客户端已自动透传 reasoning 并在 content 为空时兜底）。
	Thinking  bool      `gorm:"not null;default:false" json:"thinking"`
	Note      string    `gorm:"size:256;default:''" json:"note"` // 备注
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AiModel) TableName() string { return "ai_models" }

// ---------- 模型类型 ----------

const (
	AiKindChat  = "chat"
	AiKindImage = "image"
	AiKindVideo = "video"
)

// ---------- 上游协议族（api_kind） ----------
// 同一 kind 下不同厂商协议差异大：绘画需按协议族分发到不同适配器。
const (
	AiApiOpenAI = "openai" // OpenAI 兼容：/v1/images/generations、/v1/video/generations（豆包/gpt-image/多数中转）
	AiApiMJ     = "mj"     // Midjourney 代理协议：/mj/submit/imagine + /mj/task/{id}/fetch（含 Niji）
	AiApiGemini = "gemini" // Google generateContent（nanobanana / gemini-2.5-flash-image）
	AiApiClaude = "claude" // Anthropic Messages API（/v1/messages + x-api-key + anthropic-version）
)

// NormalizeAiApiKind 归一化协议族（空/未知 → openai）
func NormalizeAiApiKind(k string) string {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case AiApiMJ:
		return AiApiMJ
	case AiApiGemini:
		return AiApiGemini
	case AiApiClaude:
		return AiApiClaude
	default:
		return AiApiOpenAI
	}
}

// NormalizeProviderHint 归一化归属标注
func NormalizeProviderHint(h string) string {
	switch strings.ToLower(strings.TrimSpace(h)) {
	case "domestic", "境内":
		return "domestic"
	case "overseas", "境外":
		return "overseas"
	default:
		return ""
	}
}

// NormalizeAiKind 归一化类型（空串按 chat，非法按 chat）
func NormalizeAiKind(kind string) string {
	switch kind {
	case AiKindImage, AiKindVideo:
		return kind
	default:
		return AiKindChat
	}
}

// ---------- 配置开关 ----------

const (
	// AiEnabledKey AI 功能总开关（默认 false，站长在后台开启）
	AiEnabledKey = "ai_enabled"
)

// IsAiEnabled AI 功能是否开启（默认关闭）
func IsAiEnabled() bool {
	return GetPlatformConfig(AiEnabledKey) == "true"
}

// SetAiEnabled 设置 AI 功能开关
func SetAiEnabled(on bool) error {
	if on {
		return SetPlatformConfig(AiEnabledKey, "true")
	}
	return SetPlatformConfig(AiEnabledKey, "false")
}

// ---------- CRUD ----------

// ListAiModels 模型列表（管理员视角：全部；onlyEnabled=true 时仅启用项）
func ListAiModels(onlyEnabled bool) ([]AiModel, error) {
	var list []AiModel
	q := DB.Order("priority DESC, id ASC")
	if onlyEnabled {
		q = q.Where("enabled = ?", true)
	}
	err := q.Find(&list).Error
	return list, err
}

// ListAiModelsByKind 按类型列模型（空 kind 归一化为 chat，兼容旧数据）
func ListAiModelsByKind(kind string, onlyEnabled bool) ([]AiModel, error) {
	kind = NormalizeAiKind(kind)
	var list []AiModel
	q := DB.Order("priority DESC, id ASC")
	if kind == AiKindChat {
		// 旧数据 kind 列为空串（AutoMigrate 前创建），一并视为 chat
		q = q.Where("kind IN ?", []string{AiKindChat, ""})
	} else {
		q = q.Where("kind = ?", kind)
	}
	if onlyEnabled {
		q = q.Where("enabled = ?", true)
	}
	err := q.Find(&list).Error
	return list, err
}

// GetAiModel 按 ID 取模型
func GetAiModel(id uint) (*AiModel, error) {
	var m AiModel
	if err := DB.Where("id = ?", id).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetEnabledAiModel 取一个启用模型（不存在返回 nil, nil）
func GetEnabledAiModel(id uint) (*AiModel, error) {
	m, err := GetAiModel(id)
	if err != nil {
		return nil, err
	}
	if !m.Enabled {
		return nil, nil
	}
	return m, nil
}

// GetEnabledAiModelOfKind 取一个指定类型的启用模型（不存在返回 nil, nil）
func GetEnabledAiModelOfKind(id uint, kind string) (*AiModel, error) {
	m, err := GetEnabledAiModel(id)
	if err != nil || m == nil {
		return nil, err
	}
	if NormalizeAiKind(m.Kind) != NormalizeAiKind(kind) {
		return nil, nil
	}
	return m, nil
}

// CreateAiModel 创建模型；api_key 明文入参，加密落库
func CreateAiModel(m *AiModel, apiKeyPlain string) error {
	enc, err := utils.EncryptField(strings.TrimSpace(apiKeyPlain))
	if err != nil {
		return err
	}
	m.ApiKeyEnc = enc
	now := time.Now()
	m.CreatedAt = now
	m.UpdatedAt = now
	return DB.Create(m).Error
}

// UpdateAiModelFields map 精确更新（零值安全）；apiKeyPlain 非空时才轮换密钥
func UpdateAiModelFields(id uint, fields map[string]interface{}, apiKeyPlain string) error {
	if strings.TrimSpace(apiKeyPlain) != "" {
		enc, err := utils.EncryptField(strings.TrimSpace(apiKeyPlain))
		if err != nil {
			return err
		}
		fields["api_key_enc"] = enc
	}
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now()
	return DB.Model(&AiModel{}).Where("id = ?", id).Updates(fields).Error
}

// DeleteAiModel 删除模型
func DeleteAiModel(id uint) error {
	return DB.Where("id = ?", id).Delete(&AiModel{}).Error
}

// DecryptApiKey 解密模型上游密钥（调用方用后即焚，不得落日志）
func (m *AiModel) DecryptApiKey() (string, error) {
	if m.ApiKeyEnc == "" {
		return "", nil
	}
	return utils.DecryptField(m.ApiKeyEnc)
}

// CountEnabledAiModels 启用模型数量（status 页展示）
func CountEnabledAiModels() (int64, error) {
	var n int64
	err := DB.Model(&AiModel{}).Where("enabled = ?", true).Count(&n).Error
	return n, err
}
