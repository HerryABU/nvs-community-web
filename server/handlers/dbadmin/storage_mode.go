package dbadmin

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"

	"nvs-server/config"
	"nvs-server/contentstore"
	"nvs-server/handlers/admincommon"
	"nvs-server/maintenance"
	"nvs-server/models"
	blogm "nvs-server/models/blog"
	"nvs-server/novelindex"
	"nvs-server/utils/infra"
)

const storageModeConfigKey = "storage_mode"

// GET /api/admin/db/storage-mode
func AdminStorageModeGet(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	infra.Success(c, gin.H{
		"mode":        contentstore.Mode(),
		"maintenance": maintenance.Enabled(),
	})
}

// POST /api/admin/db/storage-mode — 切换正文存储模式（异步、按用户分批、迁移期禁写）
func AdminStorageModeSet(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var req struct {
		Mode string `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "请求参数错误")
		return
	}
	mode := req.Mode
	if mode != contentstore.ModeHybrid && mode != contentstore.ModeDatabase {
		infra.BadRequest(c, "模式只能是 hybrid（数据库+文件系统）或 database（纯数据库）")
		return
	}
	if mode == contentstore.Mode() && !maintenance.Enabled() {
		infra.SuccessMessage(c, "当前已是该模式", gin.H{"job_id": ""})
		return
	}

	job := newJob("storage-mode")
	go runStorageModeJob(job, mode)
	infra.Success(c, gin.H{"job_id": job.ID})
}

// runStorageModeJob 热迁移正文。
//
// 顺序很关键：先搬数据、再切配置。切换前 maintenance 已经挡住所有写入，
// 因此不会出现「搬到一半有新内容写进源库、目标库漏数据」的竞态。
// 按用户分批（50 人一批，每人内容 20 条一切）是为了让大站迁移可控、进度可见。
func runStorageModeJob(job *dbJob, target string) {
	maintenance.Begin()
	defer maintenance.End()

	job.TablesTotal = 2 // 用户批次 / 内容条目
	defer func() {
		if job.Status == "running" {
			finishJob(job, "done", "存储模式已切换为 "+target)
		}
	}()

	users := collectContentUsers()
	job.Detail["users"] = len(users)
	const userBatch = 50
	const itemBatch = 20

	doneUsers, doneItems := 0, 0
	for start := 0; start < len(users); start += userBatch {
		end := start + userBatch
		if end > len(users) {
			end = len(users)
		}
		for _, uid := range users[start:end] {
			n := migrateUserContent(uid, target, itemBatch)
			doneItems += n
		}
		doneUsers = end
		jobsMu.Lock()
		job.Table = fmt.Sprintf("用户 %d/%d", doneUsers, len(users))
		job.TablesDone = doneUsers
		job.Rows = int64(doneItems)
		jobsMu.Unlock()
	}

	// 小说 index.json 的库内副本也一并补齐（不依赖文件，纯数据库模式可读）
	rebuildNovelIndexes(job)

	if err := models.SetPlatformConfig(storageModeConfigKey, target); err != nil {
		finishJob(job, "error", "内容已迁移，但写入存储模式配置失败: "+err.Error())
		return
	}
	job.Detail["mode"] = target
	job.Detail["items"] = doneItems
}

func collectContentUsers() []uint {
	seen := map[uint]bool{}
	out := []uint{}
	if models.DB != nil {
		var chapterUsers, blogUsers []uint
		_ = models.DB.Model(&models.Chapter{}).Select("DISTINCT author_id").
			Where("author_id > 0").Scan(&chapterUsers).Error
		_ = models.DB.Model(&blogm.AuthorBlog{}).Select("DISTINCT author_id").
			Where("author_id > 0").Scan(&blogUsers).Error
		for _, ids := range [][]uint{chapterUsers, blogUsers} {
			for _, id := range ids {
				if !seen[id] {
					seen[id] = true
					out = append(out, id)
				}
			}
		}
	}
	return out
}

// migrateUserContent 返回本用户已处理的内容条数
func migrateUserContent(uid uint, target string, chunk int) int {
	if chunk <= 0 {
		chunk = 20
	}
	count := 0
	// 章节：按用户分批取，再按 20 条一切
	for offset := 0; ; offset += chunk {
		var chapters []models.Chapter
		err := models.DB.Where("author_id = ?", uid).Order("id ASC").
			Limit(chunk).Offset(offset).Find(&chapters).Error
		if err != nil || len(chapters) == 0 {
			break
		}
		for _, ch := range chapters {
			migrateChapter(ch, target)
			count++
		}
		if len(chapters) < chunk {
			break
		}
	}
	// 博客
	for offset := 0; ; offset += chunk {
		var blogs []blogm.AuthorBlog
		err := models.DB.Where("author_id = ?", uid).Order("id ASC").
			Limit(chunk).Offset(offset).Find(&blogs).Error
		if err != nil || len(blogs) == 0 {
			break
		}
		for _, b := range blogs {
			migrateBlog(uid, b, target)
			count++
		}
		if len(blogs) < chunk {
			break
		}
	}
	return count
}

func chapterFilePath(ch models.Chapter) string {
	if ch.ContentPath != "" {
		return ch.ContentPath
	}
	var n models.Novel
	if models.DB.First(&n, ch.NovelID).Error != nil {
		return ""
	}
	return filepath.Join(config.NovelDataDir, "authors", strconv.FormatUint(uint64(n.AuthorID), 10),
		strconv.FormatUint(uint64(ch.NovelID), 10), fmt.Sprintf("%d.html", ch.ChapterNumber))
}

func blogFilePath(uid uint, b blogm.AuthorBlog) string {
	if b.ContentPath != "" {
		return b.ContentPath
	}
	return filepath.Join(filepath.Dir(config.NovelDataDir), "blogs",
		strconv.FormatUint(uint64(uid), 10), fmt.Sprintf("%d.md", b.ID))
}

func migrateChapter(ch models.Chapter, target string) {
	path := chapterFilePath(ch)
	if target == contentstore.ModeDatabase {
		// 文件优先（迁移前还是混合模式），文件丢了再回退库内镜像
		text, err := os.ReadFile(path)
		if err != nil {
			if fromDB, ok := contentstore.LoadDB(contentstore.RefChapter, ch.ID); ok {
				text = []byte(fromDB)
			} else {
				return
			}
		}
		_ = contentstore.Put(contentstore.RefChapter, ch.ID, "", string(text))
		return
	}
	// 切回混合：库内正文写回文件
	if text, ok := contentstore.LoadDB(contentstore.RefChapter, ch.ID); ok && path != "" {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, []byte(text), 0o644)
	}
}

func migrateBlog(uid uint, b blogm.AuthorBlog, target string) {
	path := blogFilePath(uid, b)
	if target == contentstore.ModeDatabase {
		text, err := os.ReadFile(path)
		if err != nil {
			if fromDB, ok := contentstore.LoadDB(contentstore.RefBlog, b.ID); ok {
				text = []byte(fromDB)
			} else {
				return
			}
		}
		_ = contentstore.Put(contentstore.RefBlog, b.ID, "", string(text))
		return
	}
	if text, ok := contentstore.LoadDB(contentstore.RefBlog, b.ID); ok && path != "" {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, []byte(text), 0o644)
	}
}

func rebuildNovelIndexes(job *dbJob) {
	if models.DB == nil {
		return
	}
	var novels []models.Novel
	if err := models.DB.Unscoped().Select("id, author_id, title").Find(&novels).Error; err != nil {
		return
	}
	for _, n := range novels {
		novelindex.Write(n.AuthorID, n.ID, n.Title)
	}
	jobsMu.Lock()
	job.Detail["novel_indexes"] = len(novels)
	jobsMu.Unlock()
}
