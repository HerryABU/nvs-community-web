package handlers

import (
	"sort"
	"time"

	"nvs-server/ipgeo"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// 网络诊断 —— 访问来源地域分布（离线解析，零外部请求）。
//
// 设计要点：
//   - 先按 IP 聚合再查表。同一 IP 在日志里会出现成百上千次，逐行查表纯属浪费；
//     聚合后按访问量取 Top N，解析成本与 IP 数量成正比而不是与日志条数成正比。
//   - 数据完全离线：ip2region 库随二进制 embed，查一个 IP 是一次内存二分，
//     不发请求、不依赖任何在线接口（这也是「单文件部署」的前提）。
//   - 内网与保留地址（127.0.0.1 / 192.168.x / 10.x）单独计数，不混进省份排名 ——
//     否则本地调试的流量会把广东顶到第一。

// GeoBucket 一个地域桶（地图/排行用）。
type GeoBucket struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
	Code  string `json:"code,omitempty"`
}

// GetWebAccessGeo GET /api/admin/web-access/geo?days=7&limit=3000
func GetWebAccessGeo(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	days := atoiDefault(c.Query("days"), 7)
	if days < 1 || days > 90 {
		days = 7
	}
	topN := atoiDefault(c.Query("limit"), 3000)
	if topN < 100 || topN > 20000 {
		topN = 3000
	}

	if !ipgeo.Available() {
		// 数据没嵌进去属于构建问题，不该 500 —— 前端按「不可用」降级展示
		utils.Success(c, gin.H{
			"available": false,
			"message":   "IP 归属地数据未嵌入（构建时缺少 ipgeo/data/ip2region_v4.xdb）",
		})
		return
	}

	type ipRow struct {
		IP  string
		Cnt int64
	}
	var rows []ipRow
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	if err := models.DB.Model(&models.WebAccessLog{}).
		Where("created_at >= ?", since).
		Select("ip, COUNT(*) as cnt").
		Group("ip").
		Order("cnt desc").
		Limit(topN).
		Scan(&rows).Error; err != nil {
		utils.InternalError(c, "聚合访问来源失败")
		return
	}

	provinces := map[string]int64{}
	countries := map[string]int64{}
	countryCode := map[string]string{}
	isps := map[string]int64{}
	var domestic, overseas, reserved, unresolved int64

	for _, r := range rows {
		reg := ipgeo.Lookup(r.IP)
		switch {
		case reg.IsReserved():
			reserved += r.Cnt
			continue
		case reg.Country == "":
			unresolved += r.Cnt
			continue
		}
		countries[reg.Country] += r.Cnt
		if reg.CountryCD != "" {
			countryCode[reg.Country] = reg.CountryCD
		}
		if reg.Country == "中国" {
			domestic += r.Cnt
			if prov := ipgeo.NormalizeProvince(reg.Province); prov != "" {
				provinces[prov] += r.Cnt
			}
		} else {
			overseas += r.Cnt
		}
		if reg.ISP != "" {
			isps[reg.ISP] += r.Cnt
		}
	}

	utils.Success(c, gin.H{
		"available": true,
		"days":      days,
		"provinces": toBuckets(provinces, "", 0),
		"countries": toCountryBuckets(countries, countryCode),
		"isp":       toBuckets(isps, "", 12),
		"summary": gin.H{
			"distinct_ips": len(rows),
			"domestic":     domestic,
			"overseas":     overseas,
			"reserved":     reserved,
			"unresolved":   unresolved,
		},
		// 数据来源与口径写进响应，前端直接展示，避免「这个数字怎么来的」没人说得清
		"source": "ip2region v4（离线库，随二进制内嵌；境外按国家聚合，境内按省级聚合）",
	})
}

// toBuckets map → 按访问量倒序的桶列表；limit<=0 表示不截断。
func toBuckets(m map[string]int64, code string, limit int) []GeoBucket {
	out := make([]GeoBucket, 0, len(m))
	for k, v := range m {
		out = append(out, GeoBucket{Name: k, Value: v, Code: code})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Value != out[j].Value {
			return out[i].Value > out[j].Value
		}
		return out[i].Name < out[j].Name
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// toCountryBuckets 国家桶（带两位国家代码，前端可据此显示旗帜/简称）。
func toCountryBuckets(m map[string]int64, codes map[string]string) []GeoBucket {
	out := make([]GeoBucket, 0, len(m))
	for k, v := range m {
		out = append(out, GeoBucket{Name: k, Value: v, Code: codes[k]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Value != out[j].Value {
			return out[i].Value > out[j].Value
		}
		return out[i].Name < out[j].Name
	})
	return out
}
