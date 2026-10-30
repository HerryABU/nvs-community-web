package models

// RefreshState 记录每个用户当前「有效」刷新令牌族 ID（JTI）。
//
// 刷新令牌轮换时更新 ActiveJTI；旧刷新令牌因 JTI 不匹配而失效，
// 从而防止刷新令牌被盗用后长期可用（纵深防御：窃取者即使拿到 refresh_token，
// 一旦受害者在另一设备刷新，被盗令牌立即作废；同时天然实现单会话约束）。
// 访问令牌（token Cookie）被盗则由其 Binding 绑定在 middleware 层拦截。
type RefreshState struct {
	UserID    uint   `gorm:"primaryKey" json:"user_id"`
	ActiveJTI string `gorm:"not null;size:64" json:"active_jti"`
	UpdatedAt int64  `gorm:"autoUpdateTime" json:"updated_at"`
}

// SetRefreshState 写入 / 更新用户的有效刷新 JTI（幂等 upsert，按 UserID 主键）。
func SetRefreshState(userID uint, jti string) error {
	return DB.Save(&RefreshState{UserID: userID, ActiveJTI: jti}).Error
}

// GetRefreshState 读取用户当前有效刷新 JTI；无记录返回 gorm.ErrRecordNotFound。
func GetRefreshState(userID uint) (*RefreshState, error) {
	var s RefreshState
	if err := DB.First(&s, "user_id = ?", userID).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// DeleteRefreshState 清除用户刷新状态（登出 / 改密时调用，使仍在有效期内的刷新令牌立即失效）。
func DeleteRefreshState(userID uint) error {
	return DB.Delete(&RefreshState{}, "user_id = ?", userID).Error
}
