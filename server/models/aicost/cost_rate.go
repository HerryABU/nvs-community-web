package aicost

import (
	"encoding/json"
	"errors"
	"nvs-server/models"
	"strings"
)

// ==================== 虚拟记账单位 → 真实货币的换算比例 ====================
//
// 为什么必须有这个：New API / One API 体系里 `quota ÷ 500000` 得到的「美元」
// 只是**站内代币**，与真实充值金额没有 1:1 关系。常见情形：
//
//	充 100 元人民币 → 到账 1000 虚拟美元（1 元 = 10 虚拟美元）
//	充 100 元人民币 → 到账 100 虚拟美元（1 元 = 1 虚拟美元）
//	官方渠道       → 虚拟单位就是真实美元（1:1）
//
// 所以「余额还剩 5274」这句话，不声明比例就没有任何成本含义。
// 站长的责任是说清：**多少真实货币 = 多少虚拟单位**，本平台据此换算真实成本。
//
// 比例用「两数之比」表达而不是单个系数，这样站长可以直接按充值单填写
// （100 元 : 1000 虚拟美元），也能填小数（0.1389 元 : 1 虚拟美元）。

const costRateKey = "cost_rate"

// CostRate 虚拟单位与真实货币的兑换比例。
//
// 语义：RealAmount 单位真实货币 ⇔ VirtualAmount 虚拟单位。
// 换算：真实金额 = 虚拟金额 × RealAmount ÷ VirtualAmount。
type CostRate struct {
	RealAmount      float64 `json:"real_amount"`      // 真实货币数额，如 1
	VirtualAmount   float64 `json:"virtual_amount"`   // 对应虚拟单位数额，如 10
	RealCurrency    string  `json:"real_currency"`    // 真实货币，如 CNY / USD
	VirtualCurrency string  `json:"virtual_currency"` // 上游记账单位名，展示用，如 虚拟美元
}

// Valid 比例是否可用（必须两数都为正）
func (r CostRate) Valid() bool {
	return r.RealAmount > 0 && r.VirtualAmount > 0
}

// Ratio 虚拟 → 真实的乘数（= RealAmount / VirtualAmount）。
// 未配置时返回 0 —— 调用方必须据此显示「未设置比例」，绝不能当 1:1 假装知道真实成本。
func (r CostRate) Ratio() float64 {
	if !r.Valid() {
		return 0
	}
	return r.RealAmount / r.VirtualAmount
}

// ToReal 虚拟金额 → 真实金额。比例未配置时返回 (0,false)，由调用方决定怎么展示。
func (r CostRate) ToReal(virtual float64) (float64, bool) {
	ratio := r.Ratio()
	if ratio <= 0 {
		return 0, false
	}
	return round4(virtual * ratio), true
}

// Describe 一句话描述（后台展示用）
func (r CostRate) Describe() string {
	if !r.Valid() {
		return "未设置换算比例：金额仅为上游记账单位"
	}
	return trimNum(r.RealAmount) + " " + curOr(r.RealCurrency, "CNY") +
		" = " + trimNum(r.VirtualAmount) + " " + curOr(r.VirtualCurrency, "虚拟单位")
}

// GetCostRate 读取全局换算比例（未配置返回零值，Valid()==false）
func GetCostRate() CostRate {
	raw := strings.TrimSpace(models.GetPlatformConfig(costRateKey))
	if raw == "" {
		return CostRate{}
	}
	var r CostRate
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return CostRate{}
	}
	r.RealCurrency = strings.ToUpper(strings.TrimSpace(r.RealCurrency))
	r.VirtualCurrency = strings.TrimSpace(r.VirtualCurrency)
	return r
}

// SetCostRate 保存换算比例。
// 允许「清空比例」（两数都为 0）回到未设置状态；但不允许只填一半——
// 半套比例最容易算出看似有据、其实荒谬的成本。
func SetCostRate(r CostRate) error {
	r.RealCurrency = strings.ToUpper(strings.TrimSpace(r.RealCurrency))
	r.VirtualCurrency = strings.TrimSpace(r.VirtualCurrency)
	if r.RealAmount == 0 && r.VirtualAmount == 0 {
		b, _ := json.Marshal(CostRate{RealCurrency: r.RealCurrency, VirtualCurrency: r.VirtualCurrency})
		return models.SetPlatformConfig(costRateKey, string(b))
	}
	if r.RealAmount <= 0 || r.VirtualAmount <= 0 {
		return errors.New("比例的两个数都必须大于 0（要清空请把两个都填 0）")
	}
	if r.RealCurrency == "" {
		r.RealCurrency = "CNY"
	}
	if r.VirtualCurrency == "" {
		r.VirtualCurrency = "虚拟美元"
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return models.SetPlatformConfig(costRateKey, string(b))
}

// ResolveCostRate 取某渠道的生效比例：渠道级覆盖优先，否则用全局。
// 返回 (比例, 来源) —— 来源为 "channel" / "global" / "none"，前端据此标注可信度。
func ResolveCostRate(baseURL string) (CostRate, string) {
	if strings.TrimSpace(baseURL) != "" {
		var m models.AiModel
		if err := models.DB.Where("base_url = ? AND (rate_real_amount > 0 OR rate_virtual_amount > 0)", baseURL).
			Order("id").First(&m).Error; err == nil {
			r := CostRate{
				RealAmount: m.RateRealAmount, VirtualAmount: m.RateVirtualAmount,
				RealCurrency: m.RateRealCurrency,
				// 渠道级不单存「虚拟单位名」：统一沿用全局的命名（如「虚拟美元」），
				// 没设则回退该模型自定义的计价单位，再没有就写「虚拟单位」。
				VirtualCurrency: firstNonEmpty(GetCostRate().VirtualCurrency, m.BalanceCurrency, "虚拟单位"),
			}
			if r.Valid() {
				return r, "channel"
			}
		}
	}
	g := GetCostRate()
	if g.Valid() {
		return g, "global"
	}
	return CostRate{}, "none"
}

// ── 小工具 ──

// firstNonEmpty 取第一个非空串（渠道级单位名的回退链）
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func curOr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// trimNum 去掉多余小数位（1.0000 → 1，7.20 → 7.2）。
// ⚠️ 只在**有小数点**时才剥尾零：100 的末尾那个 0 是数值的一部分，
// 剥掉会把「100 元 = 1000 虚拟」显示成「10 元 = 100 虚拟」——
// 站长据此填表会被误导（这个坑是实机验收时发现并锁进单测的）。
func trimNum(v float64) string {
	s := strconvFormat(v)
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	s = strings.TrimSuffix(s, ".")
	return s
}

func strconvFormat(v float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
