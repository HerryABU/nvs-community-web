package models

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ==================== 在线支付充值（§7.4：微信/支付宝聚合通道） ====================
//
// 定位：卡密充值之外的第二种代币获取方式——用户在前端选档位下单，
// 跳转聚合支付收银台（微信/支付宝），网关异步通知到账，同卡密一样
// 走 AppendTokenLedger（recharge_pay）入账。默认关闭，站长配置网关
// 三件套（url/appid/secret）后开启即生效。
//
// 资金纪律：
//   - 金额一律以「分」为单位的 int64，杜绝浮点误差；
//   - 回调验签 + 金额核对 + 幂等到账（pending→paid 条件更新）；
//   - 订单过期自动关单；管理端可手动补单（留痕 transaction_id + notify_raw）。

// ---------- 配置键 ----------

const (
	// PaymentEnabledKey 支付充值总开关（默认关）
	PaymentEnabledKey = "payment_enabled"
	// PaymentConfigKey 支付通道配置 JSON
	PaymentConfigKey = "payment_config"
	// PaymentTiersKey 充值档位 JSON
	PaymentTiersKey = "payment_tiers"
)

// ---------- 渠道与状态 ----------

const (
	PayChannelWeChat = "wechat" // 微信支付（经聚合网关 wxpay）
	PayChannelAliPay = "alipay" // 支付宝（经聚合网关 alipay）
	PayChannelMock   = "mock"   // 模拟支付（开发/演示，不接真实网关）

	PayProviderMock      = "mock"      // 模拟 provider
	PayProviderAggregate = "aggregate" // 通用聚合支付 provider（易支付/码支付类）

	PayStatusPending = "pending" // 待支付
	PayStatusPaid    = "paid"    // 已支付（已到账）
	PayStatusFailed  = "failed"  // 支付失败
	PayStatusClosed  = "closed"  // 超时关单
)

// PayChannelLabels 渠道展示名
var PayChannelLabels = map[string]string{
	PayChannelWeChat: "微信支付",
	PayChannelAliPay: "支付宝",
	PayChannelMock:   "模拟支付",
}

// PayStatusLabels 状态展示名
var PayStatusLabels = map[string]string{
	PayStatusPending: "待支付",
	PayStatusPaid:    "已到账",
	PayStatusFailed:  "支付失败",
	PayStatusClosed:  "已关单",
}

// ---------- 配置与档位 ----------

// PaymentConfig 支付通道配置（secret 只写不读：管理端 GET 不返回明文）
type PaymentConfig struct {
	Provider    string `json:"provider"`      // mock / aggregate
	GatewayURL  string `json:"gateway_url"`   // 聚合网关下单地址
	AppID       string `json:"app_id"`        // 商户 ID（pid）
	Secret      string `json:"secret"`        // 商户密钥（验签用，GET 时掩码）
	NotifyURL   string `json:"notify_url"`    // 异步通知地址（公网可达）
	ReturnURL   string `json:"return_url"`    // 支付完成后跳转地址
	TokenPerFen int64  `json:"token_per_fen"` // 每 1 分钱兑换代币数（默认 10 → 1 元 = 1000）
	MinFen      int64  `json:"min_fen"`       // 单笔下限（分），默认 100（1 元）
	MaxFen      int64  `json:"max_fen"`       // 单笔上限（分），默认 1000000（10000 元）
	OrderTTLMin int    `json:"order_ttl_min"` // 订单有效期（分钟），默认 30
}

// DefaultPaymentConfig 默认配置：模拟支付、1 元=1000 代币
func DefaultPaymentConfig() PaymentConfig {
	return PaymentConfig{
		Provider: PayProviderMock, TokenPerFen: 10,
		MinFen: 100, MaxFen: 1000000, OrderTTLMin: 30,
	}
}

