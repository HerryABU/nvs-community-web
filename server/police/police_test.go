package police

import (
	"testing"

	"nvs-server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupPoliceDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:police_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	models.DB = db
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_ = db.Exec("DELETE FROM police_tickets").Error
	_ = db.Exec("DELETE FROM police_ticket_logs").Error
}

// TestTicketLifecycle 建单 → 派单 → 进展 → 结单（结论必填）+ 日志完整
func TestTicketLifecycle(t *testing.T) {
	setupPoliceDB(t)
	tk, err := Create(1, CreateInput{
		Kind: KindEvidence, Priority: PriUrgent, Title: "协助取证：批量注册",
		Summary: "某 IP 在蜜罐留下高风险记录并批量注册", SessionID: 12, DueHours: 24,
	})
	if err != nil {
		t.Fatal(err)
	}
	if tk.Status != StOpen || tk.Priority != PriUrgent || tk.No == "" {
		t.Fatalf("建单字段错: %+v", tk)
	}
	if err := Assign(tk.ID, 8, 2); err != nil {
		t.Fatal(err)
	}
	got, logs, err := Get(tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.HandlerID != 8 || got.Status != StProcessing {
		t.Fatalf("派单后应转处理中: %+v", got)
	}
	if err := AddNote(tk.ID, 8, "已联系对方站长"); err != nil {
		t.Fatal(err)
	}
	// 结单必须写结论
	if err := SetStatus(tk.ID, 8, StClosed, ""); err == nil {
		t.Fatal("空结论的结单应被拒绝")
	}
	if err := SetStatus(tk.ID, 8, StClosed, "已封禁相关账号并留存证据"); err != nil {
		t.Fatal(err)
	}
	got2, logs2, _ := Get(tk.ID)
	if got2.Status != StClosed || got2.ClosedAt == nil || got2.Result == "" {
		t.Fatalf("结单信息不全: %+v", got2)
	}
	// create + assign + comment + close
	if len(logs2) < 4 {
		t.Fatalf("日志应至少 4 条，实际 %d（首条 %d）", len(logs2), len(logs))
	}
}

// TestTicketStats 超期与未派单口径
func TestTicketStats(t *testing.T) {
	setupPoliceDB(t)
	_, _ = Create(1, CreateInput{Title: "A", DueHours: -1}) // 已超期
	_, _ = Create(1, CreateInput{Title: "B"})               // 未派单
	_, _ = Create(1, CreateInput{Title: "C", Priority: PriLow})
	st := Stats()
	by := st["by_status"].(map[string]int64)
	if by[StOpen] != 3 {
		t.Fatalf("待受理应为 3: %+v", by)
	}
	if st["overdue"].(int64) < 1 {
		t.Fatalf("应有超期单: %+v", st)
	}
	if st["unassigned"].(int64) != 3 {
		t.Fatalf("未派单应为 3: %+v", st)
	}
}

// TestTicketListOrder 未结单 + 高优在前
func TestTicketListOrder(t *testing.T) {
	setupPoliceDB(t)
	a, err := Create(1, CreateInput{Title: "低优", Priority: PriLow})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Create(1, CreateInput{Title: "紧急", Priority: PriUrgent})
	if err != nil {
		t.Fatal(err)
	}
	_ = SetStatus(a.ID, 1, StClosed, "已处理")
	list, total, err := List(1, 10, "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("总数应为 2: %d", total)
	}
	if list[0].ID != b.ID {
		t.Fatalf("未结单的紧急单应排第一: %+v", list[0])
	}
}

func TestCreateRejectsEmptyTitle(t *testing.T) {
	setupPoliceDB(t)
	if _, err := Create(1, CreateInput{Title: "  "}); err == nil {
		t.Fatal("空标题应拒绝")
	}
}
