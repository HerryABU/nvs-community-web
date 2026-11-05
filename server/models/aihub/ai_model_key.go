package aihub

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/utils"
)

// ==================== AI 模型密钥池（多密钥轮询） ====================
// 一个模型可挂多把上游客钥：按权重随机取用，失败计数超阈值自动停用，
// 池为空时回退到 models.AiModel.ApiKeyEnc（兼容单密钥老配置）。

// MaxFailBeforeDisable 连续失败达到该次数自动停用该密钥
const MaxFailBeforeDisable = 3

// AiModelKey 模型密钥
type AiModelKey struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	ModelID    uint       `gorm:"not null;index" json:"model_id"`
	Label      string     `gorm:"size:64;default:''" json:"label"`
	KeyEnc     string     `gorm:"type:text" json:"-"` // 密文，只写不读
	Weight     int        `gorm:"not null;default:1" json:"weight"`
	Enabled    bool       `gorm:"not null;default:true" json:"enabled"`
	FailCount  int        `gorm:"not null;default:0" json:"fail_count"`
	LastFailAt *time.Time `json:"last_fail_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (AiModelKey) TableName() string { return "ai_model_keys" }

// ListAiModelKeys 某模型的密钥池（不含明文）
func ListAiModelKeys(modelID uint) ([]AiModelKey, error) {
	var list []AiModelKey
	err := models.DB.Where("model_id = ?", modelID).Order("id ASC").Find(&list).Error
	return list, err
}

// GetAiModelKey 取一把密钥
func GetAiModelKey(id, modelID uint) (*AiModelKey, error) {
	var k AiModelKey
	if err := models.DB.Where("id = ? AND model_id = ?", id, modelID).First(&k).Error; err != nil {
		return nil, err
	}
	return &k, nil
}

// AddAiModelKey 新增密钥（加密存储；weight 越小权重越低，1 起）
func AddAiModelKey(modelID uint, label, plain string, weight int) (*AiModelKey, error) {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return nil, fmt.Errorf("密钥不能为空")
	}
	if len(plain) < 8 {
		return nil, fmt.Errorf("密钥过短，请检查是否粘贴完整")
	}
	if weight <= 0 {
		weight = 1
	}
	if weight > 100 {
		weight = 100
	}
	enc, err := utils.EncryptField(plain)
	if err != nil {
		return nil, err
	}
	k := &AiModelKey{
		ModelID: modelID, Label: strings.TrimSpace(label), KeyEnc: enc,
		Weight: weight, Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := models.DB.Create(k).Error; err != nil {
		return nil, err
	}
	return k, nil
}

// DeleteAiModelKey 删除密钥
func DeleteAiModelKey(id, modelID uint) error {
	res := models.DB.Where("id = ? AND model_id = ?", id, modelID).Delete(&AiModelKey{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("密钥不存在")
	}
	return nil
}

// UpdateAiModelKey 更新标签/权重/启停/重置失败计数
func UpdateAiModelKey(id, modelID uint, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now()
	res := models.DB.Model(&AiModelKey{}).Where("id = ? AND model_id = ?", id, modelID).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("密钥不存在")
	}
	return nil
}

// DecryptAiModelKey 解密
func DecryptAiModelKey(k *AiModelKey) (string, error) {
	if strings.TrimSpace(k.KeyEnc) == "" {
		return "", fmt.Errorf("密钥为空")
	}
	return utils.DecryptField(k.KeyEnc)
}

// pickCandidates 可用候选（启用且未达失败阈值、权重>0）
func pickCandidates(modelID uint) []AiModelKey {
	var list []AiModelKey
	models.DB.Where("model_id = ? AND enabled = ? AND fail_count < ? AND weight > 0",
		modelID, true, MaxFailBeforeDisable).Find(&list)
	return list
}

// PickAiModelKey 按权重随机取一把可用密钥；池内无可用返回 nil
func PickAiModelKey(modelID uint) *AiModelKey {
	list := pickCandidates(modelID)
	if len(list) == 0 {
		return nil
	}
	total := 0
	for i := range list {
		total += list[i].Weight
	}
	if total <= 0 {
		return &list[0]
	}
	r := rand.Intn(total)
	for i := range list {
		r -= list[i].Weight
		if r < 0 {
			return &list[i]
		}
	}
	return &list[len(list)-1]
}

// ReportAiModelKey 上报密钥使用结果：成功清零失败计数，失败累加并在达阈值时自动停用
func ReportAiModelKey(keyID uint, ok bool) {
	if keyID == 0 {
		return
	}
	now := time.Now()
	if ok {
		models.DB.Model(&AiModelKey{}).Where("id = ?", keyID).
			Updates(map[string]interface{}{"fail_count": 0, "last_used_at": now, "updated_at": now})
		return
	}
	var k AiModelKey
	if err := models.DB.Where("id = ?", keyID).First(&k).Error; err != nil {
		return
	}
	fails := k.FailCount + 1
	fields := map[string]interface{}{
		"fail_count": fails, "last_fail_at": now, "updated_at": now,
	}
	if fails >= MaxFailBeforeDisable {
		fields["enabled"] = false // 自动摘除，避免持续打到坏密钥
	}
	models.DB.Model(&AiModelKey{}).Where("id = ?", keyID).Updates(fields)
}

// ModelKeyLease 一次调用占用的密钥（含来源信息，便于上报与排查）
type ModelKeyLease struct {
	Key string
	// KeyID 模型密钥池的 id（老表，兼容保留）
	KeyID uint
	// ChannelKeyID 渠道卡密池的 id —— 现在的首选来源
	ChannelKeyID uint
	Source       string // channel_pool / pool / model
}

// OK 上报成功
func (l *ModelKeyLease) OK() {
	if l == nil {
		return
	}
	if l.ChannelKeyID > 0 {
		ReportAiChannelKey(l.ChannelKeyID, true)
	}
	if l.KeyID > 0 {
		ReportAiModelKey(l.KeyID, true)
	}
}

// Fail 上报失败
func (l *ModelKeyLease) Fail() {
	if l == nil {
		return
	}
	if l.ChannelKeyID > 0 {
		ReportAiChannelKey(l.ChannelKeyID, false)
	}
	if l.KeyID > 0 {
		ReportAiModelKey(l.KeyID, false)
	}
}

// AcquireModelKey 取一把可用密钥。
//
// 顺序（改动要点）：**渠道卡密池 → 模型密钥池 → 模型自带密钥**。
// 为什么渠道优先：同一条 base_url 下的模型共用同一批上游密钥，
// 按渠道池化才能让「轮询 / 熔断 / 停用」真正生效（按模型各算一套等于没有熔断）；
// 后两级是升级前的老配置，保留它老站点不用改任何设置也能继续跑。
func AcquireModelKey(m *models.AiModel) (*ModelKeyLease, error) {
	if m == nil {
		return nil, fmt.Errorf("模型不存在")
	}
	if ck := PickAiChannelKey(m.BaseURL); ck != nil {
		if plain, err := DecryptAiChannelKey(ck); err != nil {
			// 单张卡密解密失败不阻断整体：计入失败并继续往下找
			ReportAiChannelKey(ck.ID, false)
		} else if strings.TrimSpace(plain) != "" {
			return &ModelKeyLease{Key: plain, ChannelKeyID: ck.ID, Source: "channel_pool"}, nil
		}
	}
	if k := PickAiModelKey(m.ID); k != nil {
		plain, err := DecryptAiModelKey(k)
		if err != nil {
			// 单把解密失败不应阻断整体：计入失败并回退
			ReportAiModelKey(k.ID, false)
		} else {
			return &ModelKeyLease{Key: plain, KeyID: k.ID, Source: "pool"}, nil
		}
	}
	plain, err := m.DecryptApiKey()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(plain) == "" {
		return nil, fmt.Errorf("模型未配置密钥（密钥池为空且无兜底密钥）")
	}
	return &ModelKeyLease{Key: plain, Source: "model"}, nil
}
