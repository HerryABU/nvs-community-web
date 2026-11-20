package feed

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"nvs-server/models"
)

func setupTagTestDB(t *testing.T) {
	t.Helper()
	name := fmt.Sprintf("file:feedtag_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(name), &gorm.Config{})
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
}

func TestTagFilterUsesExactInvertedIndex(t *testing.T) {
	setupTagTestDB(t)
	_, err := CreatePost(1, PostInput{Kind: KindMicro, Title: "A", Content: "正文 A", Tags: "空耳,测试"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = CreatePost(2, PostInput{Kind: KindMicro, Title: "B", Content: "正文 B", Tags: "测试版"})
	if err != nil {
		t.Fatal(err)
	}
	if err := RebuildTagIndex(); err != nil {
		t.Fatal(err)
	}

	list, total, err := ListPosts(ListOptions{Tag: "空耳", PageSize: 20})
	if err != nil || total != 1 || len(list) != 1 || list[0].Title != "A" {
		t.Fatalf("按标签查询错误: total=%d list=%v err=%v", total, list, err)
	}
	list, total, _ = ListPosts(ListOptions{Tag: "测试", PageSize: 20})
	if total != 1 || len(list) != 1 || list[0].Title != "A" {
		t.Fatalf("标签应精确匹配，不能把「测试版」算进来: total=%d list=%v", total, list)
	}
	list, total, _ = ListPosts(ListOptions{Tag: "测试版", PageSize: 20})
	if total != 1 || list[0].Title != "B" {
		t.Fatalf("测试版应命中 B: total=%d list=%v", total, list)
	}
}

func TestParseTags(t *testing.T) {
	got := ParseTags(" 空耳，，测试,空耳 , ")
	if len(got) != 2 || got[0] != "空耳" || got[1] != "测试" {
		t.Fatalf("解析/去重错误: %#v", got)
	}
}
