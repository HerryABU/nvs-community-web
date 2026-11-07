package sentiment

// 舆论场「阵营标记 + 语言格式」与图谱接口，以及超级管理员的删除申请审批。
//
// 立场前提：争议内容的对错取决于阵营，AI 无裁判资格（详见 sentiment/camp/mod.go）。
// 本文件只提供「标记」与「把标记画成图谱」的能力，任何接口都不产出对错结论。

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nvs-server/handlers/admincommon"
	"nvs-server/handlers/hctx"
	"nvs-server/models"
	"nvs-server/sentiment/camp"
	"nvs-server/sentiment/store"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

// GET /api/admin/sentiment/neutral-prompt
// 返回当前生效的中立性提示词，供管理员台查看/复制（明确禁止 AI 判对错、禁止倒戈多数派）。
func GetNeutralityPrompt(c *gin.Context) {
	if !admincommon.EnsureElevated(c) {
		return
	}
	infra.Success(c, gin.H{
		"prompt":      camp.NeutralityPrompt,
		"tail_prompt": camp.NeutralityTailPrompt,
		"lang_formats": []map[string]string{
			{"value": camp.LangFandom, "label": camp.LangFormatLabel(camp.LangFandom)},
			{"value": camp.LangPolitical, "label": camp.LangFormatLabel(camp.LangPolitical)},
			{"value": camp.LangEveryday, "label": camp.LangFormatLabel(camp.LangEveryday)},
			{"value": camp.LangMixed, "label": camp.LangFormatLabel(camp.LangMixed)},
			{"value": camp.LangUnknown, "label": camp.LangFormatLabel(camp.LangUnknown)},
		},
		"notice": "对错基于阵营，AI 只能标记阵营与语言格式，不能判断谁对谁错；不得因人数多/文字多倒戈某一方。",
	})
}

type campMarkInput struct {
	ContentType  string `json:"content_type"`
	ContentID    uint   `json:"content_id"`
	CommentID    uint   `json:"comment_id"`
	CampID       string `json:"camp_id"`
	CampLabel    string `json:"camp_label"`
	Position     string `json:"position"`
	LangFormat   string `json:"lang_format"`
	LangEvidence string `json:"lang_evidence"`
	Confidence   int    `json:"confidence"`
	Note         string `json:"note"`
}

// POST /api/admin/sentiment/camp-marks
// 写入一条阵营/语言格式标记（超级管理员与管理员都可新增 —— 管理员本来就能新增规则）。
func CreateCampMark(c *gin.Context) {
	if !admincommon.EnsureElevated(c) {
		return
	}
	var in campMarkInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if in.CampID == "" {
		infra.BadRequest(c, "请填写阵营标识")
		return
	}
	if in.Confidence < 0 {
		in.Confidence = 0
	}
	if in.Confidence > 100 {
		in.Confidence = 100
	}
	m := store.CampMark{
		ContentType: in.ContentType, ContentID: in.ContentID, CommentID: in.CommentID,
		UserID: hctx.CurrentUserID(c),
		CampID: in.CampID, CampLabel: in.CampLabel, Position: in.Position,
		LangFormat: camp.NormalizeLangFormat(in.LangFormat), LangEvidence: in.LangEvidence,
		Confidence: in.Confidence, Note: in.Note, Source: "manual",
	}
	if err := store.DB.Create(&m).Error; err != nil {
		infra.InternalError(c, "保存标记失败")
		return
	}
	infra.SuccessMessage(c, "已标记", m)
}

// GET /api/admin/sentiment/camp-marks
func ListCampMarks(c *gin.Context) {
	if !admincommon.EnsureElevated(c) {
		return
	}
	var list []store.CampMark
	q := store.DB.Model(&store.CampMark{}).Order("id DESC").Limit(500)
	if ct := c.Query("content_type"); ct != "" {
		q = q.Where("content_type = ?", ct)
	}
	if err := q.Find(&list).Error; err != nil {
		infra.InternalError(c, "读取标记失败")
		return
	}
	infra.Success(c, list)
}

