package readperm

import (
	"errors"
	"strconv"

	"nvs-server/handlers/hctx"
	"nvs-server/models"
	blogm "nvs-server/models/blog"
	"nvs-server/utils/infra"

	rp "nvs-server/readperm"

	"github.com/gin-gonic/gin"
)

// resolveContentPrice 读取内容自身设定的售价，并按站长配置校正下限。
// 价格以「下单瞬间」的配置为准，避免保存后又改最低价导致历史内容错价。
func resolveContentPrice(contentType string, contentID uint) (int64, error) {
	cfg := rp.Load()
	if !cfg.IsEnabled(rp.LevelPaid) {
		return 0, errors.New("站长尚未开放付费解锁")
	}
	var permission string
	var price int64
	var wall bool
	switch contentType {
	case rp.TypeNovel:
		n, err := models.GetNovelByID(contentID)
		if err != nil {
			return 0, errors.New("作品不存在")
		}
		permission, price, wall = n.ReadPermission, n.PaidPrice, n.WallEnabled
	case rp.TypeBlog:
		b, err := blogm.GetBlogByID(contentID)
		if err != nil {
			return 0, errors.New("博客不存在")
		}
		permission, price = b.ReadPermission, b.PaidPrice
	default:
		return 0, errors.New("不支持的内容类型")
	}
	if rp.EffectivePermission(permission, wall) != rp.LevelPaid {
		return 0, errors.New("该内容不是付费内容")
	}
	if price < cfg.PaidMinPrice {
		price = cfg.PaidMinPrice
	}
	return price, nil
}

// Purchase POST /api/readperm/purchase — 用代币购买付费内容（幂等）
func Purchase(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	var req struct {
		ContentType string `json:"content_type"`
		ContentID   uint   `json:"content_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ContentID == 0 {
		infra.BadRequest(c, "参数错误")
		return
	}
	if !rp.IsKnownContentType(req.ContentType) {
		infra.BadRequest(c, "不支持的内容类型")
		return
	}
	if rp.HasPurchased(userID, req.ContentType, req.ContentID) {
		infra.Success(c, gin.H{"purchased": true, "already": true, "price": 0})
		return
	}
	price, err := resolveContentPrice(req.ContentType, req.ContentID)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	if err := rp.PurchaseContent(userID, req.ContentType, req.ContentID, price); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.Success(c, gin.H{"purchased": true, "price": price})
}

// PurchaseStatus GET /api/readperm/purchase/status?type=novel&id=1 — 查询是否已解锁
func PurchaseStatus(c *gin.Context) {
	contentType := c.Query("type")
	id, _ := strconv.ParseUint(c.Query("id"), 10, 64)
	if !rp.IsKnownContentType(contentType) || id == 0 {
		infra.BadRequest(c, "参数错误")
		return
	}
	infra.Success(c, gin.H{
		"purchased": rp.HasPurchased(hctx.CurrentUserID(c), contentType, uint(id)),
		"price":     paidPriceOrZero(contentType, uint(id)),
	})
}

func paidPriceOrZero(contentType string, contentID uint) int64 {
	p, err := resolveContentPrice(contentType, contentID)
	if err != nil {
		return 0
	}
	return p
}
