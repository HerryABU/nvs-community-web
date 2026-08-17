package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"nvs-server/config"
)

// ==================== 多数据库备份 ====================
// SQLite：使用 VACUUM INTO 生成无损快照文件（.db）
// MySQL：导出全表 JSON 逻辑快照（.json）
// 备份文件统一存放到 data/backups/，支持手动触发与定时调度。

const BackupDir = "data/backups"

// BackupFile 备份文件元信息
type BackupFile struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	Driver  string    `json:"driver"`
}

var backupMu sync.Mutex

func ensureBackupDir() error {
	return os.MkdirAll(BackupDir, 0755)
}

// BackupSQLiteFile 使用 VACUUM INTO 生成 SQLite 快照（无损、原子、最快）
func BackupSQLiteFile(destPath string) error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	// 路径为受控的时间戳文件名，转义单引号防注入
	escaped := strings.ReplaceAll(destPath, "'", "''")
	if _, err := sqlDB.Exec(fmt.Sprintf("VACUUM INTO '%s'", escaped)); err != nil {
		return fmt.Errorf("VACUUM INTO 失败: %w", err)
	}
	return nil
}

// BackupMySQLJSON 导出所有表为 JSON 逻辑快照
func BackupMySQLJSON(destPath string) (int, error) {
	tables, err := DB.Migrator().GetTables()
	if err != nil {
		return 0, err
	}
	snapshot := map[string]interface{}{
		"meta": map[string]interface{}{
			"driver":     "mysql",
			"created_at": time.Now().Format(time.RFC3339),
			"version":    "1.0",
		},
		"data": map[string]interface{}{},
	}
	data := snapshot["data"].(map[string]interface{})
	total := 0
	for _, t := range tables {
		var rows []map[string]interface{}
		if err := DB.Table(t).Find(&rows).Error; err != nil {
			return total, fmt.Errorf("导出表 %s 失败: %w", t, err)
		}
		data[t] = normalizeRowsForJSON(rows)
		total += len(rows)
	}
	f, err := os.Create(destPath)
	if err != nil {
		return total, err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(snapshot); err != nil {
		return total, err
	}
	return total, nil
}

// normalizeRowsForJSON 将 []byte/time.Time 转为可 JSON 序列化的安全类型
func normalizeRowsForJSON(rows []map[string]interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, len(rows))
	for i, row := range rows {
		m := make(map[string]interface{}, len(row))
		for k, v := range row {
			switch val := v.(type) {
			case []byte:
				m[k] = string(val)
			case time.Time:
				m[k] = val.Format(time.RFC3339Nano)
			default:
				m[k] = val
			}
		}
		out[i] = m
	}
	return out
}

// DoBackup 根据当前驱动执行一次备份，返回备份文件绝对路径与行数
func DoBackup() (string, int, error) {
	if DB == nil {
		return "", 0, fmt.Errorf("数据库未初始化")
	}
	backupMu.Lock()
	defer backupMu.Unlock()

	if err := ensureBackupDir(); err != nil {
		return "", 0, err
	}
	ts := time.Now().Format("20060102_150405")
	var path string
	var rows int
	var err error

	switch config.DBDriver {
	case "mysql":
		path = filepath.Join(BackupDir, "backup_"+ts+".json")
		rows, err = BackupMySQLJSON(path)
	default:
		path = filepath.Join(BackupDir, "backup_"+ts+".db")
		err = BackupSQLiteFile(path)
	}
	if err != nil {
		return "", 0, err
	}
	_ = SetPlatformConfig("backup_last_at", time.Now().Format(time.RFC3339))
	return path, rows, nil
}

// ListBackups 列出所有备份文件（按时间倒序）
func ListBackups() ([]BackupFile, error) {
	if err := ensureBackupDir(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(BackupDir)
	if err != nil {
		return nil, err
	}
	files := make([]BackupFile, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		info, err := e.Info()
		if err != nil {
			continue
		}
		driver := "sqlite"
		if strings.HasSuffix(name, ".json") {
			driver = "mysql"
		}
		files = append(files, BackupFile{Name: name, Size: info.Size(), ModTime: info.ModTime(), Driver: driver})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].ModTime.After(files[j].ModTime) })
	return files, nil
}

// BackupPath 返回备份文件的安全绝对路径（校验文件名防目录穿越）
func BackupPath(name string) (string, error) {
	// 仅允许文件名，禁止路径分隔符与上级目录
	if name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return "", fmt.Errorf("非法文件名")
	}
	return filepath.Join(BackupDir, name), nil
}

// DeleteBackup 删除指定备份文件
func DeleteBackup(name string) error {
	p, err := BackupPath(name)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

// ==================== 定时备份调度器 ====================

var schedulerOnce sync.Once

// StartBackupScheduler 启动后台定时备份（每分钟检查一次是否到点）
func StartBackupScheduler() {
	schedulerOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(1 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				runScheduledBackupIfDue()
			}
		}()
	})
}

func runScheduledBackupIfDue() {
	if DB == nil {
		return
	}
	if GetPlatformConfig("backup_enabled") != "true" {
		return
	}
	intervalHours := 24
	if v := GetPlatformConfig("backup_interval_hours"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			intervalHours = n
		}
	}
	if last := GetPlatformConfig("backup_last_at"); last != "" {
		if t, err := time.Parse(time.RFC3339, last); err == nil {
			if time.Since(t) < time.Duration(intervalHours)*time.Hour {
				return // 未到下次备份时间
			}
		}
	}
	if _, _, err := DoBackup(); err != nil {
		// 调度失败静默记录，不影响服务
		fmt.Printf("[backup] 定时备份失败: %v\n", err)
	}
}
