package handlers

// F19 狂暴模式 SSE 端点测试：
//   - 未开启 → 200 + SSE `disabled` 事件（EventSource 无法读 HTTP 状态码，
//     故以协议内事件告知前端永久停重连，避免对未开启站点空转重试）；频道非法 → 400
//   - 开启后长连接能收到 realtime.PublishCommentEvent 推送的评论事件
//
// 数据库用独立命名的共享内存库（file:rage_live_test），避免与其它测试互相污染。

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"nvs-server/models"
	"nvs-server/realtime"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupLiveTestDB(t *testing.T) {
	t.Helper()
	old := models.DB
	db, err := gorm.Open(sqlite.Open("file:rage_live_test?mode=memory&cache=shared"), &gorm.Config{Logger: utils.NewDBLogger()})
	if err != nil {
		t.Fatalf("打开内存测试库失败: %v", err)
	}
	if err := db.AutoMigrate(&models.RageConfig{}); err != nil {
		t.Fatalf("迁移测试库失败: %v", err)
	}
	models.DB = db
	// 重置 Hub 全局状态，避免测试间互相影响
	realtime.DefaultHub.SetEnabled(false)
	realtime.DefaultHub.SetLimits(realtime.DefaultMaxConns, realtime.DefaultBatchInterval, realtime.DefaultBatchSize)
	t.Cleanup(func() {
		models.DB = old
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
}

func saveTestRageConfig(t *testing.T, enabled bool, intervalSec, batchSize, maxConns int) {
	t.Helper()
	cfg := models.RageConfig{
		Enabled:          enabled,
		BatchIntervalSec: intervalSec,
		BatchSize:        batchSize,
		MaxConns:         maxConns,
	}
	if err := models.UpdateRageConfig(cfg); err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}
}

func newLiveEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/live/stream", LiveStream)
	r.GET("/api/live/config", LiveConfig)
	return r
}

// TestLiveStreamDisabledReturnsSSEDisabledEvent 未开启（无配置行 = 默认关闭）时，
// 返回 200 + SSE `disabled` 事件，让前端 EventSource 据此永久停止重连
// （EventSource 无法读取 HTTP 状态码，走 403 会让前端无限退避空转）。
func TestLiveStreamDisabledReturnsSSEDisabledEvent(t *testing.T) {
	setupLiveTestDB(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/live/stream?channel="+url.QueryEscape("comment:novel:1"), nil)
	newLiveEngine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("未开启应返回 200（SSE disabled 事件），实际 %d body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("未开启应返回 text/event-stream，实际 %q", ct)
	}
	if !strings.Contains(w.Body.String(), "event: disabled") {
		t.Fatalf("未开启应在 SSE 流中下发 disabled 事件，实际 body=%s", w.Body.String())
	}
}

// TestLiveStreamBadChannel 频道格式非法一律 400（白名单正则拦截）
func TestLiveStreamBadChannel(t *testing.T) {
	setupLiveTestDB(t)
	saveTestRageConfig(t, true, 1, 5, 100) // 先开启，否则 403 会先于校验返回

	for _, bad := range []string{
		"",                      // 缺失
		"comment:evil:1",        // 类型不在白名单
		"comment:novel:abc",     // ID 非数字
		"comment:novel:",        // 缺 ID
		"comment:novel:1;drop",  // 注入尝试
		"comment:NOVEL:1",       // 大小写变体
		"other:novel:1",         // 前缀不符
		"comment:novel:1 extra", // 尾随垃圾
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/live/stream?channel="+url.QueryEscape(bad), nil)
		newLiveEngine().ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("非法频道 %q 应返回 400，实际 %d", bad, w.Code)
		}
	}
}

// TestLiveStreamReceivesEvent 开启后长连接实时收到新评论（httptest 真服务器 +  goroutine 读流）
func TestLiveStreamReceivesEvent(t *testing.T) {
	setupLiveTestDB(t)
	saveTestRageConfig(t, true, 1, 5, 100) // 批量周期 1s：测试超时留足余量

	ts := httptest.NewServer(newLiveEngine())
	defer ts.Close()

	result := make(chan string, 1)
	go func() {
		resp, err := http.Get(ts.URL + "/api/live/stream?channel=" + url.QueryEscape("comment:novel:1"))
		if err != nil {
			result <- "请求失败: " + err.Error()
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			result <- "状态码异常: " + strconv.Itoa(resp.StatusCode)
			return
		}
		if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
			result <- "Content-Type 异常: " + ct
			return
		}
		br := bufio.NewReader(resp.Body)
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				result <- "读取中断: " + err.Error()
				return
			}
			if strings.HasPrefix(line, "event: comment") {
				data, _ := br.ReadString('\n')
				result <- line + data
				return
			}
		}
	}()

	// 等 Hub 中该频道出现订阅者（确保 SetLimits/SetEnabled 已执行）
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if st := realtime.DefaultHub.Stats(); st["clients"].(int) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if st := realtime.DefaultHub.Stats(); st["clients"].(int) == 0 {
		t.Fatal("等待 SSE 订阅建立超时")
	}

	// 模拟评论创建后的广播（接线后由 comment.go 调用）
	realtime.PublishCommentEvent("novel", 1, map[string]any{"id": 42, "content": "弹幕测试"})

	select {
	case out := <-result:
		if !strings.Contains(out, "event: comment") || !strings.Contains(out, `"id":42`) {
			t.Fatalf("SSE 输出不符: %q", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SSE 未在超时内收到评论事件")
	}
}

// TestLiveStreamMaxConnsRejected 全局连接超限 → 503
// （注意：LiveStream 会用配置行的 max_conns 同步 Hub，因此直接把配置写成 1）
func TestLiveStreamMaxConnsRejected(t *testing.T) {
	setupLiveTestDB(t)
	saveTestRageConfig(t, true, 1, 5, 1)

	// 预占满唯一连接额度
	cl, err := realtime.DefaultHub.Subscribe("comment:novel:999")
	if err != nil {
		t.Fatalf("预占连接失败: %v", err)
	}
	defer realtime.DefaultHub.Unsubscribe("comment:novel:999", cl)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/live/stream?channel="+url.QueryEscape("comment:novel:999"), nil)
	newLiveEngine().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("连接超限应返回 503，实际 %d body=%s", w.Code, w.Body.String())
	}
}

