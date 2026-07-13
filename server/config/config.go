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
`

var (
	// Database
	DBDriver   string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBMemory   bool

	// Redis
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	// JWT
	JWTSecret      string
	JWTExpireHours string

	// File storage
	NovelDataDir string
	UploadDir    string

	// Server
	ServerPort string

	// Network / IPv4 / IPv6
	BindIPv4   string
	BindIPv6   string
	EnableIPv6 bool
	LocalIPv4  string
	LocalIPv6  string
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
	DBMemory = getEnvBool("DB_MEMORY", false)

	RedisHost = getEnv("REDIS_HOST", "127.0.0.1")
	RedisPort = getEnv("REDIS_PORT", "6379")
	RedisPassword = getEnv("REDIS_PASSWORD", "")
	RedisDB = getEnvInt("REDIS_DB", 0)

	JWTSecret = getEnv("JWT_SECRET", "change-me-in-production")
	JWTExpireHours = getEnv("JWT_EXPIRE_HOURS", "72")

	NovelDataDir = getEnv("NOVEL_DATA_DIR", "./data/novels")
	UploadDir = getEnv("UPLOAD_DIR", "./data/uploads")

	ServerPort = getEnv("SERVER_PORT", "8080")

	// IPv4 / IPv6 网络配置
	BindIPv4 = getEnv("BIND_IPV4", "0.0.0.0")
	BindIPv6 = getEnv("BIND_IPV6", "::")
	EnableIPv6 = getEnvBool("ENABLE_IPV6", false)
	LocalIPv4 = getEnv("LOCAL_IPV4", "")
	LocalIPv6 = getEnv("LOCAL_IPV6", "")
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