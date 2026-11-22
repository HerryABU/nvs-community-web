package selfhost

import (
	"testing"

	"nvs-server/models"
)

func seedAccessProject(t *testing.T) *Project {
	t.Helper()
	p, err := CreateProject(1, Project{Name: "授权测试", Slug: "access-test", Language: "java", RuntimeID: 1, Mode: ModeCompiled})
	if err != nil {
		t.Fatal(err)
	}
	relS, _ := SafeRelPath(1, p.ID, "src.zip", "source")
	relB, _ := SafeRelPath(1, p.ID, "app.jar", "binary")
	if err := AttachFile(p, "source", relS, "src.zip"); err != nil {
		t.Fatal(err)
	}
	if err := AttachFile(p, "binary", relB, "app.jar"); err != nil {
		t.Fatal(err)
	}
	if err := models.DB.Model(&Project{}).Where("id = ?", p.ID).
		Updates(map[string]interface{}{"review_status": ReviewApproved, "visibility": VisPublic, "upstream": "http://svc.example.com:8080"}).Error; err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFileAccessDefaultDeny(t *testing.T) {
	setupDB(t)
	p := seedAccessProject(t)
	if p.SourceShared || p.BinaryShared {
		t.Fatal("共享开关默认必须为 false")
	}
	if CanDownload(p, 0, false, "source") {
		t.Error("匿名用户不应能下载源码")
	}
	if CanDownload(p, 2, false, "source") || CanDownload(p, 2, false, "binary") {
		t.Error("其他用户默认不应能下载任何文件")
	}
	if !CanDownload(p, 1, false, "source") || !CanDownload(p, 2, true, "binary") {
		t.Error("所有者与管理员应始终可下载")
	}
}

func TestFileAccessSharedAndGrant(t *testing.T) {
	setupDB(t)
	p := seedAccessProject(t)

	// 开启源码共享后，只放开源码，可执行文件仍需单独授权。
	if err := SetSharing(p.ID, 1, true, false, false); err != nil {
		t.Fatal(err)
	}
	p, _ = GetProjectRaw(p.ID)
	if !CanDownload(p, 2, false, "source") || CanDownload(p, 2, false, "binary") {
		t.Error("共享开关应只放开对应类型")
	}

	// 未审批的申请不能下载。
	g, err := RequestGrant(p.ID, 2, "binary", "需要验证构建产物")
	if err != nil {
		t.Fatal(err)
	}
	if CanDownload(p, 2, false, "binary") {
		t.Error("待审批授权不应生效")
	}
	// 非所有者不能审批。
	if err := ReviewGrant(g.ID, 3, true); err == nil {
		t.Error("非所有者审批应被拒绝")
	}
	if err := ReviewGrant(g.ID, 1, true); err != nil {
		t.Fatal(err)
	}
	if !CanDownload(p, 2, false, "binary") {
		t.Error("获批授权应能下载")
	}
	if err := RevokeGrant(g.ID, 1); err != nil {
		t.Fatal(err)
	}
	if CanDownload(p, 2, false, "binary") {
		t.Error("撤销后不应再能下载")
	}
	grants := GrantStatusFor(p.ID, 2)
	if len(grants) != 0 {
		t.Errorf("撤销后不应有授权记录: %d", len(grants))
	}
}
