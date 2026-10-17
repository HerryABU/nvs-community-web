package realname

// tencent.go — 腾讯云人脸核身 身份证信息核验（二要素）。
// API：POST https://faceid.tencentcloudapi.com/  Action=IdCardVerification Version=2018-03-01
// 结果码：0=一致（收费）/ -1=不一致（收费）/ -2..-7=输入或服务异常（不收费）。
// 签名：TC3-HMAC-SHA256（腾讯云 API 3.0 标准签名，纯标准库实现，无需 SDK）。

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

func init() { register(tencentProvider{}) }

type tencentProvider struct{}

func (tencentProvider) Name() string { return "tencent" }

const (
	tcAction  = "IdCardVerification"
	tcVersion = "2018-03-01"
	tcService = "faceid"
)

// tcHost 腾讯云人脸核身 API 域名（测试时可替换为 httptest 地址）
var tcHost = "faceid.tencentcloudapi.com"

// tcResponse 腾讯云 API 3.0 统一响应信封
type tcResponse struct {
	Response struct {
		Result      string `json:"Result"`
		Description string `json:"Description"`
		RequestID   string `json:"RequestId"`
		Error       *struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error"`
	} `json:"Response"`
}

func (tencentProvider) Verify(ctx context.Context, cfg Config, name, idCard string) (VerifyResult, error) {
	c := cfg.Tencent
	if c.SecretID == "" || c.SecretKey == "" {
		return VerifyResult{}, ErrNotConfigured
	}
	payload := fmt.Sprintf(`{"IdCard":%q,"Name":%q}`, idCard, name)
	timestamp := time.Now().Unix()

	// 1) CanonicalRequest
	hashedPayload := sha256Hex(payload)
	canonicalHeaders := "content-type:application/json\nhost:" + tcHost + "\n"
	signedHeaders := "content-type;host"
	canonicalRequest := "POST\n/\n\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + hashedPayload

	// 2) StringToSign
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	credentialScope := date + "/" + tcService + "/tc3_request"
	stringToSign := "TC3-HMAC-SHA256\n" + strconv.FormatInt(timestamp, 10) + "\n" + credentialScope + "\n" + sha256Hex(canonicalRequest)

	// 3) 派生签名密钥并计算签名
	secretDate := hmacSHA256([]byte("TC3"+c.SecretKey), date)
	secretService := hmacSHA256(secretDate, tcService)
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))

	authorization := fmt.Sprintf(
		"TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		c.SecretID, credentialScope, signedHeaders, signature,
	)

	raw, err := postJSON(ctx, "https://"+tcHost+"/", map[string]string{
		"Authorization":  authorization,
		"X-TC-Action":    tcAction,
		"X-TC-Version":   tcVersion,
		"X-TC-Timestamp": strconv.FormatInt(timestamp, 10),
	}, json.RawMessage(payload))
	if err != nil {
		return VerifyResult{}, err
	}
	var resp tcResponse
	return parseTencentResp(raw, &resp)
}

// parseTencentResp 解析腾讯云响应（独立函数便于单测）
func parseTencentResp(raw []byte, resp *tcResponse) (VerifyResult, error) {
	if err := json.Unmarshal(raw, resp); err != nil {
		return VerifyResult{}, errLoose("腾讯云响应解析失败")
	}
	if resp.Response.Error != nil {
		return VerifyResult{}, fmt.Errorf("腾讯云返回错误 %s: %s", resp.Response.Error.Code, resp.Response.Error.Message)
	}
	switch resp.Response.Result {
	case "0":
		return VerifyResult{Matched: true, Code: "0", Message: orDefault(resp.Response.Description, "姓名与身份证号一致")}, nil
	case "-1":
		return VerifyResult{Matched: false, Code: "-1", Message: orDefault(resp.Response.Description, "姓名与身份证号不一致")}, nil
	default:
		// -2..-7：输入非法/证件库异常/次数超限等，视为链路故障而非「不一致」
		return VerifyResult{}, fmt.Errorf("腾讯云核验异常（Result=%s）：%s", resp.Response.Result, resp.Response.Description)
	}
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
