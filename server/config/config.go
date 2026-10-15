package config

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

// ==================== 默认 .env 模板（内嵌） ====================
const defaultEnvTemplate = `# ==================== 数据库 ====================
# DB_DRIVER: sqlite (默认，零配置) | mysql (需额外安装 MySQL)
DB_DRIVER=sqlite
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=nvs_user
DB_PASSWORD=nvs_pass_2026
DB_NAME=nvs
DB_ROOT_PASSWORD=nvs_root_2026

# ==================== Redis (SQLite模式下不需要) ====================
REDIS_HOST=127.0.0.1
REDIS_PORT=6379

# ==================== JWT ====================
JWT_SECRET=change-me-in-production-please-use-a-strong-random-string
JWT_EXPIRE_HOURS=72

# ==================== 文件存储 ====================
NOVEL_DATA_DIR=./data/novels
UPLOAD_DIR=./data/uploads

# ==================== 用户自定义内容 ====================
USER_FRAME_DIR=./data/userframe
USER_HTML_DIR=./data/userhtml

# ==================== 服务端口与网络 ====================
SERVER_PORT=8080

# 绑定地址：IPv4 地址（默认 0.0.0.0 监听所有 IPv4 网卡）
BIND_IPV4=0.0.0.0

# 绑定地址：IPv6 地址（默认 :: 监听所有 IPv6 网卡）
BIND_IPV6=::

# 是否启用 IPv6 双栈监听（true/false）
# 开启后服务器同时监听 IPv4 和 IPv6 地址
ENABLE_IPV6=false

# 本机 IPv4 地址（自动检测，用于对外展示/联邦互通）
LOCAL_IPV4={{AUTO_IPV4}}

# 本机 IPv6 地址（自动检测，用于对外展示/联邦互通）
LOCAL_IPV6={{AUTO_IPV6}}

# ==================== SMTP 邮件配置（用于邮箱验证码） ====================
# 支持 QQ邮箱/Gmail/163 等 SMTP 服务
SMTP_HOST=smtp.qq.com
SMTP_PORT=587
SMTP_USER=your-email@qq.com
SMTP_PASSWORD=your-smtp-auth-code
SMTP_FROM=your-email@qq.com

# ==================== 平台配置 ====================
PLATFORM_FEE_RATE=0.10
SITE_NAME=NVS 类型文学平台
# 邮箱验证功能开关（true/false）
EMAIL_VERIFY_ENABLED=false
# 滑块验证码开关（true/false）
CAPTCHA_ENABLED=false

# ==================== 安全 / 反向代理 ====================
# 信任的反向代理地址（逗号分隔的 IP 或 CIDR，如 127.0.0.1,10.0.0.0/8）。
# 留空 = 不信任任何代理，客户端 IP 取真实 TCP 对端地址（默认，可防 X-Forwarded-For 伪造越权）。
# 仅当部署在反向代理（如 nginx）之后且需记录真实客户端 IP 时，才填写代理地址。
TRUSTED_PROXIES=

# JWT Cookie 是否强制 Secure（仅 HTTPS 下传输，true/false，默认 false）。
# 生产环境启用 HTTPS 后务必设为 true，防止令牌在明文 HTTP 中被截获（安全审计 L3）。
COOKIE_SECURE=false

# 站点路径前缀（URL_PREFIX，默认留空=挂在域名根）。
# 用于反向代理场景：如 nginx 将 /go-nvs/ 前缀转发到本站，则填 /go-nvs，
# 访问 :8088/go-nvs/login 即可正常使用（自动剥离前缀 + 前端路由/API 自适应）。
# 注意：必须以 / 开头且不带尾斜杠，如 /go-nvs；留空 = 无前缀（默认）。
URL_PREFIX=

# 建站向导 / 安装接口是否仅允许本机(localhost)访问（true/false，默认 true）。
# 防止反向代理/公网直连越权进入安装向导；如需从局域网远程初始化可设为 false。
SETUP_LOCAL_ONLY=true

# ==================== 日志敏感信息脱敏（公安部令第176号合规） ====================
# 访问日志中的 Token / 密码 / 敏感字段脱敏开关：
# true  = 生产模式：自动脱敏 Cookie/Authorization 令牌、body/query 敏感字段，合规安全
# false = 开发模式：保留原始数据，方便调试
LOG_REDACT_SENSITIVE=false

# 自定义脱敏掩码字符（默认 ***REDACTED***；密码类字段固定使用 ******）
LOG_REDACT_MASK="***REDACTED***"

# 自定义敏感字段列表（逗号分隔，默认 password,token,authorization,cookie,secret,apikey）
LOG_SENSITIVE_FIELDS="password,token,authorization,cookie,secret,apikey"
`

