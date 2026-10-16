// auth_wechat.go — 微信扫码登录/绑定（F21.2）处理器：
//
//	WeChatAuthStart   GET  /api/auth/wechat/url         获取授权 URL（生成一次性 state）
//	WeChatCallback    GET  /api/auth/wechat/callback    微信回调：换 token → 找号/自动注册 → 下发登录态 → 302 回前端
//	WeChatBindStatus  GET  /api/auth/wechat/bind-status 当前账号绑定状态（登录态）
//	WeChatBind        POST /api/auth/wechat/bind        已登录账号绑定微信 {code,state}
//	WeChatUnbind      POST /api/auth/wechat/unbind      解绑微信（登录态）
//
// 回调设计：无论成功/失败都 302 重定回前端（避免微信侧展示错误页）——
//
//	登录用途成功 → {prefix}/login?wechat=bound（JWT + refresh Cookie 已下发）
//	绑定用途成功 → {prefix}/user/wechat-bind?wechat=bound
//	失败         → 对应路径 ?wechat=failed&reason=<cancel|state|exchange|userinfo|taken|locked|server|register>
package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 审计动作（UserAuditLog.Action 为 varchar(30)，以下均远小于上限）
const (
	auditWeChatBind   = "wechat_bind"
	auditWeChatUnbind = "wechat_unbind"
)

// 前端重定向路径（相对路径，自动拼接 config.URLPrefix 站点前缀）
const (
	wechatLoginRedirectPath = "/login"
	wechatBindRedirectPath  = "/user/wechat-bind"
)

// WeChatAuthStart GET /api/auth/wechat/url?purpose=login|bind
// 生成一次性 state（服务端存储，TTL 5 分钟）并返回微信授权 URL。
// purpose=login（默认）：扫码登录/自动注册；purpose=bind：已登录用户绑定微信。
func WeChatAuthStart(c *gin.Context) {
	purpose := strings.ToLower(strings.TrimSpace(c.Query("purpose")))
	if purpose != utils.WeChatPurposeBind {
		purpose = utils.WeChatPurposeLogin
	}
	// 登录用途受站长「登录方式」开关约束；绑定用途仅要求服务端已配置微信参数
	if purpose == utils.WeChatPurposeLogin && !models.GetLoginMethods().WeChat {
		utils.Forbidden(c, "站长已关闭微信扫码登录")
		return
	}
	state, err := utils.GenerateWeChatState()
	if err != nil {
		utils.InternalError(c, "生成登录状态失败")
		return
	}
	authURL, err := utils.WeChatAuthURL(state)
	if err != nil {
		// 服务端未配置 appid/secret/callback，直接把中文原因透出便于站长排查
		utils.InternalError(c, err.Error())
		return
	}
	if err := utils.SaveWeChatState(state, purpose); err != nil {
		utils.InternalError(c, "保存登录状态失败")
		return
	}
	utils.Success(c, gin.H{"url": authURL, "state": state, "purpose": purpose})
}