// POST /api/admin/sentiment/camp-analyze
// **真正调用 AI**：拿强化后的中立性提示词分析一段争议文本，产出阵营 + 语言格式标记并落库。
// 这是「AI 只能标记阵营、无法判断对错」的运行时落点；此前提示词只被展示、从未被调用。
func AnalyzeCamp(c *gin.Context) {
	if !admincommon.EnsureElevated(c) {
		return
	}
	var in struct {
		Text        string `json:"text"`
		ModelID     uint   `json:"model_id"`
		ContentType string `json:"content_type"`
		ContentID   uint   `json:"content_id"`
		CommentID   uint   `json:"comment_id"`
		Save        *bool  `json:"save"` // 默认 true；false 只预览不落库
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if strings.TrimSpace(in.Text) == "" {
		infra.BadRequest(c, "请填写待分析的争议文本")
		return
	}
	doSave := in.Save == nil || *in.Save
	ct := in.ContentType
	if ct == "" {
		ct = "forum"
	}
	if doSave {
		marks, err := camp.AnalyzeAndPersist(c.Request.Context(), in.Text, in.ModelID, ct, in.ContentID, in.CommentID)
		if err != nil {
			infra.BadRequest(c, err.Error())
			return
		}
		infra.SuccessMessage(c, "AI 已完成阵营标记", gin.H{
			"marks": marks, "source": "ai",
			"notice": "AI 只标记阵营与语言格式，不判断谁对谁错；阵营对抗属结构性不可裁决，不会对任何立场做处置。",
		})
		return
	}
	a, err := camp.Analyze(c.Request.Context(), in.Text, in.ModelID)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "AI 分析完成（未落库）", gin.H{
		"marks": a.Marks, "confrontation": a.Confrontation, "source": "ai",
		"notice": "仅预览，未写入标记表。",
	})
}

// GET /api/admin/sentiment/camp-graph
// 生成管理员台图谱数据：节点=阵营（大小=标记数，颜色=主导语言格式），
// 边=同一内容下共同出现的阵营（权重=共现次数）。
//
// ⚠️ 节点大小/边权重只是**规模**，不是「谁更有理」；前端必须显示这条免责说明。
func GetCampGraph(c *gin.Context) {
	if !admincommon.EnsureElevated(c) {
		return
	}
	var marks []store.CampMark
	if err := store.DB.Order("id ASC").Find(&marks).Error; err != nil {
		infra.InternalError(c, "读取标记失败")
		return
	}
	type node struct {
		ID         string         `json:"id"`
		Label      string         `json:"label"`
		Size       int            `json:"size"`
		LangFormat string         `json:"lang_format"`
		LangLabel  string         `json:"lang_label"`
		LangMix    map[string]int `json:"lang_mix"`
		Confidence int            `json:"confidence"`
	}
	type edge struct {
		Source string `json:"source"`
		Target string `json:"target"`
		Weight int    `json:"weight"`
	}
	nodes := map[string]*node{}
	edges := map[string]*edge{}
	// 同一内容下出现的阵营视为共现（不做立场推断）
	byContent := map[string][]string{}
	for _, m := range marks {
		n, ok := nodes[m.CampID]
		if !ok {
			n = &node{ID: m.CampID, Label: m.CampLabel, LangMix: map[string]int{}}
			if n.Label == "" {
				n.Label = m.CampID
			}
			nodes[m.CampID] = n
		}
		n.Size++
		n.LangMix[m.LangFormat]++
		n.Confidence += m.Confidence
		key := fmt.Sprintf("%s:%d:%d", m.ContentType, m.ContentID, m.CommentID)
		byContent[key] = append(byContent[key], m.CampID)
	}
	for id, n := range nodes {
		if n.Size > 0 {
			n.Confidence /= n.Size
		}
		// 主导语言格式只取出现次数最多者，不下任何价值判断
		best, bestN := camp.LangUnknown, 0
		for k, v := range n.LangMix {
			if v > bestN {
				best, bestN = k, v
			}
		}
		n.LangFormat = best
		n.LangLabel = camp.LangFormatLabel(best)
		_ = id
	}
	for _, camps := range byContent {
		for i := 0; i < len(camps); i++ {
			for j := i + 1; j < len(camps); j++ {
				if camps[i] == camps[j] {
					continue
				}
				a, b := camps[i], camps[j]
				if a > b {
					a, b = b, a
				}
				k := a + "|" + b
				if e, ok := edges[k]; ok {
					e.Weight++
				} else {
					edges[k] = &edge{Source: a, Target: b, Weight: 1}
				}
			}
		}
	}
	outNodes := make([]*node, 0, len(nodes))
	for _, n := range nodes {
		outNodes = append(outNodes, n)
	}
	outEdges := make([]*edge, 0, len(edges))
	for _, e := range edges {
		outEdges = append(outEdges, e)
	}
	infra.Success(c, gin.H{
		"nodes": outNodes, "edges": outEdges,
		"notice": "图谱只呈现阵营与语言格式的分布和共现；节点大小与边权重仅代表规模，不代表谁对谁错。",
	})
}

