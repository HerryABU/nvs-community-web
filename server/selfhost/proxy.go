package selfhost

// 反向代理适配层：机制与 gatewayhub 完全一致（proxycore），
// NVS 只负责「把已审核 + 已启用的路由喂给 Manager」以及 NoRoute 的接入。
//
// 与 GWh 一样是**路径级反向代理**：路由前缀挂在根路径上（如 /java-order/...），
// 最长前缀匹配、剥离前缀、响应改写、跳数守卫全部走 proxycore。
// NVS 独有的差异只有上游 SSRF 门禁（拒绝 localhost/回环）与审核状态门禁。

import (
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"nvs-server/config"
	"nvs-server/models"
	platformmodel "nvs-server/models/platform"
	"nvs-server/selfhost/proxycore"
)

// ProxyBase 仅为兼容旧文案保留；新模型下路由前缀本身就在根路径，不再有固定基座。
const ProxyBase = "/sh"

var (
	proxyOnce      sync.Once
	proxyMgr       *proxycore.Manager
	trafficMu      sync.RWMutex
	prefixProjects = map[string]uint{} // 代理前缀 → 项目 id（直派路由不在其中）
)

func manager() *proxycore.Manager {
	proxyOnce.Do(func() {
		port, _ := strconv.Atoi(config.ServerPort)
		proxyMgr = proxycore.NewManager("", proxycore.SelfInfo{Port: port, MaxHops: 5})
		// 代理路径已由 routes.conditionalGzip 跳过全局 gzip，这里按 GWh 方式
		// 自己解压→改写→重压（plain=false），避免 gin-contrib/gzip 的 CL 回填竞态。
		proxyMgr.SetTrafficReporter(func(prefix string, up, down int64) {
			trafficMu.RLock()
			pid := prefixProjects[prefix]
			trafficMu.RUnlock()
			if pid != 0 {
				RecordTraffic(pid, up, down)
			}
		})
	})
	return proxyMgr
}

// InitManager 启动时装一次；之后每次路由/审核变化调 InvalidateRoutes() 热加载。
func InitManager() {
	ReloadIsoServices()
	InvalidateRoutes()
}

func InvalidateRoutes() {
	routes, err := SortedEnabledRoutes()
	if err != nil {
		log.Printf("[selfhost] 代理路由加载失败: %v", err)
		return
	}
	core := make([]proxycore.Route, 0, len(routes))
	nextPrefixes := make(map[string]uint, len(routes))
	for _, r := range routes {
		nextPrefixes[strings.TrimPrefix(r.Prefix, "/")] = r.ProjectID
		core = append(core, proxycore.Route{
			ID:        r.ID,
			Prefix:    strings.TrimPrefix(r.Prefix, "/"), // NVS 库里带 /，GWh 模型不带
			Target:    r.Target,
			Timeout:   r.Timeout,
			Status:    proxycore.StatusActive,
			CookieHat: r.CookieHat, DropForeign: r.DropForeign, TLSSkipVerify: r.TLSSkipVerify, SDKOriginAuth: r.SDKOriginAuth, WXAppid: r.WXAppid,
		})
	}
	// 管理员直接指派的本机端口路由（无需项目审核，只由管理员配置）。
	direct, _ := ListEnabledDirectRoutes()
	for _, d := range direct {
		target := d.TargetURL
		if target == "" {
			target = "http://127.0.0.1:" + strconv.Itoa(d.Port)
			if d.RootPath != "" {
				target += d.RootPath
			}
		}
		core = append(core, proxycore.Route{
			Prefix: strings.TrimPrefix(d.Prefix, "/"), Target: target, Timeout: 10, Status: proxycore.StatusActive,
			CookieHat: d.CookieHat, DropForeign: d.DropForeign, TLSSkipVerify: d.TLSSkipVerify, SDKOriginAuth: d.SDKOriginAuth, WXAppid: d.WXAppid,
		})
	}
	// 联邦站点的外站路径反代：/fed/<siteID>/... → 源站根，复用 proxycore 的
	// 前缀剥离、响应改写、shim/base 注入、gzip 与跳数守卫（与 GWh 外站反代一致）。
	var fedSites []platformmodel.FederatedSite
	models.DB.Where("status = ?", "active").Find(&fedSites)
	for _, fs := range fedSites {
		upstream := strings.TrimRight(fs.APIURL, "/")
		upstream = strings.TrimSuffix(upstream, "/api")
		if upstream == "" {
			continue
		}
		core = append(core, proxycore.Route{
			Prefix:  "fed/" + strconv.Itoa(int(fs.ID)),
			Target:  upstream,
			Timeout: 15,
			Status:  proxycore.StatusActive,
		})
	}
	trafficMu.Lock()
	prefixProjects = nextPrefixes
	trafficMu.Unlock()
	if err := manager().Load(core); err != nil {
		log.Printf("[selfhost] 代理路由热加载失败: %v", err)
		return
	}
	log.Printf("[selfhost] 路径级代理路由已加载 %d 条（最长前缀匹配）", len(core))
}

// Matchable 供 NoRoute 先判「这是代理路由」还是「前端 SPA 路由」。
func Matchable(path, host string) bool {
	if IsoEngine().Matches(path) {
		return true
	}
	return manager().Matchable(path, host)
}

// Handler 供 NoRoute 命中后转发；与 GWh 的 proxy.Manager.Handle 完全同构。
func Handler(c *gin.Context) {
	if IsoEngine().Matches(c.Request.URL.Path) {
		IsoEngine().ServeHTTP(c.Writer, c.Request)
		return
	}
	manager().Handle(c)
}

// RouteCount 当前内存表中的路由数（管理页展示用）。
func LoadedRouteCount() int { return manager().Count() }

// HasRoutePrefix 判断首段是否属于已注册的路径级代理路由（含多级前缀的首段）。
// 供外层部署前缀剥离避让：业务路由首段不能被当成反代前缀吃掉。
func HasRoutePrefix(first string) bool {
	return IsoEngine().HasPrefix(first) || manager().HasPathPrefix(first)
}
