package utils

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm/logger"
)

// DBLogger 实现 gorm.Logger 接口，将数据库操作（SQL、绑定参数、耗时、影响行数、错误）
// 按小时追加写入 log/database/<YYYY-MM-DD>/<HH>.log（database 一个文件夹，
// 每天内按小时切分）。满足「数据库日志」留存与审计需求。
//
// 全量原则：作为合规审计日志，本记录器对**每一条数据库操作**无条件落盘——
// 普通查询、慢查询、错误、事务 BEGIN/COMMIT/ROLLBACK、建表迁移全部记录，
// 不受 LogMode 级别降级影响（任何 LogMode 调用均不打断全量记录）。
type DBLogger struct {
	LogLevel logger.LogLevel
}

func NewDBLogger() *DBLogger {
	return &DBLogger{LogLevel: logger.Info}
}

// LogMode 返回新实例；全量审计模式下级别仅作标记，不影响是否落盘。
func (l *DBLogger) LogMode(level logger.LogLevel) logger.Interface {
	return &DBLogger{LogLevel: level}
}

// formatMsg 格式化 GORM 的 (msg, data...) 日志：msg 含占位符时格式化，
// 否则追加 data 避免出现 "%!EXTRA" 噪音。
func formatMsg(msg string, data ...interface{}) string {
	if len(data) == 0 {
		return msg
	}
	if strings.Contains(msg, "%") {
		return fmt.Sprintf(msg, data...)
	}
	parts := make([]string, 0, len(data)+1)
	parts = append(parts, msg)
	for _, d := range data {
		parts = append(parts, fmt.Sprint(d))
	}
	return strings.Join(parts, " ")
}

func (l *DBLogger) Info(_ context.Context, msg string, data ...interface{}) {
	l.write("INFO", formatMsg(msg, data...), "", 0, nil)
}

func (l *DBLogger) Warn(_ context.Context, msg string, data ...interface{}) {
	l.write("WARN", formatMsg(msg, data...), "", 0, nil)
}

func (l *DBLogger) Error(_ context.Context, msg string, data ...interface{}) {
	l.write("ERROR", formatMsg(msg, data...), "", 0, nil)
}

// Trace 记录每一条 SQL（含绑定参数、影响行数、耗时；错误 SQL 标 ERROR，慢查询标 WARN）。
// 无条件全量记录：即使 LogLevel 被外部降级（如 Silent），合规审计仍不遗漏。
func (l *DBLogger) Trace(_ context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, rows := fc()
	elapsed := time.Since(begin)

	level := "INFO"
	if err != nil {
		level = "ERROR"
	}
	// 慢查询（>100ms）升级为 WARN，便于排查性能问题（仍计入全量）
	if level == "INFO" && elapsed > 100*time.Millisecond {
		level = "WARN"
	}

	l.write(level, sql, fmt.Sprintf("rows=%d cost=%s", rows, elapsed), rows, err)
}

func (l *DBLogger) write(level, sql, extra string, rows int64, err error) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] %s | sql=%q", level, ts, sql)
	if extra != "" {
		line += " | " + extra
	}
	if err != nil {
		line += fmt.Sprintf(" | error=%v", err)
	}
	_ = WriteDailyLog("database", line)
}
