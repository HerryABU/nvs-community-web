package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"nvs-server/handlers/aimedia"
	"nvs-server/utils/infra"
	"nvs-server/utils/secure"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"nvs-server/aisafety"
	"nvs-server/chaptervcs"
	"nvs-server/collab"
	"nvs-server/config"
	"nvs-server/crypto"
	"nvs-server/gate"
	"nvs-server/gmcmd"
	"nvs-server/groups"
	"nvs-server/handlers/badword"
	moderationhandler "nvs-server/handlers/moderation"
	"nvs-server/honeypot"
	"nvs-server/middleware"
	"nvs-server/models"
	_ "nvs-server/models/authorai" // 触发 authorai_projects 模型注册（作者工作台 AI 写作流，与中台分库）
	_ "nvs-server/models/softdel"  // 触发迁移辅助 / 留证钩子注册
	"nvs-server/moderation"
	"nvs-server/oc"
	"nvs-server/police"
	"nvs-server/prompttpl"
	_ "nvs-server/readperm" // 触发 content_purchases 模型注册（#65 付费解锁）
	"nvs-server/realtime"
	"nvs-server/risk"
	"nvs-server/routes"
	"nvs-server/scriptmode"
	"nvs-server/security"
	"nvs-server/security/sms"
	sentimentstore "nvs-server/sentiment/store"
	"nvs-server/sentiment/updater"
	"nvs-server/supervise"
	"nvs-server/tlsconfig"
	"nvs-server/version"
	_ "nvs-server/vipcard" // 触发 vip_cards 注册 + 覆盖 VipLevelOf 多等级钩子（#66）

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	aim "nvs-server/models/ai"
	aihub "nvs-server/models/aihub"
	aiops "nvs-server/models/aiops"
	apikey "nvs-server/models/apikey"
	siteops "nvs-server/models/siteops"
)