// WeChatCallback GET /api/auth/wechat/callback?code&state
// 微信授权回调。流程：校验并一次性消费 state → 换 token → 拉用户信息 →
// 按 openid 找号 → 按 unionid 再找（同主体另一 App）→ 自动注册 → 下发登录态 → 302 回前端。
// 绑定用途（purpose=bind 且请求携带有效登录态）：改为把微信绑定到当前账号，不切换登录态。
func WeChatCallback(c *gin.Context) {
	code := strings.TrimSpace(c.Query("code"))
	state := strings.TrimSpace(c.Query("state"))

	basePath := wechatLoginRedirectPath
	fail := func(reason string) {
		c.Redirect(http.StatusFound, config.URLPrefix+basePath+"?wechat=failed&reason="+url.QueryEscape(reason))
	}

	// 用户扫码后取消授权：微信只回 error/state，不带 code
	if code == "" {
		fail("cancel")
		return
	}
	purpose, err := utils.ConsumeWeChatState(state)
	if err != nil {
		fail("state")
		return
	}
	if purpose == utils.WeChatPurposeBind {
		basePath = wechatBindRedirectPath
	}

	openID, unionID, accessToken, err := utils.WeChatExchangeCode(code)
	if err != nil {
		fail("exchange")
		return
	}
	nickname, headImg, err := utils.WeChatUserInfo(accessToken, openID)
	if err != nil {
		fail("userinfo")
		return
	}

	// 绑定用途且当前请求携带有效登录态（public 组 OptionalAuth 已解析 Cookie）→ 绑定到当前账号
	if purpose == utils.WeChatPurposeBind {
		if uid := currentUserID(c); uid > 0 {
			bindWeChatToUser(c, uid, openID, unionID, basePath)
			return
		}
		// 未登录的绑定意图退化为登录流程（state 已消费，用户仍可完成扫码登录）
	}

	user, err := models.GetUserByWeChatOpenID(openID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			fail("server")
			return
		}
		// openid 未命中 → 按 unionid 再找（同主体另一 App 已注册过账号）
		if unionID != "" {
			user, err = models.GetUserByWeChatUnionID(unionID)
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				fail("server")
				return
			}
		}
		if user == nil {
			// 首次微信登录 → 自动注册（176号令合规：记录注册 IP）
			user, err = createWeChatUser(c, openID, unionID, nickname, headImg)
			if err != nil {
				fail("register")
				return
			}
			recordUserAudit(c, user.ID, models.AuditRegister, "微信扫码注册 "+openID, "success")
		} else {
			// unionid 命中历史账号：补绑 openid（同主体另一 App 首次经本应用登录）
			if bindErr := models.BindWeChat(user.ID, openID, unionID); bindErr != nil {
				fail("server")
				return
			}
		}
	}

	// 账号锁定检查（与邮箱/手机登录一致）
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		fail("locked")
		return
	}

	issueSession(c, user, "微信扫码登录 "+openID)
	c.Redirect(http.StatusFound, config.URLPrefix+basePath+"?wechat=bound")
}

// bindWeChatToUser 绑定用途回调：把微信写入当前登录账号并重定回绑定页。
// 约束：一个微信只能绑一个账号（openid 被他人占用时拒绝）。
func bindWeChatToUser(c *gin.Context, userID uint, openID, unionID, basePath string) {
	fail := func(reason string) {
		c.Redirect(http.StatusFound, config.URLPrefix+basePath+"?wechat=failed&reason="+url.QueryEscape(reason))
	}
	if models.WeChatOpenIDTaken(openID) {
		existing, err := models.GetUserByWeChatOpenID(openID)
		if err != nil || existing == nil || existing.ID != userID {
			fail("taken")
			return
		}
	}
	if err := models.BindWeChat(userID, openID, unionID); err != nil {
		fail("server")
		return
	}
	recordUserAudit(c, userID, auditWeChatBind, "微信扫码绑定 "+openID, "success")
	c.Redirect(http.StatusFound, config.URLPrefix+basePath+"?wechat=bound")
}

// createWeChatUser 微信首次登录自动注册：占位邮箱 + 随机密码哈希 + 默认读者角色，
// 同时写入 openid/unionid 绑定关系（参考 auth_phone.go createPhoneUser）。
func createWeChatUser(c *gin.Context, openID, unionID, nickname, headImg string) (*models.User, error) {
	username, err := generateUniqueWeChatUsername(openID)
	if err != nil {
		return nil, err
	}
	// 随机密码哈希：微信用户不经密码登录，但 PasswordHash 为 not null 列
	randPass := make([]byte, 24)
	if _, err := rand.Read(randPass); err != nil {
		return nil, err
	}
	hash, err := utils.HashPassword("wx!" + hex.EncodeToString(randPass))
	if err != nil {
		return nil, err
	}
	user := &models.User{
		Username:       username,
		Email:          openID + "@wechat.nvs.local", // 占位邮箱（唯一，不可用于登录）
		PasswordHash:   hash,
		Nickname:       nickname,
		AvatarURL:      headImg,
		Role:           "reader",
		WeChatOpenID:   openID,
		WeChatUnionID:  unionID,
		RegistrationIP: c.ClientIP(),
	}
	if err := models.CreateUser(user); err != nil {
		return nil, err
	}
	return user, nil
}

