package models

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupCommentSortTestDB 独立内存库（唯一名，避免与同包其他测试共享污染）
func setupCommentSortTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:cmtsort_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	DB = db
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// seedSortFloors 造三栋楼：
//
//	A 楼（最早建，2 条回复）、B 楼（最晚建，0 条回复）、C 楼（居中，1 条回复）
//
// 期望：new → B,C,A；old → A,C,B；hot → A,C,B
func seedSortFloors(t *testing.T) (a, b, c *Comment) {
	t.Helper()
	base := time.Now().Add(-3 * time.Hour)
	mk := func(content string, at time.Time) *Comment {
		cm := &Comment{UserID: 1, NovelID: 9001, Content: content, FloorNo: 1, CreatedAt: at, UpdatedAt: at}
		if err := DB.Create(cm).Error; err != nil {
			t.Fatalf("创建楼失败: %v", err)
		}
		return cm
	}
	a = mk("A楼", base)
	c = mk("C楼", base.Add(time.Hour))
	b = mk("B楼", base.Add(2*time.Hour))

	mkReply := func(root *Comment, content string, at time.Time) {
		rid := root.ID
		r := &Comment{UserID: 2, NovelID: 9001, Content: content, RootID: &rid, ParentID: &rid, CreatedAt: at, UpdatedAt: at}
		if err := DB.Create(r).Error; err != nil {
			t.Fatalf("创建回复失败: %v", err)
		}
	}
	mkReply(a, "A-1", base.Add(10*time.Minute))
	mkReply(a, "A-2", base.Add(20*time.Minute))
	mkReply(c, "C-1", base.Add(70*time.Minute))
	return a, b, c
}

// floorIDsOf 取分页结果里的楼层顺序（只保留顶层评论）
func floorIDsOf(pg CommentsPage) []uint {
	ids := make([]uint, 0, len(pg.List))
	for _, cm := range pg.List {
		if cm.ParentID == nil {
			ids = append(ids, cm.ID)
		}
	}
	return ids
}

func assertFloorOrder(t *testing.T, pg CommentsPage, want []uint, label string) {
	t.Helper()
	got := floorIDsOf(pg)
	if len(got) != len(want) {
		t.Fatalf("%s：楼层数应为 %d，得到 %d（%v）", label, len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s：第 %d 栋楼应为 %d，得到 %v", label, i+1, want[i], got)
		}
	}
	// 楼内回复必须始终按时间正序，不随楼层排序变化
	var prev time.Time
	for _, cm := range pg.List {
		if cm.ParentID == nil {
			continue
		}
		if !prev.IsZero() && cm.CreatedAt.Before(prev) {
			t.Fatalf("%s：楼内回复未按时间正序（%v 早于 %v）", label, cm.CreatedAt, prev)
		}
		prev = cm.CreatedAt
	}
}

// TestCommentsSortOrders 三种排序（new/old/hot）的楼层顺序与统计口径
func TestCommentsSortOrders(t *testing.T) {
	setupCommentSortTestDB(t)
	a, b, c := seedSortFloors(t)

	base := func(sort string) CommentsPage {
		pg, err := GetCommentsByNovel(9001, 0, 1, 20, false, sort)
		if err != nil {
			t.Fatalf("查询评论失败(sort=%q): %v", sort, err)
		}
		if pg.FloorTotal != 3 {
			t.Fatalf("sort=%q 楼层数应为 3，得到 %d", sort, pg.FloorTotal)
		}
		if pg.Total != 6 {
			t.Fatalf("sort=%q 评论总数应为 6（3 楼 + 3 回复），得到 %d", sort, pg.Total)
		}
		return pg
	}

	assertFloorOrder(t, base("new"), []uint{b.ID, c.ID, a.ID}, "new")
	assertFloorOrder(t, base("old"), []uint{a.ID, c.ID, b.ID}, "old")
	assertFloorOrder(t, base("hot"), []uint{a.ID, c.ID, b.ID}, "hot")
	// 非法/空缺排序值一律回落 new（白名单）
	assertFloorOrder(t, base(""), []uint{b.ID, c.ID, a.ID}, "空值回落 new")
	assertFloorOrder(t, base("id; DROP TABLE comments"), []uint{b.ID, c.ID, a.ID}, "非法值回落 new")
}

// TestCommentsSortHotIgnoresDeletedReplies 删除的楼内回复不计入热度
func TestCommentsSortHotIgnoresDeletedReplies(t *testing.T) {
	setupCommentSortTestDB(t)
	a, b, c := seedSortFloors(t)

	// 把 A 楼的两条回复删掉 → A 热度归零；C 楼仍有 1 条回复排第一，
	// A 与 B 同热度（0）时按建楼时间倒序 → B 在 A 前
	var replies []Comment
	DB.Where("root_id = ?", a.ID).Find(&replies)
	for _, r := range replies {
		if err := DeleteComment(r.ID); err != nil {
			t.Fatalf("删除回复失败: %v", err)
		}
	}

	pg, err := GetCommentsByNovel(9001, 0, 1, 20, false, "hot")
	if err != nil {
		t.Fatalf("查询评论失败: %v", err)
	}
	assertFloorOrder(t, pg, []uint{c.ID, b.ID, a.ID}, "hot（A 楼回复已删）")
}
