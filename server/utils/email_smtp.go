package utils

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// ============================================================
// SMTP 配置与发送基础设施
// ============================================================

// SMTPConfig SMTP 配置
type SMTPConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
	// InsecureSkipVerify 是否跳过 TLS 证书校验。
	// 默认 false（校验证书，安全）；内网/自签名证书的 SMTP 服务器可开启。
	InsecureSkipVerify bool
}

// tlsConfigFor 构建 SMTP TLS 配置（ServerName 校验始终开启；跳过证书校验按配置）。
func tlsConfigFor(cfg SMTPConfig) *tls.Config {
	return &tls.Config{
		ServerName:         cfg.Host,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.InsecureSkipVerify, // 仅当管理员显式开启时跳过证书校验
	}
}

// SMTPConfigProvider SMTP 配置提供者（由外部设置以避免循环依赖）
var SMTPConfigProvider func() SMTPConfig

// getSMTPConfig 获取 SMTP 配置：先查 provider（平台配置），再查环境变量
func getSMTPConfig() SMTPConfig {
	if SMTPConfigProvider != nil {
		cfg := SMTPConfigProvider()
		if cfg.Host != "" && cfg.User != "" {
			return cfg
		}
	}
	// 环境变量 fallback
	return SMTPConfig{
		Host:     getEnvOr("SMTP_HOST", "smtp.qq.com"),
		Port:     getEnvOr("SMTP_PORT", "587"),
		User:     os.Getenv("SMTP_USER"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     getEnvOr("SMTP_FROM", os.Getenv("SMTP_USER")),
	}
}

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// SendVerificationEmail 发送验证邮件
func SendVerificationEmail(to, code string) error {
	cfg := getSMTPConfig()
	return sendEmail(cfg, to, "【星海文学】邮箱验证码", fmt.Sprintf(verificationEmailTemplate, code))
}

// SendPasswordResetEmail 发送密码重置邮件
func SendPasswordResetEmail(to, code string) error {
	cfg := getSMTPConfig()
	return sendEmail(cfg, to, "【星海文学】密码重置", fmt.Sprintf(passwordResetEmailTemplate, code))
}

// smtpDialTimeout SMTP 连接超时，避免长时间阻塞。
// 默认 smtp.SendMail 无 Dial 超时，连不上时会卡到系统 TCP 超时（约 20s）才返回 500。
const smtpDialTimeout = 10 * time.Second

// loginAuth 实现 AUTH LOGIN 认证（部分企业邮箱/自建服务器仅支持 LOGIN）。
type loginAuth struct {
	username, password string
}

func (a *loginAuth) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch strings.TrimSpace(strings.ToLower(string(fromServer))) {
		case "username:":
			return []byte(a.username), nil
		case "password:":
			return []byte(a.password), nil
		}
	}
	return nil, nil
}

// smtpAuth 按服务器支持的机制做认证回退：PLAIN → LOGIN → CRAM-MD5。
// PLAIN 在 TLS 加密连接下最通用（163/QQ/Outlook 等主流均支持）；
// LOGIN 兼容企业邮箱/自建服务器；CRAM-MD5 不传输明文密码。
// 服务器未声明 AUTH 扩展（或无认证需求）时跳过认证，返回 nil。
// 返回的错误包含每个机制的失败详情，便于诊断。
func smtpAuth(client *smtp.Client, cfg SMTPConfig) error {
	supported, params := client.Extension("AUTH")
	mechs := strings.ToUpper(params) // 形如 "PLAIN LOGIN CRAM-MD5"

	if !supported || mechs == "" {
		return nil // 服务器无 AUTH 扩展：跳过认证（内网/免认证服务器）
	}

	var errs []string
	tryAuth := func(name string, a smtp.Auth) bool {
		if err := client.Auth(a); err == nil {
			return true
		} else {
			errs = append(errs, fmt.Sprintf("%s:%v", name, err))
			return false
		}
	}

	// 1) PLAIN：最通用（未加密连接会被 Go smtp 自动拒绝，此时继续尝试 LOGIN）
	if strings.Contains(mechs, "PLAIN") && tryAuth("PLAIN", smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)) {
		return nil
	}
	// 2) LOGIN：企业邮箱常见
	if strings.Contains(mechs, "LOGIN") && tryAuth("LOGIN", &loginAuth{username: cfg.User, password: cfg.Password}) {
		return nil
	}
	// 3) CRAM-MD5：不传输明文密码
	if strings.Contains(mechs, "CRAM-MD5") && tryAuth("CRAM-MD5", smtp.CRAMMD5Auth(cfg.User, cfg.Password)) {
		return nil
	}

	return fmt.Errorf("SMTP 认证失败（服务器机制: %s；尝试详情: %s），请检查账号与授权码", mechs, strings.Join(errs, "；"))
}

