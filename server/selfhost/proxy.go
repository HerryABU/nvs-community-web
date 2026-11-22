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
	"nvs-server/selfhost/proxycore"
)

// ProxyBase 仅为兼容旧文案保留；新模型下路由前缀本身就在根路径，不再有固定基座。
const ProxyBase = "/sh"

var (
	proxyOnce sync.Once
	proxyMgr  *proxycore.Manager
)

func manager() *proxycore.Manager {
	proxyOnce.Do(func() {
		port, _ := strconv.Atoi(config.ServerPort)
		proxyMgr = proxycore.NewManager("", proxycore.SelfInfo{Port: port, MaxHops: 5})
	})
	return proxyMgr
}

// InitManager 启动时装一次；之后每次路由/审核变化调 InvalidateRoutes() 热加载。
func InitManager() { InvalidateRoutes() }

func InvalidateRoutes() {
	routes, err := SortedEnabledRoutes()
	if err != nil {
		log.Printf("[selfhost] 代理路由加载失败: %v", err)
		return
	}
	core := make([]proxycore.Route, 0, len(routes))
	for _, r := range routes {
		core = append(core, proxycore.Route{
			ID:      r.ID,
			Prefix:  strings.TrimPrefix(r.Prefix, "/"), // NVS 库里带 /，GWh 模型不带
			Target:  r.Target,
			Timeout: r.Timeout,
			Status:  proxycore.StatusActive,
		})
	}
	if err := manager().Load(core); err != nil {
		log.Printf("[selfhost] 代理路由热加载失败: %v", err)
		return
	}
	log.Printf("[selfhost] 路径级代理路由已加载 %d 条（最长前缀匹配）", len(core))
}

// Matchable 供 NoRoute 先判「这是代理路由」还是「前端 SPA 路由」。
func Matchable(path, host string) bool { return manager().Matchable(path, host) }

// Handler 供 NoRoute 命中后转发；与 GWh 的 proxy.Manager.Handle 完全同构。
func Handler(c *gin.Context) { manager().Handle(c) }

// RouteCount 当前内存表中的路由数（管理页展示用）。
func LoadedRouteCount() int { return manager().Count() }
