package adminsite

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nvs-server/models"
	platform "nvs-server/models/platform"
)

// TestRecursiveFederation 验证多跳联邦：A 的内容 + A 推荐的 B 的内容（二跳）都被拉取，
// 且来源链、跳数、kind 正确记录；同一 URL 只访问一次（防环）。
func TestRecursiveFederation(t *testing.T) {
	// 友站 B（二跳）
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"code": 0, "data": map[string]any{
			"list": []map[string]any{
				{"id": 201, "kind": "forum", "title": "B站帖子", "source_url": "http://b/thread/201"},
			},
			"sites": []map[string]any{},
			"site":  map[string]any{"name": "友站B", "url": "http://b"},
		}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer b.Close()

	visits := 0
	// 友站 A（一跳）：推荐 B
	a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visits++
		resp := map[string]any{"code": 0, "data": map[string]any{
			"list": []map[string]any{
				{"id": 101, "kind": "novel", "title": "A站小说", "source_url": "http://a/novel/101"},
				{"id": 102, "kind": "blog", "title": "A站博客", "source_url": "http://a/blogs/102"},
			},
			"sites": []map[string]any{{"id": 2, "name": "友站B", "url": b.URL}},
			"site":  map[string]any{"name": "友站A", "url": "http://a"},
		}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer a.Close()

	// 用独立内存库
	models.DB = testDB(t)
	models.DB.AutoMigrate(&platform.FederatedSite{}, &platform.FederatedNovel{}, &models.PlatformConfig{})
	unsafeFedURLForTest = true
	defer func() { unsafeFedURLForTest = false }()

	rootID := uint(99)
	synced := 0
	visited := map[string]bool{}
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/", nil)
	st := syncFedNode(ctx, a.URL+"/api", "", visited, 1, 3, "本站", rootID, &synced)

	if st.err != nil {
		t.Fatalf("递归同步失败: %v", st.err)
	}
	if synced != 3 {
		t.Errorf("应同步 3 条（A站2 + B站1），got %d", synced)
	}
	var rows []platform.FederatedNovel
	models.DB.Where("site_id = ?", rootID).Find(&rows)
	if len(rows) != 3 {
		t.Fatalf("落库应 3 条，got %d", len(rows))
	}
	kinds := map[string]bool{}
	for _, r := range rows {
		kinds[r.Kind] = true
		if r.HopCount < 1 || r.SourceChain == "" {
			t.Errorf("来源链/跳数缺失: %+v", r)
		}
		if !strings.HasPrefix(r.ProxyPath, "/fed/99/") {
			t.Errorf("反代路径应指向 /fed/99/，got %s", r.ProxyPath)
		}
	}
	if !kinds["novel"] || !kinds["blog"] || !kinds["forum"] {
		t.Errorf("类型不全: %v", kinds)
	}
	if visits != 1 {
		t.Errorf("A 站应只访问一次，got %d", visits)
	}
}

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:federation_sync_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("测试库创建失败: %v", err)
	}
	return db
}
