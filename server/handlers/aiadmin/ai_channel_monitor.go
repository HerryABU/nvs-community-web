package aiadmin

import (
	"encoding/json"
	"io"
	"net/http"
	"nvs-server/handlers/admincommon"
	"strconv"
	"strings"
	"time"

	"nvs-server/aiclient"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	aihub "nvs-server/models/aihub"
)

// ==================== 渠道监控（临时凭据探测） ====================
//
// 场景：站长拿到一条新渠道，只想先看看「还剩多少 / 能用哪些模型 / 最近花了多少」，
// 不想为了试一下就在模型池里建一条记录再把密钥存进去。
//
// 与「渠道余额」的区别：
//   - 渠道余额：基于**已保存**的模型配置，按渠道聚合，可长期采样、可估算单次成本；
//   - 渠道监控：**不落库**，当场输令牌 + 用户 ID 探一次，用完即弃。
//
// 安全约束（写死，不给开关）：
//   - 令牌只在本次请求内存里用，**绝不写库、不写日志**（日志里出现也被脱敏层盖掉）；
//   - 不返回上游原始响应体（可能含账户信息），只返回解析后的几个数字；
//   - 单次只允许查一条渠道，避免被当成批量扫描工具。

type channelMonitorReq struct {
	BaseURL string `json:"base_url"`
	Token   string `json:"token"`
	UserID  string `json:"user_id"` // New API 体系需要与访问令牌配对
	// ModelKey 可选的模型密钥（sk_）。
	// 为什么单独一项：New API 体系里账户令牌（nap_）只能查账户数据，
	// 调 /v1/models 会被 401；要探「能拉多少模型」必须用模型密钥。
	// 两者分工不同，硬塞进一个字段只会让人以为是「令牌无效」。
	ModelKey string `json:"model_key"`
	Kind     string `json:"kind"` // 留空=自动判断：nap_/用户ID→newapi，sk_+deepseek域名→deepseek，否则通用
}

// AdminAiChannelMonitor POST /api/admin/ai/channel-monitor
//
// 一次请求同时回答三件事：
//  1. 这条渠道通不通（GET /models，不烧额度）；
//  2. 账户还剩多少（能拿到就返回，拿不到明确说原因）；
//  3. 最近几笔实际扣费（仅 New API 系，需令牌含「查看日志」权限）。
func AdminAiChannelMonitor(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var req channelMonitorReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	in := normalizeMonitorInputs(req.BaseURL, req.Token, req.UserID, req.ModelKey)
	base, token, uid, modelKey, warnings := in.Base, in.Token, in.UserID, in.ModelKey, in.Warnings
	if base == "" {
		utils.BadRequest(c, "请填写渠道地址")
		return
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		utils.BadRequest(c, "渠道地址需以 http:// 或 https:// 开头")
		return
	}
	// 没填令牌 → 用该渠道**加密入库**的凭据（解密后当场用、用完即弃，不回显）
	usedSaved := false
	var notices []string
	if token == "" {
		if acc, aerr := aihub.GetAiChannelAccount(base); aerr == nil && acc.Enabled {
			if t, derr := acc.DecryptAccessToken(); derr == nil && t != "" {
				token = t
				usedSaved = true
			}
			if uid == "" {
				uid = acc.UserID
			}
			if modelKey == "" {
				if mk, derr := acc.DecryptModelKey(); derr == nil && mk != "" {
					modelKey = mk
				}
			}
			if usedSaved {
				notices = append(notices, "已使用该渠道保存的凭据（令牌不回显）")
			}
		}
	}
	if token == "" {
		utils.BadRequest(c, "请填写令牌（模型密钥 sk_ 或系统访问令牌 nap_），或先在本页「保存到渠道」")
		return
	}

	kind := resolveMonitorKind(strings.ToLower(strings.TrimSpace(req.Kind)), token, base, uid)

	out := gin.H{
		"kind":     kind,
		"base_url": base,
	}
	if len(warnings) > 0 {
		out["warnings"] = warnings
	}
	if len(notices) > 0 {
		out["notices"] = notices
	}

	// ── 1. 连通性 + 可用模型数（最便宜的一步，先给个确定结论）──
	// 账户令牌（nap_）不用于调用模型，直接跳过硬探，否则只会拿到一个误导性的 401。
	probeKey := modelKey
	if probeKey == "" && !strings.HasPrefix(token, "nap_") {
		probeKey = token
	}
	if probeKey == "" {
		out["probe_ok"] = false
		out["probe_skipped"] = true
		out["probe_error"] = "账户令牌（nap_）不能用于调用模型接口，已跳过模型列表探测；如需探测可用模型，请填「模型密钥」（sk_）"
	} else if modelsN, probeErr := probeUpstreamModels(base, probeKey, uid); probeErr != "" {
		out["probe_ok"] = false
		out["probe_error"] = probeErr
	} else {
		out["probe_ok"] = true
		out["model_count"] = modelsN
	}

	// ── 2. 余额 ──
	switch kind {
	case "newapi":
		acct := aiclient.QueryNewAPIAccount(base, token, uid, 15*time.Second)
		out["balance_ok"] = acct.OK
		if acct.OK {
			out["remaining"] = acct.Quota
			out["used"] = acct.UsedQuota
			out["currency"] = acct.Currency
		} else {
			out["balance_error"] = acct.Error
		}
		// ── 3. 最近使用日志（精确单次扣费）──
		if logs, lerr := aiclient.QueryNewAPIRecentLogs(base, token, uid, 5, 15*time.Second); lerr != "" {
			out["logs_error"] = lerr
		} else {
			out["recent_logs"] = logs
		}
	case "deepseek":
		b := aiclient.QueryChannelBalanceWithUser(base, token, "", uid)
		out["balance_ok"] = b.Supported
		if b.Supported {
			out["remaining"] = b.TotalRemaining
			out["used"] = b.TotalUsed
			out["currency"] = b.Currency
		} else {
			out["balance_error"] = b.Error
		}
	default:
		// 通用/自建：走一次 OpenAI 兼容的 credit_grants 探测，拿不到就明说
		b := aiclient.QueryChannelBalanceWithUser(base, token, "", uid)
		out["balance_ok"] = b.Supported
		if b.Supported {
			out["remaining"] = b.TotalRemaining
			out["used"] = b.TotalUsed
			out["currency"] = b.Currency
		} else {
			out["balance_error"] = firstNonEmpty(b.Error,
				"该渠道未提供标准余额接口；可改用「自定义规则」方式长期配置")
		}
	}

	// 有保存凭据的渠道才记快照（监控是「临时看一眼」的主路径，别给所有探测都留痕）
	if _, aerr := aihub.GetAiChannelAccount(base); aerr == nil {
		okFlag, _ := out["balance_ok"].(bool)
		if v, has := out["probe_ok"].(bool); has && v {
			okFlag = true
		}
		aihub.RecordAiChannelProbe(base, okFlag,
			intOf(out["model_count"]),
			floatOf(out["remaining"]), floatOf(out["used"]),
			stringOf(out["currency"]), worstError(out))
	}

	utils.Success(c, out)
}

