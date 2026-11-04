package models

import (
	"fmt"
	"strconv"
)

// ResolveUserKey 解析用户标识（数字主键 id 或 username），返回用户。
// 规则：纯数字 → 按 id 查；其余 → 按 username 查（大小写不敏感）。
// 从 handlers/author 迁移至数据层，避免 author ↔ blacklist 的循环依赖。
func ResolveUserKey(key string) (*User, error) {
	if key == "" {
		return nil, fmt.Errorf("用户标识为空")
	}
	if id, err := strconv.ParseUint(key, 10, 64); err == nil {
		return GetUserByID(uint(id))
	}
	var u User
	if err := DB.Where("LOWER(username) = LOWER(?)", key).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}
