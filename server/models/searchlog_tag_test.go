package models_test

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"nvs-server/models"
	"nvs-server/models/blog"
)

// TestAggregateTagHotWords 验证标签热词聚合（博客结构化标签 + 作品 JSON 标签）。
// 放在外部测试包 models_test：根包（models）不可反向 import 拆出的 blog 子包，
// 而本测试需要直接造博客数据。zz_domain_import_test.go 已 blank-import blog 触发其
// init() 注册，使 AutoMigrateAll 一并迁移博客表。
func TestAggregateTagHotWords(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:taghot?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	models.DB = db
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})

	// 1) 博客结构化标签
	tagA := blog.BlogTag{Name: "修真"}
	tagB := blog.BlogTag{Name: "科幻"}
	models.DB.Create(&tagA)
	models.DB.Create(&tagB)
	blog1 := blog.AuthorBlog{AuthorID: 1, Title: "博客一", ContentPath: "x", CreatedAt: time.Now()}
	blog2 := blog.AuthorBlog{AuthorID: 1, Title: "博客二", ContentPath: "x", CreatedAt: time.Now()}
	models.DB.Create(&blog1)
	models.DB.Create(&blog2)
	models.DB.Create(&blog.BlogTagLink{BlogID: blog1.ID, TagID: tagA.ID})
	models.DB.Create(&blog.BlogTagLink{BlogID: blog2.ID, TagID: tagA.ID})
	models.DB.Create(&blog.BlogTagLink{BlogID: blog2.ID, TagID: tagB.ID})

	// 2) 作品 JSON 标签
	models.DB.Create(&models.Novel{AuthorID: 1, Title: "作品一", Tags: `["修真","末日"]`, CreatedAt: time.Now()})
	models.DB.Create(&models.Novel{AuthorID: 1, Title: "作品二", Tags: `["修真"]`, CreatedAt: time.Now()})

	// 窗口外博客不计入
	oldBlog := blog.AuthorBlog{AuthorID: 1, Title: "旧博客", ContentPath: "x", CreatedAt: time.Now().AddDate(0, 0, -30)}
	models.DB.Create(&oldBlog)
	models.DB.Create(&blog.BlogTagLink{BlogID: oldBlog.ID, TagID: tagB.ID})

	words, err := models.AggregateTagHotWords(7, 50)
	if err != nil {
		t.Fatalf("标签热词聚合失败: %v", err)
	}
	byTag := map[string]int{}
	for _, w := range words {
		byTag[w.Word] = w.Count
	}
	// 修真：博客 2 + 作品 2 = 4
	if byTag["修真"] != 4 {
		t.Errorf("修真 应计 4（博客2+作品2），实际 %d", byTag["修真"])
	}
	// 科幻：仅窗口内博客 1（旧博客不计）
	if byTag["科幻"] != 1 {
		t.Errorf("科幻 应计 1，实际 %d", byTag["科幻"])
	}
	// 末日：作品 JSON 标签 1
	if byTag["末日"] != 1 {
		t.Errorf("末日 应计 1，实际 %d", byTag["末日"])
	}
	// 降序校验
	for i := 1; i < len(words); i++ {
		if words[i-1].Count < words[i].Count {
			t.Errorf("结果应按计数降序: %v", words)
			break
		}
	}
}
