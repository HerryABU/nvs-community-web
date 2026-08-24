package middleware

import (
	"bytes"
	"net/http"
	"strings"
)

// StripURLPrefixHandler 站点路径前缀剥离（反向代理挂载场景，如 :8088/go-nvs/login）。
// 必须包在 gin 引擎外层（net/http.Handler 层）执行——gin 的路由匹配发生在中间件链之前，
// 内层中间件改写 URL.Path 无法触发重新匹配；本层在进入 gin 前剥离前缀：
//   - /go-nvs/login          → /login          （SPA 页面路由）
//   - /go-nvs/api/health     → /api/health     （业务 API）
//   - /go-nvs/assets/x.js    → /assets/x.js    （前端静态资源）
//   - /go-nvs/               → /               （前缀根）
//
// 未命中前缀的请求原样透传；未配置前缀（空串）时直接返回 next，零开销。
func StripURLPrefixHandler(prefix string, next http.Handler) http.Handler {
	prefix = strings.TrimRight(prefix, "/")
	if prefix == "" || prefix == "/" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case p == prefix:
			r.URL.Path = "/"
		case strings.HasPrefix(p, prefix+"/"):
			r.URL.Path = strings.TrimPrefix(p, prefix)
		}
		next.ServeHTTP(w, r)
	})
}

// InjectPrefixIntoHTML 向 index.html 注入站点路径前缀支持（反向代理挂载场景）：
//  1. <base href="/{prefix}/"> —— Vite 相对 base（./assets/...）据此解析到前缀下，
//     使首屏 js/css、动态加载 chunk、modulepreload 等静态资源全部同步带前缀；
//  2. window.__NVS_PREFIX__   —— 供前端 Vue Router base 与 axios baseURL 自适应。
// 前缀为空时仅注入 <base href="/">（与默认行为一致），保证非前缀场景深层路由资源也正确。
// 前缀为管理员 env 配置，注入前做转义防 HTML/脚本注入。
func InjectPrefixIntoHTML(html []byte, prefix string) []byte {
	prefix = strings.TrimRight(prefix, "/")

	// 转义：单引号/反斜杠/</script> 防止前缀被当作脚本内容
	escaped := strings.ReplaceAll(prefix, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `'`, `\'`)
	escaped = strings.ReplaceAll(escaped, `</`, `<\/`)

	baseHref := "/"
	if prefix != "" {
		baseHref = prefix + "/"
	}
	// base 标签需在 head 内其他资源引用之前（Vue Router 使用 createWebHistory 显式 base，
	// 不受 <base> 影响；此处仅影响相对资源解析）
	baseTag := "<base href=\"" + baseHref + "\">"
	script := "<script>window.__NVS_PREFIX__='" + escaped + "';</script>"

	if idx := bytes.Index(html, []byte("<head>")); idx >= 0 {
		insertAt := idx + len("<head>")
		out := make([]byte, 0, len(html)+len(baseTag)+len(script))
		out = append(out, html[:insertAt]...)
		out = append(out, baseTag...)
		out = append(out, script...)
		out = append(out, html[insertAt:]...)
		return out
	}
	return html
}