// normalizePaymentConfig 钳制配置，防异常值
func normalizePaymentConfig(cfg PaymentConfig) PaymentConfig {
	cfg.Provider = strings.ToLower(strings.TrimSpace(cfg.Provider))
	if cfg.Provider != PayProviderAggregate {
		cfg.Provider = PayProviderMock
	}
	cfg.GatewayURL = strings.TrimSpace(cfg.GatewayURL)
	cfg.AppID = strings.TrimSpace(cfg.AppID)
	cfg.Secret = strings.TrimSpace(cfg.Secret)
	cfg.NotifyURL = strings.TrimSpace(cfg.NotifyURL)
	cfg.ReturnURL = strings.TrimSpace(cfg.ReturnURL)
	if cfg.TokenPerFen < 1 {
		cfg.TokenPerFen = 1
	}
	if cfg.TokenPerFen > 10000 {
		cfg.TokenPerFen = 10000
	}
	if cfg.MinFen < 1 {
		cfg.MinFen = 100
	}
	if cfg.MaxFen < cfg.MinFen {
		cfg.MaxFen = cfg.MinFen
	}
	if cfg.MaxFen > 100000000 { // 100 万元封顶
		cfg.MaxFen = 100000000
	}
	if cfg.OrderTTLMin < 1 {
		cfg.OrderTTLMin = 30
	}
	if cfg.OrderTTLMin > 1440 {
		cfg.OrderTTLMin = 1440
	}
	return cfg
}

// PaymentTier 会员套餐（面额 + 赠送代币 + 会员时长）
//
// 历史背景：它最早只是「充值档位」（面额 + 赠送代币），没有时长概念。
// 参考 SparkAi 类站点的会员页（天卡/周卡/月卡 + 权益清单）后扩展为套餐：
// Days=0 的档位仍是纯充值（兼容老配置），Days>0 的档位会同时延长会员有效期。
type PaymentTier struct {
	Fen      int64    `json:"fen"`                // 面额（分）
	Bonus    int64    `json:"bonus"`              // 赠送代币
	Label    string   `json:"label"`              // 展示名（如「月卡」）
	Days     int      `json:"days,omitempty"`     // 会员有效天数（0 = 纯充值，不加时长）
	Badge    string   `json:"badge,omitempty"`    // 角标：推荐 / 最超值 / 限时
	Features []string `json:"features,omitempty"` // 权益说明（前端逐条展示）
}

// DefaultPaymentTiers 默认套餐：天 / 周 / 月 / 季。
//
// 权益里刻意不写「赠送多少代币」——那由 TokenPerFen 实时换算（默认 1 元 = 1000），
// 写死在文案里一旦站长改了比例就会与实际到账不一致。
func DefaultPaymentTiers() []PaymentTier {
	return []PaymentTier{
		{Fen: 600, Bonus: 0, Label: "天卡体验", Days: 1,
			Features: []string{"会员 1 天", "每日免费 AI 次数 ×2", "全部对话 / 绘画 / 视频模型"}},
		{Fen: 1200, Bonus: 0, Label: "周卡特惠", Days: 7,
			Features: []string{"会员 7 天", "每日免费 AI 次数 ×2", "额度按周结算更省"}},
		{Fen: 3800, Bonus: 0, Label: "月卡", Days: 30, Badge: "推荐",
			Features: []string{"会员 30 天", "每日免费 AI 次数 ×2", "优先使用高峰期通道"}},
		{Fen: 9800, Bonus: 0, Label: "季卡", Days: 90, Badge: "最超值",
			Features: []string{"会员 90 天", "每日免费 AI 次数 ×2", "折合每月更便宜"}},
	}
}

// IsPaymentEnabled 支付充值是否开启（默认关闭）
func IsPaymentEnabled() bool {
	return GetPlatformConfig(PaymentEnabledKey) == "true"
}

// SetPaymentEnabled 设置支付充值开关
func SetPaymentEnabled(on bool) error {
	if on {
		return SetPlatformConfig(PaymentEnabledKey, "true")
	}
	return SetPlatformConfig(PaymentEnabledKey, "false")
}

// GetPaymentConfig 读取支付配置（缺省回退默认值）
func GetPaymentConfig() PaymentConfig {
	raw := strings.TrimSpace(GetPlatformConfig(PaymentConfigKey))
	if raw == "" {
		return DefaultPaymentConfig()
	}
	var cfg PaymentConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return DefaultPaymentConfig()
	}
	return normalizePaymentConfig(cfg)
}

