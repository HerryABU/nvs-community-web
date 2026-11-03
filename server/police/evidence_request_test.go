package police

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"nvs-server/models"
)

// setupDB 用独立命名的内存库（避免同名共享库在 -count=N / 重跑时复用数据）。
func setupDB(t *testing.T) {
	t.Helper()
	prev := models.DB
	dsn := fmt.Sprintf("file:police_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("打开内存库失败：%v", err)
	}
	if err := MigrateEvidenceRequest(db); err != nil {
		t.Fatalf("建表失败：%v", err)
	}
	models.DB = db
	t.Cleanup(func() { models.DB = prev })
}

// TestEvidenceRequest_ApprovalFlow 覆盖「警方申请 → 站长批准 → 校验放行」主链路。
func TestEvidenceRequest_ApprovalFlow(t *testing.T) {
	setupDB(t)

	const policeUID, targetUID, adminUID = uint(11), uint(22), uint(99)

	er, err := CreateEvidenceRequest(policeUID, targetUID, "alice", "涉嫌发布管制信息，依法调取")
	if err != nil {
		t.Fatalf("创建申请失败：%v", err)
	}
	if er.Status != ERPending {
		t.Fatalf("新申请状态应为 pending，实为 %s", er.Status)
	}
	if er.No == "" {
		t.Fatal("申请编号不应为空")
	}

	// 批准前：无任何已批准申请
	if HasApprovedRequest(policeUID, targetUID) {
		t.Fatal("批准前不应有已批准申请")
	}
	if HasAnyApproved(policeUID) {
		t.Fatal("批准前不应有任意已批准申请")
	}

	// 列表接口：管理员可见、警方可见自己的
	if _, total, err := ListEvidenceRequests(1, 20, ""); err != nil || total != 1 {
		t.Fatalf("管理员列表应含 1 条：total=%d err=%v", total, err)
	}
	if _, total, err := ListMyEvidenceRequests(policeUID, 1, 20); err != nil || total != 1 {
		t.Fatalf("我的申请应含 1 条：total=%d err=%v", total, err)
	}

	// 站长批准
	if err := ReviewEvidenceRequest(er.ID, adminUID, true, "情况属实，准予调取"); err != nil {
		t.Fatalf("审批失败：%v", err)
	}

	// 批准后校验
	if !HasApprovedRequest(policeUID, targetUID) {
		t.Fatal("批准后应存在针对该目标的已批准申请")
	}
	if !HasAnyApproved(policeUID) {
		t.Fatal("批准后应存在任意已批准申请")
	}

	// 重复审批应被拒
	if err := ReviewEvidenceRequest(er.ID, adminUID, true, ""); err == nil {
		t.Fatal("重复审批应返回错误")
	}

	// 状态回填正确
	got, err := GetEvidenceRequest(er.ID)
	if err != nil {
		t.Fatalf("读取申请失败：%v", err)
	}
	if got.Status != ERApproved || got.ReviewerID != adminUID || got.ReviewNote == "" {
		t.Fatalf("审批后字段回填异常：%s / reviewer=%d / note=%q", got.Status, got.ReviewerID, got.ReviewNote)
	}
}

// TestEvidenceRequest_Reject 驳回须放行（并返回 rejected 状态）。
func TestEvidenceRequest_Reject(t *testing.T) {
	setupDB(t)
	er, err := CreateEvidenceRequest(7, 8, "bob", "线索待核实")
	if err != nil {
		t.Fatalf("创建申请失败：%v", err)
	}
	if err := ReviewEvidenceRequest(er.ID, 1, false, "理由不充分，暂缓"); err != nil {
		t.Fatalf("驳回失败：%v", err)
	}
	if HasApprovedRequest(7, 8) {
		t.Fatal("驳回后不应视为已批准")
	}
	got, _ := GetEvidenceRequest(er.ID)
	if got.Status != ERRejected {
		t.Fatalf("状态应为 rejected，实为 %s", got.Status)
	}
}

// TestCreateEvidenceRequest_Validation 空理由 / 非法参数应报错。
func TestCreateEvidenceRequest_Validation(t *testing.T) {
	setupDB(t)
	if _, err := CreateEvidenceRequest(0, 1, "", "x"); err == nil {
		t.Fatal("policeUserID=0 应报错")
	}
	if _, err := CreateEvidenceRequest(1, 0, "", "x"); err == nil {
		t.Fatal("targetUserID=0 应报错")
	}
	if _, err := CreateEvidenceRequest(1, 2, "", ""); err == nil {
		t.Fatal("空理由应报错")
	}
}
