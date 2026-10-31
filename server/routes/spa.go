package routes

// routes.go — 全部 HTTP 路由注册集中在此（从 main.go 原样搬出，未改动任何路由）。
//
// 为什么单独成包：main.go 此前 1852 行、近 1000 行都是路由注册，改一个接口要在
// 上千行里翻；而 main 真正的职责只有「初始化 → 装配 → 启动」。
// 搬出来后 main 只留一行 routes.Register(r)，路由的增删改都在这一个文件里。
//
// 依赖方向是单向的：main → routes → handlers/models/middleware，不存在回环。

import (
	"io/fs"
	"log"
	"net/http"
	"nvs-server/config"
	"nvs-server/handlers"
	"nvs-server/honeypot"
	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

// 静态资源与 SPA 回退。frontendFS 被 NoRoute 闭包捕获，故两者同函数。
func registerStaticAndSPA(r *gin.Engine) {
	// ==================== 静态文件服务 ====================

	// 注意：小说正文不再以 StaticFS 直链暴露。章节内容统一经 GetChapterContent
	// （/api/novels/:id/chapters/:num）按访客门禁（3.1/3.2/3.3）与签名校验后返回，
	// 避免匿名用户/爬虫绕过登录直接下载原始 .html 全文。沙盒代理对 /novels 的读取走
	// proxy.go 的文件处理器，与本静态路由无关，不受影响。

	sandboxGroup := r.Group("/sandbox")
	sandboxGroup.Use(
		middleware.SandboxFrameHeaders(),
		middleware.WasmContentType(),
	)
	sandboxGroup.StaticFS("/userframe", gin.Dir(config.UserFrameDir, false))
	sandboxGroup.StaticFS("/userhtml", gin.Dir(config.UserHTMLDir, false))
	sandboxGroup.GET("/frame/:userID/*filepath", handlers.ServeSandboxPreview)
	// 端口反向代理（/sandbox/proxy/:port/*path） + 严格安全
	// 命名代理（/sandbox/proxy/{authorId}/{projectName}/*path）— 隐藏端口号防打洞
	proxyGroup := r.Group("/sandbox/proxy")
	proxyGroup.Use(
		middleware.PortProxySecurity(),
		middleware.SandboxWriteProtect(),
	)
	proxyGroup.Any("/:authorId/:projectName/*path", middleware.NamedProxyHandler())

	// 上传资源静态文件
	r.StaticFS("/uploads", gin.Dir(config.UploadDir, false))

	// 嵌入的前端 (SPA)
	// 注意：`-tags nodist` 构建（CI 后端 job / 新克隆跑测试）不含真实前端产物，
	// 这里拿到的是占位页 FS，启动日志会给出醒目提示（见 embed_nodist.go）。
	frontendFS, err := FrontendFS()
	if err != nil {
		log.Fatalf("前端资源加载失败（dist 目录不存在，请先运行 npm run build）: %v", err)
	}
	if NodistBuild {
		log.Println("⚠️  本二进制为 nodist 构建：未嵌入前端产物，仅可用于编译/接口调试，不可作为发布物")
	}

	// 注入前端资源探针（供 /health 检查前端 SPA 是否可服务）
	handlers.FrontendProbe = func() (string, bool) {
		indexBytes, err := fs.ReadFile(frontendFS, "index.html")
		if err != nil {
			return "", false
		}
		re := frontendBundleRe.FindString(string(indexBytes))
		if re == "" {
			return "", false
		}
		if _, err := fs.Stat(frontendFS, "assets/"+re); err != nil {
			return re, false
		}
		return re, true
	}

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		// 蜜罐：可疑探路（.php/.env/admin/backup…）静默记录，不改变响应
		honeypot.ProbeNotFound(c)

		// 1) 未匹配路由的 /api/* 一律 404（JSON），绝不落入 SPA fallback。
		//    防止任意 /api/xxx（含被反向代理污染的前缀，如 /api/go-nvs/...，以及 ../ 穿越）被当作前端路由返回 200。
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"code": utils.CodeNotFound, "message": "接口不存在", "data": nil})
			return
		}

		// 2) 拒绝路径穿越：URL 中含 ".." 的请求一律 404（防止目录穿越/路径混淆）
		if strings.Contains(path, "..") {
			c.Status(http.StatusNotFound)
			return
		}

		// 3) 安装完成后：建站向导根段（install / setup，含任意子路径）一律 404。
		//    用首段匹配而非精确匹配，堵住 /install/xxx、/setup/xxx 等子路径穿越
		//    （此前 path=="/install" 精确匹配，/install/xxx 会绕过并落入 SPA 白名单返回 200）。
		if config.IsInstalled() {
			seg := firstPathSegment(strings.TrimPrefix(path, "/"))
			if seg == "install" || seg == "setup" {
				c.Status(http.StatusNotFound)
				return
			}
		}

		// robots.txt（站长可在后台自定义，兼容尾斜杠/大小写变体）
		normalizedPath := strings.ToLower(strings.TrimSuffix(path, "/"))
		if normalizedPath == "/robots.txt" || normalizedPath == "/robot.txt" {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(models.GetRobotsTxt()))
			return
		}

		// SPA 路由请求（Vue Router），检查是否匹配到前端静态资源
		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		// 根路径 / index.html：显式返回并注入站点路径前缀（供前端 router/API base 自适应）
		if cleanPath == "index.html" {
			indexBytes, err := fs.ReadFile(frontendFS, "index.html")
			if err != nil {
				c.String(http.StatusInternalServerError, "前端资源缺失")
				return
			}
			// index.html 不缓存：每次重新验证，保证发布新版（新 hash 资源）后浏览器立即拿到
			c.Header("Cache-Control", "no-cache")
			c.Data(http.StatusOK, "text/html; charset=utf-8",
				middleware.InjectPrefixIntoHTML(indexBytes, config.URLPrefix))
			return
		}

		// 尝试打开请求的文件（嵌入式文件系统中路径不带前导 /）
		f, err := frontendFS.Open(cleanPath)
		if err == nil {
			f.Close()
			// 静态资源缓存策略（解决浏览器每次刷新全量重复下载 js/css）：
			// - assets/*（Vite 产物，文件名带内容 hash）：immutable 永久缓存——
			//   文件名即指纹，内容不变 URL 不变，浏览器直接命中缓存不再发请求
			// - 其余静态文件（favicon 等无 hash 文件名）：短缓存 + Last-Modified/ETag 条件验证
			if strings.HasPrefix(cleanPath, "assets/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				c.Header("Cache-Control", "public, max-age=3600")
			}
			http.FileServer(http.FS(frontendFS)).ServeHTTP(c.Writer, c.Request)
			return
		}

		// 严格 SPA 回退白名单：仅允许已知前端路由根段回退到 index.html；
		// 其余（未配置前缀时的 /go-nvs/xxx、数字前缀 /123/xxx、以及任意未知前缀）一律 404，
		// 彻底禁止任何形式的代理/前缀造成的 /install 等组件穿越。
		if !knownSPARoots[firstPathSegment(cleanPath)] {
			c.Status(http.StatusNotFound)
			return
		}

		// 文件不存在 → SPA fallback（注入站点路径前缀）
		indexContent, err := fs.ReadFile(frontendFS, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "前端资源缺失")
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8",
			middleware.InjectPrefixIntoHTML(indexContent, config.URLPrefix))
	})
}
