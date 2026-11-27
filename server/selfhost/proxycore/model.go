package proxycore

// Route 是 gatewayhub 的路径级路由结构在 NVS 侧的等价体。
// Prefix **不带前导斜杠**（与 GWh 完全一致，如 "java-order" 或 "team/app"）；
// NVS 数据库里存的是 "/java-order"，由 server/selfhost 适配层负责转换。
type Route struct {
	ID      uint
	Prefix  string
	Target  string
	Timeout int
	Status  string
	// CookieHat 非空时启用 GWh 同款 cookie 帽子：请求侧脱帽 + 响应侧加帽 + Path 收紧。
	// 为空时保持普通代理行为（只收紧 Path、不改名），避免后端读不到自己下的 cookie。
	CookieHat   string
	DropForeign bool
}

// StatusActive 与 GWh 的 models.StatusActive 语义一致。
const StatusActive = "active"
