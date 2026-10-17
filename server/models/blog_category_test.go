package models

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupTestDB 创建内存 sqlite 测试库并迁移全部表，替换全局 models.DB。
// 每个测试独立调用，避免相互污染。
func setupTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存测试库失败: %v", err)
	}
	if err := AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移测试库失败: %v", err)
	}
	DB = db
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
}

// seedTestBlog 造一篇测试博客
func seedTestBlog(t *testing.T, authorID, categoryID uint, title string) *AuthorBlog {
	t.Helper()
	b := &AuthorBlog{AuthorID: authorID, Title: title, CategoryID: categoryID}
	if err := CreateBlog(b); err != nil {
		t.Fatalf("创建测试博客失败: %v", err)
	}
	return b
}

// TestToggleBlogLike 点赞 toggle：点赞+1 → 再点取消-1；不会为负数
func TestToggleBlogLike(t *testing.T) {
	setupTestDB(t)
	b := seedTestBlog(t, 1, 0, "点赞测试博客")

	liked, err := ToggleBlogLike(b.ID, 100)
	if err != nil || !liked {
		t.Fatalf("首次点赞应返回 liked=true: %v", err)
	}
	var cnt int64
	DB.Model(&AuthorBlog{}).Where("id = ?", b.ID).Pluck("like_count", &cnt)
	// 重新查询避免缓存
	var fresh AuthorBlog
	DB.First(&fresh, b.ID)
	if fresh.LikeCount != 1 {
		t.Errorf("点赞后 like_count 应为 1，实际 %d", fresh.LikeCount)
	}
	if !UserLikedBlog(b.ID, 100) {
		t.Errorf("点赞后 UserLikedBlog 应为 true")
	}

	// 再次点赞 → 取消
	liked, err = ToggleBlogLike(b.ID, 100)
	if err != nil || liked {
		t.Fatalf("第二次点赞应取消（liked=false）: %v", err)
	}
	DB.First(&fresh, b.ID)
	if fresh.LikeCount != 0 {
		t.Errorf("取消后 like_count 应为 0，实际 %d", fresh.LikeCount)
	}
	if UserLikedBlog(b.ID, 100) {
		t.Errorf("取消后 UserLikedBlog 应为 false")
	}

	// 不同用户各点一次 → 2
	ToggleBlogLike(b.ID, 101)
	ToggleBlogLike(b.ID, 102)
	DB.First(&fresh, b.ID)
	if fresh.LikeCount != 2 {
		t.Errorf("3 个不同用户点赞后 like_count 应为 2，实际 %d", fresh.LikeCount)
	}
}

// TestGetBlogCategoryName 分类名（含父级前缀）/ 不存在 / 0
func TestGetBlogCategoryName(t *testing.T) {
	setupTestDB(t)
	p := &BlogCategory{Name: "技术", ParentID: 0, Status: "approved"}
	DB.Create(p)
	s := &BlogCategory{Name: "Go 开发", ParentID: p.ID, Status: "approved"}
	DB.Create(s)

	if got := GetBlogCategoryName(p.ID); got != "技术" {
		t.Errorf("一级分类名 = %q，期望 技术", got)
	}
	if got := GetBlogCategoryName(s.ID); got != "技术/Go 开发" {
		t.Errorf("二级分类名 = %q，期望 技术/Go 开发", got)
	}
	if got := GetBlogCategoryName(0); got != "" {
		t.Errorf("category_id=0 应返回空，实际 %q", got)
	}
	if got := GetBlogCategoryName(99999); got != "" {
		t.Errorf("不存在的分类应返回空，实际 %q", got)
	}
}

// TestGetBlogCategoryTree 分类树：一级挂二级 + blog_count 统计
func TestGetBlogCategoryTree(t *testing.T) {
	setupTestDB(t)
	p := &BlogCategory{Name: "技术", Status: "approved"}
	DB.Create(p)
	s1 := &BlogCategory{Name: "Go", ParentID: p.ID, Status: "approved"}
	DB.Create(s1)
	s2 := &BlogCategory{Name: "Python", ParentID: p.ID, Status: "approved"}
	DB.Create(s2)
	orphan := &BlogCategory{Name: "随笔", Status: "approved"}
	DB.Create(orphan)

	seedTestBlog(t, 1, p.ID, "博客A")  // 一级分类直接挂博客
	seedTestBlog(t, 1, s1.ID, "博客B") // 二级分类挂博客
	seedTestBlog(t, 1, 0, "未分类博客")   // 未分类

	tree := GetBlogCategoryTree()
	if len(tree) != 2 {
		t.Fatalf("树应含 2 个一级分类（技术/随笔），实际 %d", len(tree))
	}
	var tech *BlogCategoryNode
	for i := range tree {
		if tree[i].Name == "技术" {
			tech = &tree[i]
		}
	}
	if tech == nil {
		t.Fatal("找不到「技术」分类")
	}
	// 一级分类 blog_count 应含直接挂载 + 其下二级（博客A 直接挂一级 + 博客B 挂二级 = 2）
	if tech.BlogCount != 2 {
		t.Errorf("「技术」blog_count 应为 2（含二级），实际 %d", tech.BlogCount)
	}
	if len(tech.Children) != 2 {
		t.Errorf("「技术」应有 2 个子分类，实际 %d", len(tech.Children))
	}
	// 二级分类 blog_count 只统计自己直接挂载：Go=1（博客B）、Python=0
	if tech.Children[0].BlogCount != 1 || tech.Children[1].BlogCount != 0 {
		t.Errorf("二级分类 blog_count 不符: %+v", tech.Children)
	}

	if !BlogCategoryHasChildren(p.ID) {
		t.Errorf("「技术」应有子分类")
	}
	if BlogCategoryHasChildren(s1.ID) || BlogCategoryHasChildren(orphan.ID) {
		t.Errorf("二级/无子分类不应有子分类")
	}
}

// TestAuthorBlogDeletedBy 用户自删 → deleted_by='user'，且列表查询排除
func TestAuthorBlogDeletedBy(t *testing.T) {
	setupTestDB(t)
	b := seedTestBlog(t, 1, 0, "删除测试博客")
	if err := DeleteBlog(b.ID); err != nil {
		t.Fatalf("删除博客失败: %v", err)
	}
	// 模拟 handler 写入 deleted_by
	DB.Unscoped().Model(&AuthorBlog{}).Where("id = ?", b.ID).Update("deleted_by", "user")

	var inTrash AuthorBlog
	if err := DB.Unscoped().First(&inTrash, b.ID).Error; err != nil {
		t.Fatalf("软删除记录应保留: %v", err)
	}
	if inTrash.DeletedBy != "user" {
		t.Errorf("deleted_by 应为 user，实际 %q", inTrash.DeletedBy)
	}
	if !inTrash.DeletedAt.Valid {
		t.Errorf("deleted_at 应为有效软删除标记")
	}

	// 正常查询（非 Unscoped）应查不到
	var cnt int64
	DB.Model(&AuthorBlog{}).Where("id = ?", b.ID).Count(&cnt)
	if cnt != 0 {
		t.Errorf("软删除博客不应出现在普通查询中，count=%d", cnt)
	}
}
