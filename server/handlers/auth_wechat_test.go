// auth_wechat_test.go — 微信扫码登录（F21.2）测试：
//   - state 存取与一次性消费（utils 双后端存储）
//   - 授权 URL 构造（含 QueryEscape 编码与未配置错误分支）
//   - httptest mock 微信 API：ExchangeCode / UserInfo 的正常与错误分支
//   - models 层 GetUserByWeChatOpenID / BindWeChat / WeChatOpenIDTaken（独立命名内存 SQLite 库）
//   - WeChatCallback 集成测试：自动注册 + 登录态 Cookie 下发 + 302 重定向（含绑定用途与各类失败分支）
package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"nvs-server/config"
	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupWeChatTestDB 独立命名内存 SQLite 库（file:...?mode=memory&cache=shared 按名隔离，
// 避免与其他测试的共享缓存库串数据），并临时替换 models.DB。
func setupWeChatTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:wechat_handler_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移测试库失败: %v", err)
	}
	old := models.DB
	models.DB = db
	t.Cleanup(func() {
		models.DB = old
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// withWeChatConfig 临时设置微信配置（测试结束后恢复）
func withWeChatConfig(t *testing.T, appID, secret, callback string) {
	t.Helper()
	oldID, oldSecret, oldCB := config.WeChatAppID, config.WeChatAppSecret, config.WeChatCallback
	t.Cleanup(func() {
		config.WeChatAppID, config.WeChatAppSecret, config.WeChatCallback = oldID, oldSecret, oldCB
	})
	config.WeChatAppID, config.WeChatAppSecret, config.WeChatCallback = appID, secret, callback
}

// mockWeChatAPI 覆盖微信 API 端点为 httptest 服务（测试结束后恢复）
func mockWeChatAPI(t *testing.T, tokenBody, infoBody string) {
	t.Helper()
	oldTok, oldInfo := utils.WeChatAccessTokenURL, utils.WeChatUserInfoURL
	t.Cleanup(func() {
		utils.WeChatAccessTokenURL, utils.WeChatUserInfoURL = oldTok, oldInfo
	})
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(tokenBody))
	}))
	infoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(infoBody))
	}))
	t.Cleanup(tokenSrv.Close)
	t.Cleanup(infoSrv.Close)
	utils.WeChatAccessTokenURL = tokenSrv.URL
	utils.WeChatUserInfoURL = infoSrv.URL
}

// ---------------- state 存取与一次性消费 ----------------

func TestWeChatStateStoreLifecycle(t *testing.T) {
	state, err := utils.GenerateWeChatState()
	if err != nil {
		t.Fatalf("生成 state 失败: %v", err)
	}
	if len(state) != 32 {
		t.Errorf("state 应为 32 位 hex，实际 %d", len(state))
	}
	if err := utils.SaveWeChatState(state, utils.WeChatPurposeBind); err != nil {
		t.Fatalf("保存 state 失败: %v", err)
	}
	purpose, err := utils.ConsumeWeChatState(state)
	if err != nil {
		t.Fatalf("消费 state 失败: %v", err)
	}
	if purpose != utils.WeChatPurposeBind {
		t.Errorf("state 用途应为 bind，实际 %q", purpose)
	}
	// 一次性：二次消费必须失败（防 CSRF 重放）
	if _, err := utils.ConsumeWeChatState(state); !errors.Is(err, utils.ErrWeChatStateInvalid) {
		t.Errorf("state 应一次性消费，二次消费错误: %v", err)
	}
	// 伪造 / 空 state 必须失败
	if _, err := utils.ConsumeWeChatState("forged-state-xyz"); !errors.Is(err, utils.ErrWeChatStateInvalid) {
		t.Errorf("伪造 state 应被拒绝，实际: %v", err)
	}
	if _, err := utils.ConsumeWeChatState(""); !errors.Is(err, utils.ErrWeChatStateInvalid) {
		t.Errorf("空 state 应被拒绝，实际: %v", err)
	}
	// 未指定用途时按 login 处理
	state2, _ := utils.GenerateWeChatState()
	if err := utils.SaveWeChatState(state2, ""); err != nil {
		t.Fatalf("保存 state 失败: %v", err)
	}
	if p, err := utils.ConsumeWeChatState(state2); err != nil || p != utils.WeChatPurposeLogin {
		t.Errorf("缺省用途应为 login，实际 %q (%v)", p, err)
	}
}

