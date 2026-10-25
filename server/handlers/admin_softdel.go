package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ─────────────────────────────────────────────────────────────
// 管理员内容治理：删除 / 封禁（软删除 + softdel_ 留证）
// 所有「删除/封禁」都会先归档完整快照到 softdel_ 表，再软删除原始记录。
// ─────────────────────────────────────────────────────────────

var errContentNotFound = fmt.Errorf("content not found")

// GET /api/admin/softdel — 软删除留证记录列表（type 可选过滤）
func AdminListSoftDel(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	ctype := c.Query("type")
	if ctype != "" {
		if _, ok := models.SoftDelTables[ctype]; !ok {
			utils.BadRequest(c, "无效的内容类型: "+ctype)
			return
		}
	}
	limit := 500
	if n, err := strconv.Atoi(c.Query("limit")); err == nil && n > 0 {
		limit = n
	}
	items, err := models.ListSoftDel(ctype, limit)
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}
	utils.Success(c, items)
}

// POST /api/admin/softdel — 管理员删除/封禁内容（软删除 + 留证）
// body: { type, id, reason?, action? }  action 可为 "delete"(默认) / "ban"
func AdminModerateContent(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Type   string `json:"type" binding:"required"`
		ID     uint   `json:"id" binding:"required"`
		Reason string `json:"reason"`
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := softDeleteContent(c, req.Type, req.ID, req.Reason, "admin", req.Action); err != nil {
		if err == errContentNotFound {
			utils.NotFound(c, "内容不存在或已被删除")
		} else {
			utils.InternalError(c, err.Error())
		}
		return
	}
	utils.Success(c, gin.H{"message": "已软删除并留证"})
}

// softDeleteContent 归档 + 软删除的通用实现（供管理员治理、举报处置等复用）。
// 返回 errContentNotFound 表示目标不存在。
func softDeleteContent(c *gin.Context, ctype string, id uint, reason, source, action string) error {
	return doSoftDelete(ctype, id, reason, source, action, currentUserID(c), c)
}

// softDeleteContentSystem 系统自动处置（AI 审核判定违规后自动下架）：
// 没有 HTTP 请求上下文，操作者记 0（系统），审计写审计表。
func softDeleteContentSystem(ctype string, id uint, reason, source, action string) error {
	return doSoftDelete(ctype, id, reason, source, action, 0, nil)
}

