package selfhost

import (
	"testing"

	"nvs-server/models"
)

func TestIsolationDetection(t *testing.T) {
	setupDB(t)
	if issues := CheckIsolation(); len(issues) != 0 {
		t.Fatalf("空库不应有问题: %+v", issues)
	}
	p1, _ := CreateProject(1, Project{Name: "空间一", Slug: "space-one", Language: "java", RuntimeID: 1, Mode: ModeCompiled})
	models.DB.Model(&Project{}).Where("id = ?", p1.ID).Update("upstream", "http://svc.example.com:9000/app")
	if err := isolationBlocks(p1.ID); err != nil {
		t.Fatalf("独立上游不应被拦: %v", err)
	}

	// 第二个项目复用完全相同的上游 ⇒ 视为跨空间访问。
	p2, _ := CreateProject(2, Project{Name: "空间二", Slug: "space-two", Language: "node", RuntimeID: 1, Mode: ModeSource})
	models.DB.Model(&Project{}).Where("id = ?", p2.ID).Update("upstream", "http://svc.example.com:9000/app")
	warned := false
	for _, i := range CheckIsolation() {
		if i.ProjectID == p2.ID && i.Level == "warn" {
			warned = true
		}
	}
	if !warned {
		t.Error("复用其他项目上游应给出隔离提醒")
	}

	// 先给空间一建代理路由，再让空间二的上游根路径指向它 ⇒ 视为跨空间。
	models.DB.Create(&Route{ProjectID: p1.ID, Prefix: "/space-one", Target: "http://svc.example.com:9000", Enabled: true, Status: RouteRunning})
	models.DB.Model(&Project{}).Where("id = ?", p2.ID).Update("upstream", "http://space-one.example.com:9000/space-one")
	if err := isolationBlocks(p2.ID); err == nil {
		t.Error("上游根路径指向其他项目空间应被拦截")
	}

	issues := CheckIsolation()
	if len(issues) < 1 {
		t.Fatalf("检测问题数量不足: %+v", issues)
	}
	found := false
	for _, i := range issues {
		if i.ProjectID == p2.ID && i.Level == "block" {
			found = true
		}
	}
	if !found {
		t.Errorf("应报告空间二的阻断问题: %+v", issues)
	}
}

func TestStartCommandRequired(t *testing.T) {
	setupDB(t)
	p, _ := CreateProject(1, Project{Name: "启动参数", Slug: "start-args", Language: "java", RuntimeID: 1, Mode: ModeCompiled})
	relS, _ := SafeRelPath(1, p.ID, "src.zip", "source")
	relB, _ := SafeRelPath(1, p.ID, "app.jar", "binary")
	_ = AttachFile(p, "source", relS, "src.zip")
	_ = AttachFile(p, "binary", relB, "app.jar")
	models.DB.Model(&Project{}).Where("id = ?", p.ID).Update("upstream", "http://svc.example.com:9000")
	if err := SubmitForReview(p.ID, 1); err == nil {
		t.Error("未填启动参数不应允许提交")
	}
	models.DB.Model(&Project{}).Where("id = ?", p.ID).Update("start_command", "java -jar app.jar")
	if err := SubmitForReview(p.ID, 1); err != nil {
		t.Fatalf("填写启动参数后应可提交: %v", err)
	}
}
