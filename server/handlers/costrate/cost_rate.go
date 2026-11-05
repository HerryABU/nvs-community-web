package costrate

import (
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
	aicost "nvs-server/models/aicost"
)

// AdminGetCostRate GET /api/admin/cost-rate
//
// 返回站点全局的「虚拟记账单位 → 真实货币」比例。
// 为什么需要它：中转站里 `quota ÷ 500000` 得到的「美元」只是站内代币，
// 充 100 元可能到账 1000 虚拟美元。不声明比例，任何「余额还剩多少」都没有成本含义。
func AdminGetCostRate(c *gin.Context) {
	r := aicost.GetCostRate()
	infra.Success(c, gin.H{
		"rate":            r,
		"valid":           r.Valid(),
		"ratio":           r.Ratio(),
		"describe":        r.Describe(),
		"real_currencies": []string{"CNY", "USD", "EUR", "JPY", "HKD"},
	})
}

// AdminSaveCostRate PUT /api/admin/cost-rate
//
// 入参沿用 CostRate 结构。允许「两个都填 0」表示清空（回到未设置状态）。
// 只填一个数直接拒绝：半套比例算出来的是看似有据、实则荒谬的成本。
func AdminSaveCostRate(c *gin.Context) {
	var req aicost.CostRate
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if err := aicost.SetCostRate(req); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	r := aicost.GetCostRate()
	infra.Success(c, gin.H{
		"rate":     r,
		"valid":    r.Valid(),
		"ratio":    r.Ratio(),
		"describe": r.Describe(),
	})
}

// AdminCostRatePreview POST /api/admin/cost-rate/preview
//
// 换算预览：站长填完比例想知道「我账上 5274 虚拟美元等于多少钱」，
// 与其自己按计算器，不如让后端按同一套算法算一遍（前后端只留一处换算逻辑）。
func AdminCostRatePreview(c *gin.Context) {
	var req struct {
		Virtual float64 `json:"virtual"`
		// 可选：临时比例（未保存也想先试试）
		RealAmount      float64 `json:"real_amount"`
		VirtualAmount   float64 `json:"virtual_amount"`
		RealCurrency    string  `json:"real_currency"`
		VirtualCurrency string  `json:"virtual_currency"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	rate := aicost.CostRate{
		RealAmount: req.RealAmount, VirtualAmount: req.VirtualAmount,
		RealCurrency: req.RealCurrency, VirtualCurrency: req.VirtualCurrency,
	}
	if !rate.Valid() {
		rate = aicost.GetCostRate()
	}
	real, ok := rate.ToReal(req.Virtual)
	infra.Success(c, gin.H{
		"virtual":       req.Virtual,
		"real":          real,
		"ok":            ok,
		"real_currency": rate.RealCurrency,
		"describe":      rate.Describe(),
	})
}
