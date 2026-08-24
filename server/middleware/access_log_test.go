package middleware

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"nvs-server/config"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// TestAccessLogPreservesFullBody 回归测试：请求体 >64KB 时，handler 必须能读到完整 body。
// 历史 bug：AccessLog 用 64KB LimitReader 读取后整体还原 → 超长 JSON 被截断成非法数据，
// 导致 CreateChapter/CreateBlog 的 ShouldBindJSON 报「请填写章节标题和内容/参数错误」。
func TestAccessLogPreservesFullBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AccessLog())
	r.POST("/echo", func(c *gin.Context) {
		raw, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(500, gin.H{"err": err.Error()})
			return
		}
		c.JSON(200, gin.H{"len": len(raw)})
	})

	// 构造 300KB 的 JSON body（远超旧的 64KB 截断阈值）
	big := `{"title":"第38章 长文","content":"` + strings.Repeat("这是非常长的章节内容。", 20000) + `"}`
	if len(big) < 64*1024 {
		t.Fatalf("测试数据应大于 64KB，实际 %d", len(big))
	}

	req := httptest.NewRequest("POST", "/echo", strings.NewReader(big))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("handler 应读到完整 body 并返回 200，实际 %d：%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"len":`+strconv.Itoa(len(big))) {
		t.Fatalf("handler 读到的 body 长度应为 %d，实际响应 %s", len(big), w.Body.String())
	}
}

// TestRedactorBodyJSON 生产模式：body JSON 中 password→******、token→可配置掩码；
// 嵌套结构递归脱敏；非 JSON / 非法 JSON 保持不变（F6）。
func TestRedactorBodyJSON(t *testing.T) {
	r := NewRedactor(config.RedactConfig{
		Enabled:       true,
		Mask:          "***REDACTED***",
		SensitiveKeys: []string{"password", "token", "authorization", "cookie", "secret", "apikey"},
	})

	// 需求 3.3.2 示例
	out := r.RedactBody(`{"email":"test@nvs.local","password":"MyP@ssw0rd123","token":"eyJhbGciOiJIUzI1NiJ9.abc"}`)
	for _, secret := range []string{"MyP@ssw0rd123", "eyJhbGciOiJIUzI1NiJ9.abc"} {
		if strings.Contains(out, secret) {
			t.Errorf("敏感值未脱敏 %q: %s", secret, out)
		}
	}
	if !strings.Contains(out, `"password":"******"`) || !strings.Contains(out, `"token":"***REDACTED***"`) {
		t.Errorf("掩码不正确（password 应 ******、token 应 ***REDACTED***）: %s", out)
	}
	if !strings.Contains(out, `"email":"test@nvs.local"`) {
		t.Errorf("非敏感字段应保留: %s", out)
	}

	// 嵌套对象 + 数组递归脱敏
	nested := r.RedactBody(`{"user":{"password":"p1","nickname":"n"},"list":[{"token":"t1"},{"secret":"s1"}]}`)
	for _, secret := range []string{"p1", "t1", "s1"} {
		if strings.Contains(nested, secret) {
			t.Errorf("嵌套敏感值未脱敏 %q: %s", secret, nested)
		}
	}
	if !strings.Contains(nested, `"password":"******"`) {
		t.Errorf("嵌套 password 应脱敏: %s", nested)
	}

	// F6：非 JSON 与非法 JSON 原样返回、不报错
	plain := "hello world 123"
	if r.RedactBody(plain) != plain {
		t.Errorf("非 JSON body 应保持不变")
	}
	broken := `{"broken": `
	if r.RedactBody(broken) != broken {
		t.Errorf("非法 JSON 应原样返回")
	}
}

// TestRedactorHeaders 生产模式：Authorization 保留 Bearer 前缀、Cookie token 脱敏、
// 自定义敏感头整体替换、非敏感字段保留（需求 3.3.1）。
func TestRedactorHeaders(t *testing.T) {
	r := NewRedactor(config.RedactConfig{
		Enabled:       true,
		Mask:          "***REDACTED***",
		SensitiveKeys: []string{"password", "token", "authorization", "cookie", "secret", "apikey"},
	})
	h := r.RedactHeaders(map[string]string{
		"Authorization": "Bearer eyJhbGciOiJIUzI1NiJ9.hdr",
		"Cookie":        "token=eyJhbGciOiJIUzI1NiJ9.ck; theme=dark",
		"X-Request-Id":  "req-123",
	})
	if h["Authorization"] != "Bearer ***REDACTED***" {
		t.Errorf("Authorization 应脱敏为 Bearer ***REDACTED***，实际 %q", h["Authorization"])
	}
	if !strings.Contains(h["Cookie"], "token=***REDACTED***") || !strings.Contains(h["Cookie"], "theme=dark") {
		t.Errorf("Cookie token 应脱敏、其他键保留，实际 %q", h["Cookie"])
	}
	if h["X-Request-Id"] != "req-123" {
		t.Errorf("非敏感头应保留，实际 %q", h["X-Request-Id"])
	}

	// apikey 自定义敏感头整体替换
	h2 := r.RedactHeaders(map[string]string{"apikey": "sk-123456"})
	if h2["apikey"] != "***REDACTED***" {
		t.Errorf("apikey 头应整体替换为掩码，实际 %q", h2["apikey"])
	}
}

