package models

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// ─────────────────────────────────────────────────────────────
// posts 表删除语义统一（方案A）
//
// 背景：posts 表原先同时存在 gorm.DeletedAt 软删除标记与独立 softdel_posts
// 留证表，两套删除逻辑并存、语义混乱。统一方案：移除 posts.deleted_at 列，
// 删除一律「先 ArchiveSoftDelete 归档完整快照 → 再物理删除原始记录」，
// 与 novels/chapters/comments 等「软删除 + softdel_* 留证」双轨保持一致。
// softdel_posts 表保持不变（用户明确要求软删除表不动）。
// ─────────────────────────────────────────────────────────────

// DropPostDeletedAt 移除 posts 表的 deleted_at 列（幂等：列不存在时跳过）。
//   - SQLite：不支持 DROP COLUMN，采用「建新表 → 拷贝数据 → 删旧表 → 重命名」
//     重建方案，重建后由调用方触发 AutoMigrate 补回索引；
//   - MySQL：直接 ALTER TABLE DROP COLUMN。
func DropPostDeletedAt(db *gorm.DB) error {
	if !db.Migrator().HasColumn(&Post{}, "deleted_at") {
		return nil // 已是目标结构，无需迁移
	}
	if db.Dialector.Name() == "sqlite" {
		return rebuildPostsSQLite(db)
	}
	return dropPostsColumnMySQL(db)
}

func rebuildPostsSQLite(db *gorm.DB) error {
	// 重建期间关闭外键检查（PRAGMA 为连接级，不可在事务内修改，故用独立 Exec）
	_ = db.Exec("PRAGMA foreign_keys = OFF").Error
	defer func() { _ = db.Exec("PRAGMA foreign_keys = ON").Error }()

	sqls := []string{
		`CREATE TABLE IF NOT EXISTS posts_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			thread_id integer NOT NULL,
			user_id integer NOT NULL,
			content text NOT NULL,
			created_at datetime,
			updated_at datetime,
			CONSTRAINT fk_posts_user FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`INSERT INTO posts_new (id, thread_id, user_id, content, created_at, updated_at)
		 SELECT id, thread_id, user_id, content, created_at, updated_at FROM posts`,
		`DROP TABLE posts`,
		`ALTER TABLE posts_new RENAME TO posts`,
	}
	for _, s := range sqls {
		if err := db.Exec(s).Error; err != nil {
			return fmt.Errorf("posts 表重建失败（%s）: %w", strings.Fields(s)[0], err)
		}
	}
	// 修复 AUTOINCREMENT 序列，避免新插入主键冲突
	_ = db.Exec("UPDATE sqlite_sequence SET seq = (SELECT COALESCE(MAX(id),0) FROM posts) WHERE name = 'posts'").Error
	// 重建索引（posts 的复合/单列索引在重建后随旧表消失）
	// 注意：此处不用 db.AutoMigrate(&Post{})——重建表已带与模型一致的外键约束，
	// 直接显式建索引即可；若 AutoMigrate 检测到任何差异会触发 SQLite 12-step 表重建，
	// 其数据拷贝只复制差异列（GORM 已知行为），有数据丢失风险。
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_posts_thread_created ON posts(thread_id, created_at)").Error; err != nil {
		return fmt.Errorf("posts 索引重建失败: %w", err)
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_posts_user_id ON posts(user_id)").Error; err != nil {
		return fmt.Errorf("posts 索引重建失败: %w", err)
	}
	return nil
}

func dropPostsColumnMySQL(db *gorm.DB) error {
	var cnt int64
	if err := db.Raw(
		"SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'posts' AND column_name = 'deleted_at'",
	).Scan(&cnt).Error; err != nil {
		return err
	}
	if cnt == 0 {
		return nil
	}
	if err := db.Exec("ALTER TABLE posts DROP COLUMN deleted_at").Error; err != nil {
		return fmt.Errorf("posts 表移除 deleted_at 失败: %w", err)
	}
	return nil
}

// DropLegacyIndexes 清理被新复合索引替代的旧单列索引（AutoMigrate 只增不删，
// 旧索引残留会造成冗余写放大）。仅在索引确实存在时删除，不存在则静默跳过。
// 注意：posts 表若发生重建（方案A），其旧索引随 DROP TABLE 消失，无需在此处理。
func DropLegacyIndexes(db *gorm.DB) error {
	legacy := []struct {
		model interface{}
		name  string
	}{
		{&Thread{}, "idx_threads_forum_id"},
		{&Comment{}, "idx_comments_novel_id"},
		{&Comment{}, "idx_comments_parent_id"}, // 旧匿名索引（ParentID 原为 index），被具名 idx_comments_parent 替代
		{&Notification{}, "idx_notif_user_read"},
		{&UserAuditLog{}, "idx_user_audit_logs_user_id"},
		{&ZoneAccessLog{}, "idx_zone_access_logs_user_id"},
		{&ZoneAccessLog{}, "idx_zone_access_logs_zone"},
		{&ForumMute{}, "idx_forum_mutes_forum_id"},
		{&ForumMute{}, "idx_forum_mutes_user_id"},
		{&Novel{}, "idx_novels_author_id"},
	}
	for _, l := range legacy {
		if db.Migrator().HasIndex(l.model, l.name) {
			_ = db.Migrator().DropIndex(l.model, l.name)
		}
	}
	return nil
}

// ArchivePostsByThread 将某帖子（thread）下的全部回帖归档到 softdel_posts 留证表，
// 供「删帖级联删回帖」场景在物理删除前批量留证（方案A必需，否则回帖数据消失且不可追溯）。
// 返回归档条数；单条失败不阻断（best-effort，但会统计失败数）。
func ArchivePostsByThread(db *gorm.DB, threadID, deletedBy uint, operator, reason, source string) (int, error) {
	var posts []Post
	if err := db.Unscoped().Where("thread_id = ?", threadID).Find(&posts).Error; err != nil {
		return 0, err
	}
	archived := 0
	var firstErr error
	for i := range posts {
		p := posts[i]
		summary := p.Content
		if runes := []rune(summary); len(runes) > 200 {
			summary = string(runes[:200])
		}
		if err := ArchiveSoftDelete(source, "post", p.ID, deletedBy, operator, reason, summary, p); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		archived++
	}
	if firstErr != nil {
		return archived, firstErr
	}
	return archived, nil
}
