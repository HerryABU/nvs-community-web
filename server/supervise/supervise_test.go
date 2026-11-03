package supervise

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"nvs-server/models"
)

func setupDB(t *testing.T) {
	t.Helper()
	prev := models.DB
	dsn := fmt.Sprintf("file:supervise_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("打开内存库失败：%v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("建表失败：%v", err)
	}
	models.DB = db
	t.Cleanup(func() { models.DB = prev })
}

// TestUpsert_Escalate 入档 → 分数累加 → 升级恶意 → 不可被降回。
func TestUpsert_Escalate(t *testing.T) {
	setupDB(t)
	su, err := Upsert(100, "alice", TierSuspect, SrcRule, 10, "命中管制物质", nil)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if su.Tier != TierSuspect || su.Score != 10 {
		t.Fatalf("初次入档异常：%+v", su)
	}
	// 再次触发：分数累加、原因追加
	su, _ = Upsert(100, "alice", TierSuspect, SrcRule, 5, "再次命中", nil)
	if su.Score != 15 {
		t.Fatalf("分数未累加：%d", su.Score)
	}
	// 升级到恶意名单
	su, err = Escalate(100, "判定明确恶意")
	if err != nil {
		t.Fatalf("escalate: %v", err)
	}
	if su.Tier != TierMalicious {
		t.Fatalf("未升级为恶意：%s", su.Tier)
	}
	// 恶意档不可被 Upsert 降回 suspect（只允许人工 Clear/SetTier 留痕降级）
	su, _ = Upsert(100, "alice", TierSuspect, SrcRule, 1, "x", nil)
	if su.Tier != TierMalicious {
		t.Fatalf("恶意档被 Upsert 违规降档：%s", su.Tier)
	}
}

// TestEscalate_NewRecord 记录不存在时，Escalate 应以恶意档直接入档。
func TestEscalate_NewRecord(t *testing.T) {
	setupDB(t)
	if _, err := Escalate(200, "直接恶意"); err != nil {
		t.Fatalf("escalate new: %v", err)
	}
	su, err := GetByUser(200)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if su.Tier != TierMalicious {
		t.Fatalf("期望恶意档，实际 %s", su.Tier)
	}
}

// TestSetTierAndClear 人工改档 + 清除 + 清除后重新触发恢复在档。
func TestSetTierAndClear(t *testing.T) {
	setupDB(t)
	su, _ := Upsert(300, "bob", TierSuspect, SrcManual, 0, "manual", nil)
	if err := SetTier(su.ID, 1, TierMalicious, "人工升级"); err != nil {
		t.Fatalf("set tier: %v", err)
	}
	got, _, _ := Get(su.ID)
	if got.Tier != TierMalicious {
		t.Fatalf("改档失败：%s", got.Tier)
	}
	if err := Clear(su.ID, 1, "已处置"); err != nil {
		t.Fatalf("clear: %v", err)
	}
	got, _, _ = Get(su.ID)
	if got.Status != StatusCleared {
		t.Fatalf("清除失败：%s", got.Status)
	}
	// 清除后重新触发：恢复在档（保留历史原因）
	_, _ = Upsert(300, "bob", TierSuspect, SrcRule, 1, "again", nil)
	got, _, _ = Get(su.ID)
	if got.Status != StatusActive {
		t.Fatalf("应恢复在档，实际 %s", got.Status)
	}
}

// TestStats 概览按档位计数 + 已清除计数。
func TestStats(t *testing.T) {
	setupDB(t)
	Upsert(1, "u1", TierSuspect, SrcRule, 0, "a", nil)
	Upsert(2, "u2", TierSuspect, SrcRule, 0, "b", nil)
	Escalate(2, "malicious")
	Upsert(3, "u3", TierSuspect, SrcManual, 0, "c", nil)
	Clear(GetByUserMust(t, 3).ID, 1, "clear")
	stats := Stats()
	byTier, _ := stats["by_tier"].(map[string]int64)
	if byTier[TierSuspect] != 1 {
		t.Fatalf("可疑计数应为 1，实际 %d", byTier[TierSuspect])
	}
	if byTier[TierMalicious] != 1 {
		t.Fatalf("恶意计数应为 1，实际 %d", byTier[TierMalicious])
	}
	if stats["cleared"].(int64) != 1 {
		t.Fatalf("已清除计数应为 1，实际 %d", stats["cleared"].(int64))
	}
}

func GetByUserMust(t *testing.T, uid uint) *SupervisedUser {
	su, err := GetByUser(uid)
	if err != nil {
		t.Fatalf("get by user: %v", err)
	}
	return su
}
