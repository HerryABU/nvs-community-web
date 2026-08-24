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
//  1. 资源引用改写（主方案）：非空前缀时，把 Vite 相对资源引用 ./assets/xxx
//     直接改写为 {prefix}/assets/xxx —— 首屏 js/css、modulepreload 的 URL 在 HTML
//     层面就带上前缀，**不依赖浏览器对 <base> 的解析**，任何自建反代（保留/去前缀
//     转发）下资源都能命中前缀路由；动态 import chunk 相对当前模块 URL 解析，
//     自动同步前缀；
//  2. <base href="/{prefix}/">（兜底）：覆盖 HTML 中其他相对引用（如 favicon、
//     CSS 内 url() 等），使相对路径统一解析到前缀下；
//  3. window.__NVS_PREFIX__：供前端 Vue Router base 与 axios baseURL 自适应。
// 前缀为空时仅注入 <base href="/">（与默认行为一致），资源引用保持 Vite 产物原样。
// 前缀为管理员 env 配置，注入前做转义防 HTML/脚本注入。
func InjectPrefixIntoHTML(html []byte, prefix string) []byte {
	prefix = strings.TrimRight(prefix, "/")

	if prefix != "" {
		// 主方案：相对资源引用改写为绝对前缀路径（双引号/单引号两种形式都处理）
		html = bytes.ReplaceAll(html, []byte(`"./assets/`), []byte(`"`+prefix+`/assets/`))
		html = bytes.ReplaceAll(html, []byte(`'./assets/`), []byte(`'`+prefix+`/assets/`))
		html = bytes.ReplaceAll(html, []byte(`"./favicon.ico`), []byte(`"`+prefix+`/favicon.ico`))
		html = bytes.ReplaceAll(html, []byte(`'./favicon.ico`), []byte(`'`+prefix+`/favicon.ico`))
	}

	// 转义：单引号/反斜杠/</script> 防止前缀被当作脚本内容
	escaped := strings.ReplaceAll(prefix, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `'`, `\'`)
	escaped = strings.ReplaceAll(escaped, `</`, `<\/`)

	baseHref := "/"
	if prefix != "" {
		baseHref = prefix + "/"
	}
	// base 标签需在 head 内其他资源引用之前（Vue Router 使用 createWebHistory 显式 base，
	// 不受 <base> 影响；此处仅兜底相对资源解析）
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
