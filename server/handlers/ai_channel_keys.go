package handlers

import (
	"strconv"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 管理端：渠道卡密池 ====================
//
// 归属从「模型」改成「渠道」（base_url）：一条渠道一个池，池内多把卡密按权重轮询，
// 连续失败自动摘除。模型只记录「走哪条渠道」。
//
// 与「模型密钥池」(/ai/models/:id/keys) 的关系：后者是升级前的老接口，
// 保留可用（老站点脚本不会立刻报错），但新配置一律走渠道池。

func channelKeyVO(k *models.AiChannelKey) gin.H {
	return gin.H{
		"id": k.ID, "base_url": k.BaseURL, "label": k.Label,
		"weight": k.Weight, "enabled": k.Enabled, "fail_count": k.FailCount,
		"success_total": k.SuccessTotal, "fail_total": k.FailTotal,
		"last_fail_at": k.LastFailAt, "last_used_at": k.LastUsedAt,
		"note": k.Note,
		// 明文永不回传，只给尾 4 位辨认
		"hint": channelKeyHint(k),
	}
}

func channelKeyHint(k *models.AiChannelKey) string {
	plain, err := models.DecryptAiChannelKey(k)
	if err != nil || len(plain) < 4 {
		return ""
	}
	return "****" + plain[len(plain)-4:]
}

// AdminListAiChannelKeys GET /api/admin/ai/channel-keys?base_url=...
func AdminListAiChannelKeys(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	base := strings.TrimSpace(c.Query("base_url"))
	if base == "" {
		utils.BadRequest(c, "请指定渠道地址")
		return
	}
	list, err := models.ListAiChannelKeys(base)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, channelKeyVO(&list[i]))
	}
	utils.Success(c, gin.H{
		"list": items, "total": len(items), "base_url": base,
		"max_fail": models.MaxFailBeforeDisable,
	})
}

