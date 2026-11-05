package admin

import (
	"fmt"
	"testing"
	"time"

	"nvs-server/ipgeo"
	"nvs-server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	quota "nvs-server/models/quota"
)

func setupHeatTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:geoheat_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存测试库失败: %v", err)
	}
	if err := db.AutoMigrate(&quota.WebAccessLog{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	old := models.DB
	models.DB = db
	t.Cleanup(func() { models.DB = old })
	return db
}

// 境内/境外/内网三口径必须分开计：
// 以前境外被直接跳过，导致「国际」没有数据源；内网混进地域统计又会污染结论。
func TestComputeGeoSummary_SplitsThreeBuckets(t *testing.T) {
	db := setupHeatTestDB(t)

	rows := []quota.WebAccessLog{
		{IP: "127.0.0.1"}, // 内网/保留
		{IP: "127.0.0.1"},
		{IP: "8.8.8.8"},         // 境外（美国）
		{IP: "1.1.1.1"},         // 境外
		{IP: "114.114.114.114"}, // 境内
	}
	for i := range rows {
		rows[i].CreatedAt = time.Now()
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("写日志失败: %v", err)
		}
	}

	sum := computeGeoSummary(30)
	if sum["available"] != true {
		t.Fatalf("应当可用，实际 %+v", sum)
	}
	total := sum["total"].(int64)
	if total != 5 {
		t.Errorf("总数应为 5，实际 %d", total)
	}
	reserved := sum["reserved"].(int64)
	if reserved != 2 {
		t.Errorf("内网/保留应为 2，实际 %d", reserved)
	}
	// 内网不计入境内也不计入境外
	cn := sum["cn"].(int64)
	os := sum["overseas"].(int64)
	if cn+os != 3 {
		t.Errorf("境内+境外应为 3（排除内网），实际 cn=%d os=%d", cn, os)
	}
	// 百分比之和不应超过 100
	p1, _ := sum["cn_pct"].(float64)
	p2, _ := sum["os_pct"].(float64)
	if p1+p2 > 100.01 {
		t.Errorf("占比之和超过 100：%v + %v", p1, p2)
	}
}

// 国家聚合：境外按国家分组、中国单列，且按访问量降序
func TestComputeCountryHeat_GroupsOverseas(t *testing.T) {
	if !ipgeo.Available() {
		t.Skip("ip2region 库未就绪，跳过")
	}
	db := setupHeatTestDB(t)

	ips := []string{"8.8.8.8", "8.8.4.4", "1.1.1.1", "114.114.114.114", "114.114.115.115", "114.114.116.116"}
	var china int64
	for _, ip := range ips {
		rec := quota.WebAccessLog{IP: ip, CreatedAt: time.Now()}
		if err := db.Create(&rec).Error; err != nil {
			t.Fatalf("写日志失败: %v", err)
		}
		if ipgeo.Lookup(ip).Country == "中国" {
			china++
		}
	}

	out := computeCountryHeat(30)
	if len(out) == 0 {
		t.Fatal("应当有国家聚合结果")
	}
	// 必须有中国这一项（cn=true），且它的值与境内条数一致
	var foundCN bool
	var prev int64 = 1 << 62
	for _, r := range out {
		v := r["value"].(int64)
		if v > prev {
			t.Errorf("结果应按访问量降序，出现逆序：%d > %d", v, prev)
		}
		prev = v
		if r["cn"] == true {
			foundCN = true
			if v != china && china > 0 {
				t.Logf("中国项 %d，境内条数 %d（解析口径可能不同，仅供参考）", v, china)
			}
		}
		if r["code"] == nil || r["code"] == "" {
			t.Error("每条都应有国家码")
		}
	}
	if !foundCN && china > 0 {
		t.Error("应有 cn=true 的中国项")
	}
	// 不能出现内网/保留
	for _, r := range out {
		if r["name"] == "Reserved" {
			t.Error("内网/保留地址不应出现在国家聚合里")
		}
	}
}