func main() {
	// 确保工作目录为 exe 所在目录（双击启动时 Windows 工作目录可能不是 exe 目录）
	if exePath, err := os.Executable(); err == nil {
		if dir := filepath.Dir(exePath); dir != "" {
			os.Chdir(dir)
		}
	}

	// CLI 只读命令（--version / --help）：不依赖配置与数据库，打印完直接退出。
	// 放在 config.Init() 之前，这样「问个版本」不会被环境问题绊倒。
	if handled, code := runReadOnlyCLI(os.Args[1:]); handled {
		os.Exit(code)
	}

	config.Init()

	// CLI 子命令（不启动 HTTP 服务）：gm-audit / credential-age
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		os.Exit(gmcmd.Run(os.Args[1:]))
	}

	// 国密（SM）双轨配置校验（fail-fast：轨道取值 / 实现路径 / 双证书四件套）
	if err := config.CryptoCfg.Validate(); err != nil {
		log.Fatalf("国密配置校验失败: %v", err)
	}
	// 握手观测（默认关闭；配置 NVS_TLS_HANDSHAKE_LOG 后启用 JSONL 落盘，N-04 仅本地）
	if err := tlsconfig.Configure(config.TLSHandshakeLogPath); err != nil {
		log.Fatalf("握手观测配置失败: %v", err)
	}

	// 确保数据目录存在
	os.MkdirAll(config.NovelDataDir, 0755)
	os.MkdirAll(config.UploadDir, 0755)
	os.MkdirAll(config.UserFrameDir, 0755)
	os.MkdirAll(config.UserHTMLDir, 0755)

	// CLI 维护命令（--pwd）：需要连库改密码，改完直接退出，**不启动 HTTP 服务**。
	// 放在数据目录创建之后 —— initDatabase() 要往 DB_PATH 所在目录写库文件。
	if handled, code := runMaintenanceCLI(os.Args[1:]); handled {
		os.Exit(code)
	}

	// 未完成建站向导 → 进入安装模式（不连接数据库，门禁中间件拦截业务 API）
	// 已完成安装 → 连接数据库 + 迁移 + 初始化默认数据
	if config.IsInstalled() {
		if err := initDatabase(); err != nil {
			log.Fatalf("数据库初始化失败: %v", err)
		}
	} else {
		setupKey := config.GetOrCreateSetupKey()
		log.Println("========================================")
		log.Println("  首次运行：进入建站向导模式（尚未连接数据库）")
		log.Printf("  向导地址: http://localhost:%s/install", config.ServerPort)
		log.Printf("  安装密钥（一次性，仅显示一次，切勿泄露）: %s", setupKey)
		log.Println("========================================")
	}

	// 设置 SMTP 配置提供者（从平台配置读取）
	infra.SMTPConfigProvider = func() infra.SMTPConfig {
		host, port, user, pass, from := models.GetSMTPConfigFromDB()
		portInt := 587
		if p, err := strconv.Atoi(port); err == nil {
			portInt = p
		}
		return infra.SMTPConfig{
			Host: host, Port: fmt.Sprintf("%d", portInt), User: user, Password: pass, From: from,
			// 兼容自签名/内网 SMTP：管理员在后台显式开启才跳过证书校验（默认校验证书，安全）
			InsecureSkipVerify: models.GetPlatformConfig("smtp_insecure_skip_verify") == "true",
		}
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// 全部中间件与路由注册都在 routes 包里（原 main.go 的 160-1447 行原样搬出）
	// 前端资源来自 //go:embed，仅 main 包可见，注入给 routes 使用
	routes.FrontendFS = getFrontendFS
	routes.NodistBuild = nodistBuild
	routes.Register(r)

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
	log.Printf("  NVS Server v%s", version.AppVersion)
	log.Printf("  监听地址: %s", listenAddr)
	log.Println("========================================")

	// 列出所有可访问的 URL
	listAccessURLs(config.ServerPort)

	// 尝试配置 Windows 防火墙（静默进行，失败不影响启动）
	configureFirewall(config.ServerPort)

	// 启动定时备份调度器（按平台配置 interval 自动备份）
	siteops.StartBackupScheduler()

	// 启动访问日志归档调度器（每日 sha256 + gzip + 只读 + 6个月保留，176号令合规）
	siteops.StartLogArchiver()

	// 🔀 站点路径前缀（URL_PREFIX）在 gin 外层剥离：必须先于路由匹配执行。
	// 未配置前缀时该包装零开销直通。
	rootHandler := middleware.StripURLPrefixHandler(config.URLPrefix, r)
	srv := &http.Server{Addr: listenAddr, Handler: rootHandler}
	log.Println("前端已内嵌，无需额外启动 Nginx 或 npm run dev（支持路径前缀挂载，如 /go-nvs/login）")

	// 🔐 国密双轨监听装配：国际轨默认（零国密依赖）；gm/dual 轨经 crypto.Provider
	// 叠加 Tongsuo（CGO）或纯 Go gmtls 的 TLCP 能力（未配证书时保持纯 HTTP 现状）。
	listener, err := buildServiceListener(listenAddr)
	if err != nil {
		log.Fatalf("监听器构建失败: %v", err)
	}
	if err := srv.Serve(listener); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}

// buildServiceListener 构建服务监听器：按密码轨道叠加 TLS/TLCP 能力。
func buildServiceListener(listenAddr string) (net.Listener, error) {
	inner, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, err
	}
	provider, err := crypto.New(config.CryptoCfg)
	if err != nil {
		_ = inner.Close()
		return nil, err
	}
	ln, err := provider.BuildListener(inner, config.CryptoCfg)
	if err != nil {
		_ = inner.Close()
		return nil, err
	}
	if ln == net.Listener(inner) {
		log.Printf("  TLS: 未启用（纯 HTTP；反向代理终止 TLS 或未配置证书）")
	} else {
		log.Printf("  TLS: 已启用（provider=%s, track=%s）", provider.Name(), config.CryptoCfg.Track)
	}
	return ln, nil
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

