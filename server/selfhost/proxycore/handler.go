package proxycore

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"nvs-server/selfhost/hopguard"
)

// errorHandler 统一的后端错误响应（502/504），与 GWh 一致。
func (m *Manager) errorHandler(w http.ResponseWriter, req *http.Request, err error) {
	code := http.StatusBadGateway
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		code = http.StatusGatewayTimeout
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_, _ = fmt.Fprintf(w, `{"code":%d,"message":"%s"}`, code, http.StatusText(code))
}

// Handle 转发入口（供 NVS 的 NoRoute 调用）。
// 匹配优先级与 GWh 完全一致：子域名（Host，零重写）→ 路径最长前缀（需重写）。
func (m *Manager) Handle(c *gin.Context) {
	path := c.Request.URL.Path
	clientIP := clientIPFrom(c.Request.RemoteAddr, c.Request.Header)

	// 1) 子域名路由（NVS 默认 baseDomain 为空，天然禁用）
	if prefix, ok := m.matchSubdomain(c.Request.Host); ok {
		if entry, ok := m.Get(prefix); ok {
			if entry.Route.Status != StatusActive {
				c.JSON(http.StatusServiceUnavailable, gin.H{"code": http.StatusServiceUnavailable, "message": "route disabled"})
				return
			}
			m.serve(c, entry, entry.SubProxy, prefix, path, clientIP)
			return
		}
	}

	// 2) 路径路由：最长前缀匹配
	prefix, entry, ok := m.matchPath(path)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "message": "route not found"})
		return
	}
	if entry.Route.Status != StatusActive {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": http.StatusServiceUnavailable, "message": "route disabled"})
		return
	}
	m.serve(c, entry, entry.Proxy, prefix, path, clientIP)
}

// serve 执行转发（含跳数守卫、WebSocket 免超时、状态码捕获），与 GWh 同构。
// NVS 的全局 AccessLog 中间件已记录所有请求，这里不再重复落盘明细。
func (m *Manager) serve(c *gin.Context, entry *Entry, p *httputil.ReverseProxy, prefix, path, clientIP string) {
	sw := &statusWriter{ResponseWriter: c.Writer, status: http.StatusOK}
	req := c.Request

	// 自指/嵌套链式转发：超过上限直接 508，不把请求推给自己。
	hops, ok := m.hops.Next(req.Header)
	if !ok {
		c.Header(hopguard.Header, fmt.Sprint(hops))
		c.JSON(http.StatusLoopDetected, gin.H{
			"code":    http.StatusLoopDetected,
			"message": fmt.Sprintf("loop detected: 转发跳数超过上限 %d（自指路由 %s 可能绕回了自己）", m.hops.Max(), prefix),
		})
		return
	}
	hopguard.Stamp(req.Header, hops)

	if isUpgrade(req) {
		// WebSocket 跳过请求超时，避免长连接被掐断
		p.ServeHTTP(sw, req)
	} else {
		ctx, cancel := context.WithTimeout(req.Context(), time.Duration(entry.Route.Timeout)*time.Second)
		defer cancel()
		p.ServeHTTP(sw, req.WithContext(ctx))
	}
}

// isUpgrade 判断是否为 WebSocket 等协议升级请求。
func isUpgrade(req *http.Request) bool {
	return strings.EqualFold(req.Header.Get("Connection"), "Upgrade") && req.Header.Get("Upgrade") != ""
}

// statusWriter 捕获响应状态码与写出字节数。
type statusWriter struct {
	gin.ResponseWriter
	status int
	bytes  int64
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytes += int64(n)
	return n, err
}
