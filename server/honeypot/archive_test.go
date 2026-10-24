package honeypot

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"nvs-server/models"
)

// TestArchiveBeforeDelete 超期事件必须先落 gzip 归档再删库
func TestArchiveBeforeDelete(t *testing.T) {
	setupHP(t)
	if _, err := SaveSetting(Setting{Enabled: true, Level: LevelMedium, RetentionDays: 30}); err != nil {
		t.Fatal(err)
	}
	ResetCache()

	// 一条超期 + 一条未超期
	old := &Event{Kind: EventDecoyHit, Decoy: "env", Path: "/.env", Risk: 70,
		CreatedAt: time.Now().AddDate(0, 0, -60)}
	recent := &Event{Kind: EventDecoyHit, Decoy: "env", Path: "/.env", Risk: 70, CreatedAt: time.Now()}
	models.DB.Create(old)
	models.DB.Create(recent)

	res, err := ArchiveAndCleanup()
	if err != nil {
		t.Fatalf("归档失败: %v", err)
	}
	if res.Skipped || res.Archived != 1 {
		t.Fatalf("应归档 1 条: %+v", res)
	}
	if res.File == "" || res.Bytes == 0 {
		t.Fatalf("归档文件缺失: %+v", res)
	}
	if _, err := os.Stat(res.File); err != nil {
		t.Fatalf("归档文件不存在: %v", err)
	}
	// 超期被删、未超期保留
	var oldCnt, recentCnt int64
	models.DB.Model(&Event{}).Where("path = ? AND created_at < ?", "/.env", time.Now().AddDate(0, 0, -10)).Count(&oldCnt)
	models.DB.Model(&Event{}).Count(&recentCnt)
	if oldCnt != 0 || recentCnt != 1 {
		t.Fatalf("清理结果错误 old=%d recent=%d", oldCnt, recentCnt)
	}
	// 归档列表可见
	list, err := ListArchives()
	if err != nil || len(list) == 0 {
		t.Fatalf("归档列表为空: %v", err)
	}
	_ = os.RemoveAll(archiveDir())
}

// TestIsolatedDBSwitch 独立库切换后读写都落在独立库，且拒绝与主库同路径
func TestIsolatedDBSwitch(t *testing.T) {
	setupHP(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "honeypot.db")

	if err := OpenIsolated(p); err != nil {
		t.Fatalf("打开独立库失败: %v", err)
	}
	defer func() {
		ResetDB() // 复位到主库，避免污染其它测试
		_ = os.RemoveAll(dir)
	}()
	if !Isolated() || DBPath() != p {
		t.Fatalf("未切到独立库: isolated=%v path=%s", Isolated(), DBPath())
	}
	// 写配置进独立库
	if _, err := SaveSetting(Setting{Enabled: true, Level: LevelHigh, RetentionDays: 60}); err != nil {
		t.Fatal(err)
	}
	got := Current()
	if got.Level != LevelHigh || got.RetentionDays != 60 {
		t.Fatalf("独立库读写异常: %+v", got)
	}
	// 同路径应被拒（把主库路径指向同一文件）
	t.Setenv("DB_PATH", p)
	if err := SetDB(models.DB, p); err == nil {
		t.Fatal("同路径应被拒绝")
	}
	t.Setenv("DB_PATH", "")
	// 主库不应被写入这条配置
	var cnt int64
	models.DB.Model(&Setting{}).Where("level = ?", LevelHigh).Count(&cnt)
	if cnt != 0 {
		t.Fatalf("独立库配置泄漏到主库: %d", cnt)
	}
}
