// wechat.go — 微信开放平台网页扫码登录（F21.2）：
//   - 授权 URL 构造 / code 换 token / 拉取用户信息（统一 10s 超时 HTTP 客户端）
//   - state 防 CSRF：服务端生成、一次性消费、5 分钟 TTL；
//     Redis 就绪走 Redis（SETEX TTL），否则进程内内存 map（与 phonecode.go 同构）。
package utils

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"nvs-server/config"

	"github.com/redis/go-redis/v9"
)

const (
	// WeChatStateTTL state 有效期（5 分钟，与手机验证码一致）
	WeChatStateTTL = 5 * time.Minute
	// redisWeChatStatePrefix state 存储 key 前缀（Redis 与内存模式共用同一 key 空间）
	redisWeChatStatePrefix = "nvs:wxstate:"
	// wechatHTTPTimeout 微信 API 统一超时
	wechatHTTPTimeout = 10 * time.Second
	// wechatNicknameMaxLen 微信昵称清洗后限长（User.Nickname 为 varchar(64)，取 24 兼容中文昵称展示）
	wechatNicknameMaxLen = 24
	// wechatAvatarMaxLen 微信头像 URL 限长（User.AvatarURL 为 varchar(512)）
	wechatAvatarMaxLen = 512
)

// 微信开放平台端点。导出为包级变量：测试可覆盖为 httptest 本地地址 mock 微信 API，
// 生产环境保持官方域名不变。
var (
	WeChatQRConnectURL   = "https://open.weixin.qq.com/connect/qrconnect"
	WeChatAccessTokenURL = "https://api.weixin.qq.com/sns/oauth2/access_token"
	WeChatUserInfoURL    = "https://api.weixin.qq.com/sns/userinfo"
)

// state 用途：login=扫码登录/自动注册（默认）；bind=已登录用户绑定微信
const (
	WeChatPurposeLogin = "login"
	WeChatPurposeBind  = "bind"
)

// ErrWeChatStateInvalid state 不存在 / 已过期 / 已消费
var ErrWeChatStateInvalid = errors.New("state 无效或已过期")

// wechatHTTPClient 统一超时的微信 API 客户端
var wechatHTTPClient = &http.Client{Timeout: wechatHTTPTimeout}

// ---------------- 授权 URL 与 token / 用户信息 ----------------

// WeChatAuthURL 构造 PC 网站扫码授权 URL（redirect_uri 经 QueryEscape 编码，state 防 CSRF）。
// appid/secret/callback 任一未配置返回中文错误。
func WeChatAuthURL(state string) (string, error) {
	if err := checkWeChatConfig(); err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("appid", config.WeChatAppID)
	q.Set("redirect_uri", config.WeChatCallback)
	q.Set("response_type", "code")
	q.Set("scope", "snsapi_login")
	q.Set("state", state)
	return WeChatQRConnectURL + "?" + q.Encode() + "#wechat_redirect", nil
}

