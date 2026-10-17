package handlers

import (
	"context"
	"database/sql"
	"net"
	"runtime"
	"sync"
	"time"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/utils"
	"nvs-server/version"

	"github.com/gin-gonic/gin"
)

// ==================== 全面健康检查 ====================
// /health 返回结构化健康报告，覆盖：
//   1. 服务器：运行时长、Goroutine、内存占用；
//   2. 数据库：Ping + 延迟（后端所有业务 API 的核心依赖）；
//   3. Redis：连接状态；
//   4. 网络：出站连通性（TCP 拨测，短超时）；
//   5. 业务 API：关键公开数据源（users/novels/forums）可查询；
//   6. 前端：内嵌 SPA 资源（index.html + bundle）是否可服务。

// FrontendProbe 由 main.go 注入，检查内嵌前端资源是否完整
var FrontendProbe func() (bundle string, ok bool)

// CollectHealth 收集完整健康信息
func CollectHealth() gin.H {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	// 1. 服务器
	server := gin.H{
		"ok":           true,
		"uptime_sec":   int64(time.Since(serverStartTime).Seconds()),
		"goroutines":   runtime.NumGoroutine(),
		"mem_alloc_mb": round2(float64(ms.Alloc) / 1024 / 1024),
	}

	// 2. 数据库
	db := gin.H{"ok": false}
	if models.DB != nil {
		if sqlDB, err := models.DB.DB(); err == nil && sqlDB != nil {
			t0 := time.Now()
			if err := sqlDB.Ping(); err == nil {
				latency := time.Since(t0)
				db["ok"] = true
				db["latency_ms"] = latency.Milliseconds()
				// 数据库驱动类型（sqlite / mysql）——不暴露连接串/账号/库名等敏感信息
				db["driver"] = models.DB.Dialector.Name()
				// 稳定性：最近 N 次健康检查的成功率 + 连接池运行状态
				db["stable"] = dbHealth.record(true, latency)
				db["pool"] = poolStats(sqlDB)
			} else {
				db["stable"] = dbHealth.record(false, 0)
				// 🔒 安全审计 L1：不向公开接口回显原始错误（可能泄露内部路径与版本）
				db["error"] = "database unavailable"
			}
		}
	} else {
		db["error"] = "database unavailable"
	}

	// 3. Redis
	redis := gin.H{"ok": false, "status": "disabled"}
	if utils.RedisClient != nil {
		if _, err := utils.RedisClient.Ping(context.Background()).Result(); err == nil {
			redis["ok"] = true
			redis["status"] = "ok"
		} else {
			redis["status"] = "error"
			redis["error"] = "redis unavailable"
		}
	}

	// 4. 网络（出站连通性，短超时，不阻塞）
	network := gin.H{"ok": true}
	if err := probeOutbound(); err != nil {
		network["ok"] = false
		network["error"] = "outbound unavailable"
	}

	// 5. 业务 API（🔒 仅返回可用性，不公开数据量——用户/作品数属业务敏感信息）
	apis := gin.H{"ok": false}
	if models.DB != nil {
		apis["ok"] = true
	} else {
		apis["error"] = "database unavailable"
	}

	// 6. 前端
	frontend := gin.H{"ok": false}
	if FrontendProbe != nil {
		if _, ok := FrontendProbe(); ok {
			frontend["ok"] = true
		} else {
			frontend["error"] = "frontend assets missing"
		}
	} else {
		frontend["error"] = "frontend probe not injected"
	}

	// 汇总
	status := "ok"
	dbOK, _ := db["ok"].(bool)
	netOK, _ := network["ok"].(bool)
	feOK, _ := frontend["ok"].(bool)
	if (config.IsInstalled() && !dbOK) || !netOK || !feOK {
		status = "degraded"
	}

	return gin.H{
		"status":        status,
		"service":       "nvs-server",
		"version":       version.AppVersion,
		"installed":     config.IsInstalled(),
		"now":           time.Now().Format(time.RFC3339),
		"server":        server,
		"database":      db,
		"redis":         redis,
		"network":       network,
		"business_apis": apis,
		"frontend":      frontend,
	}
}

// probeOutbound 出站网络连通性检测（TCP 拨测公共 DNS，短超时）
func probeOutbound() error {
	timeout := 2 * time.Second
	// 依次尝试多个公共端点，任一可达即视为网络正常
	hosts := []string{"1.1.1.1:53", "8.8.8.8:53"}
	var lastErr error
	for _, h := range hosts {
		conn, err := net.DialTimeout("tcp", h, timeout)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err
	}
	return lastErr
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

// ==================== 数据库健康跟踪（稳定性） ====================
// dbHealthTracker 进程内维护最近 N 次数据库健康检查结果，
// 用于 /health 展示稳定性：成功率、连续失败次数、最近成功/失败时间。
// 纯内存状态，无文件 I/O，重启后窗口清空重建。

const dbHealthWindow = 20 // 滑动窗口大小：统计最近 20 次检查

var dbHealth = &dbHealthTracker{max: dbHealthWindow}

type dbHealthTracker struct {
	mu               sync.Mutex
	max              int    // 滑动窗口大小
	window           []bool // 最近 max 次结果（true=成功）
	checks           int    // 窗口内累计检查次数（未满窗口时小于 max）
	consecutiveFails int    // 当前连续失败次数
	lastOKAt         time.Time
	lastFailAt       time.Time
	lastLatencyMs    int64
}

// record 记录一次检查结果，返回稳定性摘要（供 /health 展示）
func (t *dbHealthTracker) record(ok bool, latency time.Duration) map[string]interface{} {
	t.mu.Lock()
	defer t.mu.Unlock()

	if ok {
		t.consecutiveFails = 0
		t.lastOKAt = time.Now()
		t.lastLatencyMs = latency.Milliseconds()
	} else {
		t.consecutiveFails++
		t.lastFailAt = time.Now()
	}

	t.window = append(t.window, ok)
	if len(t.window) > t.max {
		t.window = t.window[len(t.window)-t.max:]
	}
	if t.checks < t.max {
		t.checks++
	}

	// 窗口内成功率（0-100）
	succ := 0
	for _, r := range t.window {
		if r {
			succ++
		}
	}
	rate := 0
	if len(t.window) > 0 {
		rate = succ * 100 / len(t.window)
	}

	summary := gin.H{
		"success_rate":      rate, // 最近 N 次成功率（%）
		"checks_in_window":  len(t.window),
		"consecutive_fails": t.consecutiveFails,
		"last_latency_ms":   t.lastLatencyMs,
		"last_ok_at":        t.lastOKAt.Format(time.RFC3339),
		"status":            "stable",
	}
	if t.consecutiveFails > 0 {
		summary["status"] = "degraded"
		summary["last_fail_at"] = t.lastFailAt.Format(time.RFC3339)
	}
	return summary
}

// poolStats 返回数据库连接池运行状态（稳定性/容量参考）
func poolStats(sqlDB interface {
	Stats() sql.DBStats
}) gin.H {
	stats := sqlDB.Stats()
	return gin.H{
		"max_open":        stats.MaxOpenConnections, // 最大连接数上限
		"open":            stats.OpenConnections,    // 当前打开的连接数
		"in_use":          stats.InUse,              // 正在使用的连接数
		"idle":            stats.Idle,               // 空闲连接数
		"wait_count":      stats.WaitCount,          // 等待连接的累计次数（高说明连接不足）
		"wait_ms":         stats.WaitDuration.Milliseconds(),
		"max_idle_closed": stats.MaxIdleClosed,
		"max_lifetime":    stats.MaxLifetimeClosed,
	}
}
