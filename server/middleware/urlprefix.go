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

// 与 GWh 的 reservedFirstSegments 一致：这些首段是平台自己的资源/接口，永不剥离。
var stripReservedFirst = map[string]bool{
	"api": true, "assets": true, "static": true, "favicon": true,
	"uploads": true, "sandbox": true, "health": true, "hp": true,
}

// SmartStripURLPrefixHandler 在 StripURLPrefixHandler 之上补 GWh 的「特殊转发剥离」。
//
// 两种反代拓扑都要能工作：
//  1. 反代已剥前缀：请求是干净路径 /api、/java-order/login，首段为保留字或业务路由，不剥；
//  2. 反代**透传**前缀：请求是 /{name}/api、/{name}/java-order/login，
//     仅当首段既非保留字、非前端 SPA 根段、非自托管路径级代理路由，且路径含第二段时，
//     把该段判定为部署前缀并剥掉。
//
// 配置前缀优先级最高：URL_PREFIX 显式命中时无条件剥离（兼容原有固定前缀行为）。
// 必须包在 gin 引擎外层——gin 的路由匹配在中间件链之前，内层改 URL.Path 不会重新路由。
func SmartStripURLPrefixHandler(prefix string, isBusiness func(string) bool, next http.Handler) http.Handler {
	configured := strings.TrimRight(prefix, "/")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		// 1) 已配置的固定前缀：与旧行为完全一致，优先剥。
		if configured != "" && configured != "/" {
			if p == configured {
				r.URL.Path = "/"
				next.ServeHTTP(w, r)
				return
			}
			if strings.HasPrefix(p, configured+"/") {
				r.URL.Path = strings.TrimPrefix(p, configured)
				next.ServeHTTP(w, r)
				return
			}
		}
		// 2) 未命中配置前缀：按 GWh 的启发式智能剥离。
		seg, rest := firstPathSeg(p)
		// ⚠️ 尾斜杠时 rest 也是 "/"，必须要求第二段**非空**，否则
		// /unknown-prefix/ 会被剥成首页（未审核路由/未知前缀误返回 200）。
		if seg != "" && strings.Trim(rest, "/") != "" && !stripReservedFirst[seg] && (isBusiness == nil || !isBusiness(seg)) {
			r.URL.Path = rest
		}
		next.ServeHTTP(w, r)
	})
}

// firstPathSeg 切分路径首段与剩余部分（rest 恒以 / 开头；无第二段时 rest 为空）。
func firstPathSeg(p string) (string, string) {
	t := strings.TrimPrefix(p, "/")
	if t == "" {
		return "", ""
	}
	if i := strings.Index(t, "/"); i >= 0 {
		return t[:i], "/" + t[i+1:]
	}
	return t, ""
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
//
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