// ---------------- 授权 URL 构造 ----------------

func TestWeChatAuthURLConstruction(t *testing.T) {
	withWeChatConfig(t, "", "", "")
	if _, err := utils.WeChatAuthURL("st"); err == nil {
		t.Error("未配置 appid/secret 应返回中文错误")
	}

	withWeChatConfig(t, "wx1234567890abcdef", "secret123", "https://example.com/go-nvs/api/auth/wechat/callback")
	u, err := utils.WeChatAuthURL("abc123")
	if err != nil {
		t.Fatalf("构造授权 URL 失败: %v", err)
	}
	if !strings.HasPrefix(u, "https://open.weixin.qq.com/connect/qrconnect?") {
		t.Errorf("授权地址前缀错误: %s", u)
	}
	if !strings.HasSuffix(u, "#wechat_redirect") {
		t.Errorf("授权地址应以 #wechat_redirect 结尾: %s", u)
	}
	// redirect_uri 必须经 QueryEscape 编码（:// 与 / 都被转义）
	if !strings.Contains(u, "redirect_uri=https%3A%2F%2Fexample.com%2Fgo-nvs%2Fapi%2Fauth%2Fwechat%2Fcallback") {
		t.Errorf("redirect_uri 应被 QueryEscape 编码: %s", u)
	}
	parsed, perr := url.Parse(u)
	if perr != nil {
		t.Fatalf("解析授权 URL 失败: %v", perr)
	}
	q := parsed.Query()
	if q.Get("appid") != "wx1234567890abcdef" {
		t.Errorf("appid 错误: %q", q.Get("appid"))
	}
	if q.Get("redirect_uri") != "https://example.com/go-nvs/api/auth/wechat/callback" {
		t.Errorf("redirect_uri 解码错误: %q", q.Get("redirect_uri"))
	}
	if q.Get("response_type") != "code" || q.Get("scope") != "snsapi_login" || q.Get("state") != "abc123" {
		t.Errorf("授权参数错误: %v", q)
	}
}

func TestWeChatAuthURLMissingCallback(t *testing.T) {
	withWeChatConfig(t, "wxappid", "wxsecret", "")
	if _, err := utils.WeChatAuthURL("st"); err == nil {
		t.Error("缺少 WECHAT_CALLBACK 应返回中文错误")
	}
}

// ---------------- ExchangeCode / UserInfo（httptest mock） ----------------

func TestWeChatExchangeCodeMock(t *testing.T) {
	withWeChatConfig(t, "wxappid", "wxsecret", "https://example.com/api/auth/wechat/callback")
	// errcode 非 0 与正常响应分流
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("code") == "bad-code" {
			_, _ = w.Write([]byte(`{"errcode":40029,"errmsg":"invalid code"}`))
			return
		}
		if r.URL.Query().Get("code") == "no-openid" {
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":7200}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"AT-TOKEN","expires_in":7200,"refresh_token":"RT","openid":"oOpenID123","unionid":"oUnion123","scope":"snsapi_login"}`))
	}))
	defer srv.Close()

	oldTok := utils.WeChatAccessTokenURL
	defer func() { utils.WeChatAccessTokenURL = oldTok }()
	utils.WeChatAccessTokenURL = srv.URL

	// 正常分支
	openID, unionID, token, err := utils.WeChatExchangeCode("good-code")
	if err != nil {
		t.Fatalf("正常换 token 应成功: %v", err)
	}
	if openID != "oOpenID123" || unionID != "oUnion123" || token != "AT-TOKEN" {
		t.Errorf("换 token 结果错误: openid=%q unionid=%q token=%q", openID, unionID, token)
	}

	// 错误分支：errcode 非 0 → 中文错误且包含 errcode
	_, _, _, err = utils.WeChatExchangeCode("bad-code")
	if err == nil {
		t.Fatal("errcode 非 0 应返回错误")
	}
	if !strings.Contains(err.Error(), "40029") || !strings.Contains(err.Error(), "invalid code") {
		t.Errorf("错误信息应包含 errcode/errmsg: %v", err)
	}

	// 错误分支：响应缺少 openid
	if _, _, _, err := utils.WeChatExchangeCode("no-openid"); err == nil {
		t.Error("缺少 openid 的响应应返回错误")
	}

	// 错误分支：空 code
	if _, _, _, err := utils.WeChatExchangeCode("  "); err == nil {
		t.Error("空 code 应返回错误")
	}
}

