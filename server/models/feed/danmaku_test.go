package feed

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"nvs-server/models"
)

func setupDanmakuDB(t *testing.T) {
	t.Helper()
	name := fmt.Sprintf("file:danmaku_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(name), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Post{}, &Danmaku{}); err != nil {
		t.Fatal(err)
	}
	models.DB = db
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestValidateDanmaku(t *testing.T) {
	if _, _, _, err := ValidateDanmaku("   ", "scroll", "", 0); err == nil {
		t.Fatal("空内容应被拒绝")
	}
	long := ""
	for i := 0; i < 81; i++ {
		long += "字"
	}
	if _, _, _, err := ValidateDanmaku(long, "scroll", "", 0); err == nil {
		t.Fatal("超过 80 字应被拒绝")
	}
	if _, _, _, err := ValidateDanmaku("ok", "diagonal", "", 0); err == nil {
		t.Fatal("非法模式应被拒绝")
	}
	if _, _, _, err := ValidateDanmaku("ok", "scroll", "red", 0); err == nil {
		t.Fatal("非法颜色应被拒绝")
	}
	c, mode, color, err := ValidateDanmaku(" 你好\n世界 ", "", "#AABBCC", -5)
	if err != nil {
		t.Fatal(err)
	}
	if c != "你好 世界" {
		t.Fatalf("换行应替换为空格: %q", c)
	}
	if mode != DanmakuScroll {
		t.Fatalf("空模式应回落 scroll: %q", mode)
	}
	if color != "#aabbcc" {
		t.Fatalf("颜色应归一为小写: %q", color)
	}
}

func TestDanmakuListWindow(t *testing.T) {
	setupDanmakuDB(t)
	p := Post{UserID: 1, Kind: KindVideo, Title: "测试视频", Status: StatusPublished, Duration: 60}
	if err := models.DB.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	for _, ms := range []int{0, 1000, 5000, 12000} {
		if _, err := CreateDanmaku(p.ID, 2, fmt.Sprintf("弹幕%d", ms), ms, DanmakuScroll, ""); err != nil {
			t.Fatal(err)
		}
	}
	// 时间窗 (1000, 12000] 只应命中 3 条中的两条（>=1000 且 <=12000）
	got, err := ListDanmaku(p.ID, 1000, 12000, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("时间窗内应有 3 条，实际 %d", len(got))
	}
	if got[0].TimeMs != 1000 || got[2].TimeMs != 12000 {
		t.Fatalf("应按时间升序: %+v", got)
	}
	if n := CountDanmaku(p.ID); n != 4 {
		t.Fatalf("总数应为 4，实际 %d", n)
	}
	// 超出视频时长 + 5 秒容差应被拒绝
	if _, err := CreateDanmaku(p.ID, 2, "越界", 90000, DanmakuScroll, ""); err == nil {
		t.Fatal("超出时长应被拒绝")
	}
	// 只删自己的
	if err := DeleteDanmaku(3, got[0].ID); err == nil {
		t.Fatal("删他人弹幕应失败")
	}
	if err := DeleteDanmaku(2, got[0].ID); err != nil {
		t.Fatalf("删自己弹幕应成功: %v", err)
	}
}
