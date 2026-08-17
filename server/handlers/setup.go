package handlers

import (
	"log"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 建站向导（一次性安装，严防死守） ====================
// 设计要点：
//   1. 一次性：安装完成后写入加密配置 data/secure-config.json，向导永久关闭；
//      安装接口受 installMu 互斥锁保护（sync.Once 的并发安全等价实现，且失败可重试），
//      成功后内存原子标志置位，路由守卫 setupGuard 直接 404，连判断文件存在的机会都不给。
//   2. 隔离：安装接口仅在未安装时挂载；业务 API 由门禁中间件在未安装时返回 503。
//   3. 密钥：安装接口仅接受「一次性安装密钥」（只在服务器控制台打印一次）。
//   4. 数据库密码不写回明文 .env，而是 AES-256-GCM 加密后落盘。

// installMu 串行化安装请求，防止并发重复安装
var installMu sync.Mutex

// GET /api/setup/status — 查询安装状态（公开，不泄露密钥）
func SetupStatus(c *gin.Context) {
	utils.Success(c, gin.H{
		"installed":  config.IsInstalled(),
		"db_driver":  config.DBDriver,
		"server_ver": "1.0.0",
	})
}

// GET /api/setup/env-check — 环境检查（进程权限 / 目录读写 / 端口）
func SetupEnvCheck(c *gin.Context) {
	dataWritable := checkDirWritable("data")
	uploadWritable := checkDirWritable(config.UploadDir)

	// 端口占用：检测服务端口是否已被监听（本服务自身）
	portFree := checkPortFree(config.ServerPort)

	utils.Success(c, gin.H{
		"os":               runtime.GOOS,
		"arch":             runtime.GOARCH,
		"data_writable":    dataWritable,
		"upload_writable":  uploadWritable,
		"server_port":      config.ServerPort,
		"server_port_free": portFree,
	})
}

// POST /api/setup/db-test — 测试数据库连通性（Ping）
func SetupDBTest(c *gin.Context) {
	var req struct {
		Database models.DBConfig `json:"database"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误")
		return
	}
	req.Database.Driver = strings.ToLower(strings.TrimSpace(req.Database.Driver))

	db, err := models.OpenDB(req.Database)
	if err != nil {
		utils.Success(c, gin.H{"ok": false, "error": err.Error()})
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		utils.Success(c, gin.H{"ok": false, "error": err.Error()})
		return
	}
	if err := sqlDB.Ping(); err != nil {
		utils.Success(c, gin.H{"ok": false, "error": err.Error()})
		return
	}
	utils.Success(c, gin.H{"ok": true})
}

// POST /api/setup/install — 执行安装（一次性，需安装密钥）
func SetupInstall(c *gin.Context) {
	installMu.Lock()
	defer installMu.Unlock()

	if config.IsInstalled() {
		utils.Forbidden(c, "系统已完成安装，建站向导已永久关闭")
		return
	}

	var req struct {
		SetupKey string          `json:"setup_key"`
		Database models.DBConfig `json:"database"`
		SiteName string          `json:"site_name"`
		Admin    struct {
			Username string `json:"username"`
			Email    string `json:"email"`
			Password string `json:"password"`
		} `json:"admin"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误")
		return
	}

	// 1. 校验一次性安装密钥（严防死守）
	if !config.VerifySetupKey(req.SetupKey) {
		utils.Forbidden(c, "安装密钥错误，请查看服务器控制台输出的安装密钥")
		return
	}

	// 2. 校验数据库配置
	req.Database.Driver = strings.ToLower(strings.TrimSpace(req.Database.Driver))
	if req.Database.Driver != "sqlite" && req.Database.Driver != "mysql" {
		utils.BadRequest(c, "数据库驱动仅支持 sqlite 或 mysql")
		return
	}
	if req.Database.Driver == "mysql" && req.Database.Name == "" {
		utils.BadRequest(c, "MySQL 数据库名不能为空")
		return
	}

	// 3. 校验站点与管理员
	req.SiteName = strings.TrimSpace(req.SiteName)
	if req.SiteName == "" {
		utils.BadRequest(c, "站点名称不能为空")
		return
	}
	req.Admin.Username = strings.TrimSpace(req.Admin.Username)
	req.Admin.Email = strings.TrimSpace(req.Admin.Email)
	if req.Admin.Username == "" || req.Admin.Email == "" || req.Admin.Password == "" {
		utils.BadRequest(c, "管理员账号、邮箱、密码均不能为空")
		return
	}
	if !isStrongPassword(req.Admin.Password) {
		utils.BadRequest(c, "管理员密码强度不足：至少 8 位，且需同时包含字母和数字")
		return
	}

	// 4. 连接目标数据库并建表
	target, err := models.OpenDB(req.Database)
	if err != nil {
		utils.BadRequest(c, "数据库连接失败: "+err.Error())
		return
	}
	if err := models.AutoMigrateAll(target); err != nil {
		utils.InternalError(c, "数据库建表失败: "+err.Error())
		return
	}

	// 5. 切换到目标数据库并初始化默认数据
	models.DB = target
	models.InitDefaultForums()
	models.InitPlatformConfigs()
	_ = models.SetPlatformConfig("site_name", req.SiteName)
	ReloadXssRules()

	// 6. 创建管理员账号（记录，不记录明文密码）
	hash, err := utils.HashPassword(req.Admin.Password)
	if err != nil {
		utils.InternalError(c, "密码加密失败")
		return
	}
	admin := &models.User{
		Username:     req.Admin.Username,
		Email:        req.Admin.Email,
		PasswordHash: hash,
		Nickname:     "管理员",
		Role:         "admin",
	}
	if err := models.DB.Create(admin).Error; err != nil {
		utils.InternalError(c, "创建管理员失败: "+err.Error())
		return
	}
	log.Printf("[setup] 已创建管理员账号 %s (%s)", req.Admin.Username, req.Admin.Email)

	// 7. 持久化数据库配置到「加密」配置文件（不写回明文 .env）
	sc := config.SecureDBConfig{
		Driver:   req.Database.Driver,
		Host:     req.Database.Host,
		Port:     req.Database.Port,
		User:     req.Database.User,
		Password: req.Database.Password,
		Name:     req.Database.Name,
		Path:     req.Database.Path,
	}
	if sc.Driver == "sqlite" && sc.Path == "" {
		sc.Path = "data/nvs.db"
	}
	if err := config.PersistSecureConfig(sc); err != nil {
		utils.InternalError(c, "写入加密配置失败: "+err.Error())
		return
	}

	// MySQL 模式重连 Redis
	if sc.Driver == "mysql" {
		utils.ConnectRedis(config.RedisHost, config.RedisPort, config.RedisPassword, config.RedisDB)
	}

	// 8. 锁死：删除一次性密钥 + 置位安装状态（此后安装路由由守卫返回 404）
	if err := config.MarkInstalled(); err != nil {
		utils.InternalError(c, "写入安装标记失败: "+err.Error())
		return
	}

	log.Printf("[setup] 安装完成：driver=%s site=%s", sc.Driver, req.SiteName)
	utils.SuccessMessage(c, "安装成功，请使用管理员账号登录", gin.H{
		"driver":    sc.Driver,
		"site_name": req.SiteName,
	})
}

// ==================== 工具函数 ====================

// checkDirWritable 检测目录是否可写（创建临时文件后删除）
func checkDirWritable(dir string) bool {
	if dir == "" {
		dir = "data"
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false
	}
	f, err := os.CreateTemp(dir, ".wtest-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	_ = os.Remove(name)
	return true
}

// checkPortFree 检测端口是否空闲（监听失败 = 已被占用）
func checkPortFree(port string) bool {
	ln, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// isStrongPassword 高强度密码：至少 8 位，且同时包含字母和数字
func isStrongPassword(pw string) bool {
	if len(pw) < 8 {
		return false
	}
	var hasLetter, hasDigit bool
	for _, r := range pw {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	return hasLetter && hasDigit
}
