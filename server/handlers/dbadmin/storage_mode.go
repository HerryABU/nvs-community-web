package dbadmin

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"nvs-server/config"
	"nvs-server/contentstore"
	"nvs-server/handlers/admincommon"
	"nvs-server/maintenance"
	"nvs-server/models"
	blogm "nvs-server/models/blog"
	"nvs-server/novelindex"
	"nvs-server/utils/infra"
	"nvs-server/utils/sanitize"
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

// GET /api/admin/db/storage-health
//
// 判断「正文到底有没有进库」不能看数据库文件体积：SQLite 复写页面不线性增长，
// 且重跑迁移是幂等的（数据早就在库里时文件大小不会变）。这里直接按条数与字节数回答。
func AdminStorageHealth(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	infra.Success(c, storageHealth())
}

type storageCounts struct {
	Total  int64 `json:"total"`
	Blob   int64 `json:"blob"`
	Empty  int64 `json:"empty"`
	Missed int64 `json:"missing"`
	Bytes  int64 `json:"bytes"`
}

type storageHealthReport struct {
	Mode         string        `json:"mode"`
	Chapter      storageCounts `json:"chapter"`
	Blog         storageCounts `json:"blog"`
	ContentBytes int64         `json:"content_bytes"`
	DBFileBytes  int64         `json:"db_file_bytes"`
	Missing      []gin.H       `json:"missing_sample"`
	CheckedAt    time.Time     `json:"checked_at"`
}

func storageHealth() storageHealthReport {
	rep := storageHealthReport{
		Mode:      contentstore.Mode(),
		Missing:   []gin.H{},
		CheckedAt: time.Now(),
	}
	if models.DB == nil {
		return rep
	}

	rep.Chapter.Total = countRows(&models.Chapter{})
	rep.Blog.Total = countRows(&blogm.AuthorBlog{})
	rep.Chapter.Blob = countBlobRows(contentstore.RefChapter, false)
	rep.Blog.Blob = countBlobRows(contentstore.RefBlog, false)
	rep.Chapter.Empty = countBlobRows(contentstore.RefChapter, true)
	rep.Blog.Empty = countBlobRows(contentstore.RefBlog, true)
	rep.Chapter.Missed = rep.Chapter.Total - rep.Chapter.Blob
	rep.Blog.Missed = rep.Blog.Total - rep.Blog.Blob
	if rep.Chapter.Missed < 0 {
		rep.Chapter.Missed = 0
	}
	if rep.Blog.Missed < 0 {
		rep.Blog.Missed = 0
	}
	rep.Chapter.Bytes = sumBlobBytes(contentstore.RefChapter)
	rep.Blog.Bytes = sumBlobBytes(contentstore.RefBlog)
	rep.ContentBytes = rep.Chapter.Bytes + rep.Blog.Bytes
	rep.DBFileBytes = sqliteFileBytes()

	// 缺失样本：只有「未删除但既无库内副本、磁盘也没有文件」才是真孤儿，需要人工处理
	var missCh []models.Chapter
	_ = models.DB.Select("id, novel_id, chapter_number, title, content_path").
		Where("id NOT IN (SELECT ref_id FROM content_blobs WHERE ref_type = ?)", contentstore.RefChapter).
		Limit(20).Find(&missCh).Error
	for _, ch := range missCh {
		path := chapterFilePath(ch)
		_, err := os.Stat(path)
		rep.Missing = append(rep.Missing, gin.H{
			"kind":           "chapter",
			"id":             ch.ID,
			"novel_id":       ch.NovelID,
			"chapter_number": ch.ChapterNumber,
			"title":          ch.Title,
			"file_exists":    err == nil,
			"path":           path,
		})
	}
	return rep
}

func countRows(model interface{}) int64 {
	var n int64
	_ = models.DB.Model(model).Count(&n).Error
	return n
}

func countBlobRows(refType string, emptyOnly bool) int64 {
	var n int64
	q := models.DB.Model(&models.ContentBlob{}).Where("ref_type = ?", refType)
	if emptyOnly {
		q = q.Where("LENGTH(content) = 0")
	}
	_ = q.Count(&n).Error
	return n
}

func sumBlobBytes(refType string) int64 {
	var n int64
	// LENGTH(TEXT) 在 SQLite 里按「字符」算，中文会少算一半多；转 BLOB 才是真实字节
	_ = models.DB.Model(&models.ContentBlob{}).Where("ref_type = ?", refType).
		Select("COALESCE(SUM(LENGTH(CAST(content AS BLOB))), 0)").Scan(&n).Error
	return n
}

