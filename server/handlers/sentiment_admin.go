package handlers

import (
	"strconv"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/sentiment/store"
	"nvs-server/sentiment/topic"
	"nvs-server/sentiment/updater"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// 舆情子系统的后台接口。
//
// 单独成文件（而不是塞进 admin_ai_audit.go）：那边是 AI 模型/厂商，
// 这边是社区舆论，两类配置的读者与变更频率都不一样。

// GetSentimentConfig GET /api/admin/sentiment/config
func GetSentimentConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	cfg, err := store.GetConfig()
	if err != nil {
		utils.InternalError(c, "读取配置失败")
		return
	}
	utils.Success(c, gin.H{"config": cfg})
}

// SaveSentimentConfig PUT /api/admin/sentiment/config
func SaveSentimentConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req store.Config
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	req.ID = 1
	// 阈值白名单归一化：非法值一律回落到安全默认，避免「填错一个字母导致全线放行」
	req.BlockRisk = topic.ParseRisk(req.BlockRisk).String()
	req.FlagRisk = topic.ParseRisk(req.FlagRisk).String()
	if req.FoldChars < 0 {
		req.FoldChars = 0
	}
	if req.AutoUpdateHour < 0 || req.AutoUpdateHour > 23 {
		req.AutoUpdateHour = 4
	}
	if err := req.Save(); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"message": "已保存"})
}

// ListSentimentTerms GET /api/admin/sentiment/terms?status=
func ListSentimentTerms(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	status := c.Query("status")
	q := store.DB.Model(&store.Term{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		q = q.Where("word LIKE ?", "%"+kw+"%")
	}
	var total int64
	q.Count(&total)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "30"))
	if size < 1 || size > 200 {
		size = 30
	}
	var list []store.Term
	q.Order("status asc, severity desc, hit_count desc, id desc").
		Offset((page - 1) * size).Limit(size).Find(&list)
	utils.Success(c, gin.H{"items": list, "total": total, "page": page})
}

// SentimentTermReq 词条新增/修改。
type SentimentTermReq struct {
	Word            string   `json:"word"`
	Cat             string   `json:"cat"`
	EscalateUnder   []string `json:"escalate_under"`
	Severity        int      `json:"severity"`
	ActionOverride  string   `json:"action_override"`
	Status          string   `json:"status"`
	ReviewCycleDays int      `json:"review_cycle_days"`
	Note            string   `json:"note"`
}

// CreateSentimentTerm POST /api/admin/sentiment/terms
func CreateSentimentTerm(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req SentimentTermReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Word) == "" {
		utils.BadRequest(c, "请填写词条")
		return
	}
	gs := strings.Join(req.EscalateUnder, ",")
	t := &store.Term{
		Word:            strings.TrimSpace(req.Word),
		Cat:             orDefault(req.Cat, "slang"),
		EscalateUnder:   gs,
		Severity:        clampInt(req.Severity, 1, 3),
		ActionOverride:  req.ActionOverride,
		Source:          store.SourceManual,
		Status:          orDefault(req.Status, store.StatusActive),
		ReviewCycleDays: clampInt(req.ReviewCycleDays, 0, 3650),
		Confidence:      100, // 人工添加视为最高置信
		Note:            req.Note,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	var exist store.Term
	if err := store.DB.Where("word = ?", t.Word).First(&exist).Error; err == nil {
		utils.BadRequest(c, "该词已存在")
		return
	}
	if err := store.DB.Create(t).Error; err != nil {
		utils.InternalError(c, "创建失败")
		return
	}
	store.DB.Model(&store.Term{}).Where("id = ?", t.ID).Updates(map[string]interface{}{
		"severity": t.Severity, "confidence": t.Confidence,
	})
	writeTermAudit(c, t.ID, t.Word, "create", "", t.Note)
	utils.Success(c, gin.H{"id": t.ID})
}

