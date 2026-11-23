package proxycore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"nvs-server/selfhost/hopguard"
	"nvs-server/selfhost/rewrite"
)

// 跨 HTTP 自指链需要把最外层前缀与原始路径带过每一跳，否则第二跳会用 /target
// 重写已经带 /alias 的响应，导致 <base>/shim 降级成内层服务名。
const gatewayPathHeader = "X-NVS-Gateway-Path"

// restContextKey 保存「整条服务名链剥离后的后端路径」，Director 优先使用它。
type restContextKey struct{}

func withRest(req *http.Request, rest string) {
	*req = *req.WithContext(context.WithValue(req.Context(), restContextKey{}, rest))
}

func restFrom(req *http.Request) (string, bool) {
	if req == nil || req.Context() == nil {
		return "", false
	}
	v, ok := req.Context().Value(restContextKey{}).(string)
	return v, ok && v != ""
}

// resolveChain 处理 `/中转/目标/服务自身路由` 这种多段服务名链：
// 先按最长前缀命中第一段，再逐段检查剩余路径是否又命中另一个已注册前缀，
// 逐段剥掉并切换到后者，最终返回真正负责转发的条目与剩余路径。
// 这样无需把每一层都配成自指路由，也避免 root 与 rest 叠加出 `/B/B/api`。
func (m *Manager) resolveChain(path string) (finalPrefix, fullChain string, e *Entry, rest string) {
	prefix, entry, ok := m.matchPath(path)
	if !ok {
		return "", "", nil, ""
	}
	chain := prefix
	rest = sanitizeRest(path, prefix)
	for i := 0; i < 16; i++ { // 防止异常配置导致无限循环
		nextPrefix, nextEntry, hit := m.matchPath(rest)
		if !hit || nextEntry == entry || nextPrefix == prefix {
			break
		}
		nextRest := sanitizeRest(rest, nextPrefix)
		if len(nextRest) >= len(rest) {
			break
		}
		prefix, entry, rest = nextPrefix, nextEntry, nextRest
		chain = chain + "/" + nextPrefix
	}
	return prefix, chain, entry, rest
}

// chainEntry 为整条服务名链生成一个只在本次请求生效的条目：
// 响应改写必须用完整对外前缀（/中转/目标/），否则 <base> 与 shim 会指错层级。
// Transport 与 Director 仍复用原条目，只替换 ModifyResponse，避免每请求新建连接池。
func (m *Manager) chainEntry(e *Entry, fullChain string) *Entry {
	if e == nil || fullChain == "" || fullChain == e.Route.Prefix {
		return e
	}
	// 同一请求内剥掉多个服务名时没有真实外层跳，不能在请求头里伪造入站链
	// （那会让改写器误判为「非最外层」并把内部头漏给浏览器）。
	// 改为在响应侧注入完整链：改写器读到 header 里的 full，同时 inbound 仍为空，
	// 因而按最外层语义在处理完删除内部头。
	cp := *e
	proxyCopy := *e.Proxy
	inner := proxyCopy.ModifyResponse
	proxyCopy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Set(rewrite.PrefixChainHeader, fullChain)
		return inner(resp)
	}
	cp.Proxy = &proxyCopy
	return &cp
}

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

	// 2) 路径路由：最长前缀匹配 + 多段服务名链剥离
	prefix, fullChain, entry, rest := m.resolveChain(path)
	if entry == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "message": "route not found"})
		return
	}
	if entry.Route.Status != StatusActive {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": http.StatusServiceUnavailable, "message": "route disabled"})
		return
	}
	// 请求侧链头由 Director 按 GWh 规则逐跳累加；本进程内一次剥掉多个服务名
	// 则通过 chainEntry 在响应侧注入完整链，不伪造入站头。
	outerPath := strings.TrimSpace(c.Request.Header.Get(gatewayPathHeader))
	if outerPath == "" || !strings.HasPrefix(outerPath, "/") {
		outerPath = path
		c.Request.Header.Set(gatewayPathHeader, outerPath)
	}
	withRest(c.Request, rest)
	rewrite.WithGWPath(c.Request, outerPath)
	entry = m.chainEntry(entry, fullChain)
	m.serve(c, entry, entry.Proxy, prefix, path, clientIP)
}

// serve 执行转发（含跳数守卫、WebSocket 免超时、状态码捕获），与 GWh 同构。
// NVS 的全局 AccessLog 中间件已记录所有请求，这里不再重复落盘明细。
func (m *Manager) serve(c *gin.Context, entry *Entry, p *httputil.ReverseProxy, prefix, path, clientIP string) {
	sw := &statusWriter{ResponseWriter: c.Writer, status: http.StatusOK}
	req := c.Request
	// 请求体计数：转发会消费 Body，必须在交给 ReverseProxy 之前包一层。
	bodyCounter := &countingBody{ReadCloser: req.Body}
	if req.Body != nil {
		req.Body = bodyCounter
	}

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
	if m.traffic != nil {
		m.traffic(prefix, bodyCounter.n, sw.bytes)
	}
}

// countingBody 统计转发给后端的请求体字节数（上行）。
type countingBody struct {
	io.ReadCloser
	n int64
}

func (b *countingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.n += int64(n)
	return n, err
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
