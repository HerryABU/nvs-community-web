package chaptervcs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nvs-server/config"
	"nvs-server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupVCSDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:chaptervcs_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("主迁移失败: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("版本表迁移失败: %v", err)
	}
	old := models.DB
	models.DB = db
	// 快照落到临时目录（测试结束清理）
	dir := t.TempDir()
	oldDir := config.NovelDataDir
	config.NovelDataDir = dir
	t.Cleanup(func() {
		models.DB = old
		config.NovelDataDir = oldDir
		_ = os.RemoveAll(dir)
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func mkChapter(t *testing.T, novelID uint, num int) *models.Chapter {
	t.Helper()
	ch := &models.Chapter{NovelID: novelID, ChapterNumber: num, Title: "第1章", Status: "draft"}
	if err := models.DB.Create(ch).Error; err != nil {
		t.Fatalf("建章节失败: %v", err)
	}
	return ch
}

// TestCommitAndHistory 连续提交 → 版本号递增、快照可读、历史可列
func TestCommitAndHistory(t *testing.T) {
	setupVCSDB(t)
	ch := mkChapter(t, 1, 1)

	v1, conflict, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 1, Title: "第1章", Content: "甲：一\n"})
	if err != nil || conflict != nil || v1.Version != 1 {
		t.Fatalf("首次提交失败: v=%+v conflict=%+v err=%v", v1, conflict, err)
	}
	v2, _, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 1, Title: "第1章", Content: "甲：一\n乙：二\n"})
	if err != nil || v2.Version != 2 || v2.ParentVer != 1 {
		t.Fatalf("第二次提交失败: %+v err=%v", v2, err)
	}
	// 内容未变 → 不产生新版本
	v3, _, _ := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 1, Title: "第1章", Content: "甲：一\n乙：二\n"})
	if v3.Version != 2 {
		t.Fatalf("内容未变却产生了新版本: %d", v3.Version)
	}
	list, err := List(ch.ID, "main")
	if err != nil || len(list) != 2 {
		t.Fatalf("历史列表错误: %d err=%v", len(list), err)
	}
	if !list[0].IsHead {
		t.Fatalf("历史首条应为 head: %+v", list[0])
	}
	content, err := Content(1, 1, v2)
	if err != nil || !strings.Contains(content, "乙：二") {
		t.Fatalf("读取快照失败: %q err=%v", content, err)
	}
	if _, err := os.Stat(filepath.Join(config.NovelDataDir, "authors", "1", "1", ".versions", "1", "main", "2.md")); err != nil {
		t.Fatalf("快照文件未落盘: %v", err)
	}
}

// TestConflict 基于旧版本保存 → 拒绝覆盖并返回冲突现场；force 后强制提交
func TestConflict(t *testing.T) {
	setupVCSDB(t)
	ch := mkChapter(t, 2, 1)
	if _, _, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 1, Title: "t", Content: "A\n"}); err != nil {
		t.Fatal(err)
	}
	// 别人改了一版
	if _, _, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 2, Title: "t", Content: "A\nB\n"}); err != nil {
		t.Fatal(err)
	}
	// 我基于 v1 保存 → 冲突
	_, cf, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 1, Title: "t", Content: "A\nC\n", BaseVer: 1})
	if err != nil {
		t.Fatal(err)
	}
	if cf == nil || cf.ServerVersion != 2 || cf.BaseVersion != 1 {
		t.Fatalf("未检测到冲突: %+v", cf)
	}
	if !strings.Contains(cf.ServerContent, "B") || !strings.Contains(cf.MineContent, "C") {
		t.Fatalf("冲突现场内容缺失: %+v", cf)
	}
	// 未强制时不应产生新版本
	if h := Head(ch.ID, "main"); h != 2 {
		t.Fatalf("冲突时不应推进 head: %d", h)
	}
	// force → 以我的为准
	v, cf2, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 1, Title: "t", Content: "A\nC\n", BaseVer: 1, Force: true})
	if err != nil || cf2 != nil || v.Version != 3 || !v.Force {
		t.Fatalf("强制提交失败: %+v cf=%+v err=%v", v, cf2, err)
	}
}

