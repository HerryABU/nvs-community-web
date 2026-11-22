package proxycore

import (
	"log"
	"sort"
	"strings"
	"sync"

	"nvs-server/selfhost/hopguard"
)

// SelfInfo 描述网关自身，用于识别自指路由并限制链式转发跳数。
// 自指是合法配置（把控制台/另一个前缀的站点挂到新前缀下），
// 所以这里只做识别与告警，真正的防死循环靠 hopguard 的跳数上限。
type SelfInfo struct {
	Port    int // 网关监听端口；0 表示不做自指识别
	MaxHops int // 链式转发最大跳数；0 用 hopguard 默认值
}

// Manager 反向代理管理器（内存路由表 + 热加载 + 路径最长前缀匹配）。
// 与 gatewayhub 的 proxy.Manager 同构；NVS 侧去掉了 stats/accesslog 依赖，
// DB 读取交给 server/selfhost 适配层（Load 接收已经过滤好的路由切片）。
type Manager struct {
	baseDomain string
	self       SelfInfo
	hops       *hopguard.Guard
	mu         sync.RWMutex
	m          map[string]*Entry
	multi      []string // 含 "/" 的多级前缀，按长度降序
	warnSig    string
	selfSig    string
	traffic    TrafficFunc // 每请求完成后上报 {prefix, 上行, 下行}
}

// TrafficFunc 由宿主（NVS）注入，用于按代理前缀累计流量。
type TrafficFunc func(prefix string, up, down int64)

func (m *Manager) SetTrafficReporter(fn TrafficFunc) { m.traffic = fn }

func NewManager(baseDomain string, self SelfInfo) *Manager {
	return &Manager{
		baseDomain: baseDomain,
		self:       self,
		hops:       hopguard.New(self.MaxHops),
		m:          make(map[string]*Entry),
	}
}

func (m *Manager) HopsMax() int { return m.hops.Max() }

// Load 用传入的路由切片整体替换内存表（等价 GWh 的从 DB 全量 Load）。
// 单条解析失败不中断整体加载，但必须留痕。
func (m *Manager) Load(routes []Route) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m = make(map[string]*Entry, len(routes))
	var skipped []string
	selfs := make([]string, 0, len(routes))
	for _, r := range routes {
		e, err := m.buildEntry(r)
		if err != nil {
			log.Printf("[selfhost-proxy] 路由 %s 已跳过（target=%q）：%v", r.Prefix, r.Target, err)
			skipped = append(skipped, r.Prefix)
			continue
		}
		m.m[r.Prefix] = e
		if e.Self {
			selfs = append(selfs, r.Prefix)
		}
	}
	m.rebuildMultiLocked()
	m.warnSelfLocked(selfs, len(routes))
	if len(skipped) > 0 {
		sig := strings.Join(skipped, ",")
		if sig != m.warnSig {
			m.warnSig = sig
			log.Printf("[selfhost-proxy] 警告：%d/%d 条路由因 target 无法解析而未生效（%s）", len(skipped), len(routes), sig)
		}
	} else {
		m.warnSig = ""
	}
	return nil
}

// warnSelfLocked 对自指路由打一次汇总告警（调用方需持有写锁）。
func (m *Manager) warnSelfLocked(selfs []string, total int) {
	sort.Strings(selfs)
	sig := strings.Join(selfs, ",")
	if sig == m.selfSig {
		return
	}
	m.selfSig = sig
	if len(selfs) == 0 {
		return
	}
	var dead []string
	for _, p := range selfs {
		if e, ok := m.m[p]; ok && selfLoopsBack(e) {
			dead = append(dead, p)
		}
	}
	log.Printf("[selfhost-proxy] 检测到 %d/%d 条自指路由（target 指向网关自身 :%d）：%s；跳数上限 %d，超过即 508",
		len(selfs), total, m.self.Port, sig, m.hops.Max())
	if len(dead) > 0 {
		log.Printf("[selfhost-proxy] 警告：自指路由 %s 的 target 根路径等于自身前缀，必然绕回同一前缀；请改指向另一个前缀",
			strings.Join(dead, ", "))
	}
}

func selfLoopsBack(e *Entry) bool {
	root := strings.Trim(strings.TrimSuffix(e.Parsed.Root, "/"), "/")
	if root == "" {
		return false
	}
	return root == strings.Trim(e.Route.Prefix, "/")
}

func (m *Manager) selfPrefixesLocked() []string {
	out := make([]string, 0)
	for p, e := range m.m {
		if e.Self {
			out = append(out, p)
		}
	}
	return out
}

// Upsert 新增或更新路由并热加载（GWh 同构）。
func (m *Manager) Upsert(r Route) error {
	e, err := m.buildEntry(r)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.m[r.Prefix] = e
	m.rebuildMultiLocked()
	m.warnSelfLocked(m.selfPrefixesLocked(), len(m.m))
	m.mu.Unlock()
	return nil
}

func (m *Manager) Remove(prefix string) {
	m.mu.Lock()
	delete(m.m, prefix)
	m.rebuildMultiLocked()
	m.mu.Unlock()
}

func (m *Manager) Get(prefix string) (*Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.m[prefix]
	return e, ok
}

func (m *Manager) Exists(prefix string) bool {
	_, ok := m.Get(prefix)
	return ok
}

// HasPathPrefix 判断某首段是否为已注册业务路由（含多级前缀的首段）。
func (m *Manager) HasPathPrefix(first string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.m[first]; ok {
		return true
	}
	for _, p := range m.multi {
		if strings.HasPrefix(p, first+"/") {
			return true
		}
	}
	return false
}

func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.m)
}

func (m *Manager) rebuildMultiLocked() {
	multi := make([]string, 0)
	for p := range m.m {
		if strings.Contains(p, "/") {
			multi = append(multi, p)
		}
	}
	sort.Slice(multi, func(i, j int) bool { return len(multi[i]) > len(multi[j]) })
	m.multi = multi
}

// Matchable 判断请求是否命中路由，供 NoRoute 决定「转发 or SPA 回退」。
func (m *Manager) Matchable(path, host string) bool {
	if m.MatchableHost(host) {
		return true
	}
	_, _, ok := m.matchPath(path)
	return ok
}

// MatchableHost 判断 Host 是否指向启用的子域名路由（NVS 默认 baseDomain 为空 ⇒ 禁用）。
func (m *Manager) MatchableHost(host string) bool {
	prefix, ok := m.matchSubdomain(host)
	if !ok {
		return false
	}
	e, ok := m.Get(prefix)
	return ok && e.Route.Status == StatusActive
}
