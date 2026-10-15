package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"html"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ═══ 内容抢救台：隔离的应急窗口 ═══
// 不暴露通用数据库表，只提供极简的「输入 ID → 预览 → 硬删除」接口。
// 所有操作均需二级密码（bcrypt）解锁后持有短时 token 才能执行。

var (
	emergencyTokens   = map[string]time.Time{}
	emergencyTokensMu sync.Mutex
)

const emergencyTokenTTL = 5 * time.Minute

// 二级密码防爆破：失败次数锁定（按 IP）
var (
	emergencyFailCounts = map[string]int{}
	emergencyLockUntil  = map[string]time.Time{}
	emergencyFailMu     sync.Mutex
)

const (
	maxEmergencyFailures  = 5
	emergencyLockDuration = 15 * time.Minute
)

func isEmergencyLocked(ip string) bool {
	emergencyFailMu.Lock()
	defer emergencyFailMu.Unlock()
	if t, ok := emergencyLockUntil[ip]; ok {
		if time.Now().Before(t) {
			return true
		}
		// 锁定已过期，仅清理锁定标记（保留失败计数，避免每次检查清零）
		delete(emergencyLockUntil, ip)
	}
	return false
}

func recordEmergencyFailure(ip string) {
	emergencyFailMu.Lock()
	defer emergencyFailMu.Unlock()
	emergencyFailCounts[ip]++
	if emergencyFailCounts[ip] >= maxEmergencyFailures {
		emergencyLockUntil[ip] = time.Now().Add(emergencyLockDuration)
		delete(emergencyFailCounts, ip)
	}
}

func clearEmergencyFailures(ip string) {
	emergencyFailMu.Lock()
	defer emergencyFailMu.Unlock()
	delete(emergencyFailCounts, ip)
	delete(emergencyLockUntil, ip)
}

// auditEmergency 记录抢救台审计日志（谁/IP/何时/什么操作/目标）
func auditEmergency(c *gin.Context, action, targetType string, targetID uint, detail string) {
	userID := currentUserID(c)
	operator := ""
	if userID > 0 {
		if u, err := models.GetUserByID(userID); err == nil && u != nil {
			operator = u.Username
		}
	}
	models.RecordEmergencyAudit(&models.EmergencyAuditLog{
		OperatorID: userID,
		Operator:   operator,
		IP:         c.ClientIP(),
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     detail,
	})
}

func issueEmergencyToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)
	emergencyTokensMu.Lock()
	emergencyTokens[token] = time.Now().Add(emergencyTokenTTL)
	// 顺带清理过期 token
	now := time.Now()
	for k, v := range emergencyTokens {
		if now.After(v) {
			delete(emergencyTokens, k)
		}
	}
	emergencyTokensMu.Unlock()
	return token
}

func verifyEmergencyToken(token string) bool {
	emergencyTokensMu.Lock()
	defer emergencyTokensMu.Unlock()
	exp, ok := emergencyTokens[token]
	if !ok || time.Now().After(exp) {
		delete(emergencyTokens, token)
		return false
	}
	return true
}

// POST /api/admin/emergency/set-password — 设置/修改二级密码（管理员）
func SetEmergencyPassword(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.NewPassword) < 6 {
		utils.BadRequest(c, "新密码至少 6 位")
		return
	}
	oldHash := models.GetPlatformConfig("emergency_password_hash")
	// 若已设置过，需验证旧密码
	if oldHash != "" && !utils.CheckPassword(req.OldPassword, oldHash) {
		utils.Forbidden(c, "旧密码错误")
		return
	}
	hash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		utils.InternalError(c, "设置密码失败")
		return
	}
	if err := models.SetPlatformConfig("emergency_password_hash", hash); err != nil {
		utils.InternalError(c, "保存密码失败")
		return
	}
	// 修改密码后清空所有已签发 token，强制重新解锁
	emergencyTokensMu.Lock()
	emergencyTokens = map[string]time.Time{}
	emergencyTokensMu.Unlock()
	auditEmergency(c, "set_password", "", 0, "二级密码已设置/修改")
	utils.Success(c, gin.H{"message": "二级密码已设置"})
}

