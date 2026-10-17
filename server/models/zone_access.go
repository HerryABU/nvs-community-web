package models

import (
	"time"

	"gorm.io/gorm"
)

// ZoneAccessLog 隔离墙（敏感分区）进入记录表
// 176号令合规：专门留存「谁、何时、进入哪个隔离墙分区」的明细，
// 与通用审计日志 user_audit_logs 解耦，便于按分区统计与审计追溯（含将来扩展浏览时长）。
type ZoneAccessLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index:idx_zone_access_user_zone,priority:1" json:"user_id"`      // 进入用户
	Zone      string    `gorm:"size:50;not null;index:idx_zone_access_user_zone,priority:2" json:"zone"` // 隔离墙分区名，如同人区 / 政治文学区
	EnteredAt time.Time `gorm:"not null;index" json:"entered_at"`                                        // 进入（确认通过）时间
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

// ZoneAccessLogView 联表视图：在 ZoneAccessLog 基础上附带用户名/昵称，供前台仪表盘展示。
type ZoneAccessLogView struct {
	ZoneAccessLog
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}

// zoneAccessSelect 统一联表查询构造：LEFT JOIN users 取 username/nickname。
func zoneAccessSelect() *gorm.DB {
	return DB.Table("zone_access_logs").
		Select("zone_access_logs.*, COALESCE(users.username,'') AS username, COALESCE(users.nickname,'') AS nickname").
		Joins("LEFT JOIN users ON users.id = zone_access_logs.user_id")
}

// ListZoneAccess 按用户/分区筛选隔离墙进入记录（最近 N 条，兼容旧调用）
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

// ZoneAccessFilter 隔离墙进入记录查询条件
type ZoneAccessFilter struct {
	UserID   uint
	Zone     string
	From     time.Time // 进入时间下界（含）
	To       time.Time // 进入时间上界（含）
	Page     int       // 1-based，<=0 视为 1
	PageSize int       // <=0 视为 20，封顶 200
}

// ListZoneAccessPage 隔离墙进入记录分页查询（联表带出用户名）。
// 返回当前页数据与符合条件的总条数，供前台仪表盘分页展示。
func ListZoneAccessPage(f ZoneAccessFilter) ([]ZoneAccessLogView, int64, error) {
	if f.PageSize <= 0 {
		f.PageSize = 20
	}
	if f.PageSize > 200 {
		f.PageSize = 200
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	query := zoneAccessSelect()
	countQuery := DB.Model(&ZoneAccessLog{})
	if f.UserID > 0 {
		query = query.Where("zone_access_logs.user_id = ?", f.UserID)
		countQuery = countQuery.Where("user_id = ?", f.UserID)
	}
	if f.Zone != "" {
		query = query.Where("zone_access_logs.zone = ?", f.Zone)
		countQuery = countQuery.Where("zone = ?", f.Zone)
	}
	if !f.From.IsZero() {
		query = query.Where("zone_access_logs.entered_at >= ?", f.From)
		countQuery = countQuery.Where("entered_at >= ?", f.From)
	}
	if !f.To.IsZero() {
		query = query.Where("zone_access_logs.entered_at <= ?", f.To)
		countQuery = countQuery.Where("entered_at <= ?", f.To)
	}
	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []ZoneAccessLogView
	offset := (f.Page - 1) * f.PageSize
	err := query.Order("zone_access_logs.id desc").Offset(offset).Limit(f.PageSize).Find(&logs).Error
	return logs, total, err
}

// ZoneAccessStats 隔离墙进入记录聚合统计（仪表盘卡片用）。
type ZoneAccessStats struct {
	Total       int64            `json:"total"`        // 累计进入次数
	Today       int64            `json:"today"`        // 今日进入次数
	UniqueUsers int64            `json:"unique_users"` // 累计去重用户数
	Zones       []ZoneAccessZone `json:"zones"`        // 各分区进入次数
}

// ZoneAccessZone 单个分区的进入次数
type ZoneAccessZone struct {
	Zone  string `json:"zone"`
	Count int64  `json:"count"`
}

// GetZoneAccessStats 聚合统计：累计、今日、去重用户、按分区分布。
func GetZoneAccessStats() (ZoneAccessStats, error) {
	var s ZoneAccessStats
	if err := DB.Model(&ZoneAccessLog{}).Count(&s.Total).Error; err != nil {
		return s, err
	}
	todayStart := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
	if err := DB.Model(&ZoneAccessLog{}).Where("entered_at >= ?", todayStart).Count(&s.Today).Error; err != nil {
		return s, err
	}
	if err := DB.Model(&ZoneAccessLog{}).Select("COUNT(DISTINCT user_id)").Scan(&s.UniqueUsers).Error; err != nil {
		return s, err
	}
	var rows []ZoneAccessZone
	if err := DB.Model(&ZoneAccessLog{}).
		Select("zone, COUNT(*) AS count").
		Group("zone").Order("count desc").
		Scan(&rows).Error; err != nil {
		return s, err
	}
	s.Zones = rows
	return s, nil
}
