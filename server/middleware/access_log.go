package middleware

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// AccessLog 访问日志中间件（176号令合规：上网日志关联用户标识）。
// 每条日志为 JSON 一行，包含 user_id（未登录为 0）、method、path、ip、status、latency。
// 按天滚动写入 logs/access-YYYY-MM-DD.log，便于用工具按 user_id 检索追溯。
//
// 注意：本中间件为全局中间件，在 AuthRequired 之前执行；通过 c.Next() 后再取 user_id，
// 因此能拿到后续鉴权中间件写入的登录态。

const LogDir = "logs"

var (
	accessLogMu   sync.Mutex
	accessLogFile *os.File
	accessLogDate string
)

// accessLogWriter 返回当天日期的日志文件句柄（跨天自动切换文件）
func accessLogWriter() *os.File {
	date := time.Now().Format("2006-01-02")
	accessLogMu.Lock()
	defer accessLogMu.Unlock()

	if accessLogFile != nil && accessLogDate == date {
		return accessLogFile
	}
	if accessLogFile != nil {
		_ = accessLogFile.Close()
	}
	_ = os.MkdirAll(LogDir, 0755)
	f, err := os.OpenFile(filepath.Join(LogDir, "access-"+date+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// 无法写文件时回退到标准错误，保证服务不中断
		accessLogFile = os.Stderr
		accessLogDate = date
		return accessLogFile
	}
	accessLogFile = f
	accessLogDate = date
	return accessLogFile
}

// AccessLog 返回访问日志中间件
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		// 鉴权中间件在 c.Next() 内执行，此时可拿到 userID（未登录为 0）
		userID := UserID(c)

		entry := struct {
			Timestamp string `json:"timestamp"`
			UserID    uint   `json:"user_id"`
			Method    string `json:"method"`
			Path      string `json:"path"`
			IP        string `json:"ip"`
			Status    int    `json:"status"`
			Latency   string `json:"latency"`
		}{
			Timestamp: time.Now().Format("2006-01-02 15:04:05"),
			UserID:    userID,
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			IP:        c.ClientIP(),
			Status:    c.Writer.Status(),
			Latency:   time.Since(start).String(),
		}

		line, err := json.Marshal(entry)
		if err != nil {
			return
		}
		line = append(line, '\n')
		_, _ = accessLogWriter().Write(line)
	}
}