// worstError 把各种 *_error 合成一句，供快照里显示
func worstError(out gin.H) string {
	for _, k := range []string{"balance_error", "probe_error", "logs_error"} {
		if s, ok := out[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func intOf(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

func floatOf(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	}
	return 0
}

func stringOf(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// resolveMonitorKind 推断用哪种查法：
//   - 明确指定则照办；
//   - nap_ 前缀，或填了用户 ID → New API 中转站；
//   - deepseek 官方域名 → 官方 /user/balance；
//   - 其余按通用处理（sk_ 密钥本身查不到账户余额，这是上游规则，不是本程序限制）。
func resolveMonitorKind(kind, token, base, uid string) string {
	switch kind {
	case "newapi", "deepseek", "generic":
		return kind
	}
	if strings.HasPrefix(token, "nap_") || uid != "" {
		return "newapi"
	}
	if strings.Contains(base, "api.deepseek.com") {
		return "deepseek"
	}
	return "generic"
}

// probeUpstreamModels 拉一次 /models 看通不通，返回可用模型数。
// 只取数量，不留存任何模型清单——监控弹窗不需要，也避免把上游目录抄下来。
func probeUpstreamModels(base, token, uid string) (int, string) {
	url := base
	if !strings.HasSuffix(url, "/v1") {
		url += "/v1"
	}
	url += "/models"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, "请求构造失败"
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if uid != "" {
		req.Header.Set("New-Api-User", uid)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "连接失败：" + err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return 0, "令牌无效或无权限（HTTP " + strconv.Itoa(resp.StatusCode) + "）"
	}
	if resp.StatusCode >= 400 {
		return 0, "上游返回 HTTP " + strconv.Itoa(resp.StatusCode)
	}

	var parsed struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, "响应不是 OpenAI 兼容格式（可能不是模型接口）"
	}
	return len(parsed.Data), ""
}

// monitorInputs 归一化后的监控入参
type monitorInputs struct {
	Base     string
	Token    string
	UserID   string
	ModelKey string
	Warnings []string
}

// normalizeMonitorInputs 处理两种高频填错：
//  1. 渠道地址粘成了具体接口（/api/user/self、/v1/models）→ 还原成站点根，
//     否则会拼成 .../api/user/self/api/user/self 直接 404；
//  2. 令牌与用户 ID 填反（sk_ 填进「用户 ID」、56347 填进「令牌」）→ 按形态自动纠正。
//
// 抽成纯函数是为了能被单测锁住 —— 这两条都是站长实际踩过的坑。
func normalizeMonitorInputs(rawBase, token, uid, modelKey string) monitorInputs {
	rawBase = strings.TrimSpace(rawBase)
	out := monitorInputs{
		Base:     aiclient.NormalizeBaseURL(rawBase),
		Token:    strings.TrimSpace(token),
		UserID:   strings.TrimSpace(uid),
		ModelKey: strings.TrimSpace(modelKey),
	}
	if out.Base != "" && out.Base != strings.TrimRight(rawBase, "/") {
		out.Warnings = append(out.Warnings, "渠道地址已归一化为站点根："+out.Base)
	}
	if looksLikeUserID(out.Token) && out.UserID == "" {
		out.UserID, out.Token = out.Token, ""
		out.Warnings = append(out.Warnings, "「令牌」里填的是数字，已按用户 ID 处理")
	}
	if looksLikeToken(out.UserID) {
		if out.Token == "" {
			out.Token, out.UserID = out.UserID, ""
			out.Warnings = append(out.Warnings, "「用户 ID」里填的是令牌，已自动搬到「令牌」")
		} else {
			out.UserID = ""
			out.Warnings = append(out.Warnings, "「用户 ID」里填的像令牌，已忽略该值")
		}
	}
	if looksLikeUserID(out.ModelKey) {
		if out.UserID == "" {
			out.UserID = out.ModelKey
			out.Warnings = append(out.Warnings, "「模型密钥」里填的是数字，已按用户 ID 处理")
		}
		out.ModelKey = ""
	}
	return out
}

// looksLikeToken 判断是否像上游令牌（ModelKey / 系统访问令牌）
func looksLikeToken(s string) bool {
	l := strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(l, "sk_") || strings.HasPrefix(l, "sk-") || strings.HasPrefix(l, "nap_")
}

// looksLikeUserID 判断是否像 New API 的数字用户 ID
func looksLikeUserID(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 20 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
