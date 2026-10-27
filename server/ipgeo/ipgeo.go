// Package ipgeo 离线 IP 归属地解析（基于 ip2region xdb v3）。
//
// 为什么自己实现读表而不是引第三方绑定：
//   - 本机代理只放行 api.github.com，proxy.golang.org / raw.githubusercontent.com 都被拦，
//     `go get` 拿不到模块；
//   - xdb 的检索算法只有 40 行，自己实现反而少一个依赖、少一处供应链风险。
//
// 数据文件随二进制一起 embed（11MB），运行时零网络、零外部文件 ——
// 解压即用的单文件部署不该因为「查个 IP」而要求联网。
package ipgeo

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
)

// xdb 文件布局（v3）：
//
//	[0, 256)            头部（version 在前 2 字节）
//	[256, 524544)       向量索引 256×256×8（每项 8B：sPtr / ePtr，绝对文件偏移）
//	[524544, …)         段索引，每段 14B：sip(4) eip(4) dataLen(2) dataPtr(4)
//	[…, EOF)            数据区（"国家|省份|城市|ISP|国家代码"，UTF-8）
const (
	headerLength = 256
	vectorRows   = 256
	vectorCols   = 256
	vectorSize   = 8
	segmentSize  = 14
	segmentStart = headerLength + vectorRows*vectorCols*vectorSize // 524544
)

//go:embed data/ip2region_v4.xdb
var xdb []byte

// Region 一次归属地解析结果。
type Region struct {
	Country   string `json:"country"`    // 国家/地区（如 中国、United States、Reserved）
	Province  string `json:"province"`   // 省/州（境外为州名或 0）
	City      string `json:"city"`       // 城市（可能为 0）
	ISP       string `json:"isp"`        // 运营商（可能为 0）
	CountryCD string `json:"country_cd"` // 两位国家代码（CN/US/…，内网与保留地址为 0）
}

// IsReserved 内网 / 保留地址 / 未知 —— 统计时通常应排除。
func (r Region) IsReserved() bool {
	switch r.Country {
	case "", "Reserved", "0":
		return true
	}
	return false
}

// String 还原 xdb 原始串（便于日志与测试比对）。
func (r Region) String() string {
	return strings.Join([]string{r.Country, r.Province, r.City, r.ISP, r.CountryCD}, "|")
}

var (
	loadOnce sync.Once
	loadErr  error

	// 小容量缓存：Web 访问日志里同一个 IP 会出现很多次，
	// 缓存能把「逐行查表」摊平成「按 IP 查一次」。
	cacheMu sync.RWMutex
	cache   = make(map[string]Region, 4096)
)

// Available xdb 是否可用（数据文件缺失时 embed 会为空，此时调用方应降级）。
func Available() bool { return len(xdb) > segmentStart }

// Lookup 解析 IPv4 归属地。解析失败返回零值 Region（调用方可按 Unknown 处理）。
//
// 只支持 IPv4：xdb v4 库只含 IPv4 段；IPv6 直接返回零值，
// 不要拿 IPv4 的段去套 IPv6，否则会把境外 IPv6 误判成某个国内省份。
func Lookup(ip string) Region {
	ip = strings.TrimSpace(ip)
	if ip == "" || !Available() {
		return Region{}
	}

	cacheMu.RLock()
	if r, ok := cache[ip]; ok {
		cacheMu.RUnlock()
		return r
	}
	cacheMu.RUnlock()

	r := lookupUncached(ip)

	cacheMu.Lock()
	// 简单上限：不做 LRU，满了就整体丢弃重建 —— 这里不是热路径的关键资源
	if len(cache) >= 16384 {
		cache = make(map[string]Region, 4096)
	}
	cache[ip] = r
	cacheMu.Unlock()
	return r
}