// doSoftDelete 归档 + 软删除的通用实现；c 可为 nil（系统自动处置）。
func doSoftDelete(ctype string, id uint, reason, source, action string, operatorID uint, c *gin.Context) error {
	operator := ""
	if operatorID > 0 {
		if u, err := models.GetUserByID(operatorID); err == nil && u != nil {
			operator = u.Username
		}
	}
	auditAction := models.AuditContentDelete
	if action == "ban" {
		auditAction = models.AuditUserBan
	}
	// 审计留痕：有请求上下文走抢救台审计（带 IP/UA）；
	// 系统自动处置（c == nil）没有请求，改记系统操作者，避免丢痕迹。
	audit := func(a, targetType string, targetID uint, detail string) {
		if c != nil {
			auditEmergency(c, a, targetType, targetID, detail)
			return
		}
		models.RecordEmergencyAudit(&models.EmergencyAuditLog{
			OperatorID: 0, Operator: "system:moderation", IP: "-",
			Action: a, TargetType: targetType, TargetID: targetID, Detail: detail,
		})
	}
	// 封禁原因兜底：ban_reason 是「封禁 vs 作者自删」的区分标记（GetAuthorBannedContent
	// 仅展示 ban_reason 非空的记录），管理员封禁未填写原因时写默认值，保证语义清晰。
	if (ctype == "novel" || ctype == "chapter") && strings.TrimSpace(reason) == "" {
		reason = "违反平台合规规范（176号令）"
	}

	switch ctype {
	case "comment":
		cm, err := models.GetCommentByID(id)
		if err != nil {
			return errContentNotFound
		}
		summary := truncateRunes(toPlainText(cm.Content), 200)
		if err := models.ArchiveSoftDelete(source, "comment", id, operatorID, operator, reason, summary, cm); err != nil {
			return err
		}
		if err := models.DeleteComment(id); err != nil {
			return err
		}
		audit(auditAction, "comment", id, fmt.Sprintf("管理员删除评论（作者 %s）| 原因：%s", userOf(cm.UserID), reason))

	case "user":
		u, err := models.GetUserByID(id)
		if err != nil {
			return errContentNotFound
		}
		if err := models.ArchiveSoftDelete(source, "user", id, operatorID, operator, reason, u.Username, userSnapshot(u)); err != nil {
			return err
		}
		if err := models.DB.Delete(&models.User{}, id).Error; err != nil {
			return err
		}
		audit(models.AuditUserBan, "user", id, fmt.Sprintf("管理员封禁用户 %s | 原因：%s", u.Username, reason))

	case "novel":
		n, err := models.GetNovelByID(id)
		if err != nil {
			return errContentNotFound
		}
		if err := models.ArchiveSoftDelete(source, "novel", id, operatorID, operator, reason, n.Title, n); err != nil {
			return err
		}
		// 写入 176 封禁原因（软删除前写入，作者端可查）
		_ = models.DB.Model(&models.Novel{}).Where("id = ?", id).Update("ban_reason", reason)
		// 级联软删章节，并继承封禁原因到章节（作者端可见章节被封原因）
		_ = models.DB.Model(&models.Chapter{}).Where("novel_id = ?", id).Update("ban_reason", reason)
		// 软删除作品及其章节、评论（均保留 deleted_at，可追溯）
		models.DB.Where("novel_id = ?", id).Delete(&models.Chapter{})
		models.DB.Where("novel_id = ?", id).Delete(&models.Comment{})
		if err := models.DeleteNovel(id); err != nil {
			return err
		}
		audit(auditAction, "novel", id, fmt.Sprintf("管理员封禁作品《%s》（作者 %s）| 原因：%s", n.Title, userOf(n.AuthorID), reason))

	case "chapter":
		ch, err := models.GetChapterByID(id)
		if err != nil {
			return errContentNotFound
		}
		if err := models.ArchiveSoftDelete(source, "chapter", id, operatorID, operator, reason, ch.Title, ch); err != nil {
			return err
		}
		// 写入 176 封禁原因（软删除前写入，作者端可查）
		_ = models.DB.Model(&models.Chapter{}).Where("id = ?", id).Update("ban_reason", reason)
		if err := models.DeleteChapter(id); err != nil {
			return err
		}
		audit(auditAction, "chapter", id, fmt.Sprintf("管理员封禁章节《%s》（第 %d 章）| 原因：%s", ch.Title, ch.ChapterNumber, reason))

	case "thread":
		t, err := models.GetThreadByID(id)
		if err != nil {
			return errContentNotFound
		}
		if err := models.ArchiveSoftDelete(source, "thread", id, operatorID, operator, reason, t.Title, t); err != nil {
			return err
		}
		// 方案A：posts 无 deleted_at，级联删回帖前先批量归档留证，再物理删除
		if _, aerr := models.ArchivePostsByThread(models.DB, id, operatorID, operator, reason, source); aerr != nil {
			return aerr
		}
		models.DB.Where("thread_id = ?", id).Delete(&models.Post{})
		if t.ForumID > 0 {
			models.DB.Model(&models.Forum{}).Where("id = ?", t.ForumID).
				UpdateColumn("thread_count", models.DB.Raw("CASE WHEN thread_count > 0 THEN thread_count - 1 ELSE 0 END"))
		}
		if err := models.DB.Delete(&models.Thread{}, id).Error; err != nil {
			return err
		}
		audit(auditAction, "thread", id, fmt.Sprintf("管理员删除帖子《%s》（作者 %s）| 原因：%s", t.Title, userOf(t.UserID), reason))

	case "post":
		var p models.Post
		if err := models.DB.First(&p, id).Error; err != nil {
			return errContentNotFound
		}
		if err := models.ArchiveSoftDelete(source, "post", id, operatorID, operator, reason, truncateRunes(toPlainText(p.Content), 200), p); err != nil {
			return err
		}
		if err := models.DB.Delete(&models.Post{}, id).Error; err != nil {
			return err
		}
		if p.ThreadID > 0 {
			models.DB.Model(&models.Thread{}).Where("id = ?", p.ThreadID).
				UpdateColumn("post_count", models.DB.Raw("CASE WHEN post_count > 0 THEN post_count - 1 ELSE 0 END"))
		}
		audit(auditAction, "post", id, fmt.Sprintf("管理员删除回复（作者 %s）| 原因：%s", userOf(p.UserID), reason))

	case "blog":
		b, err := models.GetBlogByID(id)
		if err != nil {
			return errContentNotFound
		}
		if err := models.ArchiveSoftDelete(source, "blog", id, operatorID, operator, reason, b.Title, b); err != nil {
			return err
		}
		models.DB.Where("blog_id = ?", id).Delete(&models.Comment{})
		if err := models.DeleteBlog(id); err != nil {
			return err
		}
		audit(auditAction, "blog", id, fmt.Sprintf("管理员删除博客《%s》（作者 %s）| 原因：%s", b.Title, userOf(b.AuthorID), reason))

	case "forum":
		f, err := models.GetForumByID(id)
		if err != nil {
			return errContentNotFound
		}
		if err := models.ArchiveSoftDelete(source, "forum", id, operatorID, operator, reason, f.Name, f); err != nil {
			return err
		}
		if err := models.DeleteForum(id); err != nil {
			return err
		}
		audit(auditAction, "forum", id, fmt.Sprintf("管理员删除论坛「%s」| 原因：%s", f.Name, reason))

	default:
		return fmt.Errorf("不支持的内容类型: %s", ctype)
	}

	// 标记 deleted_by='admin'：管理员封禁/删除与用户自删（deleted_by='user'）区分。
	// 回收站只允许恢复 deleted_by='user' 的记录；被封禁内容仅管理员可恢复。
	deletedByMark := map[string]interface{}{
		"comment": &models.Comment{},
		"novel":   &models.Novel{},
		"chapter": &models.Chapter{},
		"thread":  &models.Thread{},
		"blog":    &models.AuthorBlog{},
		"forum":   &models.Forum{},
	}
	if m, ok := deletedByMark[ctype]; ok {
		_ = models.DB.Unscoped().Model(m).Where("id = ?", id).Update("deleted_by", "admin")
	}
	// 管理员封禁作品时，其级联软删的章节同样标记为 admin
	if ctype == "novel" {
		_ = models.DB.Unscoped().Model(&models.Chapter{}).Where("novel_id = ?", id).Update("deleted_by", "admin")
	}
	return nil
}

// userSnapshot 构造用户留证快照（含合规追溯所需的注册/登录 IP）
func userSnapshot(u *models.User) map[string]interface{} {
	lastLogin := ""
	if u.LastLoginAt != nil {
		lastLogin = u.LastLoginAt.Format("2006-01-02 15:04:05")
	}
	return map[string]interface{}{
		"id":                 u.ID,
		"username":           u.Username,
		"email":              u.Email,
		"nickname":           u.Nickname,
		"role":               u.Role,
		"real_name_verified": u.RealNameVerified,
		"registration_ip":    u.RegistrationIP,
		"last_login_ip":      u.LastLoginIP,
		"last_login_at":      lastLogin,
		"created_at":         u.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

// truncateRunes 按字符数截断字符串
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
