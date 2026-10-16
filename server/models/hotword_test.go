package models

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCountHotWords(t *testing.T) {
	counts := map[string]int{}
	// 英文：整词、小写化；停用词（the）过滤；重复累加
	countHotWords("Hello the world hello WORLD", counts)
	if counts["hello"] != 2 {
		t.Errorf("hello 应计 2，实际 %d", counts["hello"])
	}
	if counts["world"] != 2 {
		t.Errorf("world 应计 2（大小写不敏感），实际 %d", counts["world"])
	}
	if _, ok := counts["the"]; ok {
		t.Errorf("停用词 the 不应计入")
	}

	// 中文：二元切分；虚字（的/了）二元组被过滤
	counts = map[string]int{}
	countHotWords("星球大战星球", counts)
	if counts["星球"] != 2 {
		t.Errorf("星球 应计 2，实际 %d", counts["星球"])
	}
	if counts["大战"] != 1 {
		t.Errorf("大战 应计 1，实际 %d", counts["大战"])
	}

	counts = map[string]int{}
	countHotWords("我的了", counts) // 我的/的了 均含停用字 → 全滤
	if len(counts) != 0 {
		t.Errorf("全停用字应被滤空，实际 %v", counts)
	}

	// 空文本不应 panic
	countHotWords("   ", map[string]int{})
}

func TestAggregateHotWords(t *testing.T) {
	// 独立命名的内存库，避免与其它测试共享 file::memory: 互相污染
	db, err := gorm.Open(sqlite.Open("file:hotword_test?mode=memory&cache=shared"), &gorm.Config{})
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

	// 造数据：多条评论/回帖/帖题，令「测试」「热点」成为高频二元组
	for i := 0; i < 3; i++ {
		db.Create(&Comment{UserID: 1, NovelID: 1, Content: "测试热点测试热点", CreatedAt: time.Now()})
	}
	db.Create(&Post{ThreadID: 1, UserID: 1, Content: "测试热点内容", CreatedAt: time.Now()})
	db.Create(&Thread{ForumID: 1, UserID: 1, Title: "热点讨论测试", Content: "x", CreatedAt: time.Now()})

	words, err := AggregateHotWords(7, 50)
	if err != nil {
		t.Fatalf("聚合失败: %v", err)
	}
	byWord := map[string]int{}
	for _, w := range words {
		byWord[w.Word] = w.Count
	}
	if byWord["测试"] < 2 {
		t.Errorf("测试 应≥2，实际 %d", byWord["测试"])
	}
	if byWord["热点"] < 2 {
		t.Errorf("热点 应≥2，实际 %d", byWord["热点"])
	}
	// 只应返回 count≥2 的词
	for _, w := range words {
		if w.Count < 2 {
			t.Errorf("热词计数应≥2，实际 %s=%d", w.Word, w.Count)
		}
	}
	// 校验降序
	for i := 1; i < len(words); i++ {
		if words[i-1].Count < words[i].Count {
			t.Errorf("结果应按计数降序：%v", words)
			break
		}
	}

	// limit 收敛
	limited, _ := AggregateHotWords(7, 3)
	if len(limited) > 3 {
		t.Errorf("limit=3 应最多 3 项，实际 %d", len(limited))
	}
}
