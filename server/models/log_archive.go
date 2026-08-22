package models

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ==================== 访问日志完整性保护（176号令合规） ====================
// 访问日志作为执法证据，需防篡改：按小时滚动、每日 sha256 校验、gzip 归档、
// 只读权限（0444）、异地目录备份（backups/logs/）、保留至少 6 个月。
//
// 目录结构（与 utils/dailylog.go 一致，按小时切分 + 按类分文件夹）：
//   log/requests/<YYYY-MM-DD>/<HH>.log   请求日志（request 一个文件夹）
//   log/database/<YYYY-MM-DD>/<HH>.log   数据库日志（database 一个文件夹）
// 归档目标：backups/logs/<category>-<日期>-<小时>.log.gz
// 校验值：log/<category>/<日期>.sha256（每行 "sha256  <category>-<日期>-<小时>.log"）

const (
	LogDirName       = "log"                // 日志根目录（与 utils.DailyLogRoot 一致）
	LogBackupDirName = "backups/logs"       // 归档备份目录
	LogRetentionDur  = 180 * 24 * time.Hour // 保留 6 个月
)

// logCategories 需要归档的日志类别（文件夹名）
var logCategories = []string{"requests", "database"}

var logArchiveMu sync.Mutex

// ArchiveAccessLogs 归档除今天外的所有小时日志（requests 与 database 两类）。
// 每个小时文件：计算 sha256 → gzip 归档到 backups/logs/<category>-<日期>-<小时>.log.gz
// → 原文件改只读；每日生成 log/<category>/<日期>.sha256 汇总校验值。
// 并清理超过 6 个月的归档（保留 sha256 校验值以便追溯）。
func ArchiveAccessLogs() error {
	logArchiveMu.Lock()
	defer logArchiveMu.Unlock()

	today := time.Now().Format("2006-01-02")
	_ = os.MkdirAll(LogBackupDirName, 0755)

	for _, category := range logCategories {
		archiveCategory(category, today)
	}

	cleanupOldAccessLogs()
	return nil
}

// archiveCategory 归档某个类别（requests / database）下所有过去日期的小时日志
func archiveCategory(category, today string) {
	catDir := filepath.Join(LogDirName, category)
	dateDirs, err := os.ReadDir(catDir)
	if err != nil {
		return
	}
	for _, d := range dateDirs {
		if !d.IsDir() {
			continue
		}
		date := d.Name()
		if date >= today {
			continue // 今天仍在写入，跳过
		}
		// 校验日期格式，避免误处理非日期目录
		if _, perr := time.Parse("2006-01-02", date); perr != nil {
			continue
		}

		dateDir := filepath.Join(catDir, date)
		entries, err := os.ReadDir(dateDir)
		if err != nil {
			continue
		}

		shaLines := []string{}
		archivedAny := false
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
				continue
			}
			hour := strings.TrimSuffix(e.Name(), ".log")
			if len(hour) != 2 { // 仅处理 HH.log（如 08.log）
				continue
			}
			logPath := filepath.Join(dateDir, e.Name())
			gzPath := filepath.Join(LogBackupDirName, category+"-"+date+"-"+hour+".log.gz")
			if _, err := os.Stat(gzPath); err == nil {
				continue // 已归档
			}

			sum, err := fileSHA256(logPath)
			if err != nil {
				continue
			}
			shaLines = append(shaLines, sum+"  "+category+"-"+date+"-"+hour+".log")

			if err := gzipFile(logPath, gzPath); err != nil {
				continue
			}
			// 只读权限，防篡改
			_ = os.Chmod(logPath, 0444)
			archivedAny = true
		}

		if archivedAny {
			shaPath := filepath.Join(catDir, date+".sha256")
			_ = os.WriteFile(shaPath, []byte(strings.Join(shaLines, "\n")+"\n"), 0644)
			_ = os.Chmod(shaPath, 0444)
		}
	}
}

// fileSHA256 计算文件 SHA256
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// gzipFile 将 src 压缩为 dst
func gzipFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	if _, err := io.Copy(gz, in); err != nil {
		return err
	}
	return gz.Close()
}

// cleanupOldAccessLogs 删除超过 6 个月的归档 gz 文件（保留 log/ 下的 .sha256 校验值）
func cleanupOldAccessLogs() {
	cutoff := time.Now().Add(-LogRetentionDur)
	entries, err := os.ReadDir(LogBackupDirName)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// 匹配 <category>-<日期>-<小时>.log.gz
		base := strings.TrimSuffix(name, ".log.gz")
		if !strings.Contains(base, "-") || !strings.HasSuffix(name, ".log.gz") {
			continue
		}
		// 提取日期段：category-YYYY-MM-DD-HH → 去掉 category- 前缀与 -HH 后缀
		rest := strings.TrimPrefix(base, "requests-")
		if rest == base {
			rest = strings.TrimPrefix(base, "database-")
		}
		dateStr := rest
		if idx := strings.LastIndex(rest, "-"); idx > 0 {
			dateStr = rest[:idx]
		}
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			_ = os.Remove(filepath.Join(LogBackupDirName, name))
		}
	}
}

// StartLogArchiver 启动日志归档调度器（每小时检查一次，归档已过期的日志）
func StartLogArchiver() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			_ = ArchiveAccessLogs()
		}
	}()
}
