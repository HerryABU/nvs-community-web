package selfhost

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"nvs-server/config"
	"nvs-server/models"
)

func setupDB(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:selfhost_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	oldDB, oldDir := models.DB, config.SelfHostDir
	models.DB = db
	config.SelfHostDir = t.TempDir()
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("主迁移失败: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("自托管迁移失败: %v", err)
	}
	t.Cleanup(func() {
		models.DB, config.SelfHostDir = oldDB, oldDir
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestValidationRules(t *testing.T) {
	setupDB(t)
	cases := []struct {
		in   string
		want bool
	}{
		{"https://example.com:8443/app", true},
		{":9000", false}, // 裸端口等价于 127.0.0.1，属本机回环，拒绝
		{"localhost:8080", false},
		{"http://127.0.0.1:8080", false},
		{"http://169.254.169.254/latest", false},
		{"javascript:alert(1)", false},
		{"", false},
	}
	for _, tc := range cases {
		_, err := ValidateUpstream(tc.in)
		if (err == nil) != tc.want {
			t.Errorf("ValidateUpstream(%q): want ok=%v, err=%v", tc.in, tc.want, err)
		}
	}
	if err := ValidateSlug("Good Name"); err == nil {
		t.Error("非法 slug 未被拒绝")
	}
	if err := ValidatePrefix("/a/../b"); err == nil {
		t.Error("路径穿越前缀未被拒绝")
	}
	if !SourceRequired("java", false) || SourceRequired("java", true) || SourceRequired("python", false) {
		t.Error("源码必需规则不正确")
	}
	if !SourceRequiredByConfig() {
		t.Error("默认应强制提供源码")
	}
}

// TestReviewGateAndProxyTable 审核门禁：未过审项目不能建路由/上线；过审后热加载表只包含启用项。
func TestReviewGateAndProxyTable(t *testing.T) {
	setupDB(t)
	owner := uint(1)
	r := &Runtime{Language: "java", Version: "21", Kind: "compiled", Enabled: true}
	if _, err := CreateRuntime(*r); err != nil {
		t.Fatalf("建运行时失败: %v", err)
	}
	rt, err := FindRuntime(1)
	if err != nil {
		t.Fatalf("读运行时失败: %v", err)
	}
	p, err := CreateProject(owner, Project{Name: "订单服务", Slug: "order-svc", Language: "java", RuntimeID: rt.ID, Mode: ModeCompiled})
	if err != nil {
		t.Fatalf("建项目失败: %v", err)
	}
	if p.ReviewStatus != ReviewDraft || p.Visibility != VisPrivate {
		t.Fatalf("新项目不应公开: status=%s vis=%s", p.ReviewStatus, p.Visibility)
	}
	// 无源码不能提交
	if err := SubmitForReview(p.ID, owner); err == nil {
		t.Error("缺少源码却允许提交")
	}
	rel, _ := SafeRelPath(owner, p.ID, "src.zip", "source")
	if err := AttachFile(p, "source", rel, "src.zip"); err != nil {
		t.Fatalf("挂源码失败: %v", err)
	}
	if p.ReviewStatus != ReviewDraft {
		t.Error("换文件后应回到 draft")
	}
	relBin, _ := SafeRelPath(owner, p.ID, "app.jar", "binary")
	if err := AttachFile(p, "binary", relBin, "app.jar"); err != nil {
		t.Fatalf("挂可执行文件失败: %v", err)
	}
	if err := models.DB.Model(&Project{}).Where("id = ?", p.ID).
		Updates(map[string]interface{}{"upstream": "http://svc.example.com:8080", "start_command": "java -jar app.jar --port 9000"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := SubmitForReview(p.ID, owner); err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	// 待审不能建路由
	if _, err := UpsertRoute(Route{ProjectID: p.ID, Prefix: "/order", Target: "http://svc.example.com:8080", Enabled: true}); err == nil {
		t.Error("待审项目不应能建对外路由")
	}
	if err := ReviewProject(p.ID, 99, true, "ok", false); err != nil {
		t.Fatalf("审核失败: %v", err)
	}
	if err := SetVisibility(p.ID, owner, VisPublic, false); err != nil {
		t.Fatalf("公开失败: %v", err)
	}
	route, err := UpsertRoute(Route{ProjectID: p.ID, Prefix: "/order", Target: "http://svc.example.com:8080", Timeout: 10, StripPrefix: true})
	if err != nil {
		t.Fatalf("建路由失败: %v", err)
	}
	// 未启用 => 代理表为空
	active, _ := SortedEnabledRoutes()
	if len(active) != 0 {
		t.Fatalf("未启用路由不应进代理表: %d", len(active))
	}
	if _, err := SetRouteEnabled(route.ID, true); err != nil {
		t.Fatalf("启用路由失败: %v", err)
	}
	active, _ = SortedEnabledRoutes()
	if len(active) != 1 || active[0].Prefix != "/order" {
		t.Fatalf("代理表内容错误: %+v", active)
	}
	// 停止项目 => 路由立即失效
	if err := SuspendProject(p.ID, 99, true, "违规"); err != nil {
		t.Fatal(err)
	}
	active, _ = SortedEnabledRoutes()
	if len(active) != 0 {
		t.Fatalf("项目被停止后路由应失效: %d", len(active))
	}
	// 停止后不能重新启用
	if _, err := SetRouteEnabled(route.ID, true); err == nil {
		t.Error("被停止项目不应能重新启用路由")
	}
	logs, _ := ListReviewLogs(p.ID)
	if len(logs) < 3 {
		t.Errorf("审核留痕不完整: %d 条", len(logs))
	}
}

// TestLongestPrefix 最长前缀排序，避免 /a 抢掉 /a/b。
func TestLongestPrefix(t *testing.T) {
	setupDB(t)
	for i, prefix := range []string{"/a", "/a/b", "/a/b/c"} {
		p, err := CreateProject(1, Project{Name: fmt.Sprintf("p%d", i), Slug: fmt.Sprintf("p%d", i), Language: "node", RuntimeID: 1, Mode: ModeSource})
		if err != nil {
			t.Fatal(err)
		}
		models.DB.Model(&Project{}).Where("id = ?", p.ID).Update("review_status", ReviewApproved)
		models.DB.Model(&Project{}).Where("id = ?", p.ID).Update("source_file", "1/"+fmt.Sprint(p.ID)+"/source-src.zip")
		if _, err := UpsertRoute(Route{ProjectID: p.ID, Prefix: prefix, Target: "http://svc.example.com:8080", Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	routes, _ := SortedEnabledRoutes()
	if len(routes) != 3 || routes[0].Prefix != "/a/b/c" || routes[2].Prefix != "/a" {
		t.Fatalf("排序错误: %+v", routes)
	}
}
