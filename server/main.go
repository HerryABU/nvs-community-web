package main

import (
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"nvs-server/config"
	"nvs-server/handlers"
	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	config.Init()

	// 确保数据目录存在
	os.MkdirAll(config.NovelDataDir, 0755)
	os.MkdirAll(config.UploadDir, 0755)

	// 根据驱动连接数据库
	var dialector gorm.Dialector
	switch config.DBDriver {
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			config.DBUser, config.DBPassword, config.DBHost, config.DBPort, config.DBName)
		dialector = mysql.Open(dsn)
		log.Println("使用 MySQL 数据库")
	default:
		dbPath := "data/nvs.db"
		os.MkdirAll("data", 0755)
		dialector = sqlite.Open(dbPath)
		log.Printf("使用 SQLite 数据库: %s", dbPath)
	}

	var err error
	models.DB, err = gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}

	if config.DBDriver == "mysql" {
		sqlDB, _ := models.DB.DB()
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}

	if err := models.DB.AutoMigrate(
		&models.User{},
		&models.Novel{},
		&models.NovelCategory{},
		&models.Chapter{},
		&models.Comment{},
		&models.Rating{},
		&models.Forum{},
		&models.Thread{},
		&models.Post{},
		&models.VipApplication{},
		&models.Report{},
		&models.WithdrawalRequest{},
		&models.EarningsRecord{},
		&models.BlacklistIP{},
		&models.PlatformConfig{},
		&models.FederatedSite{},
		&models.FederatedNovel{},
		&models.BookShelf{},
	); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	models.InitDefaultForums()
	models.InitPlatformConfigs()
	initDefaultAdmin()
	log.Println("数据库迁移完成")

	// 设置 SMTP 配置提供者（从平台配置读取）
	utils.SMTPConfigProvider = func() utils.SMTPConfig {
		host, port, user, pass, from := models.GetSMTPConfigFromDB()
		return utils.SMTPConfig{
			Host: host, Port: port, User: user, Password: pass, From: from,
		}
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// 健康检查：浏览器 → HTML 页面，API 请求 → JSON
	r.GET("/health", func(c *gin.Context) {
		if strings.Contains(c.GetHeader("Accept"), "application/json") {
			c.JSON(http.StatusOK, gin.H{
				"status":  "ok",
				"service": "nvs-server",
				"version": "1.0.0",
			})
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(`<!DOCTYPE html>
<html lang="zh-CN">
<head><meta charset="UTF-8"><title>NVS 服务状态</title>
<style>
  *{margin:0;padding:0;box-sizing:border-box}
  body{font-family:system-ui,sans-serif;background:#f0f2f5;display:flex;justify-content:center;align-items:center;min-height:100vh}
  .card{background:#fff;border-radius:12px;padding:48px;text-align:center;box-shadow:0 2px 20px rgba(0,0,0,.08);max-width:420px}
  .dot{display:inline-block;width:14px;height:14px;border-radius:50%;background:#22c55e;margin-right:8px;animation:pulse 2s infinite}
  @keyframes pulse{0%,100%{opacity:1}50%{opacity:.5}}
  h1{font-size:1.5rem;color:#1a1a2e;margin-bottom:12px}
  p{color:#666;line-height:1.7}
  .tag{display:inline-block;background:#e8f0ff;color:#3366cc;padding:2px 10px;border-radius:20px;font-size:.85rem;margin:4px}
</style></head>
<body>
<div class="card">
  <h1><span class="dot"></span>NVS 服务运行中</h1>
  <p>网络小说平台 · 节点正常</p>
  <p><span class="tag">版本 1.0.0</span></p>
  <p style="margin-top:16px;font-size:.85rem">联邦社区 · 内容即文件 · 哈希可验证</p>
</div>
</body></html>`))
	})

	// ==================== API 路由 ====================

	public := r.Group("/api")
	{
		public.POST("/auth/register", handlers.Register)
		public.POST("/auth/login", security.RateLimit(5, 60), handlers.Login)
		public.POST("/auth/logout", handlers.Logout)
		public.POST("/auth/send-code", handlers.SendVerificationCode)
		public.POST("/auth/verify-code", handlers.VerifyEmailCode)
		public.GET("/categories", handlers.ListCategories)
		public.GET("/categories/stats", handlers.ListCategoryStats)
		public.GET("/wall-zone/:zone", handlers.GetPublicZoneDetail)
		public.GET("/novels", security.RateLimit(100, 60), handlers.ListNovels)
		public.GET("/novels/:id", handlers.GetNovel)
		public.GET("/novels/:id/chapters", handlers.GetChapters)
		public.GET("/novels/:id/chapters/:num", handlers.GetChapterContent)
		public.GET("/novels/:id/chapters/:num/verify", handlers.VerifyChapter)
		public.GET("/comments", handlers.GetComments)
		public.GET("/novels/:id/rating", handlers.GetNovelRating)
		public.GET("/forums", handlers.ListForums)
		public.GET("/forums/:id", handlers.GetForum)
		public.GET("/novels/:id/forum", handlers.GetNovelForum)
		public.GET("/threads/:id", handlers.GetThread)
		public.GET("/author/profile/:id", handlers.GetAuthorProfile)
		public.GET("/author/forum/:id", handlers.GetAuthorForum)
	}

	protected := r.Group("/api")
	protected.Use(middleware.AuthRequired())
	{
		protected.GET("/auth/me", handlers.GetCurrentUser)
		protected.POST("/comments", security.RateLimit(20, 60), handlers.CreateComment)
		protected.DELETE("/comments/:id", handlers.DeleteComment)
		protected.POST("/novels", handlers.CreateNovel)
		protected.PUT("/novels/:id", handlers.UpdateNovel)
		protected.DELETE("/novels/:id", handlers.DeleteNovel)
		protected.POST("/novels/:id/chapters", handlers.CreateChapter)
		protected.PUT("/novels/:id/chapters/:num", handlers.UpdateChapter)
		protected.DELETE("/novels/:id/chapters/:num", handlers.DeleteChapter)
		protected.GET("/author/novels", handlers.GetMyNovels)
		protected.GET("/author/novels/:id/stats", handlers.GetNovelStats)
		protected.GET("/author/dashboard", handlers.GetAuthorDashboard)
		protected.POST("/novels/:id/export", handlers.ExportNovel)
		protected.POST("/ratings", handlers.UpsertRating)
		protected.GET("/ratings", handlers.GetUserRating)
		protected.POST("/forums/:id/threads", handlers.CreateThread)
		protected.POST("/threads/:id/posts", handlers.CreatePost)
		// VIP & 导入导出
		protected.POST("/author/apply-vip", handlers.ApplyVip)
		protected.POST("/novels/import/preview", handlers.ImportPreview)
		protected.POST("/novels/import", handlers.ImportNovel)
		protected.POST("/novels/:id/export/epub", handlers.ExportEPUB)
		protected.POST("/novels/:id/export/markdown", handlers.ExportMarkdown)
		protected.POST("/novels/:id/export/txt", handlers.ExportTXT)
		// 举报
		protected.POST("/reports", handlers.CreateReport)
		// 收益
		protected.GET("/author/earnings", handlers.GetEarnings)
		protected.POST("/author/withdraw", handlers.RequestWithdraw)
	}

	// 书架路由
	bookshelf := r.Group("/api/bookshelf")
	bookshelf.Use(middleware.AuthRequired())
	{
		bookshelf.GET("", handlers.ListShelf)
		bookshelf.POST("", handlers.AddToShelf)
		bookshelf.DELETE("/:id", handlers.RemoveFromShelf)
		bookshelf.GET("/check/:id", handlers.CheckShelf)
		bookshelf.POST("/progress", handlers.UpdateShelfProgress)
	}

	// 管理员路由
	admin := r.Group("/api/admin")
	admin.Use(middleware.AuthRequired())
	{
		admin.GET("/stats", handlers.GetAdminStats)
		admin.GET("/dashboard", handlers.GetDashboardStats)
		admin.GET("/users", handlers.ListUsers)
		admin.PUT("/users/:id", handlers.UpdateUser)
		admin.GET("/vip-applications", handlers.ListVipApplications)
		admin.POST("/vip-applications/:id/approve", handlers.ApproveVip)
		admin.GET("/reports", handlers.ListReports)
		admin.POST("/reports/:id/handle", handlers.HandleReport)
		admin.GET("/finance", handlers.GetFinanceOverview)
		// 站长配置
		admin.GET("/config", handlers.GetPlatformConfigs)
		admin.PUT("/config", handlers.UpdatePlatformConfig)
		// 隔离墙配置
		admin.GET("/wall-config", handlers.GetWallConfig)
		admin.PUT("/wall-config", handlers.UpdateWallConfig)
		// 远程站点互通
		admin.GET("/sites", handlers.ListFederatedSites)
		admin.POST("/sites", handlers.CreateFederatedSite)
		admin.PUT("/sites/:id", handlers.UpdateFederatedSite)
		admin.DELETE("/sites/:id", handlers.DeleteFederatedSite)
		admin.POST("/sites/:id/sync", handlers.SyncFederatedSite)
		// 论坛管理
		admin.GET("/forums", handlers.AdminListForums)
		admin.POST("/forums", handlers.AdminCreateForum)
		admin.PUT("/forums/:id", handlers.AdminUpdateForum)
		admin.DELETE("/forums/:id", handlers.AdminDeleteForum)
	}

	// 远程站点公开接口
	{
		r.GET("/api/federated/novels", handlers.ListFederatedNovels)
		r.GET("/api/federated/sites", handlers.ListPublicSites)
		r.GET("/api/site-info", handlers.GetSiteInfo)
	}

	// ==================== 静态文件服务 ====================

	// 小说正文静态文件（文件系统存储）
	r.StaticFS("/novels", gin.Dir(config.NovelDataDir, false))

	// 上传资源静态文件
	r.StaticFS("/uploads", gin.Dir(config.UploadDir, false))

	// 嵌入的前端 (SPA)
	frontendFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		log.Fatalf("前端资源加载失败（dist 目录不存在，请先运行 npm run build）: %v", err)
	}

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// SPA 路由请求（Vue Router），检查是否匹配到前端静态资源
		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		// 尝试打开请求的文件（嵌入式文件系统中路径不带前导 /）
		f, err := frontendFS.Open(cleanPath)
		if err == nil {
			f.Close()
			// 文件存在，使用标准 FileServer 服务（自动处理 MIME、缓存等）
			http.FileServer(http.FS(frontendFS)).ServeHTTP(c.Writer, c.Request)
			return
		}

		// 文件不存在 → SPA fallback：返回 index.html
		indexContent, err := fs.ReadFile(frontendFS, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "前端资源缺失")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexContent)
	})

	// ==================== 启动服务器 ====================

	// 构建监听地址
	var listenAddr string
	if config.EnableIPv6 {
		// IPv6 双栈：[::] 在 Windows 上需要 syscall 设置才能同时接受 IPv4
		listenAddr = fmt.Sprintf("[%s]:%s", config.BindIPv6, config.ServerPort)
	} else {
		listenAddr = fmt.Sprintf("%s:%s", config.BindIPv4, config.ServerPort)
	}

	log.Println("========================================")
	log.Printf("  NVS Server v1.0.0")
	log.Printf("  监听地址: %s", listenAddr)
	log.Println("========================================")

	// 列出所有可访问的 URL
	listAccessURLs(config.ServerPort)

	// 尝试配置 Windows 防火墙（静默进行，失败不影响启动）
	configureFirewall(config.ServerPort)

	log.Println("前端已内嵌，无需额外启动 Nginx 或 npm run dev")

	if err := r.Run(listenAddr); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}