// POST /api/admin/emergency/unlock — 验证二级密码，签发短时 token（防爆破锁死）
func UnlockEmergency(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	ip := c.ClientIP()
	if isEmergencyLocked(ip) {
		utils.Forbidden(c, "尝试次数过多，已锁定 15 分钟")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	hash := models.GetPlatformConfig("emergency_password_hash")
	if hash == "" {
		utils.BadRequest(c, "尚未设置二级密码，请先设置")
		return
	}
	if !utils.CheckPassword(req.Password, hash) {
		recordEmergencyFailure(ip)
		auditEmergency(c, "unlock_fail", "", 0, "二级密码错误")
		utils.Forbidden(c, "二级密码错误")
		return
	}
	clearEmergencyFailures(ip)
	auditEmergency(c, "unlock", "", 0, "二级密码解锁成功")
	token := issueEmergencyToken()
	utils.Success(c, gin.H{"token": token, "expires_in": int(emergencyTokenTTL.Seconds())})
}

// stripTagRe 去除 HTML 标签（用于纯文本预览）
var stripTagRe = regexp.MustCompile(`<[^>]*>`)

func toPlainText(content string) string {
	s := html.UnescapeString(content) // 反转义 &lt; &gt; 等
	s = stripTagRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&amp;", "&")
	return strings.TrimSpace(s)
}

// buildCommentList 将评论切片转换为统一的预览列表（含作者/纯文本/章节号）
func buildCommentList(comments []models.Comment) []gin.H {
	list := make([]gin.H, 0, len(comments))
	for _, cm := range comments {
		author := ""
		if cm.User != nil {
			author = cm.User.Username
		}
		if author == "" && cm.Username != "" {
			author = cm.Username
		}
		item := gin.H{
			"id":            cm.ID,
			"type":          "comment",
			"author":        author,
			"content":       cm.Content,
			"content_plain": toPlainText(cm.Content),
			"created_at":    cm.CreatedAt,
		}
		if cm.ChapterNumber > 0 {
			item["chapter"] = cm.ChapterNumber
		}
		list = append(list, item)
	}
	return list
}

// buildPostList 将帖子回复切片转换为统一的预览列表
func buildPostList(posts []models.Post) []gin.H {
	list := make([]gin.H, 0, len(posts))
	for _, p := range posts {
		author := ""
		if p.User != nil {
			author = p.User.Username
		}
		list = append(list, gin.H{
			"id":            p.ID,
			"type":          "post",
			"author":        author,
			"content":       p.Content,
			"content_plain": toPlainText(p.Content),
			"created_at":    p.CreatedAt,
		})
	}
	return list
}

// resolveContentPath 将站内 URL 路径解析为 (类型, ID)。
// 站长不需要知道内部 ID，直接粘贴站内路径即可定位内容。
// 支持：/thread/:id /post/:id /comment/:id /blog/:id
//
//	/author/:aid/blog/:blogId /author/:aid/blogs/:blogId
//	/novel/:id /author/:aid/novel/:id
//	/forum/:id /forums/:id
//
// 也兼容误粘贴的完整 URL（自动剥离域名部分）。
func resolveContentPath(path string) (ctype string, id uint64, ok bool) {
	path = strings.TrimSpace(path)
	// 剥离完整 URL 的域名部分（如 https://example.com/thread/123 → /thread/123）
	if i := strings.Index(path, "://"); i >= 0 {
		rest := path[i+3:]
		if j := strings.IndexByte(rest, '/'); j >= 0 {
			path = rest[j:]
		} else {
			return "", 0, false
		}
	}
	path = strings.Trim(path, "/")
	if path == "" {
		return "", 0, false
	}
	parts := strings.Split(path, "/")
	parseID := func(s string) (uint64, bool) {
		n, err := strconv.ParseUint(s, 10, 64)
		return n, err == nil && n > 0
	}

	switch {
	case parts[0] == "thread" && len(parts) == 2:
		if n, ok := parseID(parts[1]); ok {
			return "thread", n, true
		}
	case parts[0] == "post" && len(parts) == 2:
		if n, ok := parseID(parts[1]); ok {
			return "post", n, true
		}
	case parts[0] == "comment" && len(parts) == 2:
		if n, ok := parseID(parts[1]); ok {
			return "comment", n, true
		}
	case parts[0] == "chapter" && len(parts) == 2:
		if n, ok := parseID(parts[1]); ok {
			return "chapter", n, true
		}
	case parts[0] == "blog" && len(parts) == 2:
		if n, ok := parseID(parts[1]); ok {
			return "blog", n, true
		}
	case (parts[0] == "forum" || parts[0] == "forums") && len(parts) == 2:
		if n, ok := parseID(parts[1]); ok {
			return "forum", n, true
		}
	case parts[0] == "novel" && len(parts) == 2:
		if n, ok := parseID(parts[1]); ok {
			return "novel", n, true
		}
	case parts[0] == "novel" && len(parts) == 4 && parts[2] == "read":
		// /novel/:id/read/:chapter → 定位到具体章节（返回 chapter 类型）
		if novelID, ok1 := parseID(parts[1]); ok1 {
			if num, ok2 := parseID(parts[3]); ok2 {
				if ch, err := models.GetChapterByNovelAndNumber(uint(novelID), int(num)); err == nil && ch != nil {
					return "chapter", uint64(ch.ID), true
				}
			}
		}
	case parts[0] == "author" && len(parts) == 4:
		if parts[2] == "blog" || parts[2] == "blogs" {
			if n, ok := parseID(parts[3]); ok {
				return "blog", n, true
			}
		}
		if parts[2] == "novel" {
			if n, ok := parseID(parts[3]); ok {
				return "novel", n, true
			}
		}
	}
	return "", 0, false
}

// loadFileContent 安全读取内容文件（博客/章节正文以文件形式存储）
func loadFileContent(path string) string {
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(utils.ResolveContentPath(path))
	if err != nil {
		return ""
	}
	return string(b)
}

// 说明：预览/删除目标已改为从 POST body 解析（见 PreviewEmergencyContent / DeleteEmergencyContent），
// 原 resolveTarget（query 解析）已移除——避免敏感参数继续走 URL。

// POST /api/admin/emergency/preview — 拉取内容预览（纯文本 + 作者 + 原始内容）
// 入参（🔒 安全审计：token 走 body，不再进 URL query——避免落入访问日志/浏览器历史）
// body: { token, path（如 /thread/123）或 type+id }
func PreviewEmergencyContent(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Token string `json:"token"`
		Path  string `json:"path"`
		Type  string `json:"type"`
		ID    uint64 `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if !verifyEmergencyToken(req.Token) {
		utils.Forbidden(c, "二级密码未验证或已过期")
		return
	}
	ctype, id := "", uint64(0)
	if req.Path != "" {
		var ok bool
		ctype, id, ok = resolveContentPath(req.Path)
		if !ok {
			utils.BadRequest(c, "请输入站内 URL 路径（如 /thread/123 或 /blog/5）")
			return
		}
	} else if req.Type != "" && req.ID > 0 {
		ctype, id = req.Type, req.ID
	} else {
		utils.BadRequest(c, "请输入站内 URL 路径（如 /thread/123 或 /blog/5），或提供 type+id")
		return
	}

	preview := gin.H{"type": ctype, "id": id, "path": buildContentPath(ctype, id)}
	switch ctype {
	case "comment":
		cm, err := models.GetCommentByID(uint(id))
		if err != nil {
			utils.NotFound(c, "评论不存在")
			return
		}
		author := ""
		if cm.User != nil {
			author = cm.User.Username
		}
		preview["author"] = author
		preview["content_raw"] = cm.Content
		preview["content_plain"] = toPlainText(cm.Content)
		preview["created_at"] = cm.CreatedAt
	case "thread":
		t, err := models.GetThreadByID(uint(id))
		if err != nil {
			utils.NotFound(c, "帖子不存在")
			return
		}
		author := ""
		if t.User != nil {
			author = t.User.Username
		}
		preview["author"] = author
		preview["title"] = t.Title
		preview["content_raw"] = t.Content
		preview["content_plain"] = toPlainText(t.Content)
		preview["created_at"] = t.CreatedAt
		// 帖子下的全部回复（楼层模型：本页楼层 + 楼内回复）
		if pg, err := models.GetPostsByThread(uint(id), 1, 500); err == nil {
			preview["comments"] = buildPostList(pg.List)
		}
	case "post":
		var p models.Post
		if err := models.DB.Preload("User").First(&p, id).Error; err != nil {
			utils.NotFound(c, "回复不存在")
			return
		}
		author := ""
		if p.User != nil {
			author = p.User.Username
		}
		preview["author"] = author
		preview["content_raw"] = p.Content
		preview["content_plain"] = toPlainText(p.Content)
		preview["created_at"] = p.CreatedAt
	case "blog":
		b, err := models.GetBlogByID(uint(id))
		if err != nil {
			utils.NotFound(c, "博客不存在")
			return
		}
		author := ""
		if b.Author != nil {
			author = b.Author.Username
		}
		content := loadFileContent(b.ContentPath)
		preview["author"] = author
		preview["title"] = b.Title
		preview["content_raw"] = content
		preview["content_plain"] = toPlainText(content)
		preview["created_at"] = b.CreatedAt
		if pg, err := models.GetCommentsByBlog(uint(id), 1, 500, false); err == nil {
			preview["comments"] = buildCommentList(pg.List)
		}
	case "novel":
		n, err := models.GetNovelByID(uint(id))
		if err != nil {
			utils.NotFound(c, "作品不存在")
			return
		}
		author := ""
		if n.Author != nil {
			author = n.Author.Username
		}
		preview["author"] = author
		preview["title"] = n.Title
		preview["content_raw"] = n.Summary
		preview["content_plain"] = n.Summary
		preview["created_at"] = n.CreatedAt
		// 输入小说 URL 时，重点展示该作品的全部评论（而非仅标题）
		if comments, err := models.GetAllCommentsByNovel(uint(id), 500); err == nil {
			preview["comments"] = buildCommentList(comments)
		}
	case "chapter":
		ch, err := models.GetChapterByID(uint(id))
		if err != nil {
			utils.NotFound(c, "章节不存在")
			return
		}
		author := ""
		novelTitle := ""
		if n, err := models.GetNovelByID(ch.NovelID); err == nil && n != nil {
			novelTitle = n.Title
			if n.Author != nil {
				author = n.Author.Username
			}
		}
		content := loadFileContent(ch.ContentPath)
		preview["author"] = author
		preview["title"] = ch.Title
		preview["novel_title"] = novelTitle
		preview["chapter_number"] = ch.ChapterNumber
		preview["content_raw"] = content
		preview["content_plain"] = toPlainText(content)
		preview["created_at"] = ch.CreatedAt
	case "forum":
		var f models.Forum
		if err := models.DB.First(&f, id).Error; err != nil {
			utils.NotFound(c, "论坛板块不存在")
			return
		}
		preview["author"] = ""
		preview["title"] = f.Name
		preview["content_raw"] = f.Description
		preview["content_plain"] = f.Description
		preview["created_at"] = f.CreatedAt
	default:
		utils.BadRequest(c, "无法识别的类型（支持 comment/thread/post/blog/novel/chapter/forum）")
		return
	}
	auditEmergency(c, "preview", ctype, uint(id), buildContentPath(ctype, id))
	utils.Success(c, preview)
}

// buildContentPath 构造类型对应的站内 URL 路径（用于展示与审计）
func buildContentPath(ctype string, id uint64) string {
	switch ctype {
	case "thread":
		return "/thread/" + strconv.FormatUint(id, 10)
	case "post":
		return "/post/" + strconv.FormatUint(id, 10)
	case "comment":
		return "/comment/" + strconv.FormatUint(id, 10)
	case "chapter":
		return "/chapter/" + strconv.FormatUint(id, 10)
	case "blog":
		return "/blog/" + strconv.FormatUint(id, 10)
	case "novel":
		return "/novel/" + strconv.FormatUint(id, 10)
	case "forum":
		return "/forum/" + strconv.FormatUint(id, 10)
	default:
		return ""
	}
}

// POST /api/admin/emergency/content — 软删除内容（归档留证 + 软删除 + 统计回退 + 日志）
// 入参（🔒 安全审计：token 走 body，不再进 URL query）
// body: { token, path（站内 URL 路径）或 type+id }
// 说明：删除前先将完整快照归档到 softdel_ 表（留证），再软删除原始记录（保留 deleted_at），
//
//	不再物理移除正文文件，确保任何应急处置均可在 softdel_ 表中追溯。
func DeleteEmergencyContent(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Token string `json:"token"`
		Path  string `json:"path"`
		Type  string `json:"type"`
		ID    uint64 `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if !verifyEmergencyToken(req.Token) {
		utils.Forbidden(c, "二级密码未验证或已过期")
		return
	}
	ctype, id := "", uint64(0)
	if req.Path != "" {
		var ok bool
		ctype, id, ok = resolveContentPath(req.Path)
		if !ok {
			utils.BadRequest(c, "请输入站内 URL 路径（如 /thread/123 或 /blog/5）")
			return
		}
	} else if req.Type != "" && req.ID > 0 {
		ctype, id = req.Type, req.ID
	} else {
		utils.BadRequest(c, "请输入站内 URL 路径，或提供 type+id")
		return
	}

	operatorID := currentUserID(c)
	operator := ""
	if operatorID > 0 {
		if u, err := models.GetUserByID(operatorID); err == nil && u != nil {
			operator = u.Username
		}
	}
	archive := func(t string, originalID uint, reason, summary string, snapshot interface{}) error {
		return models.ArchiveSoftDelete("emergency", t, originalID, operatorID, operator, reason, summary, snapshot)
	}

	switch ctype {
	case "comment":
		cm, err := models.GetCommentByID(uint(id))
		if err != nil {
			utils.NotFound(c, "评论不存在")
			return
		}
		if err := archive("comment", uint(id), "", truncateRunes(toPlainText(cm.Content), 200), cm); err != nil {
			utils.InternalError(c, "留证归档失败")
			return
		}
		if err := models.DeleteComment(uint(id)); err != nil {
			utils.InternalError(c, "删除失败")
			return
		}
		auditEmergency(c, "delete", ctype, uint(id), "删除评论（作者 "+userOf(cm.UserID)+"）")
	case "thread":
		t, err := models.GetThreadByID(uint(id))
		if err != nil {
			utils.NotFound(c, "帖子不存在")
			return
		}
		if err := archive("thread", uint(id), "", t.Title, t); err != nil {
			utils.InternalError(c, "留证归档失败")
			return
		}
		// 方案A：posts 无 deleted_at，级联删回帖前先批量归档留证，再物理删除
		if _, aerr := models.ArchivePostsByThread(models.DB, uint(id), operatorID, operator, "", "emergency"); aerr != nil {
			utils.InternalError(c, "回帖留证归档失败")
			return
		}
		if t.ForumID > 0 {
			models.DB.Model(&models.Forum{}).Where("id = ?", t.ForumID).
				UpdateColumn("thread_count", models.DB.Raw("CASE WHEN thread_count > 0 THEN thread_count - 1 ELSE 0 END"))
		}
		models.DB.Delete(&models.Thread{}, id)
		models.DB.Where("thread_id = ?", id).Delete(&models.Post{})
		auditEmergency(c, "delete", ctype, uint(id), "删除帖子（作者 "+userOf(t.UserID)+"）")
	case "post":
		var p models.Post
		if err := models.DB.First(&p, id).Error; err != nil {
			utils.NotFound(c, "回复不存在")
			return
		}
		if err := archive("post", uint(id), "", truncateRunes(toPlainText(p.Content), 200), p); err != nil {
			utils.InternalError(c, "留证归档失败")
			return
		}
		models.DB.Delete(&models.Post{}, id)
		if p.ThreadID > 0 {
			models.DB.Model(&models.Thread{}).Where("id = ?", p.ThreadID).
				UpdateColumn("post_count", models.DB.Raw("CASE WHEN post_count > 0 THEN post_count - 1 ELSE 0 END"))
		}
		auditEmergency(c, "delete", ctype, uint(id), "删除回复（作者 "+userOf(p.UserID)+"）")
	case "blog":
		b, err := models.GetBlogByID(uint(id))
		if err != nil {
			utils.NotFound(c, "博客不存在")
			return
		}
		if err := archive("blog", uint(id), "", b.Title, b); err != nil {
			utils.InternalError(c, "留证归档失败")
			return
		}
		models.DB.Delete(&models.AuthorBlog{}, id)
		models.DB.Where("blog_id = ?", id).Delete(&models.Comment{})
		auditEmergency(c, "delete", ctype, uint(id), "删除博客《"+b.Title+"》（作者 "+userOf(b.AuthorID)+"）")
	case "novel":
		n, err := models.GetNovelByID(uint(id))
		if err != nil {
			utils.NotFound(c, "作品不存在")
			return
		}
		if err := archive("novel", uint(id), "", n.Title, n); err != nil {
			utils.InternalError(c, "留证归档失败")
			return
		}
		models.DB.Where("novel_id = ?", id).Delete(&models.Chapter{})
		models.DB.Where("novel_id = ?", id).Delete(&models.Comment{})
		models.DB.Where("novel_id = ?", id).Delete(&models.Rating{})
		models.DB.Where("novel_id = ?", id).Delete(&models.BookShelf{})
		models.DB.Where("novel_id = ?", id).Delete(&models.NovelCategory{})
		models.DB.Delete(&models.Novel{}, id)
		auditEmergency(c, "delete", ctype, uint(id), "删除作品《"+n.Title+"》及其章节/评论/评分（作者 "+userOf(n.AuthorID)+"）")
	case "chapter":
		ch, err := models.GetChapterByID(uint(id))
		if err != nil {
			utils.NotFound(c, "章节不存在")
			return
		}
		novelTitle := ""
		if n, err := models.GetNovelByID(ch.NovelID); err == nil && n != nil {
			novelTitle = n.Title
		}
		if err := archive("chapter", uint(id), "", "《"+novelTitle+"》"+ch.Title, ch); err != nil {
			utils.InternalError(c, "留证归档失败")
			return
		}
		if err := models.DeleteChapter(uint(id)); err != nil {
			utils.InternalError(c, "删除失败")
			return
		}
		auditEmergency(c, "delete", ctype, uint(id), "删除章节《"+ch.Title+"》（第 "+strconv.Itoa(ch.ChapterNumber)+" 章）")
	case "forum":
		var f models.Forum
		if err := models.DB.First(&f, id).Error; err != nil {
			utils.NotFound(c, "论坛板块不存在")
			return
		}
		if err := archive("forum", uint(id), "", f.Name, f); err != nil {
			utils.InternalError(c, "留证归档失败")
			return
		}
		models.DB.Where("forum_id = ?", id).Delete(&models.Thread{})
		models.DB.Delete(&models.Forum{}, id)
		auditEmergency(c, "delete", ctype, uint(id), "删除论坛板块「"+f.Name+"」及其下帖子/回复")
	default:
		utils.BadRequest(c, "无法识别的类型（支持 comment/thread/post/blog/novel/chapter/forum）")
		return
	}
	utils.Success(c, gin.H{"message": "已软删除并留证"})
}

func userOf(userID uint) string {
	if u, err := models.GetUserByID(userID); err == nil && u != nil {
		return u.Username
	}
	return strconv.Itoa(int(userID))
}

// GET /api/admin/emergency/audit — 查看抢救台审计日志（谁/IP/何时/什么操作）
func ListEmergencyAudit(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	logs, err := models.ListEmergencyAuditLogs(200)
	if err != nil {
		utils.InternalError(c, "获取审计日志失败")
		return
	}
	utils.Success(c, logs)
}
