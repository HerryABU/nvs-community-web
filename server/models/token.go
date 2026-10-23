package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// sha256Hex 计算字符串的 sha256 十六进制摘要
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// ==================== 代币账本 / 卡密充值（需求 §7） ====================
//
// 定位（§7.1）：代币是「平台内消费积分」——不可提现、不可转账、不可兑换现金；
// 与作者收益账（earnings_records / withdrawal_requests，法币、可提现）**两套账、不混用**。
//
// 账本纪律（§7.3）：代币账本可查、可对账、不可篡改——
//   - token_ledger 为 append-only 流水，每笔带 balance_after（快照余额），只增不改不删；
//   - user_token_balances 为缓存余额，与流水在同一事务内更新，供快速读取；
//   - ReconcileTokenBalance 可由流水重算余额，用于对账纠偏。
//
// 卡密纪律（§7.2）：随机生成、**哈希存储**（sha256，不明文落库）、一次性展示、渠道可控、
// 有效期与作废规则明确。兑换原子占用（条件更新 + RowsAffected 校验）防并发重兑。

// ---------- 配置键 ----------

const (
	// TokenEnabledKey 代币系统总开关（默认 false，站长在后台开启）
	TokenEnabledKey = "token_enabled"
	// TokenNameKey 代币展示名（默认「星币」）
	TokenNameKey = "token_name"
	// DefaultTokenName 默认代币名
	DefaultTokenName = "星币"
)

// ---------- 流水类型 ----------

const (
	TokenTypeRechargeCard = "recharge_card" // 卡密充值
	TokenTypeRechargePay  = "recharge_pay"  // 支付通道充值（微信/支付宝，后续）
	TokenTypeConsume      = "consume"       // 消费（AI 调用等，后续）
	TokenTypeGrant        = "grant"         // 管理员增发
	TokenTypeDeduct       = "deduct"        // 管理员扣减
	TokenTypeRefund       = "refund"        // 退款返还
)

// ---------- 卡密状态 ----------

const (
	CardKeyUnused = "unused"
	CardKeyUsed   = "used"
	CardKeyVoid   = "void"
)

// ---------- 模型 ----------

// TokenLedger 代币流水（append-only，不可篡改）
type TokenLedger struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"not null;index:idx_token_ledger_user,priority:1" json:"user_id"`
	Type          string    `gorm:"size:24;not null;index" json:"type"`  // recharge_card/recharge_pay/consume/grant/deduct/refund
	Amount        int64     `gorm:"not null" json:"amount"`              // 签名额：充值/增发为正，消费/扣减为负
	BalanceAfter  int64     `gorm:"not null" json:"balance_after"`       // 本笔后余额快照
	RefType       string    `gorm:"size:24;default:''" json:"ref_type"`  // 关联类型：card/admin/ai_order...
	RefID         uint      `gorm:"default:0;index" json:"ref_id"`       // 关联 ID
	Remark        string    `gorm:"size:256;default:''" json:"remark"`   // 备注
	PriceSnapshot string    `gorm:"type:text" json:"price_snapshot"`     // 价格/计费快照（JSON，可空）
	CreatedAt     time.Time `gorm:"index:idx_token_ledger_user,priority:2" json:"created_at"`
}

func (TokenLedger) TableName() string { return "token_ledger" }

// UserTokenBalance 用户代币缓存余额（与流水同事务更新，供快速读取）
type UserTokenBalance struct {
	UserID    uint      `gorm:"primaryKey" json:"user_id"`
	Balance   int64     `gorm:"not null;default:0" json:"balance"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserTokenBalance) TableName() string { return "user_token_balances" }

// CardKey 卡密（哈希存储，不明文）
type CardKey struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	CodeHash  string     `gorm:"size:64;uniqueIndex;not null" json:"-"`      // sha256(规范化卡密) —— 不明文
	CodePrefix string    `gorm:"size:8;default:''" json:"code_prefix"`       // 前缀（展示用，如 ABCD）
	Value     int64      `gorm:"not null" json:"value"`                     // 面值（代币数）
	Status    string     `gorm:"size:16;not null;default:unused;index" json:"status"` // unused/used/void
	BatchID   string     `gorm:"size:32;index;default:''" json:"batch_id"`   // 批次号
	CreatedBy uint       `gorm:"default:0" json:"created_by"`               // 生成管理员 ID
	UsedBy    *uint      `json:"used_by"`                                  // 使用者 ID
	UsedAt    *time.Time `json:"used_at"`
	ExpiresAt *time.Time `json:"expires_at"` // 有效期（null = 永久）
	Note      string     `gorm:"size:256;default:''" json:"note"`
	CreatedAt time.Time  `json:"created_at"`

	UsedByUsername string `gorm:"-" json:"used_by_username,omitempty"` // 使用者用户名（列表填充）
}

func (CardKey) TableName() string { return "card_keys" }

