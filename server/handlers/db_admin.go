package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 数据库管理（健康检查 / 迁移 / 备份） ====================

var serverStartTime = time.Now()

// ---------- 后台任务（迁移/备份异步执行 + 轮询进度） ----------

type dbJob struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"` // migrate | backup | restore
	Status      string                 `json:"status"` // running | done | error
	Message     string                 `json:"message"`
	Table       string                 `json:"table"`
	TablesDone  int                    `json:"tables_done"`
	TablesTotal int                    `json:"tables_total"`
	Rows        int64                  `json:"rows"`
	Detail      map[string]interface{} `json:"detail"`
	StartedAt   time.Time              `json:"started_at"`
	FinishedAt  *time.Time             `json:"finished_at"`
}

var (
	jobsMu sync.Mutex
	jobs   = map[string]*dbJob{}
)

func newJob(typ string) *dbJob {
	id := randomJobID()
	job := &dbJob{ID: id, Type: typ, Status: "running", StartedAt: time.Now(), Detail: map[string]interface{}{}}
	jobsMu.Lock()
	jobs[id] = job
	jobsMu.Unlock()
	return job
}

func finishJob(job *dbJob, status, message string) {
	jobsMu.Lock()
	job.Status = status
	job.Message = message
	now := time.Now()
	job.FinishedAt = &now
	jobsMu.Unlock()
}

func getJob(id string) *dbJob {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	return jobs[id]
}

func randomJobID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// writeDBEnv 将数据库配置持久化到「加密」配置（迁移/切换后重启仍生效，密码不落明文 .env）
func writeDBEnv(cfg models.DBConfig) error {
	sc := config.SecureDBConfig{
		Driver:   cfg.Driver,
		Host:     cfg.Host,
		Port:     cfg.Port,
		User:     cfg.User,
		Password: cfg.Password,
		Name:     cfg.Name,
		Path:     cfg.Path,
	}
	if sc.Driver == "sqlite" && sc.Path == "" {
		sc.Path = "data/nvs.db"
	}
	return config.PersistSecureConfig(sc)
}

// ---------- 健康检查（管理员面板） ----------

// GET /api/admin/db/health — 详细健康检查
func AdminDBHealth(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}

	dbStatus := "disconnected"
	dbDriver := "unknown"
	var tableCount int
	if models.DB != nil {
		dbDriver = models.DB.Dialector.Name()
		if sqlDB, err := models.DB.DB(); err == nil && sqlDB != nil {
			if err := sqlDB.Ping(); err == nil {
				dbStatus = "ok"
			} else {
				dbStatus = "error: " + err.Error()
			}
		}
		if tables, err := models.DB.Migrator().GetTables(); err == nil {
			tableCount = len(tables)
		}
	}

	redisStatus := "disabled"
	if utils.RedisClient != nil {
		if _, err := utils.RedisClient.Ping(context.Background()).Result(); err == nil {
			redisStatus = "ok"
		} else {
			redisStatus = "error: " + err.Error()
		}
	}

	utils.Success(c, gin.H{
		"status":       dbStatus,
		"driver":       dbDriver,
		"redis":        redisStatus,
		"uptime_sec":   int64(time.Since(serverStartTime).Seconds()),
		"version":      "1.0.0",
		"table_count":  tableCount,
		"goroutines":   runtime.NumGoroutine(),
		"installed":    config.IsInstalled(),
		"backup_count": backupCount(),
		"now":          time.Now().Format(time.RFC3339),
	})
}

func backupCount() int {
	files, err := models.ListBackups()
	if err != nil {
		return 0
	}
	return len(files)
}

// GET /api/admin/db/current — 当前数据库配置摘要（密码脱敏）
func AdminDBCurrent(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	driver := config.DBDriver
	host := config.DBHost
	port := config.DBPort
	user := config.DBUser
	name := config.DBName
	if driver == "sqlite" {
		host, port, user, name = "", "", "", ""
	}
	utils.Success(c, gin.H{
		"driver": driver,
		"host":   host,
		"port":   port,
		"user":   user,
		"name":   name,
	})
}

// ---------- 数据库迁移（SQLite ↔ MySQL，MySQL → MySQL） ----------

// POST /api/admin/db/migrate — 启动异步迁移任务，返回 job id
func AdminDBMigrate(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req models.DBConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误")
		return
	}
	req.Driver = strings.ToLower(strings.TrimSpace(req.Driver))
	if req.Driver != "sqlite" && req.Driver != "mysql" {
		utils.BadRequest(c, "目标驱动仅支持 sqlite 或 mysql")
		return
	}
	if req.Driver == "mysql" && req.Name == "" {
		utils.BadRequest(c, "MySQL 数据库名不能为空")
		return
	}

	job := newJob("migrate")
	go runMigrationJob(job, req)
	utils.Success(c, gin.H{"job_id": job.ID})
}

