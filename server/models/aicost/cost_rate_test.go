package aicost

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"nvs-server/models"
	ai "nvs-server/models/ai"
)

// setupCostRateTestDB 独立内存库（唯一名，避免与其它测试共享同一内存库）
func setupCostRateTestDB(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:costrate_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存测试库失败: %v", err)
	}
	if err := db.AutoMigrate(&models.PlatformConfig{}, &ai.AiModel{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	old := models.DB
	models.DB = db
	t.Cleanup(func() { models.DB = old })
}

// 核心语义：虚拟单位不是钱。5274 虚拟美元在 1:10 的比例下只是 527.4 元。
func TestCostRate_ToReal(t *testing.T) {
	cases := []struct {
		name    string
		rate    CostRate
		virtual float64
		want    float64
		ok      bool
	}{
		{
			name:    "1元=10虚拟美元 → 5274虚拟 = 527.4元",
			rate:    CostRate{RealAmount: 1, VirtualAmount: 10},
			virtual: 5274, want: 527.4, ok: true,
		},
		{
			name:    "官方1:1 → 20虚拟 = 20美元",
			rate:    CostRate{RealAmount: 1, VirtualAmount: 1},
			virtual: 20, want: 20, ok: true,
		},
		{
			name:    "小数比例 0.1389元=1虚拟美元 → 100虚拟 = 13.89元",
			rate:    CostRate{RealAmount: 0.1389, VirtualAmount: 1},
			virtual: 100, want: 13.89, ok: true,
		},
		{
			name:    "充值单形态 100元=1000虚拟 → 250虚拟 = 25元",
			rate:    CostRate{RealAmount: 100, VirtualAmount: 1000},
			virtual: 250, want: 25, ok: true,
		},
		{
			name:    "未配置比例 → 不换算（绝不假装知道真实成本）",
			rate:    CostRate{},
			virtual: 5274, want: 0, ok: false,
		},
		{
			name:    "只填一半 → 不换算（半套比例最危险）",
			rate:    CostRate{RealAmount: 1},
			virtual: 100, want: 0, ok: false,
		},
	}
	for _, c := range cases {
		got, ok := c.rate.ToReal(c.virtual)
		if ok != c.ok {
			t.Errorf("%s：ok=%v，期望 %v", c.name, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("%s：得到 %v，期望 %v", c.name, got, c.want)
		}
	}
}

// Ratio 是「虚拟→真实」的乘数，方向不能反（反了会把成本放大 100 倍）
func TestCostRate_RatioDirection(t *testing.T) {
	r := CostRate{RealAmount: 1, VirtualAmount: 10}
	if r.Ratio() != 0.1 {
		t.Errorf("1:10 的乘数应为 0.1（虚拟 × 0.1 = 真实），实际 %v", r.Ratio())
	}
	r2 := CostRate{RealAmount: 10, VirtualAmount: 1}
	if r2.Ratio() != 10 {
		t.Errorf("10:1 的乘数应为 10，实际 %v", r2.Ratio())
	}
}

// 存读往返 + 只填一半必须被拒
func TestSetCostRate_Validation(t *testing.T) {
	setupCostRateTestDB(t)

	if err := SetCostRate(CostRate{RealAmount: 1, VirtualAmount: 10, RealCurrency: "cny"}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	got := GetCostRate()
	if got.RealAmount != 1 || got.VirtualAmount != 10 {
		t.Fatalf("读回不一致: %+v", got)
	}
	if got.RealCurrency != "CNY" {
		t.Errorf("币种应归一为大写 CNY，实际 %q", got.RealCurrency)
	}

	// 只填一个数 → 拒绝
	if err := SetCostRate(CostRate{RealAmount: 5}); err == nil {
		t.Error("只填真实数额应被拒绝")
	}
	if err := SetCostRate(CostRate{VirtualAmount: 5}); err == nil {
		t.Error("只填虚拟数额应被拒绝")
	}

	// 两个都填 0 → 清空回到未设置
	if err := SetCostRate(CostRate{RealAmount: 0, VirtualAmount: 0, RealCurrency: "CNY"}); err != nil {
		t.Fatalf("清空失败: %v", err)
	}
	if GetCostRate().Valid() {
		t.Error("清空后应为未设置状态")
	}
}

// 渠道级覆盖优先于全局；某渠道不填就跟随全局
func TestResolveCostRate_ChannelOverridesGlobal(t *testing.T) {
	setupCostRateTestDB(t)

	if err := SetCostRate(CostRate{RealAmount: 1, VirtualAmount: 10, RealCurrency: "CNY"}); err != nil {
		t.Fatalf("保存全局比例失败: %v", err)
	}

	// 渠道 A 自己填 1:1，渠道 B 不填
	a := ai.AiModel{Name: "a", BaseURL: "https://a.example.com", RateRealAmount: 1, RateVirtualAmount: 1, RateRealCurrency: "USD"}
	b := ai.AiModel{Name: "b", BaseURL: "https://b.example.com"}
	if err := models.DB.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	if err := models.DB.Create(&b).Error; err != nil {
		t.Fatal(err)
	}

	if r, src := ResolveCostRate("https://a.example.com"); src != "channel" || r.Ratio() != 1 {
		t.Errorf("渠道 A 应命中渠道级 1:1，实际 %v / %s", r.Ratio(), src)
	}
	if r, src := ResolveCostRate("https://b.example.com"); src != "global" || r.Ratio() != 0.1 {
		t.Errorf("渠道 B 应回退全局 1:10，实际 %v / %s", r.Ratio(), src)
	}
	if _, src := ResolveCostRate("https://unknown.example.com"); src != "global" {
		t.Errorf("未知渠道应回退全局，实际 %s", src)
	}
}

// Describe 的整数不能被剥尾零：100 元 = 1000 虚拟 曾显示成 10 元 = 100 虚拟
func TestCostRate_DescribeKeepsTrailingZeros(t *testing.T) {
	cases := []struct {
		rate CostRate
		want string
	}{
		{CostRate{RealAmount: 100, VirtualAmount: 1000, RealCurrency: "CNY", VirtualCurrency: "虚拟美元"}, "100 CNY = 1000 虚拟美元"},
		{CostRate{RealAmount: 10, VirtualAmount: 1, RealCurrency: "CNY"}, "10 CNY = 1 虚拟单位"},
		{CostRate{RealAmount: 1, VirtualAmount: 10, RealCurrency: "CNY"}, "1 CNY = 10 虚拟单位"},
		{CostRate{RealAmount: 7.2, VirtualAmount: 1, RealCurrency: "CNY"}, "7.2 CNY = 1 虚拟单位"},
		{CostRate{RealAmount: 1.5, VirtualAmount: 2.5, RealCurrency: "USD"}, "1.5 USD = 2.5 虚拟单位"},
	}
	for _, c := range cases {
		if got := c.rate.Describe(); got != c.want {
			t.Errorf("Describe() = %q，期望 %q", got, c.want)
		}
	}
	// 未配置时明确说明，不能编一个 0 的值
	if got := (CostRate{}).Describe(); got != "未设置换算比例：金额仅为上游记账单位" {
		t.Errorf("未配置的描述不对: %q", got)
	}
}
