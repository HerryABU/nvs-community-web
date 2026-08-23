package models

import "testing"

// TestBlogTags 博客标签：同步（去重/去#）/读取/标签云/按标签查 id
func TestBlogTags(t *testing.T) {
	setupTestDB(t)

	// 建两篇博客
	b1 := &AuthorBlog{AuthorID: 1, Title: "标签博客1"}
	if err := DB.Create(b1).Error; err != nil {
		t.Fatalf("创建博客失败: %v", err)
	}
	b2 := &AuthorBlog{AuthorID: 1, Title: "标签博客2"}
	if err := DB.Create(b2).Error; err != nil {
		t.Fatalf("创建博客失败: %v", err)
	}

	// 同步标签（含 # 前缀、重复、多余空格）
	if err := SyncBlogTags(b1.ID, []string{"#Go", "Go", " 并发编程 ", "Go"}); err != nil {
		t.Fatalf("SyncBlogTags 失败: %v", err)
	}
	tags1 := GetBlogTags(b1.ID)
	if len(tags1) != 2 {
		t.Fatalf("去重后应有 2 个标签，实际 %d: %+v", len(tags1), tags1)
	}
	names := map[string]bool{}
	for _, tg := range tags1 {
		names[tg.Name] = true
	}
	if !names["Go"] || !names["并发编程"] {
		t.Errorf("标签名应为 Go/并发编程，实际 %+v", tags1)
	}

	// 博客2 打同一标签
	if err := SyncBlogTags(b2.ID, []string{"Go"}); err != nil {
		t.Fatalf("SyncBlogTags b2 失败: %v", err)
	}

	// 标签云：Go count=2，并发编程 count=1
	cloud := GetBlogTagCloud(10)
	if len(cloud) != 2 {
		t.Fatalf("标签云应有 2 个标签，实际 %d", len(cloud))
	}
	if cloud[0].Name != "Go" || cloud[0].Count != 2 {
		t.Errorf("Go 应排第一且 count=2，实际 %+v", cloud[0])
	}

	// 按标签查博客 id
	ids, err := GetBlogIDsByTag("Go")
	if err != nil || len(ids) != 2 {
		t.Errorf("按标签 Go 应查到 2 篇博客，实际 %v err=%v", ids, err)
	}

	// 重建标签（清空）
	if err := SyncBlogTags(b1.ID, nil); err != nil {
		t.Fatalf("清空标签失败: %v", err)
	}
	if len(GetBlogTags(b1.ID)) != 0 {
		t.Errorf("清空后 b1 应无标签")
	}
	// b2 的 Go 标签仍在
	ids2, _ := GetBlogIDsByTag("Go")
	if len(ids2) != 1 {
		t.Errorf("清空 b1 后，Go 标签应只剩 b2，实际 %v", ids2)
	}
}

// TestAuthorCategoryTree 作者个人专栏分类树（仅自己 approved，非自己父分类上提）
func TestAuthorCategoryTree(t *testing.T) {
	setupTestDB(t)

	// 作者 1：一级 A + 二级 A1 + 三级 A1a（全部 approved）
	ca := &BlogCategory{Name: "A", ParentID: 0, Status: "approved", CreatedBy: 1}
	DB.Create(ca)
	ca1 := &BlogCategory{Name: "A1", ParentID: ca.ID, Status: "approved", CreatedBy: 1}
	DB.Create(ca1)
	ca1a := &BlogCategory{Name: "A1a", ParentID: ca1.ID, Status: "approved", CreatedBy: 1}
	DB.Create(ca1a)
	// 作者 1 的 pending 分类（不出现在公开专栏）
	cp := &BlogCategory{Name: "A待审", ParentID: 0, Status: "pending", CreatedBy: 1}
	DB.Create(cp)
	// 作者 2 的分类 B；作者 1 的分类挂在 B 下（父非本人 → 上提为一级）
	cb := &BlogCategory{Name: "B", ParentID: 0, Status: "approved", CreatedBy: 2}
	DB.Create(cb)
	cx := &BlogCategory{Name: "X挂B下", ParentID: cb.ID, Status: "approved", CreatedBy: 1}
	DB.Create(cx)

	tree := GetAuthorCategoryTree(1)
	if len(tree) != 2 {
		t.Fatalf("作者1 专栏应有 2 个一级分类（A、X挂B下上提；无 pending），实际 %d", len(tree))
	}
	byName := map[string]BlogCategoryNode{}
	for _, n := range tree {
		byName[n.Name] = n
	}
	if _, ok := byName["A待审"]; ok {
		t.Errorf("pending 分类不应出现在公开专栏")
	}
	if _, ok := byName["B"]; ok {
		t.Errorf("作者2 的分类不应出现在作者1 专栏")
	}
	a := byName["A"]
	if len(a.Children) != 1 || a.Children[0].Name != "A1" {
		t.Errorf("A 应有子分类 A1，实际 %+v", a.Children)
	}
	if len(a.Children[0].Children) != 1 || a.Children[0].Children[0].Name != "A1a" {
		t.Errorf("A1 应有三级子分类 A1a")
	}
	x := byName["X挂B下"]
	if x.ParentID != 0 {
		t.Errorf("挂在他人分类下的分类应上提为一级，实际 parent_id=%d", x.ParentID)
	}
}
