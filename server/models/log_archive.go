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
// 访问日志作为执法证据，需防篡改：按天滚动、每日 sha256 校验、gzip 归档、
// 只读权限（0444）、异地目录备份（backups/logs/）、保留至少 6 个月。

const (
	LogDirName       = "logs"             // 访问日志目录（与 middleware.LogDir 一致）
	LogBackupDirName = "backups/logs"     // 归档备份目录
	LogRetentionDur  = 180 * 24 * time.Hour // 保留 6 个月
)

var logArchiveMu sync.Mutex

// ArchiveAccessLogs 归档除今天外的所有访问日志。
// 每个日志文件：计算 sha256（存 logs/{date}.sha256）→ gzip 归档到 backups/logs/ → 原文件改只读。
// 并清理超过 6 个月的归档（保留 sha256 校验值以便追溯）。
func ArchiveAccessLogs() error {
	logArchiveMu.Lock()
	defer logArchiveMu.Unlock()

	today := time.Now().Format("2006-01-02")
	entries, err := os.ReadDir(LogDirName)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(LogBackupDirName, 0755)

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "access-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		date := strings.TrimSuffix(strings.TrimPrefix(name, "access-"), ".log")
		if date >= today {
			continue // 今天的日志仍在写入，跳过
		}
		gzPath := filepath.Join(LogBackupDirName, "access-"+date+".log.gz")
		if _, err := os.Stat(gzPath); err == nil {
			continue // 已归档
		}

		logPath := filepath.Join(LogDirName, name)
		sum, err := fileSHA256(logPath)
		if err != nil {
			continue
		}
		shaPath := filepath.Join(LogDirName, date+".sha256")
		_ = os.WriteFile(shaPath, []byte(sum+"  "+name+"\n"), 0644)

		if err := gzipFile(logPath, gzPath); err != nil {
			continue
		}
		// 只读权限，防篡改
		_ = os.Chmod(logPath, 0444)
		_ = os.Chmod(shaPath, 0444)
	}

	cleanupOldAccessLogs()
	return nil
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

// cleanupOldAccessLogs 删除超过 6 个月的归档 gz 文件（保留 logs/ 下的 .sha256 校验值）
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
		if !strings.HasPrefix(name, "access-") || !strings.HasSuffix(name, ".log.gz") {
			continue
		}
		dateStr := strings.TrimSuffix(strings.TrimPrefix(name, "access-"), ".log.gz")
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
