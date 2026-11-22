package oc

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"nvs-server/config"
	"nvs-server/models"
)

// setupArtDB 独立内存库（唯一名，避免与 oc_test.go 的共享库互相污染）
func setupArtDB(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:oc_art_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("主迁移失败: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("OC 迁移失败: %v", err)
	}
	old := models.DB
	oldDir := config.NovelDataDir
	models.DB = db
	config.NovelDataDir = t.TempDir()
	t.Cleanup(func() {
		models.DB = old
		config.NovelDataDir = oldDir
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
}

func mkArtUser(t *testing.T, name string) uint {
	t.Helper()
	u := &models.User{Username: name, Email: name + "@art.test", PasswordHash: "x", Role: "reader"}
	if err := models.DB.Create(u).Error; err != nil {
		t.Fatalf("建用户失败: %v", err)
	}
	return u.ID
}

// TestArtGalleryCRUD 图库增删改、权限、URL 白名单、世界聚合、角色按世界过滤
func TestArtGalleryCRUD(t *testing.T) {
	setupArtDB(t)
	owner := mkArtUser(t, "art_owner")
	other := mkArtUser(t, "art_other")

	p, err := CreateProject(owner, ProjectInput{Title: "图库测试世界", Visibility: VisPrivate})
	if err != nil {
		t.Fatalf("建项目失败: %v", err)
	}
	book, err := CreateWorldBook(owner, 0, p.ID, "图库测试世界", "", VisPrivate)
	if err != nil {
		t.Fatalf("建世界书失败: %v", err)
	}
	c1, err := CreateCharacter(owner, CharacterInput{
		Name: "测试角色", BookID: book.ID, ProjectID: p.ID,
		Appearance: "银发、左眼下有泪痣", Persona: "冷淡但护短",
	})
	if err != nil {
		t.Fatalf("建角色失败: %v", err)
	}

	// 1) 新增：立绘 + 设定参考（sort 更小的排前面）
	if _, err := CreateCharacterArt(owner, c1.ID, false, ArtInput{
		URL: "/uploads/arts/portrait.webp", Title: "常服立绘", Traits: "银发、泪痣",
	}); err != nil {
		t.Fatalf("加立绘失败: %v", err)
	}
	if _, err := CreateCharacterArt(owner, c1.ID, false, ArtInput{
		URL: "/uploads/arts/ref.webp", Kind: ArtRef, Sort: -1, Title: "兽设三视图",
	}); err != nil {
		t.Fatalf("加设定图失败: %v", err)
	}
	list, err := ListCharacterArts(c1.ID)
	if err != nil {
		t.Fatalf("查图库失败: %v", err)
	}
	if len(list) != 2 || list[0].Kind != ArtRef {
		t.Fatalf("排序/数量不符: %+v", list)
	}
	if list[1].ProjectID != p.ID {
		t.Fatalf("项目 id 应从角色回填，实际 %d", list[1].ProjectID)
	}

	// 2) 非成员无权添加
	if _, err := CreateCharacterArt(other, c1.ID, false, ArtInput{URL: "/uploads/x.webp"}); err == nil {
		t.Fatal("非成员不应能添加形象图")
	}

	// 3) URL 协议白名单（与表情包同一口径）
	if _, err := CreateCharacterArt(owner, c1.ID, false, ArtInput{URL: "javascript:alert(1)"}); err == nil {
		t.Fatal("javascript: 地址必须被拒绝")
	}

	// 4) 世界级聚合
	all, err := ListProjectArts(p.ID)
	if err != nil {
		t.Fatalf("查世界图库失败: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("世界图库应有 2 张，实际 %d", len(all))
	}

	// 5) 更新：kind 非法值回落 portrait
	up, err := UpdateCharacterArt(owner, list[0].ID, false, ArtInput{
		URL: "/uploads/arts/ref2.webp", Kind: "weird-kind", Traits: "新增：断角",
	})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if up.Kind != ArtPortrait {
		t.Fatalf("非法 kind 应回落 portrait，实际 %q", up.Kind)
	}
	if up.Traits != "新增：断角" {
		t.Fatalf("特征未保存: %q", up.Traits)
	}

	// 6) 非成员无权修改/删除
	if _, err := UpdateCharacterArt(other, list[0].ID, false, ArtInput{URL: "/uploads/y.webp"}); err == nil {
		t.Fatal("非成员不应能修改形象图")
	}
	if err := DeleteCharacterArt(other, list[0].ID, false); err == nil {
		t.Fatal("非成员不应能删除形象图")
	}

	// 7) 删除
	if err := DeleteCharacterArt(owner, list[0].ID, false); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	left, _ := ListCharacterArts(c1.ID)
	if len(left) != 1 {
		t.Fatalf("删除后应剩 1 张，实际 %d", len(left))
	}

	// 8) 角色按世界过滤：另建一个不归属任何世界的角色，必须被筛掉
	if _, err := CreateCharacter(owner, CharacterInput{Name: "散户角色", BookID: book.ID}); err != nil {
		t.Fatalf("建散户角色失败: %v", err)
	}
	inWorld, total, err := ListCharacters(owner, "mine", "", p.ID, 1, 20)
	if err != nil {
		t.Fatalf("按世界查角色失败: %v", err)
	}
	if total != 1 || len(inWorld) != 1 || inWorld[0].ID != c1.ID {
		t.Fatalf("世界内角色应只有 1 个（%d），实际 total=%d list=%d", c1.ID, total, len(inWorld))
	}
	allMine, totalAll, _ := ListCharacters(owner, "mine", "", 0, 1, 20)
	if totalAll != 2 || len(allMine) != 2 {
		t.Fatalf("不过滤时应返回 2 个，实际 total=%d", totalAll)
	}
}