func TestWeChatExchangeCodeHTTPError(t *testing.T) {
	withWeChatConfig(t, "wxappid", "wxsecret", "https://example.com/api/auth/wechat/callback")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	oldTok := utils.WeChatAccessTokenURL
	defer func() { utils.WeChatAccessTokenURL = oldTok }()
	utils.WeChatAccessTokenURL = srv.URL

	if _, _, _, err := utils.WeChatExchangeCode("good-code"); err == nil {
		t.Error("HTTP 500 应返回错误")
	}
}

func TestWeChatUserInfoMock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("openid") {
		case "err-openid":
			_, _ = w.Write([]byte(`{"errcode":40003,"errmsg":"invalid openid"}`))
		default:
			_, _ = w.Write([]byte(`{"openid":"oX","nickname":"  测试昵称  ","sex":1,"headimgurl":"https://thirdwx.qlogo.cn/mmopen/abc"}`))
		}
	}))
	defer srv.Close()
	oldInfo := utils.WeChatUserInfoURL
	defer func() { utils.WeChatUserInfoURL = oldInfo }()
	utils.WeChatUserInfoURL = srv.URL

	// 正常分支：昵称去首尾空白
	nick, head, err := utils.WeChatUserInfo("AT", "ok-openid")
	if err != nil {
		t.Fatalf("拉取用户信息应成功: %v", err)
	}
	if nick != "测试昵称" {
		t.Errorf("昵称应去首尾空白: %q", nick)
	}
	if head != "https://thirdwx.qlogo.cn/mmopen/abc" {
		t.Errorf("头像 URL 错误: %q", head)
	}

	// 错误分支：errcode 非 0
	if _, _, err := utils.WeChatUserInfo("AT", "err-openid"); err == nil {
		t.Error("errcode 非 0 应返回错误")
	}

	// 错误分支：空参数
	if _, _, err := utils.WeChatUserInfo("", ""); err == nil {
		t.Error("空 access_token/openid 应返回错误")
	}
}

func TestCleanWeChatNickname(t *testing.T) {
	if got := utils.CleanWeChatNickname("  空白昵称  "); got != "空白昵称" {
		t.Errorf("昵称应去首尾空白: %q", got)
	}
	if got := utils.CleanWeChatNickname("   "); got != "微信用户" {
		t.Errorf("空昵称应回退默认值: %q", got)
	}
	long := strings.Repeat("昵", 40)
	if got := utils.CleanWeChatNickname(long); len([]rune(got)) != 24 {
		t.Errorf("昵称应限长 24 字符: %d", len([]rune(got)))
	}
}

// ---------------- models 层查询/绑定 ----------------

