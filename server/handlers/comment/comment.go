package comment

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"nvs-server/aisafety"
	"nvs-server/crisis"
	"nvs-server/handlers/admin"
	"nvs-server/handlers/badword"
	"nvs-server/handlers/blacklist"
	"nvs-server/handlers/export"
	"nvs-server/handlers/forum"
	"nvs-server/handlers/hctx"
	"nvs-server/handlers/publish"
	"nvs-server/handlers/realname"
	"nvs-server/ipgeo"
	"nvs-server/models"
	"nvs-server/realtime"
	"nvs-server/security"
	sentimentstore "nvs-server/sentiment/store"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"nvs-server/models/blog"
)

func GetComments(c *gin.Context) {
	novelID, _ := strconv.ParseUint(c.Query("novel_id"), 10, 64)
	blogID, _ := strconv.ParseUint(c.Query("blog_id"), 10, 64)
	chapterNum, _ := strconv.Atoi(c.DefaultQuery("chapter_number", "0"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	// 排序：new(默认)/old/hot —— 白名单归一化，不拼用户输入进 SQL
	sortKey := models.CommentSortOrder(c.Query("sort"))

	// 楼层模型：List = 本页楼层 + 这些楼层下的全部回复（平铺，前端按 root_id 分组成「楼」）
	// 🔒 警方/管理员取证视图：includeDeleted=true 一并返回已删除评论，前端以红字标注
	includeDeleted := admin.IsPoliceOrAdminUser(c)
	var pg models.CommentsPage
	var err error
	if blogID > 0 {
		pg, err = models.GetCommentsByBlog(uint(blogID), page, pageSize, includeDeleted, sortKey)
	} else {
		pg, err = models.GetCommentsByNovel(uint(novelID), chapterNum, page, pageSize, includeDeleted, sortKey)
	}
	if err != nil {
		utils.InternalError(c, "获取评论失败")
		return
	}

	// 黑名单 hide 模式（F13）：隐藏当前用户主动屏蔽（hide）的作者所发布的评论。
	// block 写侧拦截在 CreateComment 中保留；过滤逻辑复用 blacklist.FilterHiddenItems。
	viewerID := hctx.CurrentUserID(c)
	pg.List = blacklist.FilterHiddenItems(viewerID, pg.List, func(cm models.Comment) uint { return cm.UserID })

	// 楼加密（入口限制）：未解锁的加密楼 content 脱敏、楼内回复剔除。
	// 豁免：楼作者本人、警方/管理员；其余用户凭 X-Floor-Password 头（id:密码 多楼格式）解锁。
	_, isStaff, floorPwdHeader := forum.ResolveFloorPasswordViewer(c)
	pg.List = models.RedactLockedComments(pg.List, viewerID, isStaff, forum.ParseFloorPasswords(floorPwdHeader))

	// 折叠阈值由服务端下发：前端不写死数字，改配置即可全站生效
	foldChars := 0
	if cfg, cerr := sentimentstore.GetConfig(); cerr == nil && cfg != nil {
		foldChars = cfg.FoldChars
	}
	utils.Success(c, gin.H{
		"fold_chars":  foldChars,
		"list":        pg.List,
		"total":       pg.Total,
		"floor_total": pg.FloorTotal,
		"has_more":    pg.HasMore,
		"page":        page,
		"sort":        sortKey,
	})
}

func CreateComment(c *gin.Context) {
	userID := hctx.CurrentUserID(c)

	// 全局禁言检查
	if forum.CheckUserMuted(c) {
		return
	}

	// 实名强制门禁（站长开启 realname_required 时生效）
	if realname.CheckRealNameRequired(c) {
		return
	}
	// 注册门槛（账号年龄 / 新号每日上限，gap-audit #14）
	if !publish.CheckPostGate(c, "comment") {
		return
	}

	var req struct {
		NovelID       uint     `json:"novel_id"`
		BlogID        uint     `json:"blog_id"`
		ChapterNumber int      `json:"chapter_number"`
		Content       string   `json:"content" binding:"required"`
		QuoteText     string   `json:"quote_text"`
		ParentID      *uint    `json:"parent_id"`
		Images        []string `json:"images"` // 评论图片：仅允许站内上传路径 /uploads/images/...
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
		if blog, berr := blog.GetBlogByID(req.BlogID); berr == nil && blog.AuthorID != userID {
			if models.IsBlacklisted(blog.AuthorID, userID) {
				utils.Forbidden(c, "你已被该作者拉黑，无法在其博客下评论")
				return
			}
		}
	}

	// 楼加密门禁：回复目标是加密楼内的评论时，未解锁用户不可回复
	// （防止借回复入口试探加密楼内容）。豁免：楼作者、警方/管理员、带对密码。
	// 同时记下被回复者 id：绝对保护模式要据此判断「这条评论是不是冲着高危用户来的」。
	//
	// 管理员可选：若站点选了「评论区不随楼加密」（默认），且明确允许对未解锁的楼回复，
	// 则放行 —— 此时楼内回复本来就对所有人可见，不存在「试探加密内容」的通道。
	parentAuthorID := uint(0)
	if req.ParentID != nil && *req.ParentID != 0 {
		parent, perr := models.GetCommentByID(*req.ParentID)
		if perr == nil {
			parentAuthorID = parent.UserID
		}
		if perr == nil && parent.HasPassword {
			// 找到所属「楼」：父即顶层时取父，否则取父的 RootID
			floor := parent
			if parent.RootID != nil {
				if root, rerr := models.GetCommentByID(*parent.RootID); rerr == nil {
					floor = root
				}
			}
			if !models.FloorAccessUnlocked(floor, userID, admin.IsPoliceOrAdminUser(c), map[uint]string{floor.ID: c.GetHeader("X-Floor-Password")}) {
				policy := models.LoadCommentPolicy()
				if policy.LockHidesReplies || !policy.LockedFloorCanReply {
					utils.Forbidden(c, "该楼已加密，输入访问密码后才可回复")
					return
				}
			}
		}
	}

	// XSS 净化 + 违禁词库净化（delete/replace 动作直接改写文本）
	raw := security.SanitizeUserContent(req.Content)
	cleaned, badHits := badword.ProcessBadWords(raw, "comment")

	// ── 评论区舆情守卫（话题驱动的条件拦截）──
	// 放在落库之前：拦下来就不该产生评论行，也不该发通知/广播。
	// 判定依据是「内容话题（AI 敲定）+ 本次文本」，不是纯关键词。
	if blocked, reason := guardComment(c, req.NovelID, req.BlogID, userID, cleaned); blocked {
		utils.Forbidden(c, "该评论在当前话题下不适宜发布（"+reason+"），请调整措辞后重试")
		return
	}

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
		// IP 属地：只存展示名（省 / 国家），不落原始 IP；内网与解析失败留空不展示
		Region: ipgeo.DisplayName(c.ClientIP()),
		// @ 提及存储（JSON）；解析失败/用户不存在已在上游过滤
		MentionsRaw: models.MentionsToRaw(mentions),
	}
	if req.ParentID != nil {
		comment.ParentID = req.ParentID
	}

	// ── 评论图片 AI 审核（落库前）──
	// 站内上传的图片也要过审：违规图（涉黄/涉暴/血腥/违禁品/二维码引流）直接拦截并留证，
	// 不让它出现在公共评论区。复用现有监管模型 B（多模态 image_url 直送）。
	images := sanitizeCommentImages(req.Images)
	if len(images) > 0 {
		comment.ImagesRaw = models.ImagesToRaw(images)
		if blocked, _ := moderateCommentImages(c, userID, comment, images, cleaned); blocked {
			return
		}
	}

	// ── 绝对保护模式（落库前判定）──
	// 抑郁倾向者几乎总是伴随着网暴。对处于高危观察期的用户，
	// 指向他的攻击性言论（哪怕只是「你太矫情了」）一律先折叠，不让它出现在受害者屏幕上。
	// 必须落库前判定：折叠的内容不该产生对外通知，更不该被推进公共频道。
	shielded := crisis.ShieldHighRiskTargets(comment, parentAuthorID, mentions)

	if err := models.CreateComment(comment); err != nil {
		utils.InternalError(c, "发表评论失败")
		return
	}

	// 违禁词 mark 动作命中 → 写入待审标记记录
	badword.FlagBadWords(badHits, "comment", comment.ID, userID, raw, "")

	// 绝对保护模式命中：柔和地告诉被保护的人（不指责攻击者，也不复述攻击内容）
	if len(shielded) > 0 {
		crisis.NotifyShieldedUser(shielded, userID)
	}

	// ── 未成年人心理危机防护 ──
	// 黑话命中 → 立即折叠 + 进入高危观察期 + 报给站长 + 向本人推送抚慰私信与危机热线。
	// 语义绝望信号（无黑话的求救）由 AI 异步分析，命中后同样走这套处置。
	if crisis := crisis.HandleMinorCrisis(comment, raw); crisis != nil {
		comment.Crisis = crisis
	}

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
	//
	// 被危机防护折叠 / 先审后发的评论不广播、不触发 AI 机器人：
	// 折叠的内容不该出现在公共频道里，机器人和它对话也没有意义。
	if comment.ModerationStatus == "" {
		if req.NovelID > 0 {
			realtime.PublishCommentEvent("novel", req.NovelID, comment)
		} else if req.BlogID > 0 {
			realtime.PublishCommentEvent("blog", req.BlogID, comment)
		}

		// @ai 评论区机器人（§5.6）：检测召唤标记，异步生成 AI 回复（默认关闭）
		export.MaybeTriggerAiBot(c, comment)
	}

	utils.Success(c, comment)
}

func DeleteComment(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
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

// sanitizeCommentImages 校验并规整评论图片 URL：
// 仅允许站内上传路径 /uploads/images/...，禁止外链/数据 URL 借道，限制最多 9 张并去重。
// 这是防「外链图片 + 数据 URL」绕过 AI 审核的关键闸门——只认后端 /api/author/image 落地的站内图。
func sanitizeCommentImages(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	const maxCommentImages = 9
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, u := range in {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		// 只允许站内相对路径：以 /uploads/images/ 开头，且不含协议头与路径穿越
		if !strings.HasPrefix(u, "/uploads/images/") {
			continue
		}
		if strings.Contains(u, "..") || strings.Contains(u, "://") || strings.Contains(u, "\\") {
			continue
		}
		if seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
		if len(out) >= maxCommentImages {
			break
		}
	}
	return out
}

// moderateCommentImages 对评论图片做 AI 多模态审核；返回 (是否拦截, 拦截理由)。
// 拦截时负责留证：软删除保留整条评论（含违规图 URL，供警方/管理员取证视图调取）
// + 写入判定留痕(ai_judge_logs) + 紧急审计(emergency_audit_logs)，并直接以 Forbidden 结束响应。
func moderateCommentImages(c *gin.Context, userID uint, comment *models.Comment, images []string, content string) (bool, string) {
	cfg, err := aisafety.GetConfig()
	if err != nil || cfg == nil || !cfg.JudgeEnabled {
		return false, ""
	}
	v := aisafety.JudgeImage(images, content)
	if v == nil {
		return false, ""
	}
	blocked := false
	if v.Failed {
		// 尊重 fail-open/fail-closed：B 挂掉时默认放行（fail-open，避免全站评论发不出），
		// 但站长关掉 fail-open 时（fail-closed）拦截。
		blocked = !cfg.JudgeFailOpen
	} else {
		blocked = v.Jailbroken && v.Score >= cfg.JudgeBlockScore
	}
	uname := comment.Username
	if uname == "" {
		if u, ue := models.GetUserByID(userID); ue == nil {
			uname = u.Username
		}
	}
	aisafety.LogImageModeration(cfg, userID, uname, v, blocked)
	if !blocked {
		return false, ""
	}
	// ── 违规图：拦截 + 留证 ──
	// 软删除保留（不调用 models.CreateComment，以免触发对外通知/广播），
	// 折叠的内容不该出现在公共频道或推给被回复者。
	comment.ResolveFloor()
	if cerr := models.DB.Create(comment).Error; cerr != nil {
		// 留证失败也要拦：以系统级失败兜底，不让违规图流出
	} else {
		if derr := models.DB.Delete(comment).Error; derr == nil {
			_ = models.DB.Unscoped().Model(&models.Comment{}).
				Where("id = ?", comment.ID).Update("deleted_by", "ai_moderation")
		}
	}
	detail := fmt.Sprintf("AI 图片审核拦截 | 类别=%s | 分数=%d | 理由=%s",
		strings.Join(v.Categories, "、"), v.Score, v.Reason)
	models.RecordEmergencyAudit(&models.EmergencyAuditLog{
		OperatorID: userID, Operator: uname, IP: c.ClientIP(),
		Action: models.AuditContentHide, TargetType: "comment", TargetID: comment.ID,
		Detail: detail + " | 处置依据=AI 多模态审核 | 是否报告=false",
	})
	reason := strings.TrimSpace(v.Reason)
	if reason == "" {
		reason = "图片内容疑似违规（" + strings.Join(v.Categories, "、") + "）"
	}
	utils.Forbidden(c, "评论中的图片未通过内容安全审核，已被拦截并留存记录（"+reason+"）")
	return true, reason
}