// generateUniqueWeChatUsername 生成唯一用户名：wx_ + openid 后 8 位，冲突加随机后缀重试。
func generateUniqueWeChatUsername(openID string) (string, error) {
	base := "wx_"
	if len(openID) > 8 {
		base += openID[len(openID)-8:]
	} else {
		base += openID
	}
	for i := 0; i < 8; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(100000))
		if err != nil {
			n = big.NewInt(int64(time.Now().UnixNano() % 100000))
		}
		candidate := base + int64ToStr(n.Int64())
		var count int64
		models.DB.Model(&models.User{}).Where("username = ?", candidate).Count(&count)
		if count == 0 {
			return candidate, nil
		}
	}
	return "", errors.New("用户名生成冲突，请重试")
}

// WeChatBindStatus GET /api/auth/wechat/bind-status（登录态）
// 返回当前账号的微信绑定状态；已绑定时附带账号昵称与头像（注册时取自微信）。
func WeChatBindStatus(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	user, err := models.GetUserByID(uid)
	if err != nil {
		utils.InternalError(c, "获取用户信息失败")
		return
	}
	utils.Success(c, gin.H{
		"bound":    user.WeChatOpenID != "",
		"nickname": user.Nickname,
		"avatar":   user.AvatarURL,
	})
}

// WeChatBind POST /api/auth/wechat/bind {code,state}（登录态）
// 已登录用户绑定微信：消费 state → 换 token 拉信息 → openid 未被其他账号占用 → 写入当前用户。
// 说明：常规绑定走「授权 URL 跳转 + 微信回调（purpose=bind）」，由 WeChatCallback 完成；
// 本接口面向自行实现弹窗授权的前端场景，两者共用同一套 state 与校验逻辑。
func WeChatBind(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	var req struct {
		Code  string `json:"code" binding:"required"`
		State string `json:"state" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if _, err := utils.ConsumeWeChatState(strings.TrimSpace(req.State)); err != nil {
		utils.BadRequest(c, "绑定状态已过期，请重新发起绑定")
		return
	}
	openID, unionID, _, err := utils.WeChatExchangeCode(strings.TrimSpace(req.Code))
	if err != nil {
		utils.BadRequest(c, "微信授权失败: "+err.Error())
		return
	}
	if models.WeChatOpenIDTaken(openID) {
		existing, eerr := models.GetUserByWeChatOpenID(openID)
		if eerr != nil || existing == nil || existing.ID != uid {
			utils.BadRequest(c, "该微信已绑定其他账号，请先在其账号下解绑")
			return
		}
	}
	if err := models.BindWeChat(uid, openID, unionID); err != nil {
		utils.InternalError(c, "绑定失败")
		return
	}
	recordUserAudit(c, uid, auditWeChatBind, "绑定微信 "+openID, "success")
	utils.SuccessMessage(c, "微信绑定成功", nil)
}

// WeChatUnbind POST /api/auth/wechat/unbind（登录态；清空 openid/unionid）
func WeChatUnbind(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	user, err := models.GetUserByID(uid)
	if err != nil {
		utils.InternalError(c, "获取用户信息失败")
		return
	}
	if user.WeChatOpenID == "" {
		utils.BadRequest(c, "尚未绑定微信")
		return
	}
	if err := models.UnbindWeChat(uid); err != nil {
		utils.InternalError(c, "解绑失败")
		return
	}
	recordUserAudit(c, uid, auditWeChatUnbind, "解绑微信 "+user.WeChatOpenID, "success")
	utils.SuccessMessage(c, "已解绑微信", nil)
}