// setupLiveAdminTestDB 管理端测试库（AutoMigrateAll + 可造管理员用户），
// 用独立命名避免与共享内存库互相污染
func setupLiveAdminTestDB(t *testing.T) {
	t.Helper()
	old := models.DB
	db, err := gorm.Open(sqlite.Open("file:rage_admin_test?mode=memory&cache=shared"), &gorm.Config{Logger: utils.NewDBLogger()})
	if err != nil {
		t.Fatalf("打开内存测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移测试库失败: %v", err)
	}
	// RageConfig 尚未纳入 AllModels（由主 agent 接线），测试库显式补迁移
	if err := db.AutoMigrate(&models.RageConfig{}); err != nil {
		t.Fatalf("迁移 rage_configs 失败: %v", err)
	}
	models.DB = db
	t.Cleanup(func() {
		models.DB = old
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
}

// TestLiveConfigPublic 公开开关探测：默认关闭；保存开启后反映 true
func TestLiveConfigPublic(t *testing.T) {
	setupLiveTestDB(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/live/config", nil)
	newLiveEngine().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("探测端点应 200，实际 %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"enabled":false`) {
		t.Fatalf("默认应为关闭: %s", w.Body.String())
	}

	saveTestRageConfig(t, true, 1, 5, 100)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/live/config", nil)
	newLiveEngine().ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), `"enabled":true`) {
		t.Fatalf("开启后应反映 true: %s", w.Body.String())
	}
}

// TestUpdateRageConfigAdmin 管理端保存：范围校验 + 落库 + 零值坑回归 + 非管理员拦截
func TestUpdateRageConfigAdmin(t *testing.T) {
	setupLiveAdminTestDB(t)
	seedTestUser(t, 1, "rage_admin", "admin")
	seedTestUser(t, 2, "rage_reader", "reader")

	// 1) 非法参数一律 400
	for _, body := range []string{
		`{"enabled":true,"batch_interval_sec":0,"batch_size":20,"max_conns":2000}`,
		`{"enabled":true,"batch_interval_sec":61,"batch_size":20,"max_conns":2000}`,
		`{"enabled":true,"batch_interval_sec":3,"batch_size":0,"max_conns":2000}`,
		`{"enabled":true,"batch_interval_sec":3,"batch_size":101,"max_conns":2000}`,
		`{"enabled":true,"batch_interval_sec":3,"batch_size":20,"max_conns":99}`,
		`{"enabled":true,"batch_interval_sec":3,"batch_size":20,"max_conns":10001}`,
	} {
		c, w := asUser("admin", 1)
		c.Request = httptest.NewRequest("PUT", "/api/admin/rage-config", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		UpdateRageConfigAdmin(c)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("非法参数 %s 应 400，实际 %d: %s", body, w.Code, w.Body.String())
		}
	}

	// 2) 合法参数 → 200 且落库生效
	c, w := asUser("admin", 1)
	setJSONBody(c, map[string]any{
		"enabled": true, "batch_interval_sec": 5, "batch_size": 30, "max_conns": 500,
	})
	UpdateRageConfigAdmin(c)
	if w.Code != http.StatusOK {
		t.Fatalf("合法参数应 200，实际 %d: %s", w.Code, w.Body.String())
	}
	got := models.GetRageConfig()
	if !got.Enabled || got.BatchIntervalSec != 5 || got.BatchSize != 30 || got.MaxConns != 500 {
		t.Fatalf("配置未正确落库: %+v", got)
	}

	// 3) 零值坑回归：再保存 enabled=false 必须写进库（Updates map 精确写列）
	c, w = asUser("admin", 1)
	setJSONBody(c, map[string]any{
		"enabled": false, "batch_interval_sec": 5, "batch_size": 30, "max_conns": 500,
	})
	UpdateRageConfigAdmin(c)
	if w.Code != http.StatusOK {
		t.Fatalf("保存 enabled=false 应 200，实际 %d: %s", w.Code, w.Body.String())
	}
	if models.GetRageConfig().Enabled {
		t.Fatal("enabled=false 应能写入（GORM 零值坑回归）")
	}

	// 4) 非管理员 → 403
	c, w = asUser("reader", 2)
	setJSONBody(c, map[string]any{
		"enabled": true, "batch_interval_sec": 3, "batch_size": 20, "max_conns": 2000,
	})
	UpdateRageConfigAdmin(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("非管理员应 403，实际 %d", w.Code)
	}
}
