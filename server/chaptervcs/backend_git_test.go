//go:build gitvcs

package chaptervcs

import (
	"os"
	"strings"
	"testing"

	"nvs-server/config"
	"nvs-server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// mkNovelGit 建作品（git 后端需要按 novel_id 反查主笔与作品目录）
func mkNovelGit(t *testing.T, title string) *models.Novel {
	t.Helper()
	n := &models.Novel{AuthorID: 1, Title: title, Category: "科幻", Status: "published"}
	if err := models.DB.Create(n).Error; err != nil {
		t.Fatalf("建作品失败: %v", err)
	}
	return n
}

func setupGitTest(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:chaptervcs_git_test?mode=memory&cache=shared"), &gorm.Config{})
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
	dir := t.TempDir()
	oldDir := config.NovelDataDir
	config.NovelDataDir = dir
	oldBackend := Current
	if err := SetBackend("git"); err != nil {
		t.Fatalf("启用 git 后端失败: %v", err)
	}
	t.Cleanup(func() {
		Current = oldBackend
		models.DB = old
		config.NovelDataDir = oldDir
		_ = os.RemoveAll(dir)
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
}

// TestGitBackendCommitAndHistory 提交 → 历史 → 读取快照
func TestGitBackendCommitAndHistory(t *testing.T) {
	setupGitTest(t)
	novel := mkNovelGit(t, "git测试1")
	ch := &models.Chapter{NovelID: novel.ID, ChapterNumber: 1, Title: "第一章", Status: "draft"}
	if err := models.DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	v1, cf, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 1, Title: "第一章", Content: "甲：一\n"})
	if err != nil || cf != nil || v1.Version != 1 {
		t.Fatalf("git 提交失败: %+v cf=%+v err=%v", v1, cf, err)
	}
	v2, _, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 1, Title: "第一章", Content: "甲：一\n乙：二\n"})
	if err != nil || v2.Version != 2 {
		t.Fatalf("第二次提交失败: %+v err=%v", v2, err)
	}
	c, err := Content(1, novel.ID, v2)
	if err != nil || !strings.Contains(c, "乙：二") {
		t.Fatalf("读取 git 快照失败: %q err=%v", c, err)
	}
	list, err := List(ch.ID, "main")
	if err != nil || len(list) < 2 {
		t.Fatalf("git 历史列表错误: %d err=%v", len(list), err)
	}
	if Head(ch.ID, "main") < 2 {
		t.Fatalf("git head 错误: %d", Head(ch.ID, "main"))
	}
	// git 历史不可删
	if err := DeleteVersion(1, novel.ID, ch.ID, "main", 1); err == nil {
		t.Fatal("git 后端删除历史应被拒绝")
	}
}

// TestGitBackendConflictAndRevert 冲突检测与回滚
func TestGitBackendConflictAndRevert(t *testing.T) {
	setupGitTest(t)
	novel := mkNovelGit(t, "git测试2")
	ch := &models.Chapter{NovelID: novel.ID, ChapterNumber: 1, Title: "t", Status: "draft"}
	if err := models.DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 1, Title: "t", Content: "A\n"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 2, Title: "t", Content: "A\nB\n"}); err != nil {
		t.Fatal(err)
	}
	_, cf, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 1, Title: "t", Content: "A\nC\n", BaseVer: 1})
	if err != nil || cf == nil || cf.ServerVersion != 2 {
		t.Fatalf("git 冲突检测失败: cf=%+v err=%v", cf, err)
	}
	v1, _ := Get(ch.ID, "main", 1)
	rv, err := Revert(1, novel.ID, ch, v1, 1)
	if err != nil {
		t.Fatalf("git 回滚失败: %v", err)
	}
	if c, _ := Content(1, novel.ID, rv); !strings.Contains(c, "A") {
		t.Fatalf("git 回滚内容错误: %q", c)
	}
}

// TestGitBackendBranchMerge 分支与三方合并（无冲突自动合 / 同处冲突报错）
func TestGitBackendBranchMerge(t *testing.T) {
	setupGitTest(t)
	novel := mkNovelGit(t, "git测试3")
	ch := &models.Chapter{NovelID: novel.ID, ChapterNumber: 1, Title: "t", Status: "draft"}
	if err := models.DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	base := "L1\nL2\nL3\nL4\nL5\n"
	if _, _, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 1, Title: "t", Content: base}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateBranch(1, novel.ID, ch, "rewrite", "main", 1, 2); err != nil {
		t.Fatalf("git 开分支失败: %v", err)
	}
	if _, _, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 2, Title: "t", Content: "L1\nL2\nL3\nL4\nL5-新\n", Branch: "rewrite"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Commit(CommitInput{OwnerID: 1, Chapter: ch, AuthorID: 1, Title: "t", Content: "L1-改\nL2\nL3\nL4\nL5\n"}); err != nil {
		t.Fatal(err)
	}
	res, err := Merge(1, novel.ID, ch, "rewrite", "main", 1, "")
	if err != nil {
		t.Fatalf("git 合并失败: %v", err)
	}
	if res.Conflict {
		t.Fatalf("不应冲突: %+v", res.Conflicts)
	}
	if !strings.Contains(res.Merged, "L1-改") || !strings.Contains(res.Merged, "L5-新") {
		t.Fatalf("git 合并结果缺内容: %q", res.Merged)
	}
}
