package utils

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DailyLogRoot 日志根目录（相对 exe 所在目录，即 /log）。
// 结构（按小时切分 + 按类分文件夹）：
//   log/requests/<YYYY-MM-DD>/<HH>.log   —— Web 请求日志（request 一个文件夹）
//   log/database/<YYYY-MM-DD>/<HH>.log   —— 数据库日志（database 一个文件夹）
// 每天内按小时切分（每小时一个文件），跨小时/跨天自动滚动，重启不覆盖（追加写）。
const DailyLogRoot = "log"

// 已打开的日志句柄缓存（key = category/date/hour），跨小时/跨天自动滚动。
var (
	dailyLogMu sync.Mutex
	dailyLogs  = map[string]*dailyLogHandle{}
)

type dailyLogHandle struct {
	key  string
	file *os.File
}

// WriteDailyLog 将一行日志追加写入 log/<category>/<日期>/<小时>.log。
// category 例如 "requests"、"database"（每个类别一个独立文件夹）；
// 文件以 O_APPEND 方式打开，项目重启后新内容追加在末尾，绝不覆盖历史记录
// （满足断点续传/审计要求）。跨小时或跨天写入时自动关闭旧文件并切换。
func WriteDailyLog(category, line string) error {
	now := time.Now()
	key := category + "/" + now.Format("2006-01-02") + "/" + now.Format("15")

	dailyLogMu.Lock()
	defer dailyLogMu.Unlock()

	w, ok := dailyLogs[key]
	if !ok || w == nil || w.file == nil {
		if w != nil && w.file != nil {
			_ = w.file.Close()
		}
		dir := filepath.Join(DailyLogRoot, category, now.Format("2006-01-02"))
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		p := filepath.Join(dir, now.Format("15")+".log")
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		w = &dailyLogHandle{key: key, file: f}
		dailyLogs[key] = w
	}

	// 确保每行以换行结尾
	if line == "" || line[len(line)-1] != '\n' {
		line += "\n"
	}
	_, err := w.file.WriteString(line)
	return err
}

// CloseDailyLogs 关闭所有日志句柄（进程退出前可选调用，便于干净释放）。
func CloseDailyLogs() {
	dailyLogMu.Lock()
	defer dailyLogMu.Unlock()
	for _, w := range dailyLogs {
		if w != nil && w.file != nil {
			_ = w.file.Close()
		}
	}
	dailyLogs = map[string]*dailyLogHandle{}
}
