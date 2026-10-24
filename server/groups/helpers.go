package groups

import "time"

// startOfDay 当日 0 点（本地时区）——每日额度计数口径。
// 放在本包内，避免依赖 models 包的未导出函数。
func startOfDay() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}
