package realname

// huawei.go — 华为云人证核身服务 IVS 证件版（二要素：姓名+身份证号）。
// API：POST https://ivs.<region>.myhuaweicloud.com/v2.0/ivs-idcard
// 签名：AK/SK + SDK-HMAC-SHA256（华为云标准签名，纯标准库实现）。
// 响应解析宽松兼容 verify_result / result / status 多种形态。

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func init() { register(huaweiProvider{}) }

type huaweiProvider struct{}

func (huaweiProvider) Name() string { return "huawei" }

func (huaweiProvider) Verify(ctx context.Context, cfg Config, name, idCard string) (VerifyResult, error) {
	c := cfg.Huawei
	if c.AK == "" || c.SK == "" {
		return VerifyResult{}, ErrNotConfigured
	}
	endpoint := orDefault(c.Endpoint, "ivs.cn-north-4.myhuaweicloud.com")
	uri := orDefault(c.URI, "/v2.0/ivs-idcard")
	region := regionFromEndpoint(endpoint) // ivs.cn-north-4.myhuaweicloud.com → cn-north-4

	payload := fmt.Sprintf(`{"name":%q,"number":%q}`, name, idCard)
	now := time.Now().UTC()
	xSdkDate := now.Format("2006-01-02T15:04:05Z")
	date := now.Format("2006-01-02")

	// CanonicalRequest：Method \n URI \n Query \n CanonicalHeaders \n SignedHeaders \n HashedPayload
	canonicalHeaders := "host:" + endpoint + "\n" + "x-sdk-date:" + xSdkDate + "\n"
	signedHeaders := "host;x-sdk-date"
	canonicalRequest := "POST\n" + uri + "\n\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + sha256Hex(payload)
	stringToSign := "SDK-HMAC-SHA256\n" + xSdkDate + "\n" + sha256Hex(canonicalRequest)

	// SigningKey：HMAC 派生（"SDK"+SK → date → region → service → sdk_request）
	kSecret := hmacSHA256([]byte("SDK"+c.SK), date)
	kRegion := hmacSHA256(kSecret, region)
	kService := hmacSHA256(kRegion, "ivs")
	kSigning := hmacSHA256(kService, "sdk_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	authorization := fmt.Sprintf(
		"SDK-HMAC-SHA256 Credential=%s/%s/%s/ivs/sdk_request, SignedHeaders=%s, Signature=%s",
		c.AK, date, region, signedHeaders, signature,
	)

	raw, err := postJSON(ctx, "https://"+endpoint+uri, map[string]string{
		"Authorization": authorization,
		"X-Sdk-Date":    xSdkDate,
	}, json.RawMessage(payload))
	if err != nil {
		return VerifyResult{}, err
	}
	return parseHuaweiResult(raw)
}

// regionFromEndpoint 从 endpoint 提取 region：ivs.cn-north-4.myhuaweicloud.com → cn-north-4
func regionFromEndpoint(endpoint string) string {
	parts := strings.Split(endpoint, ".")
	if len(parts) >= 2 && parts[0] != "" {
		return parts[1]
	}
	return "cn-north-4"
}

// parseHuaweiResult 解析 IVS 响应。兼容：
//
//	{"verify_result":"VALID"}         — 证件有效/一致
//	{"result":{"verify_result":"01"}} — 嵌套形态
//	{"result":1} / {"status":1}       — 通用数字形态（走 parseLooseResult）
func parseHuaweiResult(raw []byte) (VerifyResult, error) {
	var top map[string]interface{}
	if err := jsonUnmarshalHelper(raw, &top); err != nil {
		return VerifyResult{}, errLoose("华为云响应解析失败")
	}
	// error 形态：{"error_code":"IVS.0001","error_msg":"..."}
	if code, ok := top["error_code"].(string); ok {
		msg, _ := top["error_msg"].(string)
		return VerifyResult{}, fmt.Errorf("华为云返回错误 %s: %s", code, msg)
	}

	// 找 verify_result（可能在顶层或 result 嵌套内）
	vr := ""
	if s, ok := top["verify_result"].(string); ok {
		vr = s
	} else if rm := asMap(top["result"]); rm != nil {
		if s, ok := rm["verify_result"].(string); ok {
			vr = s
		}
	}
	if vr != "" {
		low := strings.ToUpper(strings.TrimSpace(vr))
		matched := low == "VALID" || low == "MATCH" || low == "01" || low == "1" || low == "一致"
		return VerifyResult{Matched: matched, Code: "verify_result=" + vr, Message: verifyResultMessage(matched)}, nil
	}
	// 退化通用解析（result/status 数字）
	return parseLooseResult(raw)
}

func verifyResultMessage(matched bool) string {
	if matched {
		return "姓名与身份证号核验一致"
	}
	return "姓名与身份证号核验不一致"
}