var (
	// Database
	DBDriver   string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBPath     string // SQLite 文件路径（默认 data/nvs.db）
	DBMemory   bool

	// Redis
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	// JWT
	JWTSecret      string
	JWTExpireHours string
	// JWTRefreshExpireHours 刷新令牌有效期（小时），默认 720（30 天），用于无感续期会话
	JWTRefreshExpireHours string

	// File storage
	NovelDataDir string
	UploadDir    string

	// User custom content
	UserFrameDir string
	UserHTMLDir  string

	// Server
	ServerPort string

	// Network / IPv4 / IPv6
	BindIPv4   string
	BindIPv6   string
	EnableIPv6 bool
	LocalIPv4  string
	LocalIPv6  string

	// Security / reverse proxy
	TrustedProxies []string // 信任的反向代理 IP/CIDR 列表；空=不信任任何代理（默认，防 X-Forwarded-For 伪造越权）
	SetupLocalOnly bool     // 建站向导仅允许本机(localhost)访问（默认 true）

	// 日志敏感信息脱敏（公安部令第176号合规）
	LogRedactSensitive bool     // 生产模式自动脱敏（默认 false = 开发模式保留原始数据）
	LogRedactMask      string   // 自定义脱敏掩码（默认 "***REDACTED***"）
	LogSensitiveFields []string // 自定义敏感字段列表（默认 password,token,authorization,cookie,secret,apikey）

	// JWT Cookie Secure 标志（安全审计 L3：生产 HTTPS 下应开启）
	ForceCookieSecure bool

	// 站点路径前缀（反向代理挂载场景，如 /go-nvs；空 = 域名根）
	URLPrefix string
)

// Init 初始化配置：自动加载/创建 .env，然后读取环境变量
func Init() {
	// 1. 检查 .env 是否存在，不存在则自动创建
	envPath := ".env"
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		createDefaultEnv(envPath)
		log.Printf("[config] .env 文件不存在，已自动创建: %s（请按需修改后重启）", envPath)
	}

	// 2. 加载 .env 文件到环境变量
	if err := loadEnvFile(envPath); err != nil {
		log.Printf("[config] 加载 .env 失败: %v，使用系统环境变量与默认值", err)
	}

	// 3. 从环境变量读取各配置项
	DBDriver = getEnv("DB_DRIVER", "sqlite")
	DBHost = getEnv("DB_HOST", "127.0.0.1")
	DBPort = getEnv("DB_PORT", "3306")
	DBUser = getEnv("DB_USER", "nvs_user")
	DBPassword = getEnv("DB_PASSWORD", "nvs_pass_2026")
	DBName = getEnv("DB_NAME", "nvs")
	DBPath = getEnv("DB_PATH", "data/nvs.db")
	DBMemory = getEnvBool("DB_MEMORY", false)

	// 4. 优先加载加密的数据库配置（覆盖 .env，密码不落明文）
	if sc, err := LoadSecureConfig(); err == nil && sc != nil {
		ApplySecureConfig(sc)
	}

	RedisHost = getEnv("REDIS_HOST", "127.0.0.1")
	RedisPort = getEnv("REDIS_PORT", "6379")
	RedisPassword = getEnv("REDIS_PASSWORD", "")
	RedisDB = getEnvInt("REDIS_DB", 0)

	JWTSecret = getEnv("JWT_SECRET", "change-me-in-production")
	JWTExpireHours = getEnv("JWT_EXPIRE_HOURS", "72")
	JWTRefreshExpireHours = getEnv("JWT_REFRESH_EXPIRE_HOURS", "720")
	// 🔒 安全审计 S1：默认 JWT_SECRET 为公开值，可被伪造任意管理员 Token。
	// 使用默认值时自动替换为随机密钥并持久化（data/.jwt_secret，0600），自定义值保持不变。
	if JWTSecret == "change-me-in-production" || JWTSecret == "change-me-in-production-please-use-a-strong-random-string" {
		if s, err := GetOrCreateJWTSecret(); err == nil && s != "" {
			JWTSecret = s
			log.Printf("[config] 已使用随机 JWT 签名密钥（data/.jwt_secret）——避免公开默认密钥被利用")
		}
	}

	NovelDataDir = getEnv("NOVEL_DATA_DIR", "./data/novels")
	UploadDir = getEnv("UPLOAD_DIR", "./data/uploads")

	UserFrameDir = getEnv("USER_FRAME_DIR", "./data/userframe")
	UserHTMLDir = getEnv("USER_HTML_DIR", "./data/userhtml")

	ServerPort = getEnv("SERVER_PORT", "8080")

	// IPv4 / IPv6 网络配置
	BindIPv4 = getEnv("BIND_IPV4", "0.0.0.0")
	BindIPv6 = getEnv("BIND_IPV6", "::")
	EnableIPv6 = getEnvBool("ENABLE_IPV6", false)
	LocalIPv4 = getEnv("LOCAL_IPV4", "")
	LocalIPv6 = getEnv("LOCAL_IPV6", "")

	// 反向代理信任与建站向导本地保护
	TrustedProxies = parseList(getEnv("TRUSTED_PROXIES", ""))
	SetupLocalOnly = getEnvBool("SETUP_LOCAL_ONLY", true)
	ForceCookieSecure = getEnvBool("COOKIE_SECURE", false)

	// 站点路径前缀：规范化（保证 / 开头、去尾斜杠）；留空 = 无前缀
	URLPrefix = normalizePrefix(getEnv("URL_PREFIX", ""))
	if URLPrefix != "" {
		log.Printf("[config] 站点路径前缀: %s（访问形如 %s/login）", URLPrefix, URLPrefix)
	}

	// 日志敏感信息脱敏（176号令合规）：env 提供初始默认；DB 就绪后由 main 注入持久化并覆盖
	LogRedactSensitive = getEnvBool("LOG_REDACT_SENSITIVE", false)
	LogRedactMask = getEnv("LOG_REDACT_MASK", DefaultRedactMask)
	LogSensitiveFields = parseList(getEnv("LOG_SENSITIVE_FIELDS", strings.Join(DefaultSensitiveFields, ",")))
	RedactMgr.SeedFromEnv(RedactConfig{
		Enabled:       LogRedactSensitive,
		Mask:          LogRedactMask,
		SensitiveKeys: LogSensitiveFields,
	})
	log.Printf("[config] 日志脱敏：%s（LOG_REDACT_SENSITIVE=%v, 掩码=%q, 敏感字段=%v）",
		map[bool]string{true: "生产模式-自动脱敏", false: "开发模式-保留原始数据"}[LogRedactSensitive],
		LogRedactSensitive, LogRedactMask, LogSensitiveFields)
}