// WeChatExchangeCode 用授权 code 换取 openid / unionid / access_token。
func WeChatExchangeCode(code string) (openID, unionID, accessToken string, err error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", "", "", errors.New("授权 code 不能为空")
	}
	if err := checkWeChatConfig(); err != nil {
		return "", "", "", err
	}
	q := url.Values{}
	q.Set("appid", config.WeChatAppID)
	q.Set("secret", config.WeChatAppSecret)
	q.Set("code", code)
	q.Set("grant_type", "authorization_code")

	var body struct {
		OpenID      string `json:"openid"`
		UnionID     string `json:"unionid"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := wechatGetJSON(WeChatAccessTokenURL, q, &body); err != nil {
		return "", "", "", err
	}
	if body.ErrCode != 0 {
		return "", "", "", fmt.Errorf("微信授权失败(errcode=%d): %s", body.ErrCode, wechatErrMsg(body.ErrMsg))
	}
	if body.OpenID == "" || body.AccessToken == "" {
		return "", "", "", errors.New("微信授权响应缺少 openid/access_token")
	}
	return body.OpenID, body.UnionID, body.AccessToken, nil
}

// WeChatUserInfo 拉取微信用户信息（昵称经清洗，头像 URL 限长）。
func WeChatUserInfo(accessToken, openID string) (nickname, headImg string, err error) {
	accessToken = strings.TrimSpace(accessToken)
	openID = strings.TrimSpace(openID)
	if accessToken == "" || openID == "" {
		return "", "", errors.New("access_token/openid 不能为空")
	}
	q := url.Values{}
	q.Set("access_token", accessToken)
	q.Set("openid", openID)
	q.Set("lang", "zh_CN")

	var body struct {
		Nickname   string `json:"nickname"`
		HeadImgURL string `json:"headimgurl"`
		ErrCode    int    `json:"errcode"`
		ErrMsg     string `json:"errmsg"`
	}
	if err := wechatGetJSON(WeChatUserInfoURL, q, &body); err != nil {
		return "", "", err
	}
	if body.ErrCode != 0 {
		return "", "", fmt.Errorf("获取微信用户信息失败(errcode=%d): %s", body.ErrCode, wechatErrMsg(body.ErrMsg))
	}
	return CleanWeChatNickname(body.Nickname), ClampString(strings.TrimSpace(body.HeadImgURL), wechatAvatarMaxLen), nil
}

// wechatGetJSON GET 微信 API 并解析 JSON（统一超时 + 响应体限长 1MB + 错误中文化）
func wechatGetJSON(endpoint string, query url.Values, out interface{}) error {
	resp, err := wechatHTTPClient.Get(endpoint + "?" + query.Encode())
	if err != nil {
		return fmt.Errorf("请求微信接口失败: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("读取微信接口响应失败: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("微信接口返回 HTTP %d", resp.StatusCode)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("解析微信接口响应失败: %v", err)
	}
	return nil
}

func wechatErrMsg(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return "未知错误"
	}
	return msg
}

// checkWeChatConfig 校验微信登录所需配置（appid/secret/callback）
func checkWeChatConfig() error {
	if config.WeChatAppID == "" || config.WeChatAppSecret == "" {
		return errors.New("未配置微信开放平台 AppID/AppSecret（WECHAT_APP_ID/WECHAT_APP_SECRET）")
	}
	if config.WeChatCallback == "" {
		return errors.New("未配置微信回调地址（WECHAT_CALLBACK）")
	}
	return nil
}

// CleanWeChatNickname 清洗微信昵称：去首尾空白 + 限长；为空时回退默认值
func CleanWeChatNickname(nickname string) string {
	nick := ClampString(strings.TrimSpace(nickname), wechatNicknameMaxLen)
	if nick == "" {
		return "微信用户"
	}
	return nick
}

// ---------------- state 防 CSRF 存储（Redis / 内存双后端） ----------------

// weChatStateRecord state 记录（Redis JSON / 内存结构共用）
type weChatStateRecord struct {
	Purpose   string    `json:"purpose"`    // login / bind
	ExpiresAt time.Time `json:"expires_at"` // 过期时间
}

// 内存模式存储（SQLite 单机）
var (
	weChatStateStore   = make(map[string]*weChatStateRecord)
	weChatStateStoreMu sync.RWMutex
)

func useRedisForWeChat() bool { return RedisClient != nil }

func weChatStateKey(state string) string { return redisWeChatStatePrefix + state }

// GenerateWeChatState 生成随机 state（16 字节 → 32 位 hex）
func GenerateWeChatState() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成 state 失败: %v", err)
	}
	return hex.EncodeToString(buf), nil
}

// SaveWeChatState 存储 state 及其用途（purpose 非 bind 一律按 login 处理），TTL 5 分钟
func SaveWeChatState(state, purpose string) error {
	state = strings.TrimSpace(state)
	if state == "" {
		return errors.New("state 不能为空")
	}
	if purpose != WeChatPurposeBind {
		purpose = WeChatPurposeLogin
	}
	rec := &weChatStateRecord{Purpose: purpose, ExpiresAt: time.Now().Add(WeChatStateTTL)}
	if useRedisForWeChat() {
		b, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		return RedisClient.Set(redisCtx, weChatStateKey(state), b, WeChatStateTTL).Err()
	}
	weChatStateStoreMu.Lock()
	weChatStateStore[weChatStateKey(state)] = rec
	weChatStateStoreMu.Unlock()
	return nil
}

// ConsumeWeChatState 一次性消费 state，返回其用途；
// 不存在 / 已过期 / 已消费均返回 ErrWeChatStateInvalid（防 CSRF 重放）。
func ConsumeWeChatState(state string) (string, error) {
	state = strings.TrimSpace(state)
	if state == "" {
		return "", ErrWeChatStateInvalid
	}
	rec, err := loadWeChatState(state)
	if err != nil || rec == nil {
		return "", ErrWeChatStateInvalid
	}
	deleteWeChatState(state)
	return rec.Purpose, nil
}

func loadWeChatState(state string) (*weChatStateRecord, error) {
	if useRedisForWeChat() {
		raw, err := RedisClient.Get(redisCtx, weChatStateKey(state)).Result()
		if err == redis.Nil {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		var rec weChatStateRecord
		if err := json.Unmarshal([]byte(raw), &rec); err != nil {
			return nil, err
		}
		return &rec, nil
	}
	weChatStateStoreMu.RLock()
	rec := weChatStateStore[weChatStateKey(state)]
	weChatStateStoreMu.RUnlock()
	if rec != nil && time.Now().After(rec.ExpiresAt) {
		// 惰性清理过期记录
		deleteWeChatState(state)
		return nil, nil
	}
	return rec, nil
}

func deleteWeChatState(state string) {
	if useRedisForWeChat() {
		_ = RedisClient.Del(redisCtx, weChatStateKey(state)).Err()
		return
	}
	weChatStateStoreMu.Lock()
	delete(weChatStateStore, weChatStateKey(state))
	weChatStateStoreMu.Unlock()
}

// CleanupWeChatStates 清理过期的内存 state（定时任务调用；Redis 模式由 TTL 自动过期）
func CleanupWeChatStates() {
	if useRedisForWeChat() {
		return
	}
	now := time.Now()
	weChatStateStoreMu.Lock()
	for k, rec := range weChatStateStore {
		if now.After(rec.ExpiresAt) {
			delete(weChatStateStore, k)
		}
	}
	weChatStateStoreMu.Unlock()
}

// WeChatStateBackend 当前 state 存储后端（"redis" / "memory"，供健康检查展示）
func WeChatStateBackend() string {
	if useRedisForWeChat() {
		return "redis"
	}
	return "memory"
}