// smtpProtocol 根据端口判定协议模式：
//   - "ssl"      ：465 隐式 TLS（SMTPS），连接伊始即加密
//   - "starttls" ：587/2525/25 等，先明文 EHLO 再 STARTTLS 升级（必须成功，不降级明文）
func smtpProtocol(port string) string {
	if port == "465" {
		return "ssl"
	}
	return "starttls"
}

// sendEmail 发送邮件的底层函数。
// 协议按端口区分方法：465 = 隐式 TLS（SMTPS）；其余（587/2525/25）= STARTTLS 升级。
// 带连接超时，失败快速返回清晰错误。
func sendEmail(cfg SMTPConfig, to, subject, body string) error {
	if cfg.User == "" || cfg.Password == "" {
		return fmt.Errorf("SMTP 未配置，请联系站长")
	}

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	dialer := &net.Dialer{Timeout: smtpDialTimeout}
	proto := smtpProtocol(cfg.Port)

	var conn net.Conn
	var err error
	if proto == "ssl" {
		// 465：隐式 TLS（SMTPS），连接伊始即走 TLS
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsConfigFor(cfg))
	} else {
		// 587/25：先明文 TCP 连接
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("连接 SMTP 服务器失败（%s, %s）：%w", addr, proto, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return fmt.Errorf("SMTP 握手失败：%w", err)
	}
	defer client.Quit()

	// STARTTLS 模式：必须成功升级加密（163 等要求 TLS 才能认证），不支持则明确报错、不降级明文
	if proto == "starttls" {
		ok, _ := client.Extension("STARTTLS")
		if !ok {
			return fmt.Errorf("服务器 %s 不支持 STARTTLS，但端口 %s 需要加密发送（请改用 465 SSL 或确认服务器支持 STARTTLS）", cfg.Host, cfg.Port)
		}
		if err = client.StartTLS(tlsConfigFor(cfg)); err != nil {
			return fmt.Errorf("STARTTLS 升级失败（端口 %s）：%w", cfg.Port, err)
		}
	}

	// 认证：按服务器支持机制回退（PLAIN / LOGIN / CRAM-MD5）
	if err = smtpAuth(client, cfg); err != nil {
		return fmt.Errorf("SMTP 认证失败（账号或授权码错误）：%w", err)
	}
	if err = client.Mail(cfg.From); err != nil {
		return fmt.Errorf("发件人错误：%w", err)
	}
	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("收件人错误：%w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("写入邮件体失败：%w", err)
	}
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		cfg.From, to, subject, body)
	if _, err = writer.Write([]byte(msg)); err != nil {
		return fmt.Errorf("写入邮件内容失败：%w", err)
	}
	if err = writer.Close(); err != nil {
		return fmt.Errorf("邮件发送失败：%w", err)
	}
	return nil
}

// ============================================================
// SMTP 逐阶段诊断（用于后台「测试发送」按钮，快速定位配置问题）
// ============================================================

// SMTPDiagStep 单个诊断步骤结果
type SMTPDiagStep struct {
	Step   string `json:"step"`   // 步骤标识：config/dns/connect/hello/starttls/auth/mail/data
	OK     bool   `json:"ok"`     // 是否成功
	Ms     int64  `json:"ms"`     // 耗时毫秒
	Detail string `json:"detail"` // 成功说明或错误信息（不含密码）
}

func diagOK(step, detail string, start time.Time) SMTPDiagStep {
	return SMTPDiagStep{Step: step, OK: true, Ms: time.Since(start).Milliseconds(), Detail: detail}
}
func diagFail(step string, err error, start time.Time) SMTPDiagStep {
	return SMTPDiagStep{Step: step, OK: false, Ms: time.Since(start).Milliseconds(), Detail: err.Error()}
}

