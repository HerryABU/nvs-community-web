package models

import "testing"

// TestBlogTopics 博客话题：同步（去重/去#）/读取/列表统计
func TestBlogTopics(t *testing.T) {
	setupTestDB(t)

	b1 := &AuthorBlog{AuthorID: 1, Title: "话题博客1"}
	if err := DB.Create(b1).Error; err != nil {
		t.Fatalf("创建博客失败: %v", err)
	}
	b2 := &AuthorBlog{AuthorID: 1, Title: "话题博客2"}
	if err := DB.Create(b2).Error; err != nil {
		t.Fatalf("创建博客失败: %v", err)
	}

	// 同步话题（去 # / 去重）
	if err := SyncBlogTopics(b1.ID, []string{"#Go 并发", "Go 并发", "Web 开发"}); err != nil {
		t.Fatalf("SyncBlogTopics 失败: %v", err)
	}
	topics1 := GetBlogTopics(b1.ID)
	if len(topics1) != 2 {
		t.Fatalf("去重后应有 2 个话题，实际 %d: %+v", len(topics1), topics1)
	}

	// 博客2 参与同一话题
	if err := SyncBlogTopics(b2.ID, []string{"Go 并发"}); err != nil {
		t.Fatalf("SyncBlogTopics b2 失败: %v", err)
	}

	// 列表统计：Go 并发 count=2，Web 开发 count=1
	list := ListBlogTopics(10)
	if len(list) != 2 {
		t.Fatalf("话题列表应有 2 个，实际 %d", len(list))
	}
	if list[0].Name != "Go 并发" || list[0].BlogCount != 2 {
		t.Errorf("Go 并发应排第一且 count=2，实际 %+v", list[0])
	}

	// 按名查
	tp, err := GetBlogTopicByName("Go 并发")
	if err != nil || tp == nil {
		t.Errorf("按名查话题失败: %v", err)
	}

	// 清空
	if err := SyncBlogTopics(b1.ID, nil); err != nil {
		t.Fatalf("清空话题失败: %v", err)
	}
	if len(GetBlogTopics(b1.ID)) != 0 {
		t.Errorf("清空后 b1 应无话题")
	}
}
