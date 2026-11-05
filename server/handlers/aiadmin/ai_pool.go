package aiadmin

import (
	"strconv"
	"strings"

	"nvs-server/handlers/admin"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	aim "nvs-server/models/ai"
	aiops "nvs-server/models/aiops"
)

// ==================== AI 模型池管理端（§5.3） ====================
// api_key 加密落库、只写不读：列表/详情不回传密钥，仅回传 has_api_key；
// 更新时留空表示不轮换。

// aiModelVO 列表视图（密钥脱敏）
func aiModelVO(m *aim.AiModel) gin.H {
	return gin.H{
		"id":            m.ID,
		"name":          m.Name,
		"kind":          aim.NormalizeAiKind(m.Kind),
		"model_id":      m.ModelID,
		"base_url":      m.BaseURL,
		"has_api_key":   m.ApiKeyEnc != "",
		"price_in":      m.PriceIn,
		"price_out":     m.PriceOut,
		"price_call":    m.PriceCall,
		"max_tokens":    m.MaxTokens,
		"temperature":   m.Temperature,
		"enabled":       m.Enabled,
		"priority":      m.Priority,
		"api_kind":      aim.NormalizeAiApiKind(m.ApiKind),
		"provider_hint": m.ProviderHint,
		"vendor":        m.Vendor,
		"note":          m.Note,
		// 前台可见（*bool：null=遗留模型默认可见，false=拉取导入默认隐藏）
		// 不给前端这个字段，后台就没有任何地方能看出「用户为什么看不到这个模型」。
		"user_visible": m.UserVisible,
		"thinking":     m.Thinking,
		// 联网检索能力位：不给前端的话，编辑表单保存时会把这两个勾清空
		"search_endpoint": m.SearchEndpoint,
		"search_native":   m.SearchNative,
		"created_at":      m.CreatedAt,
		"updated_at":      m.UpdatedAt,
	}
}

// AdminGetAiConfig GET /api/admin/ai/config
func AdminGetAiConfig(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	utils.Success(c, gin.H{"enabled": aim.IsAiEnabled()})
}

// AdminSetAiConfig PUT /api/admin/ai/config
func AdminSetAiConfig(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.Enabled == nil {
		utils.BadRequest(c, "缺少 enabled 字段")
		return
	}
	if err := aim.SetAiEnabled(*req.Enabled); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"message": "已保存（立即生效）"})
}

// AdminListAiModels GET /api/admin/ai/models
func AdminListAiModels(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	list, err := aim.ListAiModels(false)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, aiModelVO(&list[i]))
	}
	utils.Success(c, gin.H{"list": items, "total": len(items)})
}

// aiModelInput 创建/更新共用入参
type aiModelInput struct {
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	ModelID      string   `json:"model_id"`
	BaseURL      string   `json:"base_url"`
	ApiKey       string   `json:"api_key"`       // 留空=不轮换（sk_ 模型调用密钥）
	AccountToken string   `json:"account_token"` // 系统访问令牌（nap_，查账户余额用）；留空=不轮换
	TokenUserID  string   `json:"token_user_id"` // 中转站用户 ID（New-Api-User）
	PriceIn      *int64   `json:"price_in"`
	PriceOut     *int64   `json:"price_out"`
	PriceCall    *int64   `json:"price_call"`
	MaxTokens    *int     `json:"max_tokens"`
	Temperature  *float64 `json:"temperature"`
	Enabled      *bool    `json:"enabled"`
	Priority     *int     `json:"priority"`
	ApiKind      string   `json:"api_kind"`
	ProviderHint string   `json:"provider_hint"`
	UserVisible  *bool    `json:"user_visible"` // 是否对用户开放（前台模型选择/价目表）
	// Vendor / Note 用指针：模型收费页保存单模型价格时只传价格三件套，
	// 若这里无条件写，会把厂商与备注清空（实测会让模型在选择器里全掉进「其它」分组）。
	Vendor   *string `json:"vendor"`
	Thinking *bool   `json:"thinking"`
	Note     *string `json:"note"`

	// ── 联网检索能力位（可选，nil = 不改动）──
	// SearchEndpoint 上游提供 /search 检索接口（平台先检索再归纳）
	// SearchNative   /chat/completions 自带联网检索（平台只在请求里带上网检索参数）
	SearchEndpoint *bool `json:"search_endpoint"`
	SearchNative   *bool `json:"search_native"`

	// ── 余额查询方式（官方 / 中转站 / 陌生渠道分开）──
	// 这几个字段之前在表单里能填、但结构体没接收 ⇒ 提交后被静默丢弃，
	// 站长以为设了「自定义规则」，实际永远走自动推断。属真空转，必须接收。
	BalanceProvider   *string  `json:"balance_provider"`
	BalanceURL        *string  `json:"balance_url"`
	BalanceField      *string  `json:"balance_field"`
	BalanceDivisor    *float64 `json:"balance_divisor"`
	BalanceMultiplier *float64 `json:"balance_multiplier"`
	BalanceCurrency   *string  `json:"balance_currency"`

	// ── 渠道级真实成本换算（留空 = 跟随站点全局比例）──
	RateRealAmount    *float64 `json:"rate_real_amount"`
	RateVirtualAmount *float64 `json:"rate_virtual_amount"`
	RateRealCurrency  string   `json:"rate_real_currency"`
}

