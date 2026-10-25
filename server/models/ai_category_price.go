package models

import "time"

// ==================== AI 模型分类定价（§厂商管理：按聚合类设价） ====================
// 模型聚合类（kind）固定为 3 种：chat / image / video。
//  Admin 可针对每种分类设置默认单价；导入模型时若该模型未显式给价，则套用分类默认价。
//  单模型仍可单独覆盖（AiModel.PriceIn/Out/Call 优先）。

// AiCategoryPrice 分类默认价（每行对应一种 kind，主键 kind）
type AiCategoryPrice struct {
	Kind      string    `gorm:"primaryKey" json:"kind"`               // chat / image / video
	PriceIn   int64     `gorm:"not null;default:0" json:"price_in"`   // 每 1K 输入 tokens（chat 用）
	PriceOut  int64     `gorm:"not null;default:0" json:"price_out"`  // 每 1K 输出 tokens（chat 用）
	PriceCall int64     `gorm:"not null;default:0" json:"price_call"` // 每次调用（image/video 用）
	UpdatedAt time.Time `json:"updated_at"`
}

func (AiCategoryPrice) TableName() string { return "ai_category_prices" }

// AllCategoryPrices 返回全部 3 种分类的定价（缺哪类补零值行，保证前端始终拿到 3 项）
func AllCategoryPrices() ([]AiCategoryPrice, error) {
	var list []AiCategoryPrice
	if err := DB.Find(&list).Error; err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, p := range list {
		have[p.Kind] = true
	}
	for _, k := range []string{AiKindChat, AiKindImage, AiKindVideo} {
		if !have[k] {
			list = append(list, AiCategoryPrice{Kind: k})
		}
	}
	return list, nil
}

// GetCategoryPrice 取某分类定价（不存在返回零值）
func GetCategoryPrice(kind string) AiCategoryPrice {
	kind = NormalizeAiKind(kind)
	var p AiCategoryPrice
	if err := DB.Where("kind = ?", kind).First(&p).Error; err != nil {
		return AiCategoryPrice{Kind: kind}
	}
	return p
}

// SetCategoryPrice 写入/更新某分类定价（map 更新，零值安全）
func SetCategoryPrice(kind string, priceIn, priceOut, priceCall int64) error {
	kind = NormalizeAiKind(kind)
	now := time.Now()
	var cnt int64
	DB.Model(&AiCategoryPrice{}).Where("kind = ?", kind).Count(&cnt)
	if cnt > 0 {
		return DB.Model(&AiCategoryPrice{}).Where("kind = ?", kind).Updates(map[string]interface{}{
			"price_in": priceIn, "price_out": priceOut, "price_call": priceCall, "updated_at": now,
		}).Error
	}
	return DB.Create(&AiCategoryPrice{Kind: kind, PriceIn: priceIn, PriceOut: priceOut, PriceCall: priceCall, UpdatedAt: now}).Error
}