func runMigrationJob(job *dbJob, cfg models.DBConfig) {
	// 1. 打开目标库并建表
	target, err := models.OpenDB(cfg)
	if err != nil {
		finishJob(job, "error", "目标数据库连接失败: "+err.Error())
		return
	}
	if err := models.AutoMigrateAll(target); err != nil {
		finishJob(job, "error", "目标数据库建表失败: "+err.Error())
		return
	}

	// 2. 获取表总数用于进度
	src := models.DB
	tables, _ := src.Migrator().GetTables()
	job.TablesTotal = len(tables)
	done := 0
	var totalRows int64

	result, err := models.CopyAllTables(src, target, func(t string, rows int64) {
		done++
		totalRows += rows
		jobsMu.Lock()
		job.Table = t
		job.TablesDone = done
		job.Rows = totalRows
		jobsMu.Unlock()
	})
	if err != nil {
		finishJob(job, "error", err.Error())
		return
	}

	// 3. 持久化 .env
	if err := writeDBEnv(cfg); err != nil {
		finishJob(job, "error", "数据已复制，但写入 .env 失败: "+err.Error())
		return
	}

	// 4. 热切换全局数据库连接
	if err := models.ReconnectDatabase(cfg); err != nil {
		finishJob(job, "error", "数据已复制，但切换连接失败: "+err.Error())
		return
	}

	// 5. MySQL 模式重连 Redis，SQLite 模式禁用 Redis
	if cfg.Driver == "mysql" {
		utils.ConnectRedis(config.RedisHost, config.RedisPort, config.RedisPassword, config.RedisDB)
	} else {
		utils.RedisClient = nil
	}

	finishJob(job, "done", "迁移完成，已切换到 "+cfg.Driver)
	job.Detail = map[string]interface{}{"tables": result, "total_rows": totalRows}
}

// ---------- 备份（手动 / 定时 / 列表 / 下载 / 删除 / 恢复） ----------

// POST /api/admin/db/backup — 立即备份（异步）
func AdminDBBackup(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	job := newJob("backup")
	go func() {
		path, rows, err := models.DoBackup()
		if err != nil {
			finishJob(job, "error", "备份失败: "+err.Error())
			return
		}
		finishJob(job, "done", "备份完成")
		job.Detail = map[string]interface{}{"path": path, "rows": rows}
	}()
	utils.Success(c, gin.H{"job_id": job.ID})
}

// GET /api/admin/db/job/:id — 查询后台任务进度
func AdminDBJob(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	job := getJob(c.Param("id"))
	if job == nil {
		utils.NotFound(c, "任务不存在")
		return
	}
	utils.Success(c, job)
}

// GET /api/admin/db/backups — 备份列表
func AdminDBListBackups(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	files, err := models.ListBackups()
	if err != nil {
		utils.InternalError(c, "读取备份列表失败: "+err.Error())
		return
	}
	utils.Success(c, files)
}

// DELETE /api/admin/db/backups/:name — 删除备份
func AdminDBDeleteBackup(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	name := c.Param("name")
	if err := models.DeleteBackup(name); err != nil {
		utils.InternalError(c, "删除失败: "+err.Error())
		return
	}
	utils.SuccessMessage(c, "已删除", nil)
}

// GET /api/admin/db/backups/:name/download — 下载备份
func AdminDBDownloadBackup(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	p, err := models.BackupPath(c.Param("name"))
	if err != nil {
		utils.BadRequest(c, "非法文件名")
		return
	}
	if _, err := os.Stat(p); err != nil {
		utils.NotFound(c, "备份不存在")
		return
	}
	c.FileAttachment(p, filepath.Base(p))
}

// POST /api/admin/db/restore — 恢复 SQLite 备份（仅 sqlite 驱动，替换 .db 后重连）
func AdminDBRestore(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		utils.BadRequest(c, "请指定备份文件名")
		return
	}
	if config.DBDriver != "sqlite" {
		utils.BadRequest(c, "当前为 MySQL，仅支持恢复 SQLite 备份文件")
		return
	}
	p, err := models.BackupPath(req.Name)
	if err != nil {
		utils.BadRequest(c, "非法文件名")
		return
	}
	if _, err := os.Stat(p); err != nil {
		utils.NotFound(c, "备份不存在")
		return
	}

	job := newJob("restore")
	go func() {
		// 1. 关闭当前连接
		if models.DB != nil {
			if sqlDB, e := models.DB.DB(); e == nil && sqlDB != nil {
				_ = sqlDB.Close()
			}
		}
		// 2. 覆盖当前 db 文件
		if err := copyFile(p, "data/nvs.db"); err != nil {
			finishJob(job, "error", "恢复失败: "+err.Error())
			return
		}
		// 3. 重连
		if err := models.ReconnectDatabase(models.DBConfig{Driver: "sqlite", Path: "data/nvs.db"}); err != nil {
			finishJob(job, "error", "恢复后重连失败: "+err.Error())
			return
		}
		finishJob(job, "done", "恢复完成")
	}()
	utils.Success(c, gin.H{"job_id": job.ID})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// ---------- 定时备份配置 ----------

// GET /api/admin/db/backup-config
func AdminDBBackupConfigGet(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	enabled := models.GetPlatformConfig("backup_enabled") == "true"
	interval := 24
	if v := models.GetPlatformConfig("backup_interval_hours"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			interval = n
		}
	}
	utils.Success(c, gin.H{
		"enabled":     enabled,
		"interval":    interval,
		"last_backup": models.GetPlatformConfig("backup_last_at"),
	})
}

// PUT /api/admin/db/backup-config
func AdminDBBackupConfigPut(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Enabled  *bool `json:"enabled"`
		Interval *int  `json:"interval"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误")
		return
	}
	if req.Enabled != nil {
		_ = models.SetPlatformConfig("backup_enabled", strconv.FormatBool(*req.Enabled))
	}
	if req.Interval != nil {
		if *req.Interval <= 0 {
			utils.BadRequest(c, "备份间隔必须大于 0 小时")
			return
		}
		_ = models.SetPlatformConfig("backup_interval_hours", strconv.Itoa(*req.Interval))
	}
	utils.SuccessMessage(c, "已保存", nil)
}
