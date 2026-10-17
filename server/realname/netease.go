package realname

// netease.go — 网易易盾实名核验（实证认证：姓名+身份证号二要素）。
// 签名：所有请求参数（不含 signature）按参数名 ASCII 升序拼 key+value…+secretKey，
// 取 MD5（可配 SHA1/SHA256/SM3）十六进制摘要作为 signature。
// 请求：POST application/x-www-form-urlencoded；公共参数 businessId/secretId/
// timestamp(毫秒)/nonce/version=v1，业务参数 name/cardNo。
// 注意：实证认证接口地址以易盾控制台为准（默认给实人认证地址作占位，后台可改）。

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func init() { register(neteaseProvider{}) }

type neteaseProvider struct{}

func (neteaseProvider) Name() string { return "netease" }

func (neteaseProvider) Verify(ctx context.Context, cfg Config, name, idCard string) (VerifyResult, error) {
	c := cfg.Netease
	if c.BusinessID == "" || c.SecretID == "" || c.SecretKey == "" {
		return VerifyResult{}, ErrNotConfigured
	}
	apiURL := orDefault(c.URL, "https://verify.dun.163.com/v1/rp/check")
	algorithm := strings.ToUpper(strings.TrimSpace(c.Algorithm))
	if algorithm == "" {
		algorithm = "MD5"
	}

	params := map[string]string{
		"businessId": c.BusinessID,
		"secretId":   c.SecretID,
		"timestamp":  strconv.FormatInt(time.Now().UnixMilli(), 10),
		"nonce":      randomNonce(16),
		"version":    "v1",
		"name":       name,
		"cardNo":     idCard,
	}
	params["signature"] = dunSignature(algorithm, c.SecretKey, params)

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return VerifyResult{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := newClient().Do(req)
	if err != nil {
		return VerifyResult{}, err
	}
	defer resp.Body.Close()
	body, err := readLimited(resp)
	if err != nil {
		return VerifyResult{}, err
	}

	// 易盾响应：{"code":200,"msg":"ok","data":{"status":1}}（1=通过 2=不通过 0=待定）。
	// code!=200 时 parseLooseResult 的 code 分支会误判为一致，故先拦网关错误。
	var envelope struct {
		Code json.Number `json:"code"`
		Msg  string      `json:"msg"`
	}
	if err := jsonUnmarshalHelper(body, &envelope); err == nil {
		if f, cerr := envelope.Code.Float64(); cerr == nil && f != 200 {
			return VerifyResult{}, fmt.Errorf("网易易盾返回错误（code=%s）：%s", envelope.Code.String(), orDefault(envelope.Msg, "无描述"))
		}
	}
	return parseLooseResult(body)
}

// dunSignature 易盾签名：参数按 key 升序拼接 key+value…+secretKey 后取摘要
func dunSignature(algorithm, secretKey string, params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString(params[k])
	}
	sb.WriteString(secretKey)
	raw := []byte(sb.String())
	switch algorithm {
	case "SHA1":
		sum := sha1.Sum(raw)
		return hex.EncodeToString(sum[:])
	case "SHA256":
		sum := sha256.Sum256(raw)
		return hex.EncodeToString(sum[:])
	default: // MD5（易盾默认）
		sum := md5.Sum(raw)
		return hex.EncodeToString(sum[:])
	}
}

// randomNonce 生成 n 字节随机十六进制 nonce
func randomNonce(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}