// listAccessURLs 扫描所有网卡 IP 并输出可访问的 URL
func listAccessURLs(port string) {
	ips := collectNonLoopbackIPs()
	if len(ips) == 0 {
		return
	}

	log.Println("----------------------------------------")
	log.Println("  可访问地址：")
	log.Printf("    本机:   http://localhost:%s", port)
	log.Printf("    本机:   http://127.0.0.1:%s", port)
	for _, ip := range ips {
		isV6 := strings.Contains(ip, ":")
		if isV6 {
			log.Printf("    局域网: http://[%s]:%s", ip, port)
		} else {
			log.Printf("    局域网: http://%s:%s", ip, port)
		}
	}
	log.Println("----------------------------------------")
}

// collectNonLoopbackIPs 收集所有非回环的网卡 IP 地址
func collectNonLoopbackIPs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	var ips []string
	for _, iface := range ifaces {
		// 跳过未启用的接口
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		// 跳过回环接口
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			// 优先私有地址
			if ip.To4() != nil && ip.IsPrivate() {
				ips = append(ips, ip.String())
			} else if ip.IsGlobalUnicast() {
				ips = append(ips, ip.String())
			}
		}
	}
	return ips
}

// configureFirewall 尝试在 Windows 防火墙上为 nvs-server.exe 开放端口
func configureFirewall(port string) {
	if runtime.GOOS != "windows" {
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		return
	}

	ruleName := "NVS Server (nvs-server.exe)"

	// 检查规则是否已存在
	checkCmd := exec.Command("netsh", "advfirewall", "firewall", "show", "rule",
		"name="+ruleName)
	if err := checkCmd.Run(); err == nil {
		return // 规则已存在，跳过
	}

	// 尝试添加入站规则（需要管理员权限）
	addCmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+ruleName,
		"dir=in",
		"action=allow",
		"program="+exePath,
		"protocol=TCP",
		"localport="+port,
		"enable=yes",
		"profile=any",
	)

	output, err := addCmd.CombinedOutput()
	if err != nil {
		// 没有管理员权限，给出友好提示
		log.Println("----------------------------------------")
		log.Println("  ⚠ 未能自动配置 Windows 防火墙（可能需要管理员权限）")
		log.Println("  请以管理员身份运行以下命令以允许外部访问：")
		log.Printf("    netsh advfirewall firewall add rule name=\"NVS Server\" dir=in action=allow program=\"%s\" protocol=TCP localport=%s enable=yes profile=any", exePath, port)
		log.Println("----------------------------------------")
		return
	}
	_ = output
	log.Println("[firewall] Windows 防火墙规则已添加")
}

// initDefaultAdmin 如果数据库中没有管理员，自动创建默认管理员账号
func initDefaultAdmin() {
	var count int64
	models.DB.Model(&models.User{}).Where("role = ?", "admin").Count(&count)
	if count > 0 {
		return
	}

	hash, err := utils.HashPassword("admin123")
	if err != nil {
		log.Println("警告: 默认管理员密码哈希失败")
		return
	}

	admin := &models.User{
		Username:     "admin",
		Email:        "admin@nvs.local",
		PasswordHash: hash,
		Nickname:     "管理员",
		Role:         "admin",
	}

	if err := models.DB.Create(admin).Error; err != nil {
		log.Printf("警告: 默认管理员创建失败: %v", err)
		return
	}

	log.Println("========================================")
	log.Println("  默认管理员账号已创建")
	log.Println("  邮箱: admin@nvs.local")
	log.Println("  密码: admin123")
	log.Println("  请登录后立即修改密码！")
	log.Println("========================================")
}