// ---------- 配置 ----------

// IsTokenEnabled 代币系统是否开启（默认关闭）
func IsTokenEnabled() bool {
	return GetPlatformConfig(TokenEnabledKey) == "true"
}

// SetTokenEnabled 设置代币系统开关
func SetTokenEnabled(on bool) error {
	if on {
		return SetPlatformConfig(TokenEnabledKey, "true")
	}
	return SetPlatformConfig(TokenEnabledKey, "false")
}

// GetTokenName 代币展示名（缺省「星币」）
func GetTokenName() string {
	if n := strings.TrimSpace(GetPlatformConfig(TokenNameKey)); n != "" {
		return n
	}
	return DefaultTokenName
}

// SetTokenName 设置代币展示名
func SetTokenName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		name = DefaultTokenName
	}
	if len([]rune(name)) > 16 {
		name = string([]rune(name)[:16])
	}
	return SetPlatformConfig(TokenNameKey, name)
}

// ---------- 余额与流水 ----------

// GetTokenBalance 读取缓存余额（无记录返回 0，不建行）
func GetTokenBalance(userID uint) int64 {
	if userID == 0 {
		return 0
	}
	var b UserTokenBalance
	if err := DB.Where("user_id = ?", userID).First(&b).Error; err != nil {
		return 0
	}
	return b.Balance
}

// AppendTokenLedger 追加一笔代币流水并更新缓存余额（同一事务）。
// amount 为签名额（正=入账，负=扣减）；扣减后余额为负则拒绝（余额不足）。
// 返回本笔后的余额。
func AppendTokenLedger(userID uint, ttype string, amount int64, refType string, refID uint, remark, priceSnapshot string) (int64, error) {
	if userID == 0 {
		return 0, fmt.Errorf("用户 ID 无效")
	}
	if amount == 0 {
		return 0, fmt.Errorf("金额不能为 0")
	}
	var newBalance int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		var b UserTokenBalance
		if e := tx.Where("user_id = ?", userID).First(&b).Error; e != nil {
			// 首次：初始化余额行（balance=0 为零值但无歧义默认，直接用 map 精确写）
			if ce := tx.Create(&UserTokenBalance{UserID: userID, Balance: 0, UpdatedAt: time.Now()}).Error; ce != nil {
				return ce
			}
			b = UserTokenBalance{UserID: userID, Balance: 0}
		}
		nb := b.Balance + amount
		if nb < 0 {
			return fmt.Errorf("余额不足")
		}
		entry := TokenLedger{
			UserID: userID, Type: ttype, Amount: amount, BalanceAfter: nb,
			RefType: refType, RefID: refID, Remark: remark, PriceSnapshot: priceSnapshot,
			CreatedAt: time.Now(),
		}
		if ce := tx.Create(&entry).Error; ce != nil {
			return ce
		}
		if ue := tx.Model(&UserTokenBalance{}).Where("user_id = ?", userID).
			Updates(map[string]interface{}{"balance": nb, "updated_at": time.Now()}).Error; ue != nil {
			return ue
		}
		newBalance = nb
		return nil
	})
	return newBalance, err
}

// ListTokenLedger 分页查询某用户代币流水（按时间倒序）
func ListTokenLedger(userID uint, page, pageSize int) ([]TokenLedger, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	DB.Model(&TokenLedger{}).Where("user_id = ?", userID).Count(&total)
	var list []TokenLedger
	err := DB.Where("user_id = ?", userID).Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

// SumTokenLedger 由流水重算某用户余额（对账用：ledger 为准）
func SumTokenLedger(userID uint) int64 {
	var sum int64
	DB.Model(&TokenLedger{}).Where("user_id = ?", userID).
		Select("COALESCE(SUM(amount), 0)").Scan(&sum)
	return sum
}

// ReconcileTokenBalance 用流水重算并纠正缓存余额，返回纠正后的余额
func ReconcileTokenBalance(userID uint) (int64, error) {
	sum := SumTokenLedger(userID)
	now := time.Now()
	var b UserTokenBalance
	if err := DB.Where("user_id = ?", userID).First(&b).Error; err != nil {
		if ce := DB.Create(&UserTokenBalance{UserID: userID, Balance: sum, UpdatedAt: now}).Error; ce != nil {
			return 0, ce
		}
		return sum, nil
	}
	if b.Balance != sum {
		if ue := DB.Model(&UserTokenBalance{}).Where("user_id = ?", userID).
			Updates(map[string]interface{}{"balance": sum, "updated_at": now}).Error; ue != nil {
			return 0, ue
		}
	}
	return sum, nil
}

// ---------- 卡密 ----------

const cardAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789" // 去除易混淆 0/O/1/I/L

// genCardCode 生成 16 位随机卡密，格式 XXXX-XXXX-XXXX-XXXX
func genCardCode() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	groups := make([]string, 0, 4)
	for g := 0; g < 4; g++ {
		seg := make([]byte, 4)
		for i := 0; i < 4; i++ {
			seg[i] = cardAlphabet[int(buf[g*4+i])%len(cardAlphabet)]
		}
		groups = append(groups, string(seg))
	}
	return strings.Join(groups, "-")
}