func lookupUncached(ip string) Region {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return Region{}
	}
	v4 := parsed.To4()
	if v4 == nil {
		return Region{} // IPv6：本库不覆盖
	}
	sip := binary.BigEndian.Uint32(v4)

	// 向量索引定位：用 IP 的前 16 位选一个「段索引区间」
	idx := int(sip>>24)*vectorCols*vectorSize + int((sip>>16)&0xFF)*vectorSize
	if headerLength+idx+vectorSize > len(xdb) {
		return Region{}
	}
	sPtr := binary.LittleEndian.Uint32(xdb[headerLength+idx:])
	ePtr := binary.LittleEndian.Uint32(xdb[headerLength+idx+4:])

	for sPtr <= ePtr {
		// 二分：把区间对半后按段边界对齐
		m := sPtr + ((ePtr-sPtr)/2/segmentSize)*segmentSize
		if int(m)+segmentSize > len(xdb) {
			return Region{}
		}
		sipSeg := binary.LittleEndian.Uint32(xdb[m:])
		eipSeg := binary.LittleEndian.Uint32(xdb[m+4:])
		dataLen := binary.LittleEndian.Uint16(xdb[m+8:])
		dataPtr := binary.LittleEndian.Uint32(xdb[m+10:])

		switch {
		case sip < sipSeg:
			if m < segmentSize {
				return Region{}
			}
			ePtr = m - segmentSize
		case sip > eipSeg:
			sPtr = m + segmentSize
		default:
			end := int(dataPtr) + int(dataLen)
			if end > len(xdb) || int(dataPtr) < 0 {
				return Region{}
			}
			return parse(string(xdb[dataPtr:end]))
		}
	}
	return Region{}
}

// parse 拆 "国家|省份|城市|ISP|国家代码"。
// 字段缺失时保留 "0"（xdb 用它表示「无」），由调用方决定是否展示。
func parse(s string) Region {
	parts := strings.Split(s, "|")
	get := func(i int) string {
		if i < len(parts) {
			return strings.TrimSpace(parts[i])
		}
		return ""
	}
	r := Region{
		Country:   get(0),
		Province:  get(1),
		City:      get(2),
		ISP:       get(3),
		CountryCD: get(4),
	}
	// xdb 用 "0" 占位表示空；这里统一成空串，避免前端显示一堆 0
	for _, f := range []*string{&r.Province, &r.City, &r.ISP, &r.CountryCD} {
		if *f == "0" {
			*f = ""
		}
	}
	return r
}

// NormalizeProvince 把省名归一成前端地图能对上的名字。
//
// 地图 GeoJSON 用的是全称（"广东省"、"内蒙古自治区"），
// 而 xdb 给的是简称（"广东省" 有时就是"广东"），两边都要能对上。
func NormalizeProvince(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p == "0" {
		return ""
	}
	// 已经是全称就直接用
	if strings.HasSuffix(p, "省") || strings.HasSuffix(p, "市") ||
		strings.HasSuffix(p, "自治区") || strings.HasSuffix(p, "特别行政区") {
		return p
	}
	special := map[string]string{
		"北京": "北京市", "上海": "上海市", "天津": "天津市", "重庆": "重庆市",
		"内蒙古": "内蒙古自治区", "广西": "广西壮族自治区", "西藏": "西藏自治区",
		"宁夏": "宁夏回族自治区", "新疆": "新疆维吾尔自治区",
		"香港": "香港特别行政区", "澳门": "澳门特别行政区", "台湾": "台湾省",
	}
	if v, ok := special[p]; ok {
		return v
	}
	return p + "省"
}

// Stats 供自检/日志用：数据文件是否加载正常。
func Stats() (size int, version uint16) {
	if len(xdb) < 2 {
		return len(xdb), 0
	}
	return len(xdb), binary.LittleEndian.Uint16(xdb)
}

// ErrUnavailable 数据缺失时的统一错误（调用方据此降级为「不支持」而不是 500）。
var ErrUnavailable = fmt.Errorf("ipgeo: ip2region 数据未嵌入")
