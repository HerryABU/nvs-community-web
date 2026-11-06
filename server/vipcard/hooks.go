package vipcard

import (
	"time"

	"nvs-server/models"
	"nvs-server/readperm"
)

func init() {
	models.RegisterDomainModels(Models)
	// #65 的默认钩子只认 MemberUntil（有效即 1 级）。#66 引入多等级会员后
	// 由本域覆盖，判定层与站长配置里的 VipMinLevel 都无需改动。
	readperm.VipLevelOf = vipLevelOf
}

// vipLevelOf 返回用户当前有效会员等级；会员过期即视为 0。
// 兼容 #65 老数据：MemberLevel 为 0 但 MemberUntil 仍有效时按 1 级处理，
// 避免老会员在升级后突然读不到原本可见的 VIP 内容。
func vipLevelOf(userID uint) int {
	if userID == 0 || models.DB == nil {
		return 0
	}
	var u models.User
	if err := models.DB.Select("id", "member_until", "member_level").
		Where("id = ?", userID).First(&u).Error; err != nil {
		return 0
	}
	if u.MemberUntil == nil || !u.MemberUntil.After(time.Now()) {
		return 0
	}
	if u.MemberLevel > 0 {
		return u.MemberLevel
	}
	return 1
}