// AdminCreateAiChannelKey POST /api/admin/ai/channel-keys
// 支持一次粘贴多行（每行一把），方便从中转站后台直接复制。
func AdminCreateAiChannelKey(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		BaseURL string `json:"base_url"`
		Keys    string `json:"keys"`  // 多行，每行一把
		Label   string `json:"label"` // 可选，统一标签
		Note    string `json:"note"`
		Weight  int    `json:"weight"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	base := strings.TrimSpace(req.BaseURL)
	if base == "" {
		utils.BadRequest(c, "请填写渠道地址")
		return
	}
	lines := splitKeyLines(req.Keys)
	if len(lines) == 0 {
		utils.BadRequest(c, "请至少填写一把密钥")
		return
	}
	if len(lines) > 200 {
		utils.BadRequest(c, "单次最多导入 200 把，请分批")
		return
	}
	added, dups, failed := 0, 0, 0
	var lastErr string
	for i, line := range lines {
		label := req.Label
		if label == "" {
			label = "卡密 " + strconv.Itoa(i+1)
		}
		_, err := models.AddAiChannelKey(base, label, line, req.Note, req.Weight)
		if err != nil {
			// 重复是「可预期的正常情况」，与真正失败分开计数，避免站长以为全都失败了
			if strings.Contains(err.Error(), "已有同一把") {
				dups++
			} else {
				failed++
				lastErr = err.Error()
			}
			continue
		}
		added++
	}
	msg := "已添加 " + strconv.Itoa(added) + " 把"
	if dups > 0 {
		msg += "，跳过重复 " + strconv.Itoa(dups) + " 把"
	}
	if failed > 0 {
		msg += "，失败 " + strconv.Itoa(failed) + " 把"
		if lastErr != "" {
			msg += "（" + lastErr + "）"
		}
	}
	utils.Success(c, gin.H{"message": msg, "added": added, "dups": dups, "failed": failed})
}

// AdminUpdateAiChannelKey PATCH /api/admin/ai/channel-keys/:id
func AdminUpdateAiChannelKey(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		utils.BadRequest(c, "参数错误")
		return
	}
	var req struct {
		BaseURL string  `json:"base_url"`
		Label   *string `json:"label"`
		Weight  *int    `json:"weight"`
		Enabled *bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	// 限定渠道，避免拿着别人的 id 改到另一条渠道的卡密
	if strings.TrimSpace(req.BaseURL) != "" {
		if _, err := models.GetAiChannelKey(uint(id), req.BaseURL); err != nil {
			utils.NotFound(c, "卡密不存在")
			return
		}
	}
	if err := models.UpdateAiChannelKey(uint(id), req.Label, req.Weight, req.Enabled); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"message": "已保存"})
}

// AdminDeleteAiChannelKey DELETE /api/admin/ai/channel-keys/:id
func AdminDeleteAiChannelKey(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.DeleteAiChannelKey(uint(id)); err != nil {
		utils.InternalError(c, "删除失败")
		return
	}
	utils.Success(c, gin.H{"message": "已删除"})
}

// AdminTestAiChannelKey POST /api/admin/ai/channel-keys/:id/test
// 单把卡密连通性测试：GET /v1/models，不烧额度。
func AdminTestAiChannelKey(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	base := strings.TrimSpace(c.Query("base_url"))
	if id == 0 || base == "" {
		utils.BadRequest(c, "参数错误")
		return
	}
	k, err := models.GetAiChannelKey(uint(id), base)
	if err != nil {
		utils.NotFound(c, "卡密不存在")
		return
	}
	plain, err := models.DecryptAiChannelKey(k)
	if err != nil {
		utils.InternalError(c, "卡密解密失败")
		return
	}
	n, perr := probeUpstreamModels(k.BaseURL, plain, "")
	if perr != "" {
		models.ReportAiChannelKey(k.ID, false)
		utils.Success(c, gin.H{"ok": false, "error": perr})
		return
	}
	models.ReportAiChannelKey(k.ID, true)
	utils.Success(c, gin.H{"ok": true, "model_count": n, "at": time.Now().Format(time.RFC3339)})
}

// splitKeyLines 拆分多行密钥：去空行、去首尾空白、去掉行内逗号分隔的情况。
func splitKeyLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, ",", "\n")
	out := make([]string, 0, 8)
	seen := map[string]bool{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || len(line) < 6 {
			continue
		}
		if seen[line] {
			continue
		}
		seen[line] = true
		out = append(out, line)
	}
	return out
}

// ==================== 前台：模型价格表 ====================

// AiPublicPricing GET /api/ai/pricing
//
// 给用户看的价目表。与后台定价的区别：
//   - 只返回**启用中**的模型，且只给「展示名 / 类型 / 单价」，不给上游地址、密钥、优先级；
//   - 价格单位写清是「代币 / 1K tokens」还是「代币 / 次」，避免用户把按次当按量；
//   - 附三类默认价（chat/image/video），模型未单独定价时用户看到的就是这个默认价。
func AiPublicPricing(c *gin.Context) {
	var list []models.AiModel
	models.DB.Where("enabled = ?", true).Order("kind ASC, priority DESC, id ASC").Find(&list)

	items := make([]gin.H, 0, len(list))
	for i := range list {
		m := &list[i]
		items = append(items, gin.H{
			"id": m.ID, "name": m.Name, "kind": m.Kind, "vendor": m.Vendor,
			"price_in": m.PriceIn, "price_out": m.PriceOut, "price_call": m.PriceCall,
			"max_tokens": m.MaxTokens,
			// 是否按次计费：绘画/视频按次，对话按 tokens
			"per_call": m.Kind == "image" || m.Kind == "video",
		})
	}
	chatP := models.GetCategoryPrice("chat")
	imgP := models.GetCategoryPrice("image")
	vidP := models.GetCategoryPrice("video")
	utils.Success(c, gin.H{
		"models": items,
		"categories": gin.H{
			"chat":  gin.H{"price_in": chatP.PriceIn, "price_out": chatP.PriceOut},
			"image": gin.H{"price_call": imgP.PriceCall},
			"video": gin.H{"price_call": vidP.PriceCall},
		},
		"unit": gin.H{"chat": "代币 / 1K tokens", "image": "代币 / 次", "video": "代币 / 次"},
	})
}