// ─────────────────────────────────────────────────────────────
// 阵营标记的「修改 / 删除」权限分层（与 term 同一范式）：
//   普通管理员：只能【新增】（CreateCampMark）。既不能直改也不能直删，
//               必须提交申请（action=update/delete），由超级管理员批准/驳回。
//   超级管理员：可直改（PUT）、直删（DELETE），也可批准/驳回普通管理员的申请。
// ─────────────────────────────────────────────────────────────

// campMarkEditable 阵营标记可编辑字段的白名单（防止更新时注入任意列）。
type campMarkEditable struct {
	CampID       string `json:"camp_id"`
	CampLabel    string `json:"camp_label"`
	Position     string `json:"position"`
	LangFormat   string `json:"lang_format"`
	LangEvidence string `json:"lang_evidence"`
	Confidence   int    `json:"confidence"`
	Note         string `json:"note"`
}

// campMarkUpdateInput 修改申请的入参（可编辑字段 + 申请理由，单次绑定）。
type campMarkUpdateInput struct {
	campMarkEditable
	Reason string `json:"reason"`
}

// campMarkSnapshot 把一条标记序列化为「修改申请」的 before/after 快照（只含可编辑字段）。
func campMarkSnapshot(m store.CampMark) string {
	b, _ := json.Marshal(campMarkEditable{
		CampID: m.CampID, CampLabel: m.CampLabel, Position: m.Position,
		LangFormat: m.LangFormat, LangEvidence: m.LangEvidence,
		Confidence: m.Confidence, Note: m.Note,
	})
	return string(b)
}

// campMarkLabel 标记的可读标签（用于申请列表展示）。
func campMarkLabel(m store.CampMark) string {
	if m.CampLabel != "" {
		return fmt.Sprintf("%s(%s)", m.CampLabel, m.CampID)
	}
	return m.CampID
}

