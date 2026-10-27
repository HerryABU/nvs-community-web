package handlers

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"nvs-server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupGeoTestDB 独立内存库（唯一名，避免与同包其他测试共享污染）。
func setupGeoTestDB(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:geotest_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	models.DB = db
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
}

// seedAccess 写入 n 条来自指定 IP 的访问日志。
func seedAccess(t *testing.T, ip string, n int, at time.Time) {
	t.Helper()
	for i := 0; i < n; i++ {
		row := &models.WebAccessLog{
			IP: ip, Method: "GET", Path: "/novel/1",
			UAFamily: "browser", IdentityType: "anonymous",
			StatusCode: 200, CreatedAt: at,
		}
		if err := models.DB.Create(row).Error; err != nil {
			t.Fatalf("写入日志失败: %v", err)
		}
	}
}

// TestWebAccessGeoAggregation 地域聚合：国内按省、境外按国家、内网单列。
func TestWebAccessGeoAggregation(t *testing.T) {
	setupGeoTestDB(t)
	seedTestUser(t, 1, "boss", "admin")

	now := time.Now().Add(-1 * time.Hour)
	// 江苏 5 次、浙江 3 次、广东 2 次（省内应各自聚合）
	seedAccess(t, "114.114.114.114", 5, now) // 中国|江苏省|南京市
	seedAccess(t, "223.5.5.5", 3, now)       // 中国|浙江省|杭州市
	seedAccess(t, "202.96.128.86", 2, now)   // 中国|广东省|广州市
	// 境外 4 次（美国）应进国家维度、不进省份
	seedAccess(t, "8.8.8.8", 4, now) // United States|California
	// 内网 7 次应单列，不能混进省份排名
	seedAccess(t, "127.0.0.1", 7, now)

	c, w := asUser("admin", 1)
	GetWebAccessGeo(c)
	if w.Code != 200 {
		t.Fatalf("应为 200，实际 %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Available bool `json:"available"`
			Provinces []struct {
				Name  string `json:"name"`
				Value int64  `json:"value"`
			} `json:"provinces"`
			Countries []struct {
				Name  string `json:"name"`
				Value int64  `json:"value"`
				Code  string `json:"code"`
			} `json:"countries"`
			Summary struct {
				DistinctIPs int64 `json:"distinct_ips"`
				Domestic    int64 `json:"domestic"`
				Overseas    int64 `json:"overseas"`
				Reserved    int64 `json:"reserved"`
			} `json:"summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应非 JSON: %v", err)
	}
	d := resp.Data
	if !d.Available {
		t.Fatalf("ipgeo 应可用，响应: %s", w.Body.String())
	}

	// 省名必须是 GeoJSON 全称，否则地图对不上
	want := map[string]int64{"江苏省": 5, "浙江省": 3, "广东省": 2}
	if len(d.Provinces) != len(want) {
		t.Fatalf("省份桶应为 %d 个，实际 %d：%+v", len(want), len(d.Provinces), d.Provinces)
	}
	for _, p := range d.Provinces {
		if want[p.Name] != p.Value {
			t.Errorf("省份 %q 应为 %d，实际 %d", p.Name, want[p.Name], p.Value)
		}
	}
	// 排序：江苏省（5）必须排第一
	if d.Provinces[0].Name != "江苏省" {
		t.Errorf("省份应按访问量倒序，首位应为江苏省，实际 %q", d.Provinces[0].Name)
	}

	// 国家维度：中国 10 次 + 美国 4 次
	var cn, us int64
	for _, c := range d.Countries {
		switch c.Name {
		case "中国":
			cn = c.Value
			if c.Code != "CN" {
				t.Errorf("中国国家代码应为 CN，实际 %q", c.Code)
			}
		case "United States":
			us = c.Value
		}
	}
	if cn != 10 {
		t.Errorf("中国访问应为 10 次，实际 %d", cn)
	}
	if us != 4 {
		t.Errorf("美国访问应为 4 次，实际 %d", us)
	}

	// 汇总口径
	if d.Summary.Domestic != 10 {
		t.Errorf("境内应为 10，实际 %d", d.Summary.Domestic)
	}
	if d.Summary.Overseas != 4 {
		t.Errorf("境外应为 4，实际 %d", d.Summary.Overseas)
	}
	if d.Summary.Reserved != 7 {
		t.Errorf("内网应为 7（且不得混入省份），实际 %d", d.Summary.Reserved)
	}
	if d.Summary.DistinctIPs != 5 {
		t.Errorf("独立 IP 应为 5，实际 %d", d.Summary.DistinctIPs)
	}
}

// TestWebAccessGeoTimeWindow days 过滤应生效：窗口外的旧日志不计入。
func TestWebAccessGeoTimeWindow(t *testing.T) {
	setupGeoTestDB(t)
	seedTestUser(t, 1, "boss", "admin")

	seedAccess(t, "114.114.114.114", 2, time.Now().Add(-1*time.Hour))
	seedAccess(t, "114.114.114.114", 9, time.Now().Add(-40*24*time.Hour)) // 40 天前

	c, w := asUser("admin", 1)
	c.Request = httptest.NewRequest("GET", "/?days=7", nil)
	GetWebAccessGeo(c)

	var resp struct {
		Data struct {
			Summary struct {
				Domestic int64 `json:"domestic"`
			} `json:"summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应非 JSON: %v", err)
	}
	if resp.Data.Summary.Domestic != 2 {
		t.Errorf("近 7 天应只统计 2 次，实际 %d", resp.Data.Summary.Domestic)
	}
}

// TestWebAccessGeoRequiresAdmin 非管理员必须被拒。
func TestWebAccessGeoRequiresAdmin(t *testing.T) {
	setupGeoTestDB(t)
	seedTestUser(t, 2, "reader", "reader")

	c, w := asUser("reader", 2)
	GetWebAccessGeo(c)
	if w.Code == 200 {
		t.Fatalf("非管理员不应拿到地域数据，实际 200: %s", w.Body.String())
	}
}
