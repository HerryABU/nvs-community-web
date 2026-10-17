package realname

// national.go — 通用国家/授权数据服务商代理。
//
// 「国家实名接口」说明：公安部公民身份核验能力不对公众开放直连，落地形态是授权数据
// 服务商（如天远API、数脉API、聚合数据等对接部委权威数据源的网关），其接口形态统一为
// 「POST JSON {姓名, 身份证号, token} → {code/status/result}」，本提供商即适配此类网关。

import (
	"context"
	"encoding/json"
	"strings"
)

func init() { register(nationalProvider{}) }

type nationalProvider struct{}

func (nationalProvider) Name() string { return "national" }

func (nationalProvider) Verify(ctx context.Context, cfg Config, name, idCard string) (VerifyResult, error) {
	c := cfg.National
	if c.URL == "" {
		return VerifyResult{}, ErrNotConfigured
	}
	nameField := c.NameField
	if nameField == "" {
		nameField = "real_name"
	}
	cardField := c.CardField
	if cardField == "" {
		cardField = "id_card"
	}
	body := map[string]string{nameField: name, cardField: idCard}
	if c.Token != "" {
		body["token"] = c.Token
	}
	raw, err := postJSON(ctx, c.URL, nil, body)
	if err != nil {
		return VerifyResult{}, err
	}
	return parseLooseResult(raw)
}

// parseLooseResult 宽松解析常见响应形态：
//
//	{"code":0,...}              → code==0 视为一致
//	{"status":"match"|"一致"}    → 一致
//	{"result":true}             → 一致
//	{"code":200,"data":{"result":1}} → 阿里云市场系：200 + result==1
//	{"data":{"status":1}}       → 网易系：status==1 通过
func parseLooseResult(raw []byte) (VerifyResult, error) {
	var top map[string]interface{}
	if err := json.Unmarshal(raw, &top); err != nil {
		return VerifyResult{}, errLoose("响应不是合法 JSON")
	}
	out := VerifyResult{Message: "核验完成"}

	// data 嵌套形态：把 data 提升到顶层（不覆盖已有顶层键）
	if d, ok := top["data"]; ok {
		if dm := asMap(d); dm != nil {
			for k, v := range dm {
				if _, exists := top[k]; !exists {
					top[k] = v
				}
			}
		}
	}

	// 1) result 布尔
	if v, ok := top["result"].(bool); ok {
		out.Matched = v
		out.Code = "result=" + boolStr(v)
	}
	// 2) result/status/verify_result 数字：1=一致（阿里 result:1 / 网易 status:1 / 华为核验码）
	for _, key := range []string{"result", "status", "verify_result"} {
		if f, ok := numOf(top[key]); ok {
			out.Code = key + "=" + trimFloat(f)
			out.Matched = f == 1
			break
		}
	}
	// 3) code 数字：0 或 200 视为一致（腾讯/通用网关）
	if f, ok := numOf(top["code"]); ok && out.Code == "" {
		out.Code = "code=" + trimFloat(f)
		out.Matched = f == 0 || f == 200
	}
	// 4) status 字符串：match/consistent/一致/成功
	if s, ok := top["status"].(string); ok && out.Code == "" {
		low := strings.ToLower(strings.TrimSpace(s))
		out.Code = "status=" + s
		out.Matched = low == "match" || low == "consistent" || low == "一致" || low == "success" || low == "01" || low == "1"
	}
	// 5) matched 布尔（部分网关直给）
	if v, ok := top["matched"].(bool); ok && out.Code == "" {
		out.Matched = v
		out.Code = "matched=" + boolStr(v)
	}
	if msg := firstString(top, "message", "msg", "description", "reason"); msg != "" {
		out.Message = msg
	}
	if out.Code == "" {
		return VerifyResult{}, errLoose("无法识别的核验响应格式")
	}
	return out, nil
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func trimFloat(f float64) string {
	if f == float64(int64(f)) {
		return itoa(int64(f))
	}
	return strings.TrimRight(strings.TrimRight(formatFloat(f), "0"), ".")
}