// SetPaymentConfig 保存支付配置（钳制后落库）
func SetPaymentConfig(cfg PaymentConfig) error {
	cfg = normalizePaymentConfig(cfg)
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return SetPlatformConfig(PaymentConfigKey, string(raw))
}

// GetPaymentTiers 读取档位（缺省默认）
func GetPaymentTiers() []PaymentTier {
	raw := strings.TrimSpace(GetPlatformConfig(PaymentTiersKey))
	if raw == "" {
		return DefaultPaymentTiers()
	}
	var tiers []PaymentTier
	if err := json.Unmarshal([]byte(raw), &tiers); err != nil || len(tiers) == 0 {
		return DefaultPaymentTiers()
	}
	return tiers
}

// ValidatePaymentTiers 校验档位列表（管理端保存前调用）
func ValidatePaymentTiers(tiers []PaymentTier) error {
	if len(tiers) == 0 || len(tiers) > 12 {
		return fmt.Errorf("档位数量需在 1 ~ 12 之间")
	}
	seen := map[int64]bool{}
	for _, t := range tiers {
		if t.Fen < 100 || t.Fen > 100000000 {
			return fmt.Errorf("档位面额需在 1 元 ~ 100 万元之间")
		}
		if seen[t.Fen] {
			return fmt.Errorf("档位面额重复：%d 分", t.Fen)
		}
		seen[t.Fen] = true
		if t.Bonus < 0 || t.Bonus > t.Fen*10 {
			return fmt.Errorf("赠送代币需不小于 0 且不超过面值的 10 倍")
		}
		if len([]rune(strings.TrimSpace(t.Label))) == 0 || len([]rune(t.Label)) > 16 {
			return fmt.Errorf("档位展示名需在 1 ~ 16 字之间")
		}
		if t.Days < 0 || t.Days > 3650 {
			return fmt.Errorf("会员天数需在 0 ~ 3650 之间（0 表示纯充值）")
		}
		if len([]rune(t.Badge)) > 8 {
			return fmt.Errorf("角标不超过 8 字")
		}
		if len(t.Features) > 8 {
			return fmt.Errorf("权益条目不超过 8 条")
		}
		for _, f := range t.Features {
			if len([]rune(f)) > 40 {
				return fmt.Errorf("单条权益说明不超过 40 字")
			}
		}
	}
	return nil
}

// SetPaymentTiers 保存档位
func SetPaymentTiers(tiers []PaymentTier) error {
	if err := ValidatePaymentTiers(tiers); err != nil {
		return err
	}
	raw, err := json.Marshal(tiers)
	if err != nil {
		return err
	}
	return SetPlatformConfig(PaymentTiersKey, string(raw))
}

// ResolvePaymentTier 按面额精确匹配档位（拒绝非档位金额，防刷防错价）
func ResolvePaymentTier(fen int64) (PaymentTier, bool) {
	for _, t := range GetPaymentTiers() {
		if t.Fen == fen {
			return t, true
		}
	}
	return PaymentTier{}, false
}

// TierTokenAmount 档位应付代币 = 面额×比例 + 赠送
func TierTokenAmount(t PaymentTier, tokenPerFen int64) int64 {
	return t.Fen*tokenPerFen + t.Bonus
}

// ---------- 订单 ----------