// normalizeCardCode 规范化卡密（去分隔符、大写）用于哈希与查重
func normalizeCardCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	code = strings.NewReplacer("-", "", " ", "", "\t", "", "\n", "", "\r", "").Replace(code)
	return code
}

// hashCardCode 卡密 → sha256 hex
func hashCardCode(code string) string {
	return sha256Hex(normalizeCardCode(code))
}

// GenerateCardKeys 批量生成卡密，返回**一次性明文**（仅此一次可见，之后只存哈希）。
func GenerateCardKeys(adminID uint, count, value int, expiresAt *time.Time, note string) ([]string, error) {
	if count < 1 || count > 1000 {
		return nil, fmt.Errorf("生成数量需在 1 ~ 1000 之间")
	}
	if value <= 0 || value > 100000000 {
		return nil, fmt.Errorf("卡密面值需大于 0")
	}
	batch := "B" + time.Now().Format("20060102150405")
	codes := make([]string, 0, count)
	for i := 0; i < count; i++ {
		code := genCardCode()
		ck := CardKey{
			CodeHash:   hashCardCode(code),
			CodePrefix: strings.SplitN(code, "-", 2)[0],
			Value:      int64(value),
			Status:     CardKeyUnused,
			BatchID:    batch,
			CreatedBy:  adminID,
			ExpiresAt:  expiresAt,
			Note:       note,
		}
		if err := DB.Create(&ck).Error; err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, nil
}

// RedeemCardKey 兑换卡密：原子占用 + 入账。返回兑换后余额。
func RedeemCardKey(userID uint, code string) (int64, error) {
	if userID == 0 {
		return 0, fmt.Errorf("请先登录")
	}
	norm := normalizeCardCode(code)
	if norm == "" {
		return 0, fmt.Errorf("请输入卡密")
	}
	hash := hashCardCode(code)
	var ck CardKey
	if err := DB.Where("code_hash = ?", hash).First(&ck).Error; err != nil {
		return 0, fmt.Errorf("卡密无效")
	}
	if ck.Status == CardKeyUsed {
		return 0, fmt.Errorf("卡密已被使用")
	}
	if ck.Status == CardKeyVoid {
		return 0, fmt.Errorf("卡密已作废")
	}
	if ck.ExpiresAt != nil && ck.ExpiresAt.Before(time.Now()) {
		return 0, fmt.Errorf("卡密已过期")
	}
	// 原子占用：仅当仍为 unused 时更新，防并发重兑
	res := DB.Model(&CardKey{}).Where("id = ? AND status = ?", ck.ID, CardKeyUnused).
		Updates(map[string]interface{}{"status": CardKeyUsed, "used_by": userID, "used_at": time.Now()})
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected == 0 {
		return 0, fmt.Errorf("卡密已被使用或已作废")
	}
	nb, err := AppendTokenLedger(userID, TokenTypeRechargeCard, ck.Value, "card", ck.ID, "卡密兑换", "")
	if err != nil {
		return 0, err
	}
	return nb, nil
}

// ListCardKeys 管理端卡密列表（可按状态/批次筛选），附使用者用户名
func ListCardKeys(status, batch string, limit int) ([]CardKey, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	q := DB.Order("created_at DESC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if batch != "" {
		q = q.Where("batch_id = ?", batch)
	}
	var list []CardKey
	if err := q.Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return list, nil
	}
	ids := make([]uint, 0, len(list))
	for _, c := range list {
		if c.UsedBy != nil {
			ids = append(ids, *c.UsedBy)
		}
	}
	if len(ids) > 0 {
		var users []User
		DB.Where("id IN ?", ids).Find(&users)
		names := map[uint]string{}
		for _, u := range users {
			names[u.ID] = u.Username
		}
		for i := range list {
			if list[i].UsedBy != nil {
				list[i].UsedByUsername = names[*list[i].UsedBy]
			}
		}
	}
	return list, nil
}

// VoidCardKey 作废一张未使用的卡密
func VoidCardKey(id uint) error {
	res := DB.Model(&CardKey{}).Where("id = ? AND status = ?", id, CardKeyUnused).
		Update("status", CardKeyVoid)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("仅可作废未使用的卡密")
	}
	return nil
}

// AdminAdjustToken 管理员增发(>0)/扣减(<0)代币，返回调整后余额
func AdminAdjustToken(adminID, userID uint, amount int64, remark string) (int64, error) {
	ttype := TokenTypeGrant
	if amount < 0 {
		ttype = TokenTypeDeduct
	}
	return AppendTokenLedger(userID, ttype, amount, "admin", adminID, remark, "")
}
