package models

import "time"

// ZoneAccessLog 隔离墙（敏感分区）进入记录表
// 176号令合规：专门留存「谁、何时、进入哪个隔离墙分区」的明细，
// 与通用审计日志 user_audit_logs 解耦，便于按分区统计与审计追溯（含将来扩展浏览时长）。
type ZoneAccessLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index:idx_zone_access_user_zone,priority:1" json:"user_id"`    // 进入用户
	Zone      string    `gorm:"size:50;not null;index:idx_zone_access_user_zone,priority:2" json:"zone"` // 隔离墙分区名，如同人区 / 政治文学区
	EnteredAt time.Time `gorm:"not null;index" json:"entered_at"`  // 进入（确认通过）时间
	IP        string    `gorm:"size:45" json:"ip"`
	UserAgent string    `gorm:"type:text" json:"user_agent"`
	CreatedAt time.Time `json:"created_at"`
}

func (ZoneAccessLog) TableName() string { return "zone_access_logs" }

// RecordZoneEnter 写入一条隔离墙进入记录（异步容错，不阻塞主流程）。
func RecordZoneEnterLog(userID uint, zone, ip, ua string, enteredAt time.Time) {
	if zone == "" {
		return
	}
	if enteredAt.IsZero() {
		enteredAt = time.Now()
	}
	_ = DB.Create(&ZoneAccessLog{
		UserID:    userID,
		Zone:      zone,
		EnteredAt: enteredAt,
		IP:        ip,
		UserAgent: ua,
	}).Error
}

// ListZoneAccess 按用户/分区筛选隔离墙进入记录（最近 N 条）
func ListZoneAccess(userID uint, zone string, limit int) ([]ZoneAccessLog, error) {
	if limit <= 0 {
		limit = 500
	}
	query := DB.Model(&ZoneAccessLog{}).Order("id desc")
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if zone != "" {
		query = query.Where("zone = ?", zone)
	}
	var logs []ZoneAccessLog
	err := query.Limit(limit).Find(&logs).Error
	return logs, err
}
