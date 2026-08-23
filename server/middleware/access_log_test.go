package middleware

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

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

// TestAccessLogRedactSensitive 敏感字段（password/token）必须在日志中脱敏。
func TestAccessLogRedactSensitive(t *testing.T) {
	// 直接测 redactSensitive 函数
	out := redactSensitive(`{"username":"a","password":"secret123","token":"abc.def"}`)
	if strings.Contains(out, "secret123") || strings.Contains(out, "abc.def") {
		t.Errorf("敏感字段未脱敏: %s", out)
	}
	if !strings.Contains(out, "******") {
		t.Errorf("应出现掩码 ******: %s", out)
	}

	// 非 JSON body 应截断到 512 字符且不含全文
	long := strings.Repeat("x", 2000)
	out2 := redactSensitive(long)
	if len(out2) > 600 {
		t.Errorf("非 JSON 长 body 应截断，实际 %d", len(out2))
	}

	// 非法 JSON 也应安全返回（不 panic）
	out3 := redactSensitive(`{"broken": `)
	if out3 == "" {
		t.Errorf("非法 JSON 应返回截断内容，不应为空")
	}
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

// TestMain 兜底清理：测试产生的日志目录（go test 工作目录 = server/）
func cleanupTestLogs(t *testing.T) {
	t.Helper()
	utils.CloseDailyLogs()
	_ = os.RemoveAll(filepath.Join(".", "log"))
}