// sqliteFileBytes 用 PRAGMA 反推数据库文件占用（不依赖驱动类型断言）
func sqliteFileBytes() int64 {
	var pages, size int64
	if models.DB.Raw("PRAGMA page_count").Scan(&pages).Error != nil {
		return 0
	}
	if models.DB.Raw("PRAGMA page_size").Scan(&size).Error != nil {
		return 0
	}
	return pages * size
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
	// 同模式也允许重跑：历史数据在迁移后入库（例如老章节 author_id=0 曾被漏掉），
	// 此时需要从文件/库镜像再扫一遍补齐，不能一句「当前已是该模式」就拒绝。
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

	defer func() {
		if job.Status == "running" {
			msg := "存储模式已切换为 " + target
			// 迁移收尾文案带条数与字节，避免管理员用「库文件体积」误判迁移成败
			if s, ok := job.Detail["summary"].(string); ok && s != "" {
				msg = s
			}
			finishJob(job, "done", msg)
		}
	}()

	users := collectContentUsers()
	job.TablesTotal = len(users)
	job.Detail["users"] = len(users)
	job.Detail["missing_before"] = countMissingContent()
	setStage(job, "扫描内容", fmt.Sprintf("共发现 %d 个作者批次，正在统计待迁移正文", len(users)))
	const userBatch = 50
	const itemBatch = 20

	totalItems := countContentItems()
	job.Detail["total_items"] = totalItems
	doneUsers, doneItems := 0, 0
	var total migrateTally
	for start := 0; start < len(users); start += userBatch {
		end := start + userBatch
		if end > len(users) {
			end = len(users)
		}
		for _, uid := range users[start:end] {
			// 每切一块（20 条）就更新一次进度，首页进度条不会整段任务只跳一次
			n, tally := migrateUserContent(uid, target, itemBatch, func(handled int) {
				setProgress(job, "迁移正文", fmt.Sprintf("作者 %d/%d：已处理 %d/%d 条正文 → %s",
					doneUsers+1, len(users), doneItems+handled, totalItems, target),
					doneItems+handled, totalItems)
			})
			doneItems += n
			total.OK += tally.OK
			total.Empty += tally.Empty
			total.Skipped += tally.Skipped
		}
		doneUsers = end
		jobsMu.Lock()
		job.Table = fmt.Sprintf("用户 %d/%d", doneUsers, len(users))
		job.TablesDone = doneUsers
		job.Rows = int64(doneItems)
		jobsMu.Unlock()
		setProgress(job, "迁移正文", fmt.Sprintf("已处理 %d/%d 个作者、%d/%d 条正文 → %s",
			doneUsers, len(users), doneItems, totalItems, target), doneItems, totalItems)
	}

	// 小说 index.json 的库内副本也一并补齐（不依赖文件，纯数据库模式可读）
	setStage(job, "重建小说索引", "正在把 index.json 同步进数据库")
	rebuildNovelIndexes(job)

	setStage(job, "切换存储模式", "正在写入 storage_mode 配置")
	if err := models.SetPlatformConfig(storageModeConfigKey, target); err != nil {
		finishJob(job, "error", "内容已迁移，但写入存储模式配置失败: "+err.Error())
		return
	}
	job.Detail["mode"] = target
	job.Detail["items"] = doneItems
	job.Detail["ok_items"] = total.OK
	job.Detail["empty_items"] = total.Empty
	job.Detail["skipped_items"] = total.Skipped
	job.Detail["missing_after"] = countMissingContent()
	bodyBytes := sumBlobBytes(contentstore.RefChapter) + sumBlobBytes(contentstore.RefBlog)
	job.Detail["content_bytes"] = bodyBytes
	job.Detail["summary"] = fmt.Sprintf("已入库 %d 条（正文合计 %s），源正文为空 %d 条，源缺失跳过 %d 条",
		total.OK, humanBytes(bodyBytes), total.Empty, total.Skipped)
}