func TestWeChatModelQueries(t *testing.T) {
	setupWeChatTestDB(t)

	u1 := &models.User{Username: "wxuser1", Email: "u1@test.local", PasswordHash: "x", WeChatOpenID: "open_1", WeChatUnionID: "union_1"}
	u2 := &models.User{Username: "wxuser2", Email: "u2@test.local", PasswordHash: "x"}
	if err := models.CreateUser(u1); err != nil {
		t.Fatalf("创建用户1失败: %v", err)
	}
	if err := models.CreateUser(u2); err != nil {
		t.Fatalf("创建用户2失败: %v", err)
	}

	// openid 查找（命中 / 未命中）
	got, err := models.GetUserByWeChatOpenID("open_1")
	if err != nil || got.ID != u1.ID {
		t.Fatalf("按 openid 查找失败: %v", err)
	}
	if _, err := models.GetUserByWeChatOpenID("missing"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("未命中 openid 应返回 ErrRecordNotFound，实际: %v", err)
	}

	// unionid 查找（命中 / 空值直接未命中）
	got, err = models.GetUserByWeChatUnionID("union_1")
	if err != nil || got.ID != u1.ID {
		t.Fatalf("按 unionid 查找失败: %v", err)
	}
	if _, err := models.GetUserByWeChatUnionID(""); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("空 unionid 应返回 ErrRecordNotFound，实际: %v", err)
	}

	// 占用检查
	if !models.WeChatOpenIDTaken("open_1") {
		t.Error("open_1 应被占用")
	}
	if models.WeChatOpenIDTaken("open_2") {
		t.Error("open_2 不应被占用")
	}

	// 绑定到 u2 后立即可查
	if err := models.BindWeChat(u2.ID, "open_2", "union_2"); err != nil {
		t.Fatalf("绑定失败: %v", err)
	}
	if !models.WeChatOpenIDTaken("open_2") {
		t.Error("绑定后 open_2 应被占用")
	}
	got2, err := models.GetUserByWeChatOpenID("open_2")
	if err != nil || got2.WeChatUnionID != "union_2" {
		t.Errorf("绑定结果错误: %+v (%v)", got2, err)
	}

	// 解绑后释放占用
	if err := models.UnbindWeChat(u2.ID); err != nil {
		t.Fatalf("解绑失败: %v", err)
	}
	if models.WeChatOpenIDTaken("open_2") {
		t.Error("解绑后 open_2 不应被占用")
	}

	// 非法参数
	if err := models.BindWeChat(0, "open_x", ""); err == nil {
		t.Error("userID=0 应返回错误")
	}
	if err := models.UnbindWeChat(0); err == nil {
		t.Error("userID=0 应返回错误")
	}
}

// ---------------- WeChatCallback 集成测试 ----------------

// newCallbackContext 构造回调请求的 gin 测试上下文
func newCallbackContext(code, state string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/auth/wechat/callback?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(state), nil)
	return c, w
}

