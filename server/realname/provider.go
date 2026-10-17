// Package realname 实名核验提供商抽象（F10 / F21.1 / 第三方接口支持）。
//
// 支持五类提供商：
//
//	national — 通用国家/授权数据服务商代理（天远/数脉/聚合数据等对接部委数据的服务商，
//	           「国家」直连接口本身不对公众开放，落地形态即此类授权网关）
//	tencent  — 腾讯云人脸核身 IdCardVerification（姓名+身份证二要素，TC3-HMAC-SHA256 签名）
//	alibaba  — 阿里云云市场实名认证商品（AppCode 简单认证，网关/方法/参数名可配）
//	huawei   — 华为云人证核身 IVS 证件版二要素（AK/SK，SDK-HMAC-SHA256 签名）
//	netease  — 网易易盾实名核验（businessId/secretId/secretKey，MD5 签名，URL 可配）
//
// 合规红线：姓名与身份证号仅随核验请求转发至所选提供商；本包不落库、不打印明文。
// 本包不 import models（避免循环），配置由调用方读好后传入。
package realname

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrNotConfigured 提供商未配置/未开启
var ErrNotConfigured = errors.New("实名核验提供商未配置或未开启")

// VerifyResult 统一核验结果
type VerifyResult struct {
	Matched bool   // 姓名与身份证号是否一致
	Code    string // 原始结果码（透传，便于管理员排查）
	Message string // 结果说明
}

// ============ 各提供商凭据配置（整体以 JSON 存 platform_configs.realname_provider） ============

// NationalCfg 通用代理（国家/授权数据服务商）
type NationalCfg struct {
	URL       string `json:"url"`        // 核验端点（POST JSON）
	Token     string `json:"token"`      // 接入令牌（随请求转发）
	NameField string `json:"name_field"` // 姓名字段名，缺省 real_name
	CardField string `json:"card_field"` // 身份证字段名，缺省 id_card
}

// TencentCfg 腾讯云人脸核身（faceid.tencentcloudapi.com）
type TencentCfg struct {
	SecretID  string `json:"secret_id"`
	SecretKey string `json:"secret_key"`
	Region    string `json:"region"` // 缺省 ap-guangzhou（该接口不校验 region）
}

// AlibabaCfg 阿里云云市场商品（AppCode 简单认证）
type AlibabaCfg struct {
	AppCode   string `json:"app_code"`   // Authorization: APPCODE <code>
	Gateway   string `json:"gateway"`    // 商品网关地址，如 https://xxx.market.alicloudapi.com/idcardcheck
	Method    string `json:"method"`     // GET（默认）/ POST
	NameParam string `json:"name_param"` // 姓名字段名，缺省 name
	CardParam string `json:"card_param"` // 身份证字段名，缺省 idcard
}

// HuaweiCfg 华为云人证核身 IVS 证件版（二要素）
type HuaweiCfg struct {
	AK       string `json:"ak"`
	SK       string `json:"sk"`
	Endpoint string `json:"endpoint"` // 缺省 ivs.cn-north-4.myhuaweicloud.com
	URI      string `json:"uri"`      // 缺省 /v2.0/ivs-idcard
}

// NeteaseCfg 网易易盾实名核验
type NeteaseCfg struct {
	BusinessID string `json:"business_id"`
	SecretID   string `json:"secret_id"`
	SecretKey  string `json:"secret_key"`
	URL        string `json:"url"`       // 缺省 https://verify.dun.163.com/v1/rp/check（实证认证地址以控制台为准）
	Algorithm  string `json:"algorithm"` // MD5（默认）/ SHA1 / SHA256 / SM3
}

// Config 实名核验总配置
type Config struct {
	Provider string      `json:"provider"` // national / tencent / alibaba / huawei / netease
	National NationalCfg `json:"national"`
	Tencent  TencentCfg  `json:"tencent"`
	Alibaba  AlibabaCfg  `json:"alibaba"`
	Huawei   HuaweiCfg   `json:"huawei"`
	Netease  NeteaseCfg  `json:"netease"`
}

// Provider 实名核验提供商实现
type Provider interface {
	Name() string
	Verify(ctx context.Context, cfg Config, name, idCard string) (VerifyResult, error)
}

var registry = map[string]Provider{}

// register 由各提供商的 init 注册
func register(p Provider) { registry[p.Name()] = p }

// Providers 返回已注册的提供商名列表（管理后台渲染用）
func Providers() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	return names
}

// Get 按名取提供商
func Get(name string) (Provider, bool) {
	p, ok := registry[name]
	return p, ok
}

// Verify 按 cfg.Provider 路由到对应提供商核验。
// 返回 error 表示链路/配置故障（调用方转「服务暂时不可用」）；
// Matched=false 且 error=nil 表示「核验不一致」的正常业务结果。
func Verify(ctx context.Context, cfg Config, name, idCard string) (VerifyResult, error) {
	if cfg.Provider == "" {
		return VerifyResult{}, ErrNotConfigured
	}
	p, ok := registry[cfg.Provider]
	if !ok {
		return VerifyResult{}, fmt.Errorf("未知的实名核验提供商: %s", cfg.Provider)
	}
	return p.Verify(ctx, cfg, name, idCard)
}

// ============ 公共 HTTP 辅助 ============

const verifyTimeout = 10 * time.Second

// httpClient 包级客户端（测试时可替换为 httptest 的 TLS 信任客户端）
var httpClient = &http.Client{Timeout: verifyTimeout}

func newClient() *http.Client { return httpClient }

// postJSON POST JSON 并读取限定长度的响应体
func postJSON(ctx context.Context, url string, headers map[string]string, payload interface{}) ([]byte, error) {
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(buf)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := newClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 64<<10))
}

// getJSON GET（query 由调用方拼入 URL）并读取限定长度的响应体
func getJSON(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := newClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 64<<10))
}

// ============ 宽松 JSON 解析辅助（兼容各厂商响应形态） ============

// asMap 尝试把任意 JSON 节点转为 map（含嵌套一层容器）
func asMap(v interface{}) map[string]interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		return t
	case []interface{}:
		if len(t) > 0 {
			return asMap(t[0])
		}
	}
	return nil
}

// numOf 读取节点为 float64（支持 json.Number / float64 / 数字字符串）
func numOf(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		var f float64
		if _, err := fmt.Sscanf(t, "%g", &f); err == nil {
			return f, true
		}
	}
	return 0, false
}

// firstString 按候选键顺序取首个非空字符串
func firstString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// firstNumber 按候选键顺序取首个数字
func firstNumber(m map[string]interface{}, keys ...string) (float64, bool) {
	for _, k := range keys {
		if f, ok := numOf(m[k]); ok {
			return f, true
		}
	}
	return 0, false
}
