package siteops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"nvs-server/config"
	"nvs-server/models"
)

// ==================== 多数据库备份 ====================
// SQLite：使用 VACUUM INTO 生成无损快照文件（backup_sqlite_*.db）
// MySQL ：调用本机 mysqldump 生成原生 SQL 转储（backup_mysql_*.sql）
// PostgreSQL：调用本机 pg_dump 生成原生 SQL 转储（backup_postgres_*.sql）
// 备份文件统一存放到 data/backups/，支持手动触发与定时调度。
// 恢复：SQLite 直接覆盖数据文件后重连；MySQL/PostgreSQL 通过 mysql/psql 回放原生转储。
// 原生转储（而非逻辑 JSON）可正确还原类型、JSON 列与 PostgreSQL 序列，恢复最可靠。

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
	sqlDB, err := models.DB.DB()
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

// BackupMySQLDump 调用本机 mysqldump 生成 MySQL 原生 SQL 转储（可正确还原类型/JSON 列）。
// 密码通过 MYSQL_PWD 环境变量传入，避免出现在进程参数（ps）中。
func BackupMySQLDump(destPath string) error {
	bin, err := exec.LookPath("mysqldump")
	if err != nil {
		return fmt.Errorf("未找到 mysqldump：请在服务器安装 MySQL 客户端（如 apt install mysql-client / brew install mysql-client），再执行备份")
	}
	host := config.DBHost
	if host == "" {
		host = "127.0.0.1"
	}
	port := config.DBPort
	if port == "" {
		port = "3306"
	}
	args := []string{
		"-h", host, "-P", port, "-u", config.DBUser,
		"--single-transaction", "--no-tablespaces", "--skip-lock-tables", "--routines",
		config.DBName,
	}
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+config.DBPassword)
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()
	var stderr strings.Builder
	cmd.Stdout = out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = os.Remove(destPath)
		return fmt.Errorf("mysqldump 失败: %v；%s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// BackupPostgresDump 调用本机 pg_dump 生成 PostgreSQL 原生 SQL 转储（含 --clean --if-exists，可覆盖恢复）。
// 密码通过 PGPASSWORD 环境变量传入，避免出现在进程参数中。
func BackupPostgresDump(destPath string) error {
	bin, err := exec.LookPath("pg_dump")
	if err != nil {
		return fmt.Errorf("未找到 pg_dump：请在服务器安装 PostgreSQL 客户端（如 apt install postgresql-client），再执行备份")
	}
	host := config.DBHost
	if host == "" {
		host = "127.0.0.1"
	}
	port := config.DBPort
	if port == "" {
		port = "5432"
	}
	sslmode := config.DBSSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	conn := fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=%s",
		host, port, config.DBUser, config.DBName, sslmode)
	cmd := exec.Command(bin, "--clean", "--if-exists", "-f", destPath, conn)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+config.DBPassword)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = os.Remove(destPath)
		return fmt.Errorf("pg_dump 失败: %v；%s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// RestoreMySQLDump 通过本机 mysql 客户端回放 MySQL 原生转储（覆盖当前数据库）。
func RestoreMySQLDump(dumpPath string) error {
	bin, err := exec.LookPath("mysql")
	if err != nil {
		return fmt.Errorf("未找到 mysql 客户端：请在服务器安装 MySQL 客户端后再恢复")
	}
	host := config.DBHost
	if host == "" {
		host = "127.0.0.1"
	}
	port := config.DBPort
	if port == "" {
		port = "3306"
	}
	args := []string{"-h", host, "-P", port, "-u", config.DBUser, config.DBName}
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+config.DBPassword)
	f, err := os.Open(dumpPath)
	if err != nil {
		return err
	}
	defer f.Close()
	cmd.Stdin = f
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysql 导入失败: %v；%s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// RestorePostgresDump 通过本机 psql 回放 PostgreSQL 原生转储（--clean 转储可覆盖恢复）。
func RestorePostgresDump(dumpPath string) error {
	bin, err := exec.LookPath("psql")
	if err != nil {
		return fmt.Errorf("未找到 psql 客户端：请在服务器安装 PostgreSQL 客户端后再恢复")
	}
	host := config.DBHost
	if host == "" {
		host = "127.0.0.1"
	}
	port := config.DBPort
	if port == "" {
		port = "5432"
	}
	sslmode := config.DBSSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	conn := fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=%s",
		host, port, config.DBUser, config.DBName, sslmode)
	cmd := exec.Command(bin, conn, "-f", dumpPath)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+config.DBPassword)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("psql 导入失败: %v；%s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// DoBackup 根据当前驱动执行一次备份，返回备份文件绝对路径。
func DoBackup() (string, int, error) {
	if models.DB == nil {
		return "", 0, fmt.Errorf("数据库未初始化")
	}
	backupMu.Lock()
	defer backupMu.Unlock()

	if err := ensureBackupDir(); err != nil {
		return "", 0, err
	}
	ts := time.Now().Format("20060102_150405")
	var path string
	var err error

	switch config.DBDriver {
	case "mysql":
		path = filepath.Join(BackupDir, "backup_mysql_"+ts+".sql")
		err = BackupMySQLDump(path)
	case "postgres":
		path = filepath.Join(BackupDir, "backup_postgres_"+ts+".sql")
		err = BackupPostgresDump(path)
	default: // sqlite
		path = filepath.Join(BackupDir, "backup_sqlite_"+ts+".db")
		err = BackupSQLiteFile(path)
	}
	if err != nil {
		return "", 0, err
	}
	_ = models.SetPlatformConfig("backup_last_at", time.Now().Format(time.RFC3339))
	return path, 0, nil
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
		files = append(files, BackupFile{Name: name, Size: info.Size(), ModTime: info.ModTime(), Driver: driverFromName(name)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].ModTime.After(files[j].ModTime) })
	return files, nil
}

// driverFromName 从备份文件名前缀推导数据库驱动（新命名含驱动前缀，旧文件按扩展名兜底）。
func driverFromName(name string) string {
	switch {
	case strings.HasPrefix(name, "backup_postgres_"):
		return "postgres"
	case strings.HasPrefix(name, "backup_mysql_"):
		return "mysql"
	case strings.HasPrefix(name, "backup_sqlite_"):
		return "sqlite"
	case strings.HasSuffix(name, ".db"):
		return "sqlite"
	case strings.HasSuffix(name, ".json"):
		return "mysql" // 遗留 JSON 逻辑快照
	default:
		return "unknown"
	}
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
	if models.DB == nil {
		return
	}
	if models.GetPlatformConfig("backup_enabled") != "true" {
		return
	}
	intervalHours := 24
	if v := models.GetPlatformConfig("backup_interval_hours"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			intervalHours = n
		}
	}
	if last := models.GetPlatformConfig("backup_last_at"); last != "" {
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
