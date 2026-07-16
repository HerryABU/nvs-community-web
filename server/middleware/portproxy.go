package middleware

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"nvs-server/security"

	"github.com/gin-gonic/gin"
)

// NamedProxy 命名代理映射：authorId/projectName → 内部端口
type NamedProxy struct {
	mu      sync.RWMutex
	routes  map[string]int         // "authorId/projectName" → port
	proxies map[string]*httputil.ReverseProxy
}

var GlobalNamedProxy = &NamedProxy{
	routes:  make(map[string]int),
	proxies: make(map[string]*httputil.ReverseProxy),
}

// Register 注册命名代理
func (np *NamedProxy) Register(authorID uint, projectName string, port int, targetURL string) {
	np.mu.Lock()
	defer np.mu.Unlock()

	key := formatProxyKey(authorID, projectName)
	np.routes[key] = port
	parsed, _ := url.Parse(targetURL)
	if parsed != nil {
		np.proxies[key] = httputil.NewSingleHostReverseProxy(parsed)
	}
}

// Unregister 注销命名代理
func (np *NamedProxy) Unregister(authorID uint, projectName string) {
	np.mu.Lock()
	defer np.mu.Unlock()
	key := formatProxyKey(authorID, projectName)
	delete(np.routes, key)
	delete(np.proxies, key)
}

// GetProxy 获取代理
func (np *NamedProxy) GetProxy(authorID uint, projectName string) *httputil.ReverseProxy {
	np.mu.RLock()
	defer np.mu.RUnlock()
	return np.proxies[formatProxyKey(authorID, projectName)]
}

func formatProxyKey(authorID uint, projectName string) string {
	return fmt.Sprintf("%d/%s", authorID, projectName)
}

// NamedProxyHandler 处理 /sandbox/proxy/:authorId/:projectName/*path
func NamedProxyHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		authorIDStr := c.Param("authorId")
		projectName := c.Param("projectName")
		subPath := c.Param("path")

		if authorIDStr == "" || projectName == "" {
			c.Next()
			return
		}

		// 清除 /sandbox/proxy/ 前缀后的路径匹配
		// Gin 的 :authorId/:projectName 匹配后 subPath 包含 /*filepath
		key := authorIDStr + "/" + projectName

		proxy := GlobalNamedProxy.GetProxy(0, "") // placeholder
		GlobalNamedProxy.mu.RLock()
		for k, p := range GlobalNamedProxy.proxies {
			if k == key {
				proxy = p
				break
			}
		}
		GlobalNamedProxy.mu.RUnlock()

		if proxy == nil {
			c.String(http.StatusNotFound, "代理未注册: "+key)
			return
		}

		// 🔒 路径安全检查
		if strings.Contains(subPath, "..") || strings.Contains(subPath, "./") || strings.Contains(subPath, ".\\") {
			c.String(http.StatusForbidden, "路径穿越被阻止")
			return
		}
		if security.IsBlockedExtension(subPath) {
			c.String(http.StatusForbidden, "禁止访问脚本文件")
			return
		}

		// 重写路径
		c.Request.URL.Path = subPath
		c.Request.URL.RawPath = subPath

		c.Header("X-Forwarded-Proto", "http")
		c.Header("X-NVS-Proxy", "true")
		c.Header("Access-Control-Allow-Origin", "*")

		proxy.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}
