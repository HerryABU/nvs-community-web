package models

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

// 微信扫码登录（F21.2）模型层查询/写入。
// 注意：GetUserByWeChatOpenID 已在 user.go 中随字段一同定义，此处不重复定义，
// 仅补充 UnionID 查找、绑定、解绑与占用检查。

// GetUserByWeChatUnionID 按微信 UnionID 查询。
// 同一开放平台主体下的多个应用共享 UnionID：用于「同主体的另一 App 已注册过账号」
// 的跨应用认领（找到后补绑 openid 即可登录）。
func GetUserByWeChatUnionID(unionID string) (*User, error) {
	unionID = strings.TrimSpace(unionID)
	if unionID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var user User
	err := DB.Where("we_chat_union_id = ?", unionID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// BindWeChat 把微信 openid/unionid 绑定到指定账号（覆盖式写入）。
// 调用方须先经 WeChatOpenIDTaken 预校验 openid 未被其他账号占用。
func BindWeChat(userID uint, openID, unionID string) error {
	openID = strings.TrimSpace(openID)
	if userID == 0 || openID == "" {
		return errors.New("userID/openID 不能为空")
	}
	return DB.Model(&User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"we_chat_open_id":  openID,
		"we_chat_union_id": strings.TrimSpace(unionID),
	}).Error
}

// UnbindWeChat 解绑微信（清空 openid/unionid）
func UnbindWeChat(userID uint) error {
	if userID == 0 {
		return errors.New("userID 不能为空")
	}
	return DB.Model(&User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"we_chat_open_id":  "",
		"we_chat_union_id": "",
	}).Error
}

// WeChatOpenIDTaken openid 是否已被（任意）账号绑定。
// 约束：一个微信只能绑一个账号。
func WeChatOpenIDTaken(openID string) bool {
	openID = strings.TrimSpace(openID)
	if openID == "" {
		return false
	}
	var n int64
	DB.Model(&User{}).Where("we_chat_open_id = ?", openID).Count(&n)
	return n > 0
}
