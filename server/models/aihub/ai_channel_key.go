package aihub

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"nvs-server/utils"

	"gorm.io/gorm"
	"nvs-server/models"
)

// ==================== AI 渠道卡密池（按渠道，而不是按模型） ====================
//
// 为什么从「按模型」改成「按渠道」：
// 一条渠道（base_url）下的几十个模型，实际用的是**同一个账户的同一批密钥**。
// 按模型存会导致：
//   · 同一个 key 在 N 个模型里各存一份密文，轮询各自计数 —— 上游看到的是同一个 key
//     被同时用于多个「池」，熔断阈值形同虚设；
//   · 换一把 key 要逐个模型改，几十次操作；
//   · 余额/卡密统计按模型汇总，回答不了「这条渠道还剩多少张卡可用」。
//
// 因此本表以 base_url 为归属，模型只是「用哪条渠道」。
// 兼容策略：渠道池为空时，回退老的「模型密钥池」→ 再回退模型自带 ApiKeyEnc，
// 老站点升级后不改任何配置也能继续跑。
//
// 与「充值卡密」（card_keys）是两回事：那是发给用户兑换代币的码，本表是上游 API 密钥。

// AiChannelKey 渠道卡密（上游 API Key）
type AiChannelKey struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	BaseURL string `gorm:"size:255;not null;index:idx_channel_key_url" json:"base_url"`
	Label   string `gorm:"size:64;default:''" json:"label"`
	KeyEnc  string `gorm:"type:text" json:"-"` // 密文，只写不读
	// Fingerprint 明文密钥的 sha256，仅用于同渠道判重（不落明文、不可逆）
	Fingerprint string `gorm:"size:64;not null;default:'';index:idx_channel_key_url,unique" json:"-"`
	Weight      int    `gorm:"not null;default:1" json:"weight"`
	Enabled     bool   `gorm:"not null;default:true" json:"enabled"`
	FailCount   int    `gorm:"not null;default:0" json:"fail_count"`
	// SuccessTotal / FailTotal 累计计数：失败清零不影响这两个数，
	// 便于判断「这把 key 是不是长期不稳」而不是「刚好连挂三次」
	SuccessTotal int64      `gorm:"not null;default:0" json:"success_total"`
	FailTotal    int64      `gorm:"not null;default:0" json:"fail_total"`
	LastFailAt   *time.Time `json:"last_fail_at"`
	LastUsedAt   *time.Time `json:"last_used_at"`
	Note         string     `gorm:"size:255;default:''" json:"note"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (AiChannelKey) TableName() string { return "ai_channel_keys" }

// FingerprintKey 计算密钥指纹（判定「同一个 key」用）。
func FingerprintKey(plain string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(plain)))
	return hex.EncodeToString(sum[:])
}

// ListAiChannelKeys 某渠道的卡密池（不含明文）
func ListAiChannelKeys(baseURL string) ([]AiChannelKey, error) {
	var list []AiChannelKey
	err := models.DB.Where("base_url = ?", normalizeChannelURL(baseURL)).Order("id ASC").Find(&list).Error
	return list, err
}

// GetAiChannelKey 取一条（限定渠道，避免越权按 id 乱改其它渠道）
func GetAiChannelKey(id uint, baseURL string) (*AiChannelKey, error) {
	var k AiChannelKey
	if err := models.DB.Where("id = ? AND base_url = ?", id, normalizeChannelURL(baseURL)).First(&k).Error; err != nil {
		return nil, err
	}
	return &k, nil
}

// AddAiChannelKey 新增卡密；同渠道下重复（指纹相同）直接报错，避免把同一把 key 加两遍
func AddAiChannelKey(baseURL, label, plain, note string, weight int) (*AiChannelKey, error) {
	base := normalizeChannelURL(baseURL)
	plain = strings.TrimSpace(plain)
	if base == "" {
		return nil, fmt.Errorf("渠道地址不能为空")
	}
	if plain == "" {
		return nil, fmt.Errorf("密钥不能为空")
	}
	if len(plain) < 8 {
		return nil, fmt.Errorf("密钥过短，请检查是否粘贴完整")
	}
	if weight <= 0 {
		weight = 1
	}
	fp := FingerprintKey(plain)

	var dup int64
	models.DB.Model(&AiChannelKey{}).Where("base_url = ? AND fingerprint = ?", base, fp).Count(&dup)
	if dup > 0 {
		return nil, fmt.Errorf("该渠道下已有同一把密钥（按内容判重），无需重复添加")
	}

	enc, err := utils.EncryptField(plain)
	if err != nil {
		return nil, err
	}
	k := &AiChannelKey{
		BaseURL: base, Label: strings.TrimSpace(label), KeyEnc: enc, Fingerprint: fp,
		Weight: weight, Enabled: true, Note: strings.TrimSpace(note),
	}
	if err := models.DB.Create(k).Error; err != nil {
		return nil, err
	}
	return k, nil
}

// UpdateAiChannelKey 更新标签/权重/启用状态；启用时把连续失败计数清零（站长手动恢复）
func UpdateAiChannelKey(id uint, label *string, weight *int, enabled *bool) error {
	fields := map[string]interface{}{"updated_at": time.Now()}
	if label != nil {
		fields["label"] = strings.TrimSpace(*label)
	}
	if weight != nil {
		w := *weight
		if w <= 0 {
			w = 1
		}
		fields["weight"] = w
	}
	if enabled != nil {
		fields["enabled"] = *enabled
		if *enabled {
			fields["fail_count"] = 0
		}
	}
	return models.DB.Model(&AiChannelKey{}).Where("id = ?", id).Updates(fields).Error
}

// DeleteAiChannelKey 删除一把卡密
func DeleteAiChannelKey(id uint) error {
	return models.DB.Delete(&AiChannelKey{}, id).Error
}

// DecryptAiChannelKey 解密卡密明文（调用方用后即焚，不得落日志）
func DecryptAiChannelKey(k *AiChannelKey) (string, error) {
	if k == nil || k.KeyEnc == "" {
		return "", fmt.Errorf("卡密为空")
	}
	return utils.DecryptField(k.KeyEnc)
}

// pickChannelCandidates 可用卡密：启用 + 未达熔断阈值 + 权重 > 0
func pickChannelCandidates(baseURL string) []AiChannelKey {
	var list []AiChannelKey
	models.DB.Where("base_url = ? AND enabled = ? AND fail_count < ? AND weight > 0",
		normalizeChannelURL(baseURL), true, MaxFailBeforeDisable).Find(&list)
	return list
}

// PickAiChannelKey 按权重随机取一把可用卡密；池内无可用返回 nil
func PickAiChannelKey(baseURL string) *AiChannelKey {
	list := pickChannelCandidates(baseURL)
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

// ReportAiChannelKey 上报卡密使用结果：成功清零连续失败，失败累加并在达阈值时自动停用。
// 累计计数（SuccessTotal / FailTotal）不会因为清零而丢失，用来判断长期稳定性。
func ReportAiChannelKey(keyID uint, ok bool) {
	if keyID == 0 {
		return
	}
	now := time.Now()
	if ok {
		models.DB.Model(&AiChannelKey{}).Where("id = ?", keyID).Updates(map[string]interface{}{
			"fail_count": 0, "last_used_at": now, "updated_at": now,
			"success_total": gorm.Expr("success_total + ?", 1),
		})
		return
	}
	var k AiChannelKey
	if err := models.DB.Where("id = ?", keyID).First(&k).Error; err != nil {
		return
	}
	fails := k.FailCount + 1
	fields := map[string]interface{}{
		"fail_count": fails, "last_fail_at": now, "updated_at": now,
		"fail_total": gorm.Expr("fail_total + ?", 1),
	}
	if fails >= MaxFailBeforeDisable {
		fields["enabled"] = false // 自动摘除，避免持续打到坏卡密
	}
	models.DB.Model(&AiChannelKey{}).Where("id = ?", keyID).Updates(fields)
}

// ChannelKeyStats 渠道卡密池概况（按渠道视图/余额弹窗用）
type ChannelKeyStats struct {
	Total    int64 `json:"total"`
	Enabled  int64 `json:"enabled"`
	Disabled int64 `json:"disabled"` // 含达熔断自动停用的
}

// GetChannelKeyStats 批量统计多个渠道的卡密数量（避免 N+1 查询）
func GetChannelKeyStats() map[string]ChannelKeyStats {
	out := map[string]ChannelKeyStats{}
	type row struct {
		BaseURL string
		Total   int64
		Enabled int64
	}
	var rows []row
	models.DB.Model(&AiChannelKey{}).
		Select("base_url, COUNT(*) as total, SUM(CASE WHEN enabled THEN 1 ELSE 0 END) as enabled").
		Group("base_url").Scan(&rows)
	for _, r := range rows {
		out[r.BaseURL] = ChannelKeyStats{
			Total: r.Total, Enabled: r.Enabled, Disabled: r.Total - r.Enabled,
		}
	}
	return out
}

// MigrateModelKeysToChannels 把老配置（按模型存的密钥）迁到渠道卡密池。
//
// 幂等：按 (base_url, 指纹) 判重，重复启动不会重复插入。
// 只读不改老数据：models.AiModel.ApiKeyEnc 与 ai_model_keys 保持原样，
// 作为渠道池为空时的兜底 —— 升级后行为不变，但新配置都走渠道池。
//
// 返回新增条数，便于启动日志里给个确定结论。
func MigrateModelKeysToChannels() (int, error) {
	if models.DB == nil {
		return 0, nil
	}
	var list []models.AiModel
	if err := models.DB.Where("base_url <> ''").Find(&list).Error; err != nil {
		return 0, err
	}
	added := 0
	for i := range list {
		m := &list[i]
		base := normalizeChannelURL(m.BaseURL)
		if base == "" {
			continue
		}
		// 1) 模型自带 key（单密钥老配置）
		if plain, err := m.DecryptApiKey(); err == nil && strings.TrimSpace(plain) != "" {
			if ok, _ := ensureChannelKey(base, plain, "迁移自模型自带密钥"); ok {
				added++
			}
		}
		// 2) 按模型的密钥池
		var keys []AiModelKey
		models.DB.Where("model_id = ?", m.ID).Find(&keys)
		for j := range keys {
			plain, err := DecryptAiModelKey(&keys[j])
			if err != nil || strings.TrimSpace(plain) == "" {
				continue
			}
			label := keys[j].Label
			if label == "" {
				label = "迁移自模型密钥池"
			}
			if ok, _ := ensureChannelKey(base, plain, label); ok {
				added++
			}
		}
	}
	return added, nil
}

// ensureChannelKey 不存在才插入（按指纹判重）。返回是否真的新增。
func ensureChannelKey(base, plain, label string) (bool, error) {
	fp := FingerprintKey(plain)
	var n int64
	models.DB.Model(&AiChannelKey{}).Where("base_url = ? AND fingerprint = ?", base, fp).Count(&n)
	if n > 0 {
		return false, nil
	}
	enc, err := utils.EncryptField(plain)
	if err != nil {
		return false, err
	}
	k := &AiChannelKey{
		BaseURL: base, Label: strings.TrimSpace(label), KeyEnc: enc, Fingerprint: fp,
		Weight: 1, Enabled: true, Note: "自动迁移",
	}
	if err := models.DB.Create(k).Error; err != nil {
		return false, err
	}
	return true, nil
}

// normalizeChannelURL 统一渠道地址：去尾斜杠、转小写。
// 判重与分组都依赖它，否则 "https://API.x.com/v1/" 与 "https://api.x.com/v1" 会分成两条渠道。
func normalizeChannelURL(u string) string {
	return strings.ToLower(strings.TrimRight(strings.TrimSpace(u), "/"))
}