// TestRevertAndDelete 回滚生成新版本；删除历史只清正文且不能删 head
func TestRevertAndDelete(t *testing.T) {
	setupVCSDB(t)
	ch := mkChapter(t, 3, 1)
	v1, _, _ := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 1, Title: "t", Content: "one\n"})
	if _, _, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 1, Title: "t", Content: "two\n"}); err != nil {
		t.Fatal(err)
	}
	rv, err := Revert(1, 3, ch, v1, 1)
	if err != nil || rv.Version != 3 {
		t.Fatalf("回滚失败: %+v err=%v", rv, err)
	}
	if c, _ := Content(1, 3, rv); c != "one\n" {
		t.Fatalf("回滚内容不对: %q", c)
	}
	// 不能删 head
	if err := DeleteVersion(1, 3, ch.ID, "main", 3); err == nil {
		t.Fatal("删除 head 应被拒绝")
	}
	// 删除中间版本：元数据保留，正文清空
	if err := DeleteVersion(1, 3, ch.ID, "main", 2); err != nil {
		t.Fatalf("删除历史失败: %v", err)
	}
	got, _ := Get(ch.ID, "main", 2)
	if !got.Deleted || got.ContentPath != "" {
		t.Fatalf("删除后元数据异常: %+v", got)
	}
	if c, _ := Content(1, 3, got); c != "" {
		t.Fatalf("删除后仍可读到正文: %q", c)
	}
}

// TestBranchAndMerge 开分支 → 两侧各改 → 无冲突自动合并 / 同处冲突报冲突
func TestBranchAndMerge(t *testing.T) {
	setupVCSDB(t)
	ch := mkChapter(t, 4, 1)
	if _, _, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 1, Title: "t",
		Content: "L1\nL2\nL3\nL4\nL5\n"}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateBranch(1, 4, ch, "rewrite", "main", 1, 2); err != nil {
		t.Fatalf("开分支失败: %v", err)
	}
	// 分支改末尾，主干改开头 → 不冲突
	if _, _, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 2, Title: "t",
		Content: "L1\nL2\nL3\nL4\nL5-新\n", Branch: "rewrite"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 1, Title: "t",
		Content: "L1-改\nL2\nL3\nL4\nL5\n"}); err != nil {
		t.Fatal(err)
	}
	res, err := Merge(1, 4, ch, "rewrite", "main", 1, "")
	if err != nil {
		t.Fatalf("合并失败: %v", err)
	}
	if res.Conflict {
		t.Fatalf("不应冲突: %+v", res.Conflicts)
	}
	if !strings.Contains(res.Merged, "L1-改") || !strings.Contains(res.Merged, "L5-新") {
		t.Fatalf("合并结果缺内容: %q", res.Merged)
	}
	// 两侧同时改同一行 → 冲突
	if _, _, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 2, Title: "t",
		Content: "L1-改\nL2\nL3\nL4\nL5-新2\n", Branch: "rewrite"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 1, Title: "t",
		Content: "L1-改\nL2\nL3\nL4\nL5-主\n"}); err != nil {
		t.Fatal(err)
	}
	res2, err := Merge(1, 4, ch, "rewrite", "main", 1, "")
	if err != nil {
		t.Fatalf("合并失败: %v", err)
	}
	if !res2.Conflict || len(res2.Conflicts) == 0 {
		t.Fatal("同处修改应报冲突")
	}
}

// TestDiffLines 行级差异统计
func TestDiffLines(t *testing.T) {
	r := DiffLines("a\nb\nc\n", "a\nb2\nc\nd\n")
	if r.Added != 2 || r.Removed != 1 { // b→b2 计 1 删 1 增，d 计 1 增
		t.Fatalf("diff 统计错误: +%d -%d", r.Added, r.Removed)
	}
	eq := DiffLines("x\ny\n", "x\ny\n")
	// "x\ny\n" 切行后为 ["x","y",""]（末尾空行保留，避免写入时丢换行）
	if eq.Added != 0 || eq.Removed != 0 || len(eq.Lines) != 3 {
		t.Fatalf("相同内容应为全等: %+v", eq)
	}
}
