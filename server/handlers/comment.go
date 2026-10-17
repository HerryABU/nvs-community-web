package handlers

import (
	"strconv"
	"time"

	"nvs-server/models"
	"nvs-server/realtime"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

func GetComments(c *gin.Context) {
	novelID, _ := strconv.ParseUint(c.Query("novel_id"), 10, 64)
	blogID, _ := strconv.ParseUint(c.Query("blog_id"), 10, 64)
	chapterNum, _ := strconv.Atoi(c.DefaultQuery("chapter_number", "0"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	// 楼层模型：List = 本页楼层 + 这些楼层下的全部回复（平铺，前端按 root_id 分组成「楼」）
	// 🔒 警方/管理员取证视图：includeDeleted=true 一并返回已删除评论，前端以红字标注
	includeDeleted := isPoliceOrAdminUser(c)
	var pg models.CommentsPage
	var err error
	if blogID > 0 {
		pg, err = models.GetCommentsByBlog(uint(blogID), page, pageSize, includeDeleted)
	} else {
		pg, err = models.GetCommentsByNovel(uint(novelID), chapterNum, page, pageSize, includeDeleted)
	}
	if err != nil {
		utils.InternalError(c, "获取评论失败")
		return
	}

	// 黑名单 hide 模式（F13）：隐藏当前用户主动屏蔽（hide）的作者所发布的评论。
	// block 写侧拦截在 CreateComment 中保留；过滤逻辑复用 FilterHiddenItems。
	viewerID := currentUserID(c)
	pg.List = FilterHiddenItems(viewerID, pg.List, func(cm models.Comment) uint { return cm.UserID })

	// 楼加密（入口限制）：未解锁的加密楼 content 脱敏、楼内回复剔除。
	// 豁免：楼作者本人、警方/管理员；其余用户凭 X-Floor-Password 头（id:密码 多楼格式）解锁。
	_, isStaff, floorPwdHeader := resolveFloorPasswordViewer(c)
	pg.List = models.RedactLockedComments(pg.List, viewerID, isStaff, parseFloorPasswords(floorPwdHeader))

	utils.Success(c, gin.H{
		"list":        pg.List,
		"total":       pg.Total,
		"floor_total": pg.FloorTotal,
		"has_more":    pg.HasMore,
		"page":        page,
	})
}

func CreateComment(c *gin.Context) {
	userID := currentUserID(c)

	// 全局禁言检查
	if checkUserMuted(c) {
		return
	}

	// 实名强制门禁（站长开启 realname_required 时生效）
	if checkRealNameRequired(c) {
		return
	}

	var req struct {
		NovelID       uint   `json:"novel_id"`
		BlogID        uint   `json:"blog_id"`
		ChapterNumber int    `json:"chapter_number"`
		Content       string `json:"content" binding:"required"`
		QuoteText     string `json:"quote_text"`
		ParentID      *uint  `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请填写评论内容")
		return
	}

	if req.NovelID == 0 && req.BlogID == 0 {
		utils.BadRequest(c, "请指定作品或博客")
		return
	}

	// 黑名单门禁（F13）：被作品/博客作者拉黑（block 模式）者不可评论
	if req.NovelID > 0 {
		if novel, nerr := models.GetNovelByID(req.NovelID); nerr == nil && novel.AuthorID != userID {
			if models.IsBlacklisted(novel.AuthorID, userID) {
				utils.Forbidden(c, "你已被该作者拉黑，无法在其作品下评论")
				return
			}
		}
	}
	if req.BlogID > 0 {
		if blog, berr := models.GetBlogByID(req.BlogID); berr == nil && blog.AuthorID != userID {
			if models.IsBlacklisted(blog.AuthorID, userID) {
				utils.Forbidden(c, "你已被该作者拉黑，无法在其博客下评论")
				return
			}
		}
	}

	// 楼加密门禁：回复目标是加密楼内的评论时，未解锁用户不可回复
	// （防止借回复入口试探加密楼内容）。豁免：楼作者、警方/管理员、带对密码。
	if req.ParentID != nil && *req.ParentID != 0 {
		parent, perr := models.GetCommentByID(*req.ParentID)
		if perr == nil && parent.HasPassword {
			// 找到所属「楼」：父即顶层时取父，否则取父的 RootID
			floor := parent
			if parent.RootID != nil {
				if root, rerr := models.GetCommentByID(*parent.RootID); rerr == nil {
					floor = root
				}
			}
			if !models.FloorAccessUnlocked(floor, userID, isPoliceOrAdminUser(c), map[uint]string{floor.ID: c.GetHeader("X-Floor-Password")}) {
				utils.Forbidden(c, "该楼已加密，输入访问密码后才可回复")
				return
			}
		}
	}

	// XSS 净化 + 违禁词库净化（delete/replace 动作直接改写文本）
	raw := security.SanitizeUserContent(req.Content)
	cleaned, badHits := processBadWords(raw, "comment")

	// @ 提及：从已净化正文解析被提及用户（按 username/nickname 命中）
	mentions := models.ParseMentions(cleaned)

	comment := &models.Comment{
		UserID:        userID,
		NovelID:       req.NovelID,
		BlogID:        req.BlogID,
		ChapterNumber: req.ChapterNumber,
		Content:       cleaned,
		QuoteText:     security.SanitizePlainText(req.QuoteText),
		IsMarkdown:    true,
		// 微秒级时间戳：精确排序与评论网分析的时间估计；旧数据回填为 0
		CreatedMicro: time.Now().UnixMicro(),
		// @ 提及存储（JSON）；解析失败/用户不存在已在上游过滤
		MentionsRaw: models.MentionsToRaw(mentions),
	}
	if req.ParentID != nil {
		comment.ParentID = req.ParentID
	}

	if err := models.CreateComment(comment); err != nil {
		utils.InternalError(c, "发表评论失败")
		return
	}

	// 违禁词 mark 动作命中 → 写入待审标记记录
	flagBadWords(badHits, "comment", comment.ID, userID, raw, "")

	if user, err := models.GetUserByID(userID); err == nil {
		comment.Username = user.Username
	}
	// 让返回给前端的 comment 带上解析出的提及（AfterFind 仅对 DB 查询结果生效，此处手动填）
	comment.Mentions = mentions

	// @ 提及通知：被提及用户收到站内信（best-effort；跳过自己与已删除用户）
	if len(mentions) > 0 {
		excerpt := cleaned
		if len(excerpt) > 60 {
			excerpt = excerpt[:60] + "…"
		}
		for _, m := range mentions {
			if m.UserID == userID {
				continue
			}
			_ = models.PushNotification(userID, m.UserID, "comment_mention",
				"有人在评论中提到了你", "@"+m.Name+": "+excerpt, models.LinkForComment(comment))
		}
	}

	// 狂暴模式（F19）：向评论所在频道广播新评论（hub 内部按 Enabled/批量/带宽控制，
	// 慢客户端自动断开；关闭时为零开销直返）
	if req.NovelID > 0 {
		realtime.PublishCommentEvent("novel", req.NovelID, comment)
	} else if req.BlogID > 0 {
		realtime.PublishCommentEvent("blog", req.BlogID, comment)
	}

	utils.Success(c, comment)
}

func DeleteComment(c *gin.Context) {
	userID := currentUserID(c)
	commentID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	comment, err := models.GetCommentByID(uint(commentID))
	if err != nil {
		utils.NotFound(c, "评论不存在")
		return
	}

	if comment.UserID != userID {
		utils.Forbidden(c, "无权删除此评论")
		return
	}

	if err := models.DeleteComment(uint(commentID)); err != nil {
		utils.InternalError(c, "删除评论失败")
		return
	}

	// 标记为「用户自删」（区别于管理员删除：deleted_by='admin'）
	_ = models.DB.Unscoped().Model(&models.Comment{}).Where("id = ?", commentID).Update("deleted_by", "user")
	utils.Success(c, gin.H{"message": "评论已删除"})
}