// POST /api/admin/sentiment/camp-marks/:id/request-update
// 普通管理员提交「修改申请」：不直改，写一条 action=update 的申请并通知站长。
func RequestUpdateCampMark(c *gin.Context) {
	if !admincommon.EnsureElevated(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var m store.CampMark
	if err := store.DB.First(&m, id).Error; err != nil {
		infra.NotFound(c, "标记不存在")
		return
	}
	var in campMarkUpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if in.CampID == "" {
		infra.BadRequest(c, "阵营标识不能为空")
		return
	}
	if in.Confidence < 0 {
		in.Confidence = 0
	}
	if in.Confidence > 100 {
		in.Confidence = 100
	}
	uid, name := admincommon.DescribeActor(c)
	req := store.DeleteRequest{
		Action:      "update",
		TargetType:  "camp_mark",
		TargetID:    m.ID,
		TargetLabel: campMarkLabel(m),
		RequesterID: uid,
		Requester:   name,
		Reason:      strings.TrimSpace(in.Reason),
		Before:      campMarkSnapshot(m),
		After:       mustJSON(in.campMarkEditable),
		Status:      "pending",
	}
	if err := store.DB.Create(&req).Error; err != nil {
		infra.InternalError(c, "提交修改申请失败")
		return
	}
	admincommon.NotifySuperAdmin(uid, "sentiment_camp_change_request",
		"管理员提交阵营标记修改申请",
		fmt.Sprintf("管理员 %s 申请修改阵营标记「%s」（#%d），请前往管理员台审核。", name, campMarkLabel(m), m.ID),
		"/admin/sentiment")
	infra.Success(c, gin.H{"pending": true, "request_id": req.ID, "message": admincommon.DeleteRequestHint()})
}

// POST /api/admin/sentiment/camp-marks/:id/request-delete
// 普通管理员提交「删除申请」：不直删，写一条 action=delete 的申请并通知站长。
func RequestDeleteCampMark(c *gin.Context) {
	if !admincommon.EnsureElevated(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var m store.CampMark
	if err := store.DB.First(&m, id).Error; err != nil {
		infra.NotFound(c, "标记不存在")
		return
	}
	uid, name := admincommon.DescribeActor(c)
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	req := store.DeleteRequest{
		Action:      "delete",
		TargetType:  "camp_mark",
		TargetID:    m.ID,
		TargetLabel: campMarkLabel(m),
		RequesterID: uid,
		Requester:   name,
		Reason:      strings.TrimSpace(body.Reason),
		Before:      campMarkSnapshot(m),
		After:       "",
		Status:      "pending",
	}
	if err := store.DB.Create(&req).Error; err != nil {
		infra.InternalError(c, "提交删除申请失败")
		return
	}
	admincommon.NotifySuperAdmin(uid, "sentiment_camp_change_request",
		"管理员提交阵营标记删除申请",
		fmt.Sprintf("管理员 %s 申请删除阵营标记「%s」（#%d），请前往管理员台审核。", name, campMarkLabel(m), m.ID),
		"/admin/sentiment")
	infra.Success(c, gin.H{"pending": true, "request_id": req.ID, "message": admincommon.DeleteRequestHint()})
}

// PUT /api/admin/sentiment/camp-marks/:id
// 仅超级管理员（站长）可直接修改阵营标记。普通管理员请走「修改申请」。
func UpdateCampMark(c *gin.Context) {
	if !admincommon.EnsureSuperAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var m store.CampMark
	if err := store.DB.First(&m, id).Error; err != nil {
		infra.NotFound(c, "标记不存在")
		return
	}
	var in campMarkEditable
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if in.CampID == "" {
		infra.BadRequest(c, "阵营标识不能为空")
		return
	}
	if in.Confidence < 0 {
		in.Confidence = 0
	}
	if in.Confidence > 100 {
		in.Confidence = 100
	}
	patch := map[string]interface{}{
		"camp_id":       in.CampID,
		"camp_label":    in.CampLabel,
		"position":      in.Position,
		"lang_format":   camp.NormalizeLangFormat(in.LangFormat),
		"lang_evidence": in.LangEvidence,
		"confidence":    in.Confidence,
		"note":          in.Note,
	}
	if err := store.DB.Model(&store.CampMark{}).Where("id = ?", id).Updates(patch).Error; err != nil {
		infra.InternalError(c, "修改失败")
		return
	}
	infra.Success(c, gin.H{"message": "已修改（站长）"})
}

// DELETE /api/admin/sentiment/camp-marks/:id
// 仅超级管理员（站长）可直接删除阵营标记。普通管理员请走「删除申请」。
func DeleteCampMark(c *gin.Context) {
	if !admincommon.EnsureSuperAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var m store.CampMark
	if err := store.DB.First(&m, id).Error; err != nil {
		infra.NotFound(c, "标记不存在")
		return
	}
	if err := store.DB.Delete(&store.CampMark{}, id).Error; err != nil {
		infra.InternalError(c, "删除失败")
		return
	}
	infra.Success(c, gin.H{"message": "已删除（站长）"})
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// GET /api/admin/sentiment/delete-requests
func ListDeleteRequests(c *gin.Context) {
	if !admincommon.EnsureElevated(c) {
		return
	}
	var list []store.DeleteRequest
	if err := store.DB.Order("id DESC").Limit(200).Find(&list).Error; err != nil {
		infra.InternalError(c, "读取申请失败")
		return
	}
	infra.Success(c, gin.H{"list": list, "can_approve": admincommon.IsSuperAdmin(c)})
}

// POST /api/admin/sentiment/delete-requests/:id/approve
// 仅超级管理员（站长）可批准；批准后真正执行删除，并通知申请人。
func ApproveDeleteRequest(c *gin.Context) {
	if !admincommon.EnsureSuperAdmin(c) {
		return
	}
	var req store.DeleteRequest
	if err := store.DB.First(&req, c.Param("id")).Error; err != nil {
		infra.NotFound(c, "申请不存在")
		return
	}
	if req.Status != "pending" {
		infra.BadRequest(c, "申请已处理")
		return
	}
	if req.TargetType == "term" {
		if err := store.DB.Delete(&store.Term{}, req.TargetID).Error; err != nil {
			infra.InternalError(c, "删除失败")
			return
		}
	}
	if req.TargetType == "camp_mark" {
		if req.Action == "update" {
			// 批准修改：把申请里的 after 快照落到标记（只允许白名单字段，防止注入任意列）
			var patch campMarkEditable
			if err := json.Unmarshal([]byte(req.After), &patch); err != nil || patch.CampID == "" {
				infra.BadRequest(c, "修改快照无效")
				return
			}
			if patch.Confidence < 0 {
				patch.Confidence = 0
			}
			if patch.Confidence > 100 {
				patch.Confidence = 100
			}
			apply := map[string]interface{}{
				"camp_id":       patch.CampID,
				"camp_label":    patch.CampLabel,
				"position":      patch.Position,
				"lang_format":   camp.NormalizeLangFormat(patch.LangFormat),
				"lang_evidence": patch.LangEvidence,
				"confidence":    patch.Confidence,
				"note":          patch.Note,
			}
			if err := store.DB.Model(&store.CampMark{}).Where("id = ?", req.TargetID).Updates(apply).Error; err != nil {
				infra.InternalError(c, "应用修改失败")
				return
			}
		} else {
			if err := store.DB.Delete(&store.CampMark{}, req.TargetID).Error; err != nil {
				infra.InternalError(c, "删除失败")
				return
			}
		}
	}
	now := time.Now()
	req.Status = "approved"
	req.ReviewerID = hctx.CurrentUserID(c)
	req.ReviewedAt = &now
	_ = store.DB.Save(&req).Error
	verb := "删除"
	if req.Action == "update" {
		verb = "修改"
	}
	_ = models.PushNotification(hctx.CurrentUserID(c), req.RequesterID, "sentiment_change_approved",
		"修改/删除申请已批准", fmt.Sprintf("你申请%s的「%s」已由超级管理员（站长）批准并生效。", verb, req.TargetLabel),
		"/admin/sentiment")
	infra.Success(c, gin.H{"message": fmt.Sprintf("已批准并%s", verb)})
}

// POST /api/admin/sentiment/delete-requests/:id/reject
func RejectDeleteRequest(c *gin.Context) {
	if !admincommon.EnsureSuperAdmin(c) {
		return
	}
	var req store.DeleteRequest
	if err := store.DB.First(&req, c.Param("id")).Error; err != nil {
		infra.NotFound(c, "申请不存在")
		return
	}
	if req.Status != "pending" {
		infra.BadRequest(c, "申请已处理")
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	now := time.Now()
	req.Status = "rejected"
	req.ReviewerID = hctx.CurrentUserID(c)
	req.ReviewedAt = &now
	_ = store.DB.Save(&req).Error
	verb := "删除"
	if req.Action == "update" {
		verb = "修改"
	}
	_ = models.PushNotification(hctx.CurrentUserID(c), req.RequesterID, "sentiment_change_rejected",
		"修改/删除申请未批准", fmt.Sprintf("你申请%s的「%s」未获批准。原因：%s", verb, req.TargetLabel, body.Reason),
		"/admin/sentiment")
	infra.Success(c, gin.H{"message": "已驳回"})
}