// normalizeBalanceProvider 归一「余额查询方式」。
// 空 = 自动推断（按域名/是否配了账户令牌判断），非法值也回退空，
// 避免前端传个拼错的字符串就让余额查询彻底失效。
// strVal 取指针字符串的值（nil 视为空），用于「不传就别写」的可选字段
func strVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func normalizeBalanceProvider(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "deepseek":
		return "deepseek"
	case "newapi":
		return "newapi"
	case "custom":
		return "custom"
	case "none":
		return "none"
	}
	return ""
}

// AdminCreateAiModel POST /api/admin/ai/models
func AdminCreateAiModel(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	var req aiModelInput
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	name := strings.TrimSpace(req.Name)
	modelID := strings.TrimSpace(req.ModelID)
	baseURL := strings.TrimSpace(req.BaseURL)
	if name == "" || modelID == "" || baseURL == "" {
		utils.BadRequest(c, "展示名、模型标识、上游地址均为必填")
		return
	}
	kind := aim.NormalizeAiKind(strings.TrimSpace(req.Kind))
	m := &aim.AiModel{
		Name: name, Kind: kind, ModelID: modelID, BaseURL: baseURL,
		ApiKind:      aim.NormalizeAiApiKind(req.ApiKind),
		ProviderHint: aim.NormalizeProviderHint(req.ProviderHint),
		Vendor:       strings.TrimSpace(strVal(req.Vendor)),
		Note:         strings.TrimSpace(strVal(req.Note)),

		// 余额查询方式（官方 / 中转站 / 自定义规则）
		BalanceProvider: normalizeBalanceProvider(strVal(req.BalanceProvider)),
		BalanceURL:      strings.TrimSpace(strVal(req.BalanceURL)),
		BalanceField:    strings.TrimSpace(strVal(req.BalanceField)),
		BalanceCurrency: strings.TrimSpace(strVal(req.BalanceCurrency)),
		// 渠道级换算比例（两个都为 0 = 跟随全局）
		RateRealCurrency: strings.ToUpper(strings.TrimSpace(req.RateRealCurrency)),
	}
	if req.UserVisible != nil {
		m.UserVisible = req.UserVisible
	}
	if req.BalanceDivisor != nil {
		m.BalanceDivisor = *req.BalanceDivisor
	}
	if req.BalanceMultiplier != nil {
		m.BalanceMultiplier = *req.BalanceMultiplier
	}
	if req.RateRealAmount != nil {
		m.RateRealAmount = *req.RateRealAmount
	}
	if req.RateVirtualAmount != nil {
		m.RateVirtualAmount = *req.RateVirtualAmount
	}
	if req.Thinking != nil {
		m.Thinking = *req.Thinking
	}
	if req.SearchEndpoint != nil {
		m.SearchEndpoint = *req.SearchEndpoint
	}
	if req.SearchNative != nil {
		m.SearchNative = *req.SearchNative
	}
	if req.PriceIn != nil {
		m.PriceIn = *req.PriceIn
	}
	if req.PriceOut != nil {
		m.PriceOut = *req.PriceOut
	}
	if req.PriceCall != nil {
		m.PriceCall = *req.PriceCall
	}
	if req.MaxTokens != nil {
		m.MaxTokens = *req.MaxTokens
	}
	if req.Temperature != nil {
		m.Temperature = *req.Temperature
	}
	if req.Enabled != nil {
		m.Enabled = *req.Enabled
	}
	if req.Priority != nil {
		m.Priority = *req.Priority
	}
	m.TokenUserID = strings.TrimSpace(req.TokenUserID)
	if strings.TrimSpace(req.AccountToken) != "" {
		m.AccountTokenEnc = strings.TrimSpace(req.AccountToken) // CreateAiModel 内部加密
	}
	if err := aim.CreateAiModel(m, req.ApiKey); err != nil {
		utils.InternalError(c, "创建失败")
		return
	}
	utils.Success(c, gin.H{"message": "已创建", "id": m.ID})
}

