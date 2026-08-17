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
	userID := c.GetUint("userID")
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

// resolveContentPath 将站内 URL 路径解析为 (类型, ID)。
// 站长不需要知道内部 ID，直接粘贴站内路径即可定位内容。
// 支持：/thread/:id /post/:id /comment/:id /blog/:id
//       /author/:aid/blog/:blogId /author/:aid/blogs/:blogId
//       /novel/:id /author/:aid/novel/:id
//       /forum/:id /forums/:id
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
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

// resolveTarget 解析预览/删除目标：优先 URL 路径，回退 type+id
func resolveTarget(c *gin.Context) (ctype string, id uint64, ok bool) {
	if path := c.Query("path"); path != "" {
		return resolveContentPath(path)
	}
	ctype = c.Query("type")
	n, err := strconv.ParseUint(c.Query("id"), 10, 64)
	if err != nil || n == 0 {
		return "", 0, false
	}
	return ctype, n, true
}

// GET /api/admin/emergency/preview — 拉取内容预览（纯文本 + 作者 + 原始内容）
// 参数：path（站内 URL 路径，如 /thread/123）或 type+id（兼容）
func PreviewEmergencyContent(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	if !verifyEmergencyToken(c.Query("token")) {
		utils.Forbidden(c, "二级密码未验证或已过期")
		return
	}
	ctype, id, ok := resolveTarget(c)
	if !ok {
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
		utils.BadRequest(c, "无法识别的类型（支持 comment/thread/post/blog/novel/forum）")
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

// DELETE /api/admin/emergency/content — 硬删除内容（物理删除 + 统计回退 + 日志）
// 参数：path（站内 URL 路径）或 type+id（兼容）
func DeleteEmergencyContent(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	if !verifyEmergencyToken(c.Query("token")) {
		utils.Forbidden(c, "二级密码未验证或已过期")
		return
	}
	ctype, id, ok := resolveTarget(c)
	if !ok {
		utils.BadRequest(c, "请输入站内 URL 路径，或提供 type+id")
		return
	}

	switch ctype {
	case "comment":
		if cm, err := models.GetCommentByID(uint(id)); err == nil {
			if err := models.DeleteComment(uint(id)); err != nil {
				utils.InternalError(c, "删除失败")
				return
			}
			auditEmergency(c, "delete", ctype, uint(id), "删除评论（作者 "+userOf(cm.UserID)+"）")
		} else {
			utils.NotFound(c, "评论不存在")
			return
		}
	case "thread":
		t, err := models.GetThreadByID(uint(id))
		if err != nil {
			utils.NotFound(c, "帖子不存在")
			return
		}
		// 回退论坛帖子计数
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
		if err := models.DeleteBlog(uint(id)); err != nil {
			utils.InternalError(c, "删除失败")
			return
		}
		models.DB.Where("blog_id = ?", id).Delete(&models.Comment{})
		if b.ContentPath != "" {
			_ = os.Remove(b.ContentPath)
		}
		auditEmergency(c, "delete", ctype, uint(id), "删除博客《"+b.Title+"》（作者 "+userOf(b.AuthorID)+"）")
	case "novel":
		n, err := models.GetNovelByID(uint(id))
		if err != nil {
			utils.NotFound(c, "作品不存在")
			return
		}
		// 删除章节及其正文文件
		var chapters []models.Chapter
		models.DB.Where("novel_id = ?", id).Find(&chapters)
		for _, ch := range chapters {
			if ch.ContentPath != "" {
				_ = os.Remove(ch.ContentPath)
			}
		}
		models.DB.Where("novel_id = ?", id).Delete(&models.Chapter{})
		models.DB.Where("novel_id = ?", id).Delete(&models.Comment{})
		models.DB.Where("novel_id = ?", id).Delete(&models.Rating{})
		models.DB.Where("novel_id = ?", id).Delete(&models.BookShelf{})
		models.DB.Where("novel_id = ?", id).Delete(&models.NovelCategory{})
		models.DeleteNovel(uint(id))
		auditEmergency(c, "delete", ctype, uint(id), "删除作品《"+n.Title+"》及其章节/评论/评分（作者 "+userOf(n.AuthorID)+"）")
	case "forum":
		var f models.Forum
		if err := models.DB.First(&f, id).Error; err != nil {
			utils.NotFound(c, "论坛板块不存在")
			return
		}
		var threads []models.Thread
		models.DB.Where("forum_id = ?", id).Find(&threads)
		for _, t := range threads {
			models.DB.Where("thread_id = ?", t.ID).Delete(&models.Post{})
		}
		models.DB.Where("forum_id = ?", id).Delete(&models.Thread{})
		models.DeleteForum(uint(id))
		auditEmergency(c, "delete", ctype, uint(id), "删除论坛板块「"+f.Name+"」及其下帖子/回复")
	default:
		utils.BadRequest(c, "无法识别的类型（支持 comment/thread/post/blog/novel/forum）")
		return
	}
	utils.Success(c, gin.H{"message": "已物理删除"})
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