// RechargeOrder 在线充值订单
type RechargeOrder struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	OrderNo     string `gorm:"size:32;uniqueIndex;not null" json:"order_no"`  // 平台订单号
	UserID      uint   `gorm:"index;not null" json:"user_id"`                 // 下单用户
	Channel     string `gorm:"size:16;not null" json:"channel"`               // wechat/alipay/mock
	Provider    string `gorm:"size:16;not null;default:mock" json:"provider"` // mock/aggregate
	AmountFen   int64  `gorm:"not null" json:"amount_fen"`                    // 实付金额（分）
	TokenAmount int64  `gorm:"not null" json:"token_amount"`                  // 到账代币
	// 会员天数快照：下单时从档位取值，避免事后站长改档位影响已下单未支付的订单
	MemberDays    int        `gorm:"not null;default:0" json:"member_days"`
	Status        string     `gorm:"size:16;not null;default:pending;index" json:"status"` // pending/paid/failed/closed
	PayURL        string     `gorm:"type:text" json:"pay_url"`                             // 收银台地址（mock 为站内模拟页）
	TransactionID string     `gorm:"size:64;default:'';index" json:"transaction_id"`       // 渠道流水号
	ExpiresAt     time.Time  `gorm:"index" json:"expires_at"`                              // 过期时间
	PaidAt        *time.Time `json:"paid_at"`                                              // 到账时间
	NotifyRaw     string     `gorm:"type:text" json:"notify_raw"`                          // 回调原始报文（存证）
	CreatedAt     time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`

	Username string `gorm:"-" json:"username,omitempty"` // 下单用户名（管理端列表填充）
}

func (RechargeOrder) TableName() string { return "recharge_orders" }

// GenerateOrderNo 生成平台订单号：R + 年月日时分秒 + 4 位随机
func GenerateOrderNo() string {
	b := make([]byte, 2)
	_, _ = rand.Read(b)
	return "R" + time.Now().Format("20060102150405") + strings.ToUpper(hex.EncodeToString(b))
}

// CreateRechargeOrder 创建充值订单（pending）。pay_url 由 handler 调 provider 后回填。
func CreateRechargeOrder(userID uint, channel string, fen int64) (*RechargeOrder, error) {
	if userID == 0 {
		return nil, fmt.Errorf("请先登录")
	}
	if !IsPaymentEnabled() {
		return nil, fmt.Errorf("在线充值未开启")
	}
	if !IsTokenEnabled() {
		return nil, fmt.Errorf("代币系统未开启")
	}
	cfg := GetPaymentConfig()
	if channel != PayChannelWeChat && channel != PayChannelAliPay && channel != PayChannelMock {
		return nil, fmt.Errorf("不支持的支付渠道")
	}
	if channel == PayChannelMock && cfg.Provider != PayProviderMock {
		return nil, fmt.Errorf("当前通道未启用模拟支付")
	}
	if channel != PayChannelMock && cfg.Provider != PayProviderAggregate {
		return nil, fmt.Errorf("当前通道未启用在线支付")
	}
	tier, ok := ResolvePaymentTier(fen)
	if !ok {
		return nil, fmt.Errorf("请选择有效的充值档位")
	}
	if fen < cfg.MinFen || fen > cfg.MaxFen {
		return nil, fmt.Errorf("单笔充值需在 %d ~ %d 元之间", cfg.MinFen/100, cfg.MaxFen/100)
	}
	order := &RechargeOrder{
		OrderNo:     GenerateOrderNo(),
		UserID:      userID,
		Channel:     channel,
		Provider:    cfg.Provider,
		AmountFen:   tier.Fen,
		TokenAmount: TierTokenAmount(tier, cfg.TokenPerFen),
		MemberDays:  tier.Days,
		Status:      PayStatusPending,
		ExpiresAt:   time.Now().Add(time.Duration(cfg.OrderTTLMin) * time.Minute),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := DB.Create(order).Error; err != nil {
		return nil, err
	}
	return order, nil
}

// GetRechargeOrderByNo 按订单号查询
func GetRechargeOrderByNo(orderNo string) (*RechargeOrder, error) {
	var o RechargeOrder
	if err := DB.Where("order_no = ?", orderNo).First(&o).Error; err != nil {
		return nil, fmt.Errorf("订单不存在")
	}
	return &o, nil
}

// UpdateOrderPayURL 回填收银台地址
func UpdateOrderPayURL(orderNo, payURL string) error {
	return DB.Model(&RechargeOrder{}).Where("order_no = ?", orderNo).
		Update("pay_url", payURL).Error
}

// ListUserRechargeOrders 分页查询我的充值订单（时间倒序）
func ListUserRechargeOrders(userID uint, page, pageSize int) ([]RechargeOrder, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	DB.Model(&RechargeOrder{}).Where("user_id = ?", userID).Count(&total)
	var list []RechargeOrder
	err := DB.Where("user_id = ?", userID).Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

// MarkOrderPaid 订单到账（幂等）：仅 pending→paid 条件更新成功才入账。
// 返回 (订单, 是否本次到账, 错误)。重复通知/重复补单返回 credited=false、订单为已到账态。
func MarkOrderPaid(orderNo, txnID, notifyRaw string) (*RechargeOrder, bool, error) {
	var order *RechargeOrder
	credited := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var o RechargeOrder
		if e := tx.Where("order_no = ?", orderNo).First(&o).Error; e != nil {
			return fmt.Errorf("订单不存在")
		}
		if o.Status == PayStatusPaid {
			order = &o // 已到账：幂等返回，不再入账
			return nil
		}
		if o.Status != PayStatusPending {
			return fmt.Errorf("订单状态为「%s」，无法到账", PayStatusLabels[o.Status])
		}
		now := time.Now()
		// 条件更新占位：防并发回调/补单双重入账
		res := tx.Model(&RechargeOrder{}).
			Where("order_no = ? AND status = ?", orderNo, PayStatusPending).
			Updates(map[string]interface{}{
				"status": PayStatusPaid, "transaction_id": txnID,
				"paid_at": &now, "notify_raw": notifyRaw, "updated_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// 并发下已被处理：重读确认已到账则幂等返回
			var again RechargeOrder
			if e := tx.Where("order_no = ?", orderNo).First(&again).Error; e == nil && again.Status == PayStatusPaid {
				order = &again
				return nil
			}
			return fmt.Errorf("订单状态已变化，请刷新后重试")
		}
		o.Status = PayStatusPaid
		o.TransactionID = txnID
		o.PaidAt = &now
		o.NotifyRaw = notifyRaw
		// 同一事务内入账（与 AppendTokenLedger 同逻辑，避免「状态已改但入账失败」的不一致）
		if _, e := appendTokenLedgerTx(tx, o.UserID, TokenTypeRechargePay, o.TokenAmount,
			"pay_order", o.ID, "在线充值（"+PayChannelLabels[o.Channel]+"）", ""); e != nil {
			return e
		}
		// 会员时长与代币在同一事务：避免「钱已到账但会员没生效」这种要人工补单的脏状态
		if o.MemberDays > 0 {
			if e := extendMembershipTx(tx, o.UserID, o.MemberDays); e != nil {
				return e
			}
		}
		order = &o
		credited = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return order, credited, nil
}

// ---------- 会员 ----------

// extendMembershipTx 在同一事务内延长会员有效期。
// 起点取「现在」与「现有到期时间」中较晚者：提前续费不会吞掉剩余天数。
func extendMembershipTx(tx *gorm.DB, userID uint, days int) error {
	if userID == 0 || days <= 0 {
		return nil
	}
	var u User
	if err := tx.Select("id", "member_until").Where("id = ?", userID).First(&u).Error; err != nil {
		return err
	}
	base := time.Now()
	if u.MemberUntil != nil && u.MemberUntil.After(base) {
		base = *u.MemberUntil
	}
	until := base.AddDate(0, 0, days)
	return tx.Model(&User{}).Where("id = ?", userID).Update("member_until", until).Error
}

// MembershipStatus 会员状态
type MembershipStatus struct {
	IsMember bool       `json:"is_member"`
	Until    *time.Time `json:"until,omitempty"`
	DaysLeft int        `json:"days_left"`
}

// GetMembershipStatus 读取会员状态。只选必要列，避免把用户整行（含密文）读出来。
func GetMembershipStatus(userID uint) MembershipStatus {
	st := MembershipStatus{}
	if userID == 0 {
		return st
	}
	var u User
	if err := DB.Select("id", "member_until").Where("id = ?", userID).First(&u).Error; err != nil {
		return st
	}
	if u.MemberUntil == nil {
		return st
	}
	now := time.Now()
	if u.MemberUntil.After(now) {
		st.IsMember = true
		st.Until = u.MemberUntil
		// 向上取整：还剩 12 小时算 1 天，避免显示「0 天」却仍可享受权益
		st.DaysLeft = int(math.Ceil(u.MemberUntil.Sub(now).Hours() / 24))
	}
	return st
}

// IsMember 当前是否为有效会员
func IsMember(userID uint) bool {
	return GetMembershipStatus(userID).IsMember
}

// appendTokenLedgerTx 在既有事务内追加流水（与 AppendTokenLedger 同逻辑，
// 供 MarkOrderPaid 复用同一事务，避免「状态已改但入账失败」的不一致）。
func appendTokenLedgerTx(tx *gorm.DB, userID uint, ttype string, amount int64, refType string, refID uint, remark, priceSnapshot string) (int64, error) {
	if userID == 0 {
		return 0, fmt.Errorf("用户 ID 无效")
	}
	if amount == 0 {
		return 0, fmt.Errorf("金额不能为 0")
	}
	var b UserTokenBalance
	if e := tx.Where("user_id = ?", userID).First(&b).Error; e != nil {
		if ce := tx.Create(&UserTokenBalance{UserID: userID, Balance: 0, UpdatedAt: time.Now()}).Error; ce != nil {
			return 0, ce
		}
		b = UserTokenBalance{UserID: userID, Balance: 0}
	}
	nb := b.Balance + amount
	if nb < 0 {
		return 0, fmt.Errorf("余额不足")
	}
	entry := TokenLedger{
		UserID: userID, Type: ttype, Amount: amount, BalanceAfter: nb,
		RefType: refType, RefID: refID, Remark: remark, PriceSnapshot: priceSnapshot,
		CreatedAt: time.Now(),
	}
	if ce := tx.Create(&entry).Error; ce != nil {
		return 0, ce
	}
	if ue := tx.Model(&UserTokenBalance{}).Where("user_id = ?", userID).
		Updates(map[string]interface{}{"balance": nb, "updated_at": time.Now()}).Error; ue != nil {
		return 0, ue
	}
	return nb, nil
}

// CloseExpiredOrders 关闭所有超时的待支付订单，返回关闭笔数
func CloseExpiredOrders() (int64, error) {
	res := DB.Model(&RechargeOrder{}).
		Where("status = ? AND expires_at < ?", PayStatusPending, time.Now()).
		Updates(map[string]interface{}{"status": PayStatusClosed, "updated_at": time.Now()})
	return res.RowsAffected, res.Error
}

// AdminListRechargeOrders 管理端订单列表（status/user_id 筛选 + 分页），附用户名
func AdminListRechargeOrders(status string, userID uint, page, pageSize int) ([]RechargeOrder, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	q := DB.Model(&RechargeOrder{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	var total int64
	q.Count(&total)
	var list []RechargeOrder
	qq := DB.Order("created_at DESC")
	if status != "" {
		qq = qq.Where("status = ?", status)
	}
	if userID > 0 {
		qq = qq.Where("user_id = ?", userID)
	}
	if err := qq.Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	if len(list) > 0 {
		ids := make([]uint, 0, len(list))
		for _, o := range list {
			ids = append(ids, o.UserID)
		}
		var users []User
		DB.Where("id IN ?", ids).Find(&users)
		names := map[uint]string{}
		for _, u := range users {
			names[u.ID] = u.Username
		}
		for i := range list {
			list[i].Username = names[list[i].UserID]
		}
	}
	return list, total, nil
}

// PaymentStats 支付统计（管理端概览）
func PaymentStats() map[string]int64 {
	var paidCount, paidFen, paidToken, pendingCount, closedCount int64
	DB.Model(&RechargeOrder{}).Where("status = ?", PayStatusPaid).
		Select("COUNT(*)").Scan(&paidCount)
	DB.Model(&RechargeOrder{}).Where("status = ?", PayStatusPaid).
		Select("COALESCE(SUM(amount_fen), 0)").Scan(&paidFen)
	DB.Model(&RechargeOrder{}).Where("status = ?", PayStatusPaid).
		Select("COALESCE(SUM(token_amount), 0)").Scan(&paidToken)
	DB.Model(&RechargeOrder{}).Where("status = ?", PayStatusPending).
		Select("COUNT(*)").Scan(&pendingCount)
	DB.Model(&RechargeOrder{}).Where("status = ?", PayStatusClosed).
		Select("COUNT(*)").Scan(&closedCount)
	return map[string]int64{
		"paid_count": paidCount, "paid_fen": paidFen, "paid_token": paidToken,
		"pending_count": pendingCount, "closed_count": closedCount,
	}
}