func TestWeChatCallbackAutoRegisterLogin(t *testing.T) {
	setupWeChatTestDB(t)
	withWeChatConfig(t, "wxappid", "wxsecret", "https://example.com/api/auth/wechat/callback")
	mockWeChatAPI(t,
		`{"access_token":"AT","expires_in":7200,"openid":"oCALLBACK","unionid":"uCALLBACK"}`,
		`{"openid":"oCALLBACK","nickname":"回调用户","headimgurl":"https://thirdwx.qlogo.cn/mmopen/avatar"}`)

	state, err := utils.GenerateWeChatState()
	if err != nil {
		t.Fatal(err)
	}
	if err := utils.SaveWeChatState(state, utils.WeChatPurposeLogin); err != nil {
		t.Fatal(err)
	}

	c, w := newCallbackContext("good-code", state)
	WeChatCallback(c)

	if w.Code != http.StatusFound {
		t.Fatalf("应 302 重定向，实际 %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login?wechat=bound" {
		t.Errorf("重定向目标错误: %q", loc)
	}

	// 自动注册成功：openid 绑定 + 昵称/头像来自微信 + 占位邮箱
	u, err := models.GetUserByWeChatOpenID("oCALLBACK")
	if err != nil {
		t.Fatalf("应自动注册并绑定 openid: %v", err)
	}
	if u.Nickname != "回调用户" {
		t.Errorf("昵称应取微信昵称: %q", u.Nickname)
	}
	if u.AvatarURL != "https://thirdwx.qlogo.cn/mmopen/avatar" {
		t.Errorf("头像应取微信头像: %q", u.AvatarURL)
	}
	if u.Email != "oCALLBACK@wechat.nvs.local" {
		t.Errorf("占位邮箱错误: %q", u.Email)
	}
	if !strings.HasPrefix(u.Username, "wx_") {
		t.Errorf("用户名应以 wx_ 开头: %q", u.Username)
	}
	if u.WeChatUnionID != "uCALLBACK" {
		t.Errorf("unionid 应写入: %q", u.WeChatUnionID)
	}

	// 登录态 Cookie 下发（与邮箱/手机登录一致：token + refresh_token）
	var hasToken, hasRefresh bool
	for _, ck := range w.Result().Cookies() {
		switch ck.Name {
		case "token":
			hasToken = ck.Value != ""
		case "refresh_token":
			hasRefresh = ck.Value != ""
		}
	}
	if !hasToken || !hasRefresh {
		t.Errorf("应下发 token(%v) 与 refresh_token(%v) Cookie", hasToken, hasRefresh)
	}
}

func TestWeChatCallbackFailureRedirects(t *testing.T) {
	setupWeChatTestDB(t)
	withWeChatConfig(t, "wxappid", "wxsecret", "https://example.com/api/auth/wechat/callback")
	mockWeChatAPI(t,
		`{"access_token":"AT","expires_in":7200,"openid":"oFAIL","unionid":""}`,
		`{"openid":"oFAIL","nickname":"失败用户"}`)

	cases := []struct {
		name       string
		code       string
		state      string
		wantReason string
	}{
		{"取消授权（无 code）", "", "whatever", "cancel"},
		{"state 无效", "good-code", "forged-state", "state"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := newCallbackContext(tc.code, tc.state)
			WeChatCallback(c)
			if w.Code != http.StatusFound {
				t.Fatalf("应 302 重定向，实际 %d", w.Code)
			}
			want := "/login?wechat=failed&reason=" + tc.wantReason
			if loc := w.Header().Get("Location"); loc != want {
				t.Errorf("重定向目标错误: %q，期望 %q", loc, want)
			}
		})
	}

	// state 一次性：同一 state 二次回调应判 state 失败
	state, _ := utils.GenerateWeChatState()
	if err := utils.SaveWeChatState(state, utils.WeChatPurposeLogin); err != nil {
		t.Fatal(err)
	}
	c, w := newCallbackContext("good-code", state)
	WeChatCallback(c)
	if loc := w.Header().Get("Location"); loc != "/login?wechat=bound" {
		t.Fatalf("首次回调应成功: %q", loc)
	}
	c2, w2 := newCallbackContext("good-code", state)
	WeChatCallback(c2)
	if loc := w2.Header().Get("Location"); loc != "/login?wechat=failed&reason=state" {
		t.Errorf("二次回调（state 已消费）应失败: %q", loc)
	}
}

func TestWeChatCallbackBindPurpose(t *testing.T) {
	db := setupWeChatTestDB(t)
	withWeChatConfig(t, "wxappid", "wxsecret", "https://example.com/api/auth/wechat/callback")
	mockWeChatAPI(t,
		`{"access_token":"AT","expires_in":7200,"openid":"oBIND","unionid":"uBIND"}`,
		`{"openid":"oBIND","nickname":"绑定用户"}`)

	// 已登录用户（上下文携带 userID，模拟 public 组 OptionalAuth 解析结果）
	current := &models.User{Username: "current", Email: "current@test.local", PasswordHash: "x"}
	if err := models.CreateUser(current); err != nil {
		t.Fatal(err)
	}

	state, _ := utils.GenerateWeChatState()
	if err := utils.SaveWeChatState(state, utils.WeChatPurposeBind); err != nil {
		t.Fatal(err)
	}
	c, w := newCallbackContext("good-code", state)
	c.Set(middleware.CtxUserID, current.ID)
	WeChatCallback(c)

	if w.Code != http.StatusFound {
		t.Fatalf("应 302 重定向，实际 %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/user/wechat-bind?wechat=bound" {
		t.Errorf("绑定用途应重定向回绑定页: %q", loc)
	}
	bound, err := models.GetUserByWeChatOpenID("oBIND")
	if err != nil || bound.ID != current.ID {
		t.Fatalf("微信应绑定到当前登录账号: %+v (%v)", bound, err)
	}
	_ = db
}

func TestWeChatCallbackBindTakenByOther(t *testing.T) {
	setupWeChatTestDB(t)
	withWeChatConfig(t, "wxappid", "wxsecret", "https://example.com/api/auth/wechat/callback")
	mockWeChatAPI(t,
		`{"access_token":"AT","expires_in":7200,"openid":"oTAKEN","unionid":""}`,
		`{"openid":"oTAKEN","nickname":"占用用户"}`)

	// 其他账号已占用该 openid
	other := &models.User{Username: "other", Email: "other@test.local", PasswordHash: "x", WeChatOpenID: "oTAKEN"}
	if err := models.CreateUser(other); err != nil {
		t.Fatal(err)
	}
	current := &models.User{Username: "current2", Email: "current2@test.local", PasswordHash: "x"}
	if err := models.CreateUser(current); err != nil {
		t.Fatal(err)
	}

	state, _ := utils.GenerateWeChatState()
	if err := utils.SaveWeChatState(state, utils.WeChatPurposeBind); err != nil {
		t.Fatal(err)
	}
	c, w := newCallbackContext("good-code", state)
	c.Set(middleware.CtxUserID, current.ID)
	WeChatCallback(c)

	if w.Code != http.StatusFound {
		t.Fatalf("应 302 重定向，实际 %d", w.Code)
	}
	want := "/user/wechat-bind?wechat=failed&reason=taken"
	if loc := w.Header().Get("Location"); loc != want {
		t.Errorf("openid 被他人占用时应重定向 taken: %q", loc)
	}
	// 当前账号不应被写入
	reload, _ := models.GetUserByID(current.ID)
	if reload.WeChatOpenID != "" {
		t.Error("占用冲突时不应写入当前账号")
	}
}

func TestWeChatCallbackLoginRedirectWithPrefix(t *testing.T) {
	setupWeChatTestDB(t)
	withWeChatConfig(t, "wxappid", "wxsecret", "https://example.com/go-nvs/api/auth/wechat/callback")
	// 站点路径前缀（反向代理挂载场景）
	oldPrefix := config.URLPrefix
	defer func() { config.URLPrefix = oldPrefix }()
	config.URLPrefix = "/go-nvs"

	mockWeChatAPI(t,
		`{"access_token":"AT","expires_in":7200,"openid":"oPREFIX","unionid":""}`,
		`{"openid":"oPREFIX","nickname":"前缀用户"}`)

	state, _ := utils.GenerateWeChatState()
	if err := utils.SaveWeChatState(state, utils.WeChatPurposeLogin); err != nil {
		t.Fatal(err)
	}
	c, w := newCallbackContext("good-code", state)
	WeChatCallback(c)

	if w.Code != http.StatusFound {
		t.Fatalf("应 302 重定向，实际 %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/go-nvs/login?wechat=bound" {
		t.Errorf("重定向应带站点前缀: %q", loc)
	}
}

// ---------------- WeChatAuthStart / BindStatus / Bind / Unbind ----------------

func TestWeChatAuthStart(t *testing.T) {
	setupWeChatTestDB(t)
	withWeChatConfig(t, "wxappid", "wxsecret", "https://example.com/api/auth/wechat/callback")
	// 开启微信登录方式
	if err := models.SetLoginMethods(models.LoginMethods{Email: true, Phone: true, WeChat: true}); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/auth/wechat/url?purpose=login", nil)
	WeChatAuthStart(c)

	if w.Code != http.StatusOK {
		t.Fatalf("应 200，实际 %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "open.weixin.qq.com/connect/qrconnect") {
		t.Errorf("响应应包含授权 URL: %s", body)
	}
	if !strings.Contains(body, `"state"`) {
		t.Errorf("响应应包含 state: %s", body)
	}

	// 关闭微信登录方式 → 403
	if err := models.SetLoginMethods(models.LoginMethods{Email: true, Phone: true, WeChat: false}); err != nil {
		t.Fatal(err)
	}
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/auth/wechat/url", nil)
	WeChatAuthStart(c2)
	if w2.Code != http.StatusForbidden {
		t.Errorf("关闭微信登录应 403，实际 %d", w2.Code)
	}

	// 绑定用途不受登录方式开关约束（仅要求配置齐全）
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodGet, "/api/auth/wechat/url?purpose=bind", nil)
	WeChatAuthStart(c3)
	if w3.Code != http.StatusOK {
		t.Errorf("绑定用途应放行，实际 %d", w3.Code)
	}
}

func TestWeChatAuthStartUnconfigured(t *testing.T) {
	withWeChatConfig(t, "", "", "")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/auth/wechat/url?purpose=bind", nil)
	WeChatAuthStart(c)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("未配置微信参数应 500，实际 %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "WECHAT_APP_ID") {
		t.Errorf("错误信息应提示缺失的配置项: %s", w.Body.String())
	}
}

func TestWeChatBindStatusAndUnbind(t *testing.T) {
	setupWeChatTestDB(t)
	u := &models.User{Username: "binduser", Email: "bind@test.local", PasswordHash: "x", WeChatOpenID: "oBOUND", WeChatUnionID: "uBOUND"}
	if err := models.CreateUser(u); err != nil {
		t.Fatal(err)
	}

	// 未登录 → 401
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/auth/wechat/bind-status", nil)
	WeChatBindStatus(c)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("未登录应 401，实际 %d", w.Code)
	}

	// 已绑定 → bound=true
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/auth/wechat/bind-status", nil)
	c2.Set(middleware.CtxUserID, u.ID)
	WeChatBindStatus(c2)
	if w2.Code != http.StatusOK || !strings.Contains(w2.Body.String(), `"bound":true`) {
		t.Errorf("已绑定状态错误: %d %s", w2.Code, w2.Body.String())
	}

	// 解绑
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodPost, "/api/auth/wechat/unbind", nil)
	c3.Set(middleware.CtxUserID, u.ID)
	WeChatUnbind(c3)
	if w3.Code != http.StatusOK {
		t.Fatalf("解绑应 200，实际 %d", w3.Code)
	}
	if models.WeChatOpenIDTaken("oBOUND") {
		t.Error("解绑后 openid 应释放")
	}

	// 重复解绑 → 400
	w4 := httptest.NewRecorder()
	c4, _ := gin.CreateTestContext(w4)
	c4.Request = httptest.NewRequest(http.MethodPost, "/api/auth/wechat/unbind", nil)
	c4.Set(middleware.CtxUserID, u.ID)
	WeChatUnbind(c4)
	if w4.Code != http.StatusBadRequest {
		t.Errorf("重复解绑应 400，实际 %d", w4.Code)
	}
}

func TestWeChatBind(t *testing.T) {
	setupWeChatTestDB(t)
	withWeChatConfig(t, "wxappid", "wxsecret", "https://example.com/api/auth/wechat/callback")
	mockWeChatAPI(t,
		`{"access_token":"AT","expires_in":7200,"openid":"oBINDAPI","unionid":"uBINDAPI"}`,
		`{"openid":"oBINDAPI","nickname":"接口绑定用户"}`)

	u := &models.User{Username: "bindapi", Email: "bindapi@test.local", PasswordHash: "x"}
	if err := models.CreateUser(u); err != nil {
		t.Fatal(err)
	}

	// state 过期 → 400
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/auth/wechat/bind", strings.NewReader(`{"code":"good-code","state":"forged"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(middleware.CtxUserID, u.ID)
	WeChatBind(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("无效 state 应 400，实际 %d", w.Code)
	}

	// 正常绑定
	state, _ := utils.GenerateWeChatState()
	if err := utils.SaveWeChatState(state, utils.WeChatPurposeBind); err != nil {
		t.Fatal(err)
	}
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/api/auth/wechat/bind", strings.NewReader(`{"code":"good-code","state":"`+state+`"}`))
	c2.Request.Header.Set("Content-Type", "application/json")
	c2.Set(middleware.CtxUserID, u.ID)
	WeChatBind(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("绑定应 200，实际 %d: %s", w2.Code, w2.Body.String())
	}
	bound, err := models.GetUserByWeChatOpenID("oBINDAPI")
	if err != nil || bound.ID != u.ID {
		t.Fatalf("openid 应绑定到当前用户: %+v (%v)", bound, err)
	}
}
