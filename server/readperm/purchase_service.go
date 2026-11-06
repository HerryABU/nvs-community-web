package readperm

import (
	"errors"
	"fmt"

	"nvs-server/models"
	wallet "nvs-server/models/wallet"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TokenTypeContentPurchase 代币流水类型：购买付费内容（负数扣费）。
const TokenTypeContentPurchase = "content_purchase"

// PurchaseContent 用代币购买内容解锁（#67 打通 #65 的付费解锁闭环）。
//
// 「扣费」与「发放购买记录」在同一事务内：任一步失败整体回滚，不会出现
// 扣了代币却没解锁，或解锁了却没扣费。已购买时直接返回成功（幂等）。
func PurchaseContent(userID uint, contentType string, contentID uint, price int64) error {
	if userID == 0 || contentID == 0 || price <= 0 || !IsKnownContentType(contentType) {
		return errors.New("购买参数无效")
	}
	if models.DB == nil {
		return errors.New("数据库未就绪")
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		if hasPurchasedTx(tx, userID, contentType, contentID) {
			return nil
		}
		_, err := wallet.AppendTokenLedgerTx(tx, userID, TokenTypeContentPurchase, -price,
			contentType, contentID, "购买付费内容", fmt.Sprintf("price=%d", price))
		if err != nil {
			return err // 含「余额不足」
		}
		return grantPurchaseTx(tx, userID, contentType, contentID, price)
	})
}

func hasPurchasedTx(tx *gorm.DB, userID uint, contentType string, contentID uint) bool {
	var n int64
	err := tx.Model(&ContentPurchase{}).
		Where("user_id = ? AND content_type = ? AND content_id = ?", userID, contentType, contentID).
		Count(&n).Error
	return err == nil && n > 0
}

func grantPurchaseTx(tx *gorm.DB, userID uint, contentType string, contentID uint, price int64) error {
	rec := ContentPurchase{
		UserID: userID, ContentType: contentType, ContentID: contentID, Price: price,
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rec).Error
}
