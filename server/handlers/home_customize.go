package handlers

// home_customize.go — 自定义主页（alist 式单框代码 + 双作用域 + 站长审核）。
//
// 安全模型（务必先读）：
//   - 我的主页装扮（/me）：仅本人登录态可见，允许原始 HTML+<script>，等同 alist 个人自定义（自 XSS）。
//   - 作者公开主页（/author/:id）：他人可见，写入即消毒（utils.SanitizeCustomHTML，去脚本/危险 CSS），
//     并进入站长/管理员审核队列（pending）。自动化预检若发现问题（removed 非空），原文 raw_html
//     一并送站长复核；站长检查原文后通过，则 public_html = safe_html（绝不发布原文脚本）对外可见。
//   - 通知站长不依赖硬编码：按 role 查询管理员，复用现有 PushNotification 模式（见 notifyAdminsHTMLReview）。

import (
	"fmt"
	"strings"

	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// GetMyHomeCustomize GET /api/me/home-customize（需登录）
// 返回本人的「我的主页装扮」原始 HTML，与「作者公开主页」提交/审核状态。
func GetMyHomeCustomize(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	var pub models.AuthorHomeCustomize
	models.DB.Where("author_id = ?", uid).First(&pub)
	utils.Success(c, gin.H{
		"my_raw_html": models.GetUserHomeCustomize(uid),
		"author": gin.H{
			"raw_html":      pub.RawHTML,
			"safe_html":     pub.SafeHTML,
			"public_html":   pub.PublicHTML,
			"review_status": pub.ReviewStatus,
			"had_issues":    pub.HadIssues,
			"review_note":   pub.ReviewNote,
		},
	})
}

// UpdateMyHomeCustomize PUT /api/me/home-customize（需登录）
// 保存「我的主页装扮」原始 HTML（不消毒，仅本人可见）。
func UpdateMyHomeCustomize(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	var req struct {
		RawHTML string `json:"raw_html"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.SetUserHomeCustomize(uid, req.RawHTML); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"raw_html": req.RawHTML})
}

// GetAuthorHomeCustomize GET /api/author/:id/home-customize（公开）
// 返回该作者已通过审核的对外可见片段；无/未通过返回空串。
func GetAuthorHomeCustomize(c *gin.Context) {
	id, ok := parseAuthorID(c)
	if !ok {
		return
	}
	utils.Success(c, gin.H{"public_html": models.GetApprovedAuthorHomeCustomize(id)})
}

// UpdateAuthorHomeCustomize PUT /api/author/:id/home-customize（作者本人或管理员）
// 提交作者公开主页：服务端消毒 + 置 pending；若预检发现问题，通知站长复核（附原文）。
func UpdateAuthorHomeCustomize(c *gin.Context) {
	id, ok := parseAuthorID(c)
	if !ok {
		return
	}
	uid := currentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}
	if uid != id && !isAdminUser(c) {
		utils.Forbidden(c, "只能修改自己的作者主页")
		return
	}
	var req struct {
		RawHTML string `json:"raw_html"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	safe, removed, hadIssues, err := models.SubmitAuthorHomeCustomize(id, req.RawHTML)
	if err != nil {
		utils.InternalError(c, "提交失败")
		return
	}
	// 自动化预检查出问题 → 原文送站长复核（按 role 查询，不硬编码具体账号）
	if hadIssues {
		notifyAdminsHomeCustomize(id)
	}
	utils.Success(c, gin.H{
		"safe_html":     safe,
		"removed":       removed,
		"had_issues":    hadIssues,
		"review_status": "pending",
	})
}

// ListHomeCustomizeReviews GET /api/admin/home-customize/reviews（仅管理员）
// 列出全部作者公开主页提交（可按 review_status 过滤）；含原文 raw_html 供站长复核。
func ListHomeCustomizeReviews(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	status := c.Query("status")
	q := models.DB.Order("updated_at DESC")
	if status != "" {
		q = q.Where("review_status = ?", status)
	}
	var list []models.AuthorHomeCustomize
	if err := q.Find(&list).Error; err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	// 补作者名，便于站长一眼看出是谁提交的
	ids := make([]uint, 0, len(list))
	for _, it := range list {
		ids = append(ids, it.AuthorID)
	}
	names := map[uint]string{}
	if len(ids) > 0 {
		var users []models.User
		models.DB.Where("id IN ?", ids).Find(&users)
		for _, u := range users {
			names[u.ID] = u.Username
		}
	}
	type row struct {
		models.AuthorHomeCustomize
		Username string `json:"username"`
	}
	rows := make([]row, 0, len(list))
	for _, it := range list {
		rows = append(rows, row{AuthorHomeCustomize: it, Username: names[it.AuthorID]})
	}
	utils.Success(c, rows)
}

// ReviewHomeCustomize POST /api/admin/home-customize/reviews/:id/review（仅管理员）
// body: { action: "approve" | "reject", note? }
//   - approve：public_html = safe_html（消毒版）对外可见；
//   - reject：拒绝，public_html 置空。
func ReviewHomeCustomize(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := parseAuthorID(c)
	adminID := currentUserID(c)
	var req struct {
		Action string `json:"action"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	if req.Action != "approve" && req.Action != "reject" {
		utils.BadRequest(c, "action 只能是 approve 或 reject")
		return
	}
	note := security.SanitizePlainText(req.Note)
	var err error
	if req.Action == "approve" {
		err = models.ApproveAuthorHomeCustomize(id, adminID, note)
	} else {
		err = models.RejectAuthorHomeCustomize(id, adminID, note)
	}
	if err != nil {
		utils.InternalError(c, "操作失败")
		return
	}
	utils.Success(c, gin.H{"message": "已" + map[string]string{"approve": "通过", "reject": "驳回"}[req.Action], "review_status": req.Action + "d"})
}

// notifyAdminsHomeCustomize 自动化预检发现问题时，按 role 查询站长/管理员并推送站内通知（不硬编码账号）。
func notifyAdminsHomeCustomize(authorID uint) {
	title := "作者自定义主页待复核"
	body := fmt.Sprintf("作者 #%d 提交了自定义主页，自动化预检发现需清理的内容，已附原文供你复核。", authorID)
	var admins []models.User
	models.DB.Where("role = ?", "admin").Find(&admins)
	for _, a := range admins {
		_ = models.PushNotification(0, a.ID, "home_customize_review", title, body, "/admin/home-customize-review")
	}
}