// AdminUpdateAiModel PUT /api/admin/ai/models/:id
func AdminUpdateAiModel(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if _, err := aim.GetAiModel(uint(id)); err != nil {
		utils.NotFound(c, "模型不存在")
		return
	}
	var req aiModelInput
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	fields := map[string]interface{}{}
	if s := strings.TrimSpace(req.Name); s != "" {
		fields["name"] = s
	}
	if s := strings.TrimSpace(req.Kind); s != "" {
		fields["kind"] = aim.NormalizeAiKind(s)
	}
	if s := strings.TrimSpace(req.ModelID); s != "" {
		fields["model_id"] = s
	}
	if s := strings.TrimSpace(req.BaseURL); s != "" {
		fields["base_url"] = s
	}
	if req.PriceIn != nil {
		fields["price_in"] = *req.PriceIn
	}
	if req.PriceOut != nil {
		fields["price_out"] = *req.PriceOut
	}
	if req.PriceCall != nil {
		fields["price_call"] = *req.PriceCall
	}
	if req.MaxTokens != nil {
		fields["max_tokens"] = *req.MaxTokens
	}
	if req.Temperature != nil {
		fields["temperature"] = *req.Temperature
	}
	if req.Enabled != nil {
		fields["enabled"] = *req.Enabled
	}
	if req.Priority != nil {
		fields["priority"] = *req.Priority
	}
	if s := strings.TrimSpace(req.ApiKind); s != "" {
		fields["api_kind"] = aim.NormalizeAiApiKind(s)
	}
	if s := strings.TrimSpace(req.ProviderHint); s != "" {
		fields["provider_hint"] = aim.NormalizeProviderHint(s)
	}
	if req.UserVisible != nil {
		fields["user_visible"] = *req.UserVisible
	}
	// 以下字段只在请求里显式出现过才写：模型收费页只改价格时不能把厂商/备注刷空
	if req.Vendor != nil {
		fields["vendor"] = strings.TrimSpace(*req.Vendor)
	}
	if req.Note != nil {
		fields["note"] = strings.TrimSpace(*req.Note)
	}
	if req.BalanceProvider != nil {
		fields["balance_provider"] = normalizeBalanceProvider(*req.BalanceProvider)
	}
	if req.BalanceURL != nil {
		fields["balance_url"] = strings.TrimSpace(*req.BalanceURL)
	}
	if req.BalanceField != nil {
		fields["balance_field"] = strings.TrimSpace(*req.BalanceField)
	}
	if req.BalanceCurrency != nil {
		fields["balance_currency"] = strings.TrimSpace(*req.BalanceCurrency)
	}
	if req.BalanceDivisor != nil {
		fields["balance_divisor"] = *req.BalanceDivisor
	}
	if req.BalanceMultiplier != nil {
		fields["balance_multiplier"] = *req.BalanceMultiplier
	}
	// 渠道级换算比例：允许改回 0（跟随全局）
	if req.RateRealAmount != nil {
		fields["rate_real_amount"] = *req.RateRealAmount
	}
	if req.RateVirtualAmount != nil {
		fields["rate_virtual_amount"] = *req.RateVirtualAmount
	}
	if s := strings.ToUpper(strings.TrimSpace(req.RateRealCurrency)); s != "" {
		fields["rate_real_currency"] = s
	}
	if req.Thinking != nil {
		fields["thinking"] = *req.Thinking
	}
	if req.SearchEndpoint != nil {
		fields["search_endpoint"] = *req.SearchEndpoint
	}
	if req.SearchNative != nil {
		fields["search_native"] = *req.SearchNative
	}
	if strings.TrimSpace(req.TokenUserID) != "" {
		fields["token_user_id"] = strings.TrimSpace(req.TokenUserID)
	}
	if at := strings.TrimSpace(req.AccountToken); at != "" {
		if enc, e := utils.EncryptField(at); e == nil {
			fields["account_token_enc"] = enc
		}
	}
	if err := aim.UpdateAiModelFields(uint(id), fields, req.ApiKey); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"message": "已保存（立即生效）"})
}

// AdminDeleteAiModel DELETE /api/admin/ai/models/:id
func AdminDeleteAiModel(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := aim.DeleteAiModel(uint(id)); err != nil {
		utils.InternalError(c, "删除失败")
		return
	}
	utils.Success(c, gin.H{"message": "已删除"})
}

// AdminAiModelPresets GET /api/admin/ai/presets — 模型预设（绘画/视频/对话）
// 站长点「预设」即可填充 base_url / model_id / api_kind / 计费默认值，只需补自己的密钥。
func AdminAiModelPresets(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	kind := strings.TrimSpace(c.Query("kind"))
	all := aiops.ListAiModelPresets()
	if kind == "" {
		utils.Success(c, gin.H{"list": all})
		return
	}
	out := make([]aiops.AiModelPreset, 0, len(all))
	for _, p := range all {
		if p.Kind == aim.NormalizeAiKind(kind) {
			out = append(out, p)
		}
	}
	utils.Success(c, gin.H{"list": out})
}