// humanBytes 给任务文案用的粗略体积（决定「体积没变」是不是错觉时更直观）
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	v := float64(n)
	i := -1
	for v >= unit && i < len(units)-1 {
		v /= unit
		i++
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

// countContentItems 待迁移正文总数（用于按条目而非按用户显示百分比）
func countContentItems() int {
	if models.DB == nil {
		return 0
	}
	var chapters, blogs int64
	_ = models.DB.Model(&models.Chapter{}).Count(&chapters).Error
	_ = models.DB.Model(&blogm.AuthorBlog{}).Count(&blogs).Error
	return int(chapters + blogs)
}

// countMissingContent 统计没有库内正文的内容条数（迁移修复效果可直接看出来）。
func countMissingContent() int64 {
	if models.DB == nil {
		return 0
	}
	var chapters, blogs int64
	_ = models.DB.Model(&models.Chapter{}).
		Where("id NOT IN (SELECT ref_id FROM content_blobs WHERE ref_type = ?)", contentstore.RefChapter).
		Count(&chapters).Error
	_ = models.DB.Model(&blogm.AuthorBlog{}).
		Where("id NOT IN (SELECT ref_id FROM content_blobs WHERE ref_type = ?)", contentstore.RefBlog).
		Count(&blogs).Error
	return chapters + blogs
}

func collectContentUsers() []uint {
	seen := map[uint]bool{}
	out := []uint{}
	if models.DB != nil {
		var chapterUsers, blogUsers []uint
		// ⚠️ 不能过滤 author_id > 0：老数据大量章节 author_id=0（作者信息只在 novel 上），
		// 之前按 >0 分批导致 2040 章漏迁、切到纯数据库模式后正文读不出来。
		_ = models.DB.Model(&models.Chapter{}).Select("DISTINCT author_id").Scan(&chapterUsers).Error
		_ = models.DB.Model(&blogm.AuthorBlog{}).Select("DISTINCT author_id").Scan(&blogUsers).Error
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

// migrateTally 迁移结果分类计数：区分「正常入库」「源正文为空」「源缺失跳过」，
// 否则管理员看到条数对不上时无从判断是漏迁还是本来就空。
type migrateTally struct {
	OK      int
	Empty   int
	Skipped int
}

// migrateUserContent 返回本用户已处理的内容条数
func migrateUserContent(uid uint, target string, chunk int, onProgress func(int)) (int, migrateTally) {
	if chunk <= 0 {
		chunk = 20
	}
	count := 0
	var tally migrateTally
	add := func(st string) {
		switch st {
		case statusEmpty:
			tally.Empty++
		case statusSkipped:
			tally.Skipped++
		default:
			tally.OK++
		}
	}
	// 章节：按用户分批取，再按 20 条一切
	for offset := 0; ; offset += chunk {
		var chapters []models.Chapter
		err := models.DB.Where("author_id = ?", uid).Order("id ASC").
			Limit(chunk).Offset(offset).Find(&chapters).Error
		if err != nil || len(chapters) == 0 {
			break
		}
		for _, ch := range chapters {
			add(migrateChapter(ch, target))
			count++
		}
		if onProgress != nil {
			onProgress(count)
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
			add(migrateBlog(uid, b, target))
			count++
		}
		if onProgress != nil {
			onProgress(count)
		}
		if len(blogs) < chunk {
			break
		}
	}
	return count, tally
}

func chapterFilePath(ch models.Chapter) string {
	if ch.ContentPath != "" {
		return sanitize.ResolveContentPath(ch.ContentPath)
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
		return sanitize.ResolveContentPath(b.ContentPath)
	}
	return filepath.Join(filepath.Dir(config.NovelDataDir), "blogs",
		strconv.FormatUint(uint64(uid), 10), fmt.Sprintf("%d.md", b.ID))
}

const (
	statusOK      = "ok"
	statusEmpty   = "empty"
	statusSkipped = "skipped"
)

func migrateChapter(ch models.Chapter, target string) string {
	path := chapterFilePath(ch)
	if target == contentstore.ModeDatabase {
		// 文件优先（迁移前还是混合模式），文件丢了再回退库内镜像
		text, err := os.ReadFile(path)
		if err != nil {
			if fromDB, ok := contentstore.LoadDB(contentstore.RefChapter, ch.ID); ok {
				text = []byte(fromDB)
			} else {
				// 源文件与库内副本都没有：这条正文已经丢了，别写空副本假装迁移成功
				return statusSkipped
			}
		}
		_ = contentstore.Put(contentstore.RefChapter, ch.ID, "", string(text))
		if len(text) == 0 {
			return statusEmpty
		}
		return statusOK
	}
	// 切回混合：库内正文写回文件
	if text, ok := contentstore.LoadDB(contentstore.RefChapter, ch.ID); ok && path != "" {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, []byte(text), 0o644)
		return statusOK
	}
	return statusSkipped
}

func migrateBlog(uid uint, b blogm.AuthorBlog, target string) string {
	path := blogFilePath(uid, b)
	if target == contentstore.ModeDatabase {
		text, err := os.ReadFile(path)
		if err != nil {
			if fromDB, ok := contentstore.LoadDB(contentstore.RefBlog, b.ID); ok {
				text = []byte(fromDB)
			} else {
				return statusSkipped
			}
		}
		_ = contentstore.Put(contentstore.RefBlog, b.ID, "", string(text))
		if len(text) == 0 {
			return statusEmpty
		}
		return statusOK
	}
	if text, ok := contentstore.LoadDB(contentstore.RefBlog, b.ID); ok && path != "" {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, []byte(text), 0o644)
		return statusOK
	}
	return statusSkipped
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