// initDatabase 连接数据库、执行迁移、初始化默认数据与默认管理员
func initDatabase() error {
	var dialector gorm.Dialector
	switch config.DBDriver {
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			config.DBUser, config.DBPassword, config.DBHost, config.DBPort, config.DBName)
		dialector = mysql.Open(dsn)
		log.Println("使用 MySQL 数据库")
	case "sqlite":
		dbPath := config.DBPath
		if dbPath == "" {
			dbPath = "data/nvs.db"
		}
		if config.DBMemory {
			dbPath = ":memory:"
			log.Println("使用 SQLite 内存数据库（重启数据丢失）")
		} else {
			os.MkdirAll(filepath.Dir(dbPath), 0755)
			log.Printf("使用 SQLite 数据库: %s", dbPath)
		}
		dialector = sqlite.Open(dbPath)
	default:
		os.MkdirAll("data", 0755)
		dialector = sqlite.Open("data/nvs.db")
		log.Println("使用 SQLite 数据库: data/nvs.db")
	}

	var err error
	models.DB, err = gorm.Open(dialector, &gorm.Config{
		Logger: infra.NewDBLogger(),
	})
	if err != nil {
		return err
	}

	if config.DBDriver == "mysql" {
		sqlDB, _ := models.DB.DB()
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(time.Hour)
		// MySQL 模式 → 自动启用 Redis 缓存
		infra.ConnectRedis(config.RedisHost, config.RedisPort, config.RedisPassword, config.RedisDB)
	}

	if config.DBDriver == "sqlite" && config.DBMemory {
		log.Println("[SQLite] 内存模式：验证码和限流使用进程内存储")
	}

	// 短信网关（可插拔：log/http）与手机验证码存储后端（Redis/内存，随 ConnectRedis 决定）
	sms.Init()
	log.Printf("[sms] 验证码存储后端: %s", infra.PhoneCodeBackend())

	if err := models.AutoMigrateAll(models.DB); err != nil {
		return err
	}
	// AIGC 标识：挂载站长自定义角标字体（没有就用内置 Noto Sans SC 子集）
	aimedia.InitAigcFont()
	// 群组模块（独立功能包）建表：紧随主迁移，避免 models 包反向依赖子包
	if err := groups.Migrate(models.DB); err != nil {
		return err
	}
	// 提示词模板引擎（独立功能包）：建表 + 类型文学默认框架种子（幂等）
	if err := prompttpl.Migrate(models.DB); err != nil {
		return err
	}
	prompttpl.InitSeeds()
	// 多人协作（独立功能包）：协作成员 + 协作审计
	if err := collab.Migrate(models.DB); err != nil {
		return err
	}
	// 章节版本控制（独立功能包）：提交历史 + 分支指针
	if err := chaptervcs.Migrate(models.DB); err != nil {
		return err
	}
	// OC 模式（独立功能包）：角色卡 / 世界书 / 群内 AI 角色 / 对话流 / 审核窗口 / 预算
	if err := oc.Migrate(models.DB); err != nil {
		return err
	}
	// 蜜罐（独立功能包）：等级与日志策略 + 会话/事件/指纹
	// 设 NVS_HONEYPOT_DB_PATH 则使用独立库（与业务库物理隔离，后台仍可读）
	if err := honeypot.Bootstrap(models.DB); err != nil {
		return err
	}
	// 注册门槛（邀请码/设备指纹/账号年龄）+ 充值风控 + 警方工单 + OC 表情
	if err := gate.Migrate(models.DB); err != nil {
		return err
	}
	if err := risk.Migrate(models.DB); err != nil {
		return err
	}
	if err := police.Migrate(models.DB); err != nil {
		return err
	}
	// AI 内容审核（独立功能包）：审核记录 / 配置 / 恶意举报者 / 隐藏内容
	if err := moderation.Migrate(models.DB); err != nil {
		return err
	}
	// AI 对话内容安全闸：配置 / 用户画像 / 事件 / 蓝队判定日志
	if err := aisafety.Migrate(models.DB); err != nil {
		return err
	}
	// 以疏代堵的「名单」子系统：可疑 / 恶意名单 + 操作日志（绥靖模式对接警方取证）
	if err := supervise.Migrate(models.DB); err != nil {
		return err
	}
	// 社区舆情守卫（话题驱动条件拦截 + 词表 + 命中记录）
	sentimentstore.SetDB(models.DB)
	if err := sentimentstore.Migrate(models.DB); err != nil {
		return err
	}
	// 每日词表自动更新（默认关闭，站长在后台开启后才会真正请求上游）
	updater.Start()
	// 对话分支：把老数据（parent_id 全 0）串成链并补活跃叶子。
	// 不做的话升级后打开旧对话只能看到最后一条 —— 数据没丢但用户以为丢了。
	aihub.BackfillMessageParents()
	// 审核调度器（定时批量审核论坛/评论区），失败不阻断启动
	moderation.Init()
	// 「AI 判违规自动下架」接到软删留证（moderation 包不依赖 handlers，靠回调注入）
	moderationhandler.RegisterModerationHooks()
	if err := oc.MigrateEmote(models.DB); err != nil {
		return err
	}
	// 自定义表情注册到渲染器（气泡由服务端渲染，需全局表；失败不影响启动）
	oc.ReloadCustomEmotes()
	scriptmode.RegisterCustomEmotes(oc.CustomEmoteMap())
	// 版本控制后端选择：NVS_VCS_BACKEND=git 时启用真实 git 仓库（需 -tags gitvcs 构建），
	// 失败或不可用自动回落自研版本链，不阻断启动
	chaptervcs.Init()
	models.InitDefaultForums()
	models.InitPlatformConfigs()
	aim.SeedDefaultAiAgents() // AI 智能体内置种子（表为空时写入）
	aiops.InitAiPrompts()     // AI 提示词库平台预设（幂等）
	aiops.SeedAiVendors()     // AI 厂商字典（幂等）
	aihub.SeedAiStyles()      // 作品风格字典（幂等）
	// 管理端配置（DB）覆盖 .env 初始值：微信扫码凭据热更新进 config 包级变量
	// （sms 网关由 resolveGateway 每次发送前读 DB，无需在此处理）
	if wxID, wxSecret, wxCallback, fromDB := models.GetWeChatConfigFromDB(); fromDB {
		config.WeChatAppID = wxID
		config.WeChatAppSecret = wxSecret
		config.WeChatCallback = wxCallback
		log.Println("[wechat] 已从数据库恢复管理端保存的微信扫码配置")
	}
	// 限流与 API Key：连库后重载管理员配置（含自定义爬虫规则），并清理过期访问日志
	security.ReloadBotRules()
	apikey.CleanupAccessLogs(apikey.LoadAPIRateLimitConfig().LogRetentionDays)
	// 日志脱敏：注入数据库持久化钩子并加载已保存的配置（后台从未保存过则使用 .env 默认）
	config.RedactMgr.SetPersistence(models.LoadRedactConfig, models.StoreRedactConfig)
	config.RedactMgr.LoadFromDB()
	moderationhandler.ReloadXssRules() // 加载管理员配置的 XSS 正则规则
	badword.ReloadBadWords()           // 加载管理员配置的违禁词库规则
	initDefaultAdmin()
	// 老站点升级：把「按模型存的密钥」并入「渠道卡密池」。
	// 幂等（按 base_url + 密钥指纹判重），只增不改，老配置继续作为兜底。
	if n, err := aihub.MigrateModelKeysToChannels(); err != nil {
		log.Printf("[ai] 渠道卡密池迁移失败（不影响启动）：%v", err)
	} else if n > 0 {
		log.Printf("[ai] 渠道卡密池迁移完成：新增 %d 把卡密", n)
	}
	log.Println("数据库迁移完成")
	// 狂暴模式传输层装配：按 RageConfig 拉起/关停 MQTT、gRPC（保存即生效；此处为启动态装配）
	realtime.GetManager().ApplyConfig(siteops.GetRageConfig())
	return nil
}

// initDefaultAdmin 如果数据库中没有管理员，自动创建默认管理员账号
func initDefaultAdmin() {
	var count int64
	models.DB.Model(&models.User{}).Where("role = ?", "admin").Count(&count)
	if count > 0 {
		return
	}

	hash, err := secure.HashPassword("admin123")
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
