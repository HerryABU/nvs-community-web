package selfhost

// 反向代理：只转发「审核通过 + 路由启用」的项目。
// 核心改写/Cookie 收紧/ETag/前缀 shim 与跳数守卫来自 gatewayhub（AGPL-3.0，同作者）。

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"nvs-server/config"
	"nvs-server/selfhost/hopguard"
	"nvs-server/selfhost/rewrite"
	"nvs-server/selfhost/target"
)

// ProxyBase 对外固定前缀。用独立段，避免与静态段同级通配冲突（Gin 会 panic）。
const ProxyBase = "/sh"

type proxyEntry struct {
	prefix  string // /demo 或 /team/app
	target  target.Parsed
	strip   bool
	timeout time.Duration
	rw      *rewrite.Rewriter
	rp      *httputil.ReverseProxy
}

// Manager 保存热加载的路由表。最长前缀优先匹配。
type Manager struct {
	mu      sync.RWMutex
	entries []*proxyEntry
	guard   *hopguard.Guard
	once    sync.Once
}

var proxyManager = &Manager{guard: hopguard.New(hopguard.DefaultMax)}

func GetManager() *Manager { return proxyManager }

// InitManager 启动时装一次；之后每次路由增删改调 InvalidateRoutes() 热加载。
func InitManager() { proxyManager.once.Do(func() { proxyManager.Load() }) }

func InvalidateRoutes() { proxyManager.Load() }

func (m *Manager) Load() {
	routes, err := SortedEnabledRoutes()
	if err != nil {
		log.Printf("[selfhost] 代理路由加载失败: %v", err)
		return
	}
	next := make([]*proxyEntry, 0, len(routes))
	for _, r := range routes {
		t, err := target.Parse(r.Target)
		if err != nil {
			log.Printf("[selfhost] 路由 %s 的 upstream 非法，跳过: %v", r.Prefix, err)
			continue
		}
		// 拦「指向本平台监听端口的回环地址」，避免把 NVS 自己当 upstream。
		selfPort, _ := strconv.Atoi(config.ServerPort)
		if target.IsSelf(t, selfPort, "") {
			log.Printf("[selfhost] 路由 %s 指向自身，跳过（防自指环路）", r.Prefix)
			continue
		}
		pub := strings.TrimSuffix(config.URLPrefix, "/") + ProxyBase + r.Prefix
		pub = strings.TrimPrefix(pub, "/")
		to := time.Duration(r.Timeout) * time.Second
		e := &proxyEntry{
			prefix: r.Prefix, target: t, strip: r.StripPrefix, timeout: to,
			rw: rewrite.New(pub, t.Root, t.Host),
		}
		e.rp = buildReverseProxy(e)
		next = append(next, e)
	}
	m.mu.Lock()
	m.entries = next
	m.mu.Unlock()
	log.Printf("[selfhost] 代理路由已加载 %d 条", len(next))
}

func (m *Manager) match(p string) *proxyEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// 不依赖表已排序：始终取最长匹配，避免 /a 抢掉 /a/b。
	var best *proxyEntry
	for _, e := range m.entries {
		if p == e.prefix || strings.HasPrefix(p, e.prefix+"/") {
			if best == nil || len(e.prefix) > len(best.prefix) {
				best = e
			}
		}
	}
	return best
}

func buildReverseProxy(e *proxyEntry) *httputil.ReverseProxy {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		TLSHandshakeTimeout:   8 * time.Second,
		ResponseHeaderTimeout: e.timeout,
		MaxIdleConns:          128,
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       60 * time.Second,
		DialContext:           (&net.Dialer{Timeout: e.timeout, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
	}
	return &httputil.ReverseProxy{
		Transport:     transport,
		FlushInterval: -1 * time.Second,
		Director: func(req *http.Request) {
			gwPath := req.URL.Path // 网关侧原始路径（含 /sh），给改写器算 CSS 相对基准
			origPath := stripBase(gwPath)
			rest := strings.TrimPrefix(origPath, e.prefix)
			if !e.strip {
				rest = e.prefix + rest
			}
			if rest == "" {
				rest = "/"
			}
			if !strings.HasPrefix(rest, "/") {
				rest = "/" + rest
			}
			// 段边界归一化，防 ../ 穿透到后端其他路径。
			rest = path.Clean(rest)
			req.URL.Scheme = e.target.Scheme
			req.URL.Host = e.target.Host
			req.URL.Path = target.JoinPath(e.target.Root, rest)
			req.Host = e.target.Host
			req.Header.Set("X-Real-IP", clientIP(req))
			req.Header.Set("X-Forwarded-Proto", forwardedProto(req))
			req.Header.Del("X-Forwarded-Prefix") // 后端不应感知平台前缀
			// gatewayhub 实测：强制 uncompressed 才能可靠改写文本；解码由 Modify 完成。
			req.Header.Set("Accept-Encoding", "gzip")
			*req = *rewrite.WithGWPath(req, gwPath)
		},
		ModifyResponse: e.rw.Modify,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("[selfhost] 转发失败 %s: %v", r.URL.Path, err)
			http.Error(w, "自托管服务暂不可用", http.StatusBadGateway)
		},
	}
}

func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		return strings.TrimSpace(strings.Split(v, ",")[0])
	}
	h, _, _ := net.SplitHostPort(r.RemoteAddr)
	return h
}

func forwardedProto(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if v := r.Header.Get("X-Forwarded-Proto"); v != "" {
		return v
	}
	return "http"
}

func isUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Connection"), "upgrade") && r.Header.Get("Upgrade") != ""
}

// stripBase 把入站 /sh/<prefix>/... 剥成路由表里的 <prefix>/...。
// 路由表只存项目前缀（不含 /sh 基座），匹配与后端 rest 计算都必须先剥掉基座。
func stripBase(p string) string {
	if p == ProxyBase {
		return "/"
	}
	if strings.HasPrefix(p, ProxyBase+"/") {
		return strings.TrimPrefix(p, ProxyBase)
	}
	return p
}

// Handler 挂在 r.Any("/sh/*path")。判 404/508/504 都在代理层完成，不落 SPA fallback。
func (m *Manager) Handler(c *gin.Context) {
	p := stripBase(c.Request.URL.Path)
	e := m.match(p)
	if e == nil {
		c.String(http.StatusNotFound, "自托管路由不存在")
		return
	}
	hops, ok := m.guard.Next(c.Request.Header)
	if !ok {
		c.Header(hopguard.Header, strconv.Itoa(hops))
		c.String(http.StatusLoopDetected, "检测到自指循环，已停止转发")
		return
	}
	hopguard.Stamp(c.Request.Header, hops)

	// WebSocket 不套请求超时，否则长连接会被掐断。
	ctx := c.Request.Context()
	if !isUpgrade(c.Request) {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.timeout)
		defer cancel()
	}
	c.Request = c.Request.WithContext(ctx)
	e.rp.ServeHTTP(c.Writer, c.Request)
}

func Handler(c *gin.Context) { proxyManager.InitOnce(); proxyManager.Handler(c) }

func (m *Manager) InitOnce() { m.once.Do(func() { m.Load() }) }

// ResolveURL 供后台健康探测展示（只解析，不发起请求）。
func ResolveURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
