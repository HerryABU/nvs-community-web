package novelindex

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"nvs-server/models"
)

func setupIndexTestDB(t *testing.T) {
	t.Helper()
	name := fmt.Sprintf("file:novelindex_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(name), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	models.DB = db
	// 单测不往磁盘写文件；文件行为由集成验收覆盖
	old := FileEnabled
	FileEnabled = func() bool { return false }
	t.Cleanup(func() {
		FileEnabled = old
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestWriteMirrorsJSONIntoDB(t *testing.T) {
	setupIndexTestDB(t)
	buf := Write(7, 42, "书名")
	if len(buf) == 0 {
		t.Fatal("Write 没有产出 JSON")
	}
	s := string(buf)
	for _, want := range []string{`"novel_id": 42`, `"title": "书名"`, `"chapters": []`, `"updated_at"`} {
		if !strings.Contains(s, want) {
			t.Errorf("清单缺少 %s：%s", want, s)
		}
	}

	got, ok := Load(42)
	if !ok {
		t.Fatal("库内副本不存在")
	}
	if string(got) != s {
		t.Fatalf("库内副本与写出的 JSON 不一致:\nDB=%s\nWrite=%s", got, buf)
	}
	var row models.NovelIndex
	if err := models.DB.Where("novel_id = ?", 42).First(&row).Error; err != nil {
		t.Fatalf("novel_indexes 表无记录: %v", err)
	}
	sum := sha256.Sum256(buf)
	if row.Hash != hex.EncodeToString(sum[:]) {
		t.Fatalf("Hash 与内容不符: %s", row.Hash)
	}
}

func TestUpdateAndRemove(t *testing.T) {
	setupIndexTestDB(t)
	first := Write(1, 9, "旧标题")
	if len(first) == 0 {
		t.Fatal("首次写入失败")
	}
	second := Write(1, 9, "新标题")
	if !strings.Contains(string(second), "新标题") || strings.Contains(string(second), "旧标题") {
		t.Fatalf("覆盖写入没有更新标题: %s", second)
	}
	got, _ := Load(9)
	if string(got) != string(second) {
		t.Fatal("库内副本没有跟着覆盖")
	}
	Remove(9)
	if _, ok := Load(9); ok {
		t.Fatal("Remove 后仍能读到副本")
	}
}