// createDefaultEnv 基于内嵌模板自动创建 .env 文件，自动检测本机 IP 填入
func createDefaultEnv(path string) {
	content := defaultEnvTemplate

	// 自动检测本机 IP
	ipv4, ipv6 := detectLocalIPs()

	content = strings.ReplaceAll(content, "{{AUTO_IPV4}}", ipv4)
	content = strings.ReplaceAll(content, "{{AUTO_IPV6}}", ipv6)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		log.Printf("[config] 创建 .env 失败: %v", err)
	}
}

// detectLocalIPs 自动检测本机首选 IPv4 和 IPv6 地址
func detectLocalIPs() (ipv4, ipv6 string) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1", "::1"
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipNet.IP

		if ip.IsLoopback() {
			continue
		}
		if ip.IsLinkLocalUnicast() {
			continue
		}

		if ip.To4() != nil {
			// 首选私有地址，其次公有地址
			if ipv4 == "" || ip.IsPrivate() {
				ipv4 = ip.String()
			}
		} else {
			// IPv6：排除 ULA（Unique Local Address）以降低优先级，选全局单播
			if ipv6 == "" || ip.IsGlobalUnicast() {
				ipv6 = ip.String()
			}
		}
	}

	if ipv4 == "" {
		ipv4 = "127.0.0.1"
	}
	if ipv6 == "" {
		ipv6 = "::1"
	}
	return
}

// loadEnvFile 手动解析 .env 文件，将每行 KEY=VALUE 写入环境变量
func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释行
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 分割 KEY=VALUE（只分割第一个等号，值中可能包含 =）
		idx := strings.Index(line, "=")
		if idx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		// 去除引号（支持 VALUE="..." 和 VALUE='...'）
		value = strings.Trim(value, "\"'")

		if key == "" {
			continue
		}

		// 只设置尚未被系统环境变量覆盖的项（.env 优先级低于系统环境变量）
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("line %d: 设置环境变量 %s 失败: %w", lineNo, key, err)
			}
		}
	}

	return scanner.Err()
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}

// parseList 解析逗号分隔的字符串列表，去除空白与空项
func parseList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func getEnvInt(key string, fallback int) int {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		return fallback
	}
	result, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return result
}

func getEnvBool(key string, fallback bool) bool {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		return fallback
	}
	switch strings.ToLower(val) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return fallback
	}
}

// normalizePrefix 规范化站点路径前缀：保证 / 开头、去除尾斜杠；空/非法返回空串。
func normalizePrefix(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimRight(p, "/")
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	// 拒绝形如 //、含 .. 的前缀（防路径混淆）
	if strings.Contains(p, "..") || p == "/" || strings.HasPrefix(p, "//") {
		return ""
	}
	return p
}