// DiagnoseSMTP 对 SMTP 配置做逐阶段诊断，并尝试发送一封测试邮件到 to。
// 每一步独立计时、独立记录成败，返回步骤列表与总体是否成功。
// 密码绝不出现在任何 Detail 中。
func DiagnoseSMTP(cfg SMTPConfig, to string) ([]SMTPDiagStep, bool) {
	steps := []SMTPDiagStep{}
	allOK := true

	// 1) 配置完整性
	{
		start := time.Now()
		if cfg.Host == "" || cfg.User == "" || cfg.Password == "" {
			allOK = false
			steps = append(steps, diagFail("config", fmt.Errorf("SMTP 配置不完整：host/user/password 均不能为空"), start))
			return steps, allOK
		}
		if cfg.Port == "" {
			cfg.Port = "587"
		}
		if cfg.From == "" {
			cfg.From = cfg.User
		}
		steps = append(steps, diagOK("config",
			fmt.Sprintf("配置有效：%s:%s（发件人 %s）", cfg.Host, cfg.Port, cfg.From), start))
	}

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)

	// 2) DNS 解析（host 为 IP 时跳过）
	{
		start := time.Now()
		if ip := net.ParseIP(cfg.Host); ip != nil {
			steps = append(steps, diagOK("dns", fmt.Sprintf("主机是 IP 地址 %s，跳过解析", cfg.Host), start))
		} else {
			addrs, err := net.LookupHost(cfg.Host)
			if err != nil || len(addrs) == 0 {
				allOK = false
				steps = append(steps, diagFail("dns", fmt.Errorf("DNS 解析失败：%v", err), start))
				return steps, allOK
			}
			steps = append(steps, diagOK("dns", fmt.Sprintf("解析到 %d 个地址：%s", len(addrs), strings.Join(addrs, ", ")), start))
		}
	}

	// 3) TCP 连接（协议按端口区分：465=隐式 TLS；其他=STARTTLS）
	dialer := &net.Dialer{Timeout: smtpDialTimeout}
	proto := smtpProtocol(cfg.Port)
	start := time.Now()
	var conn net.Conn
	var err error
	if proto == "ssl" {
		tlsConf := tlsConfigFor(cfg)
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsConf)
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		allOK = false
		steps = append(steps, diagFail("connect", fmt.Errorf("连接 %s 失败（协议:%s）：%w", addr, proto, err), start))
		return steps, allOK
	}
	stepConn := diagOK("connect", fmt.Sprintf("TCP 连接 %s 成功（协议:%s）", addr, proto), start)
	if proto == "ssl" {
		stepConn.Detail += "（隐式 TLS 握手完成）"
	}
	steps = append(steps, stepConn)

	// 4) SMTP 握手（读取服务器横幅）
	start = time.Now()
	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		allOK = false
		conn.Close()
		steps = append(steps, diagFail("hello", fmt.Errorf("SMTP 握手失败：%w", err), start))
		return steps, allOK
	}
	defer client.Quit()
	steps = append(steps, diagOK("hello", "SMTP 握手成功（服务器已响应）", start))

	// 5) TLS 升级（STARTTLS 模式必须成功；不支持则明确报错，不降级明文）
	if proto == "starttls" {
		start = time.Now()
		hasStartTLS, _ := client.Extension("STARTTLS")
		if !hasStartTLS {
			allOK = false
			steps = append(steps, diagFail("starttls", fmt.Errorf("服务器 %s 不支持 STARTTLS，端口 %s 无法加密发送（请改用 465 SSL 端口）", cfg.Host, cfg.Port), start))
			return steps, allOK
		}
		if err = client.StartTLS(tlsConfigFor(cfg)); err != nil {
			allOK = false
			steps = append(steps, diagFail("starttls", fmt.Errorf("STARTTLS 升级失败（端口 %s）：%w", cfg.Port, err), start))
			return steps, allOK
		}
		steps = append(steps, diagOK("starttls", "STARTTLS 升级成功（连接已加密）", start))
	}

	// 6) 认证（按服务器支持机制回退：CRAM-MD5 / LOGIN / PLAIN）
	start = time.Now()
	if err = smtpAuth(client, cfg); err != nil {
		allOK = false
		steps = append(steps, diagFail("auth", fmt.Errorf("SMTP 认证失败（请检查账号与授权码）：%w", err), start))
		return steps, allOK
	}
	steps = append(steps, diagOK("auth", fmt.Sprintf("认证成功（账号 %s）", cfg.User), start))

	// 7) 发件人 / 收件人
	start = time.Now()
	if err = client.Mail(cfg.From); err != nil {
		allOK = false
		steps = append(steps, diagFail("mail", fmt.Errorf("发件人被拒绝（%s）：%w", cfg.From, err), start))
		return steps, allOK
	}
	if err = client.Rcpt(to); err != nil {
		allOK = false
		steps = append(steps, diagFail("mail", fmt.Errorf("收件人被拒绝（%s）：%w", to, err), start))
		return steps, allOK
	}
	steps = append(steps, diagOK("mail", fmt.Sprintf("发件人 %s → 收件人 %s 均被接受", cfg.From, to), start))

	// 8) 写入邮件体并发送
	start = time.Now()
	writer, err := client.Data()
	if err != nil {
		allOK = false
		steps = append(steps, diagFail("data", fmt.Errorf("进入发送态失败：%w", err), start))
		return steps, allOK
	}
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		cfg.From, to, "【星海文学】SMTP 配置测试",
		"<h3>SMTP 配置测试</h3><p>如果您收到这封邮件，说明 SMTP 配置一切正常，可以正常发送验证码。</p><p>发送时间："+time.Now().Format("2006-01-02 15:04:05")+"</p>")
	if _, err = writer.Write([]byte(msg)); err != nil {
		allOK = false
		steps = append(steps, diagFail("data", fmt.Errorf("写入邮件内容失败：%w", err), start))
		return steps, allOK
	}
	if err = writer.Close(); err != nil {
		allOK = false
		steps = append(steps, diagFail("data", fmt.Errorf("邮件发送失败：%w", err), start))
		return steps, allOK
	}
	steps = append(steps, diagOK("data", fmt.Sprintf("测试邮件已成功发送到 %s（请查收）", to), start))

	return steps, allOK
}