// UpdateSentimentTerm PUT /api/admin/sentiment/terms/:id
func UpdateSentimentTerm(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var t store.Term
	if err := store.DB.First(&t, id).Error; err != nil {
		utils.NotFound(c, "词条不存在")
		return
	}
	var req SentimentTermReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	before := t.Note
	updates := map[string]interface{}{
		"cat":               orDefault(req.Cat, t.Cat),
		"escalate_under":    strings.Join(req.EscalateUnder, ","),
		"severity":          clampInt(req.Severity, 1, 3),
		"action_override":   req.ActionOverride,
		"status":            orDefault(req.Status, t.Status),
		"review_cycle_days": clampInt(req.ReviewCycleDays, 0, 3650),
		"note":              req.Note,
		"reviewed_at":       time.Now(),
		"updated_at":        time.Now(),
	}
	if err := store.DB.Model(&store.Term{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	writeTermAudit(c, t.ID, t.Word, "update", before, req.Note)
	utils.Success(c, gin.H{"message": "已保存"})
}

// ReviewSentimentTerm POST /api/admin/sentiment/terms/:id/review
// body: {"action":"approve|reject"}
// AI 学来的词必须过人这一关：approve → active，reject → rejected（保留记录不删）。
func ReviewSentimentTerm(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Action string `json:"action"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	var t store.Term
	if err := store.DB.First(&t, id).Error; err != nil {
		utils.NotFound(c, "词条不存在")
		return
	}
	status := store.StatusActive
	if req.Action == "reject" {
		status = store.StatusRejected
	}
	if err := store.DB.Model(&store.Term{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": status, "reviewed_at": time.Now(), "updated_at": time.Now(),
	}).Error; err != nil {
		utils.InternalError(c, "操作失败")
		return
	}
	writeTermAudit(c, t.ID, t.Word, req.Action, "", req.Note)
	utils.Success(c, gin.H{"message": "已处理"})
}

// DeleteSentimentTerm DELETE /api/admin/sentiment/terms/:id
func DeleteSentimentTerm(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var t store.Term
	if err := store.DB.First(&t, id).Error; err != nil {
		utils.NotFound(c, "词条不存在")
		return
	}
	if err := store.DB.Delete(&store.Term{}, id).Error; err != nil {
		utils.InternalError(c, "删除失败")
		return
	}
	writeTermAudit(c, t.ID, t.Word, "delete", t.Note, "")
	utils.Success(c, gin.H{"message": "已删除"})
}

// ListSentimentHits GET /api/admin/sentiment/hits
// 只返回判定摘要，不返回评论正文——正文在评论表里，权限与脱敏走评论自己的口径。
func ListSentimentHits(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	q := store.DB.Model(&store.Hit{})
	if d := c.Query("decision"); d != "" {
		q = q.Where("decision = ?", d)
	}
	if r := c.Query("risk"); r != "" {
		q = q.Where("risk = ?", r)
	}
	if g := c.Query("group"); g != "" {
		q = q.Where("groups LIKE ?", "%"+g+"%")
	}
	var total int64
	q.Count(&total)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "30"))
	if size < 1 || size > 200 {
		size = 30
	}
	var list []store.Hit
	q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list)
	utils.Success(c, gin.H{"items": list, "total": total, "page": page})
}

// GetContentTopic GET /api/admin/sentiment/topics?content_type=&content_id=
// 读某内容的话题标注（供人工修正入口预填）。
func GetContentTopic(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	ct, err := store.TopicOf(c.Query("content_type"), parseUint(c.Query("content_id")))
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"topic": ct})
}

// SetContentTopic PUT /api/admin/sentiment/topics
//
// 人工修正话题标注：这是「标签错了怎么办」的兜底入口。
// 人工改过的标注 source 记为 manual、confidence 置 100、复核状态置 confirmed，
// 后续不会再被 AI 自动覆盖（AI 写标注时应跳过 manual）。
func SetContentTopic(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		ContentType string   `json:"content_type"`
		ContentID   uint     `json:"content_id"`
		Tags        []string `json:"tags"`
		Policy      string   `json:"policy"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ContentID == 0 || req.ContentType == "" {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.Policy != "" && req.Policy != store.PolicyBlock &&
		req.Policy != store.PolicyFlag && req.Policy != store.PolicyOff {
		utils.BadRequest(c, "策略取值非法")
		return
	}
	a := topic.Assess(req.Tags, "")
	if err := store.ApplyAssessment(req.ContentType, req.ContentID, a, "manual"); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	if err := store.DB.Model(&store.ContentTopic{}).
		Where("content_type = ? AND content_id = ?", req.ContentType, req.ContentID).
		Updates(map[string]interface{}{
			"confidence":    100,
			"review_status": store.ReviewConfirmed,
			"policy":        req.Policy,
			"reviewed_by":   currentUserID(c),
			"reviewed_at":   time.Now(),
		}).Error; err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"message": "已更新话题标注", "groups": a.Groups, "risk": a.Risk.String()})
}

// RunSentimentUpdate POST /api/admin/sentiment/run-update
// 手动触发一次词表更新（站长不想等到凌晨时用）。
func RunSentimentUpdate(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	res, err := updater.RunOnce("manual")
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"result": res})
}

// ListTermAudits GET /api/admin/sentiment/audits?term_id=
func ListTermAudits(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	q := store.DB.Model(&store.TermAudit{})
	if tid := parseUint(c.Query("term_id")); tid > 0 {
		q = q.Where("term_id = ?", tid)
	}
	var list []store.TermAudit
	q.Order("id desc").Limit(100).Find(&list)
	utils.Success(c, gin.H{"items": list})
}

// writeTermAudit 统一写审计（失败只记日志级，不影响主操作返回）。
func writeTermAudit(c *gin.Context, termID uint, word, action, before, after string) {
	uid := currentUserID(c)
	name := ""
	if u, err := models.GetUserByID(uid); err == nil && u != nil {
		name = u.Username
	}
	_ = store.DB.Create(&store.TermAudit{
		TermID: termID, Word: word, Action: action,
		OperatorID: uid, Operator: name,
		Before: truncateRunes(before, 400), After: truncateRunes(after, 400),
		CreatedAt: time.Now(),
	}).Error
}

// ── 小工具 ──

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func parseUint(s string) uint {
	v, _ := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	return uint(v)
}
