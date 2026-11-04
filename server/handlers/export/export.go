package export

// export.go 反向桥接层（极薄）。
//
// 评论创建后由 comment 包调用 MaybeTriggerAiBot 触发 @ai 召唤；其实现位于
// handlers/ai，由那边 init 时注册进来。若让 comment 直接 import ai、ai 又 import
// comment，就会成环，所以用本中性包承载这个「回调变量」。本包不依赖任何业务子包。

import (
	"github.com/gin-gonic/gin"

	"nvs-server/models"
)

// MaybeTriggerAiBot 评论创建后触发 @ai 召唤。
//
// 实现位于子包 handlers/ai，由那边 init 时注册 —— 未注册时为空实现，语义与
// 「没有该功能」一致。
var MaybeTriggerAiBot = func(c *gin.Context, comment *models.Comment) {}
