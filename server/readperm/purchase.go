package readperm

import (
	"time"

	"nvs-server/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ContentPurchase 内容购买记录（paid 等级的判定依据）。
//
// 本轮只建表 + 判定 + 发放函数；**扣代币 / 支付回调的购买入口属于 #67**，
// 届时在钱包事务成功后调用 GrantPurchase 即可，无需改动判定层。
type ContentPurchase struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"not null;uniqueIndex:uk_content_purchase,priority:1;index" json:"user_id"`
	ContentType string    `gorm:"size:16;not null;uniqueIndex:uk_content_purchase,priority:2" json:"content_type"` // novel / blog
	ContentID   uint      `gorm:"not null;uniqueIndex:uk_content_purchase,priority:3" json:"content_id"`
	Price       int64     `gorm:"not null;default:0" json:"price"` // 成交时代币价，留作对账
	CreatedAt   time.Time `json:"created_at"`
}

func (ContentPurchase) TableName() string { return "content_purchases" }

// Models 供根包 AutoMigrateAll 聚合（与 models/<域> 子包同一注册机制）。
func Models() []interface{} { return []interface{}{&ContentPurchase{}} }

func init() {
	models.RegisterDomainModels(Models)
}

// HasPurchased 用户是否已购买该内容。
func HasPurchased(userID uint, contentType string, contentID uint) bool {
	if userID == 0 || contentID == 0 || models.DB == nil {
		return false
	}
	var n int64
	err := models.DB.Model(&ContentPurchase{}).
		Where("user_id = ? AND content_type = ? AND content_id = ?", userID, contentType, contentID).
		Count(&n).Error
	return err == nil && n > 0
}

// GrantPurchase 记一笔购买（幂等，重复调用不产生第二行）。
// 供 #67 支付 / 钱包消费成功后调用；本轮只提供能力，不提供 HTTP 入口。
func GrantPurchase(userID uint, contentType string, contentID uint, price int64) error {
	if userID == 0 || contentID == 0 || models.DB == nil {
		return gorm.ErrInvalidData
	}
	rec := ContentPurchase{
		UserID: userID, ContentType: contentType, ContentID: contentID, Price: price,
	}
	return models.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&rec).Error
}