// TestRedactorQuery 生产模式：query 参数敏感 key 脱敏、其他参数保留（需求 3.3.3）。
func TestRedactorQuery(t *testing.T) {
	r := NewRedactor(config.RedactConfig{
		Enabled:       true,
		Mask:          "***REDACTED***",
		SensitiveKeys: []string{"password", "token", "authorization", "cookie", "secret", "apikey"},
	})
	q := r.RedactQuery("token=eyJhbGciOiJIUzI1NiJ9.qry&email=test@nvs.local")
	if strings.Contains(q, "eyJhbGciOiJIUzI1NiJ9.qry") {
		t.Errorf("query token 未脱敏: %s", q)
	}
	if !strings.Contains(q, "token=***REDACTED***") || !strings.Contains(q, "email=test@nvs.local") {
		t.Errorf("query 脱敏结果不正确: %s", q)
	}
}

// TestRedactorDisabled 开发模式（LOG_REDACT_SENSITIVE=false，F1）：日志保留原始数据，
// body / headers / query 均不做任何脱敏。
func TestRedactorDisabled(t *testing.T) {
	r := NewRedactor(config.RedactConfig{
		Enabled:       false,
		Mask:          "***REDACTED***",
		SensitiveKeys: []string{"password", "token", "authorization", "cookie", "secret", "apikey"},
	})
	body := `{"email":"test@nvs.local","password":"MyP@ssw0rd123","token":"abc.def"}`
	if r.RedactBody(body) != body {
		t.Errorf("关闭脱敏时 body 应保持原样")
	}
	if r.RedactQuery("token=abc.def") != "token=abc.def" {
		t.Errorf("关闭脱敏时 query 应保持原样")
	}
	h := r.RedactHeaders(map[string]string{"Authorization": "Bearer abc.def", "Cookie": "token=abc.def"})
	if h["Authorization"] != "Bearer abc.def" || h["Cookie"] != "token=abc.def" {
		t.Errorf("关闭脱敏时 headers 应保持原样: %v", h)
	}
}

// TestAccessLogManagerRedactsInProdMode 端到端验收（C1/C2）：
// 生产模式（RedactMgr enabled=true）下，写入日志的 headers/body/query 不包含明文 Token 与密码，
// 且包含脱敏结果；非敏感字段（email）保留。
func TestAccessLogManagerRedactsInProdMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 生产模式：启用脱敏
	config.RedactMgr.SeedFromEnv(config.RedactConfig{
		Enabled:       true,
		Mask:          "***REDACTED***",
		SensitiveKeys: []string{"password", "token", "authorization", "cookie", "secret", "apikey"},
	})
	// 还原为默认关闭（开发模式），避免影响其他测试
	defer config.RedactMgr.SeedFromEnv(config.RedactConfig{
		Enabled:       false,
		Mask:          "***REDACTED***",
		SensitiveKeys: config.DefaultSensitiveFields,
	})

	r := gin.New()
	r.Use(AccessLog())
	r.POST("/login", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	body := `{"email":"test@nvs.local","password":"MyP@ssw0rd123","token":"eyJhbGciOiJIUzI1NiJ9.abc"}`
	req := httptest.NewRequest("POST", "/login?token=eyJhbGciOiJIUzI1NiJ9.qry", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.hdr")
	req.Header.Set("Cookie", "token=eyJhbGciOiJIUzI1NiJ9.ck; theme=dark")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 读取刚写入的日志行（写日志为同步操作，可直接读取）
	now := time.Now()
	logPath := filepath.Join("log", "requests", now.Format("2006-01-02"), now.Format("15")+".log")
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("读取日志失败: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	line := lines[len(lines)-1]

	// C1/C2：生产环境日志不包含明文 Token 与密码
	for _, secret := range []string{
		"MyP@ssw0rd123",
		"eyJhbGciOiJIUzI1NiJ9.abc",
		"eyJhbGciOiJIUzI1NiJ9.hdr",
		"eyJhbGciOiJIUzI1NiJ9.ck",
		"eyJhbGciOiJIUzI1NiJ9.qry",
	} {
		if strings.Contains(line, secret) {
			t.Errorf("生产模式日志不应包含明文 %q\n日志: %s", secret, line)
		}
	}
	// 应包含脱敏结果与保留的非敏感字段
	// （body 字符串在日志 JSON 中会转义引号，故按掩码原文断言）
	for _, expect := range []string{"******", "***REDACTED***", "token=***REDACTED***", "Bearer ***REDACTED***", "test@nvs.local", "theme=dark"} {
		if !strings.Contains(line, expect) {
			t.Errorf("日志应包含脱敏/保留结果 %q\n日志: %s", expect, line)
		}
	}
	cleanupTestLogs(t)
}

// TestAccessLogMultipartSkipped multipart 上传绝不读取 body（避免截断文件导致 boundary 缺失）。
func TestAccessLogMultipartSkipped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AccessLog())
	r.POST("/up", func(c *gin.Context) {
		raw, _ := io.ReadAll(c.Request.Body)
		c.JSON(200, gin.H{"len": len(raw)})
	})

	body := "----boundary\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.txt\"\r\n\r\nhello\r\n----boundary--\r\n"
	req := httptest.NewRequest("POST", "/up", strings.NewReader(body))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=----boundary")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// multipart 下中间件不读 body，handler 应完整读到原始字节
	if !strings.Contains(w.Body.String(), `"len":`+strconv.Itoa(len(body))) {
		t.Fatalf("multipart body 不应被中间件读取截断，响应 %s", w.Body.String())
	}
}

// cleanupTestLogs 兜底清理：测试产生的日志目录（go test 工作目录 = server/）
func cleanupTestLogs(t *testing.T) {
	t.Helper()
	utils.CloseDailyLogs()
	_ = os.RemoveAll(filepath.Join(".", "log"))
}
