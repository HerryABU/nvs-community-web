package models

import "time"

// ─────────────────────────────────────────────────────────────
// 狂暴模式配置（F19：评论区实时更新 / SSE 弹幕）
// 单行表（固定 ID=1）：管理员开关 + 带宽控制参数。
// 默认关闭——未保存过配置时 GetRageConfig 返回全默认值，SSE 端点拒绝连接（403）。
// ─────────────────────────────────────────────────────────────

// RageConfig 狂暴模式配置（单行，ID 固定为 1）
type RageConfig struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	Enabled          bool      `gorm:"default:false" json:"enabled"`        // 总开关（默认关闭）
	BatchIntervalSec int       `gorm:"default:3" json:"batch_interval_sec"` // 批量 flush 周期（秒，1-60）
	BatchSize        int       `gorm:"default:20" json:"batch_size"`        // pending 累积立即 flush 阈值（1-100）
	MaxConns         int       `gorm:"default:2000" json:"max_conns"`       // 全局 SSE 连接数上限（100-10000）
	UpdatedAt        time.Time `json:"updated_at"`
}

func (RageConfig) TableName() string { return "rage_configs" }

// 狂暴模式默认值（与 gorm tag / 文档保持一致，读取时兜底）
const (
	rageDefaultBatchIntervalSec = 3
	rageDefaultBatchSize        = 20
	rageDefaultMaxConns         = 2000
)

// GetRageConfig 读取狂暴模式配置。
// 无记录（从未保存过）返回默认值：Enabled=false + 保守带宽参数。
// 另兜底纠正历史脏数据（0/负值），避免批量周期为 0 导致忙循环或连接上限失效。
func GetRageConfig() RageConfig {
	cfg := RageConfig{
		Enabled:          false,
		BatchIntervalSec: rageDefaultBatchIntervalSec,
		BatchSize:        rageDefaultBatchSize,
		MaxConns:         rageDefaultMaxConns,
	}
	if DB == nil {
		return cfg
	}
	var row RageConfig
	if err := DB.First(&row, 1).Error; err != nil {
		return cfg
	}
	// Enabled 零值即 false（未保存过 = 关闭），无需纠正
	if row.BatchIntervalSec <= 0 {
		row.BatchIntervalSec = rageDefaultBatchIntervalSec
	}
	if row.BatchSize <= 0 {
		row.BatchSize = rageDefaultBatchSize
	}
	if row.MaxConns <= 0 {
		row.MaxConns = rageDefaultMaxConns
	}
	return row
}

// UpdateRageConfig 保存狂暴模式配置（单行 upsert）。
// 🔒 GORM 零值坑：结构体 Updates 会跳过零值字段，Enabled=false 永远写不进库，
// 因此必须用 map 精确写列。行不存在时创建（ID 固定 1）。
func UpdateRageConfig(cfg RageConfig) error {
	if DB == nil {
		return nil
	}
	var existing RageConfig
	if err := DB.First(&existing, 1).Error; err != nil {
		cfg.ID = 1
		cfg.UpdatedAt = time.Now()
		return DB.Create(&cfg).Error
	}
	return DB.Model(&RageConfig{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"enabled":            cfg.Enabled,
		"batch_interval_sec": cfg.BatchIntervalSec,
		"batch_size":         cfg.BatchSize,
		"max_conns":          cfg.MaxConns,
		"updated_at":         time.Now(),
	}).Error
}
