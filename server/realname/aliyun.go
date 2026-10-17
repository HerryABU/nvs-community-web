package realname

// aliyun.go — 阿里云云市场实名认证商品（身份证二要素）。
//
// 云市场商品通过 API 网关暴露，支持两种认证：
//   - AppCode 简单认证（推荐，本实现采用）：请求头 Authorization: APPCODE <AppCode>
//   - AppKey/AppSecret 签名认证（X-Ca-Signature，HMAC-SHA1）
// 各商品网关地址/请求方法/参数名不一（如 GET /idcardcheck?name=&idcard= 或
// POST /realname/xxx {name, idcard_num}），故网关与方法、参数名均在后台可配。

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func init() { register(aliyunProvider{}) }

type aliyunProvider struct{}

func (aliyunProvider) Name() string { return "alibaba" }

func (aliyunProvider) Verify(ctx context.Context, cfg Config, name, idCard string) (VerifyResult, error) {
	c := cfg.Alibaba
	if c.AppCode == "" || c.Gateway == "" {
		return VerifyResult{}, ErrNotConfigured
	}
	nameParam := orDefault(c.NameParam, "name")
	cardParam := orDefault(c.CardParam, "idcard")
	method := strings.ToUpper(strings.TrimSpace(c.Method))
	if method == "" {
		method = http.MethodGet
	}

	var raw []byte
	var err error
	if method == http.MethodGet {
		u := c.Gateway + "?" + url.Values{nameParam: {name}, cardParam: {idCard}}.Encode()
		raw, err = getJSON(ctx, u, map[string]string{"Authorization": "APPCODE " + c.AppCode})
	} else {
		// POST：表单提交（云市场网关最通用形态）
		form := url.Values{nameParam: {name}, cardParam: {idCard}}.Encode()
		req, rerr := http.NewRequestWithContext(ctx, http.MethodPost, c.Gateway, strings.NewReader(form))
		if rerr != nil {
			return VerifyResult{}, rerr
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "APPCODE "+c.AppCode)
		resp, derr := newClient().Do(req)
		if derr != nil {
			return VerifyResult{}, derr
		}
		defer resp.Body.Close()
		raw, err = io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	}
	if err != nil {
		return VerifyResult{}, err
	}

	// 阿里云市场系响应：{"code":200,"data":{"result":1}} / {"status":"01"} 等，
	// 由 parseLooseResult 宽松解析（data 提升 + result/status/code 多形态兼容；
	// result=1 一致 / result=2 不一致，code=200 仅表示网关调用成功）。
	return parseLooseResult(raw)
}
