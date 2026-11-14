package sentiment

// 舆论场「阵营标记 + 语言格式」与图谱接口，以及超级管理员的删除申请审批。
//
// 立场前提：争议内容的对错取决于阵营，AI 无裁判资格（详见 sentiment/camp/mod.go）。
// 本文件只提供「标记」与「把标记画成图谱」的能力，任何接口都不产出对错结论。

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"nvs-server/handlers/admincommon"
	"nvs-server/handlers/hctx"
	"nvs-server/models"
	"nvs-server/models/ai"
	"nvs-server/sentiment/camp"
	"nvs-server/sentiment/store"
	"nvs-server/sentiment/topic"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

// 平台级默认开关：阵营分析默认开启；可被「按评论区」设置覆盖。
// 仅当某内容被显式关闭（sentiment_camp_target_settings.enabled=false）时才关闭。
const platformCampAnalysisEnabled = "sentiment_camp_analysis_enabled"

// campAnalysisEnabled 判断某评论区（内容）是否开启阵营分析：
// 优先看显式的按内容设置；无记录则回退平台默认（默认开启）。
func campAnalysisEnabled(ct string, cid uint) bool {
	var s store.SentimentCampTargetSetting
	if err := store.DB.Where("content_type = ? AND content_id = ?", ct, cid).First(&s).Error; err == nil {
		return s.Enabled
	}
	raw := models.GetPlatformConfig(platformCampAnalysisEnabled)
	if raw == "" {
		return true
	}
	b, _ := strconv.ParseBool(raw)
	return b
}

// disabledCampTargets 返回所有被显式关闭阵营分析的评论区键（"type:id"），
// 供「全局总览 / 按议题」聚合时把已关闭评论区的内容排除出图谱。
func disabledCampTargets() map[string]bool {
	out := map[string]bool{}
	if store.DB == nil {
		return out
	}
	var rows []store.SentimentCampTargetSetting
	if err := store.DB.Where("enabled = ?", false).Find(&rows).Error; err != nil {
		return out
	}
	for _, r := range rows {
		out[fmt.Sprintf("%s:%d", r.ContentType, r.ContentID)] = true
	}
	return out
}

// GET /api/admin/sentiment/chat-models
// 返回可用于阵营分析的对话模型（enabled + chat + 有 base_url），供后台「模型选择」下拉，
// 让选择可被记住（锁定），而不是每次都默认第一个。
func ListSentimentChatModels(c *gin.Context) {
	if !admincommon.EnsureElevated(c) {
		return
	}
	list, err := ai.ListAiModels(false)
	if err != nil {
		infra.InternalError(c, "读取模型失败")
		return
	}
	out := make([]gin.H, 0, len(list))
	for i := range list {
		if list[i].Enabled && list[i].Kind == ai.AiKindChat && strings.TrimSpace(list[i].BaseURL) != "" {
			out = append(out, gin.H{
				"id":       list[i].ID,
				"name":     list[i].Name,
				"model_id": list[i].ModelID,
				"thinking": list[i].Thinking,
			})
		}
	}
	infra.Success(c, gin.H{"models": out})
}

// GET /api/admin/sentiment/neutral-prompt
// 返回当前生效的中立性提示词，供管理员台查看/复制（明确禁止 AI 判对错、禁止倒戈多数派）。
func GetNeutralityPrompt(c *gin.Context) {
	if !admincommon.EnsureElevated(c) {
		return
	}
	infra.Success(c, gin.H{
		"prompt":      camp.NeutralityPrompt,
		"tail_prompt": camp.NeutralityTailPrompt,
		// 语言格式已是**自由文本字段**：AI 自行归纳，不再限定为固定三选一。
		// 下列仅作「常见示例」展示，不是可选项集合；前端手动标记也改自由输入。
		"lang_format_free": true,
		"lang_formats": []map[string]string{
			{"value": camp.LangFandom, "label": camp.LangFormatLabel(camp.LangFandom)},
			{"value": camp.LangPolitical, "label": camp.LangFormatLabel(camp.LangPolitical)},
			{"value": camp.LangEveryday, "label": camp.LangFormatLabel(camp.LangEveryday)},
			{"value": camp.LangMixed, "label": camp.LangFormatLabel(camp.LangMixed)},
		},
		"notice": "对错基于阵营，AI 只能标记阵营与语言格式，不能判断谁对谁错；语言格式由 AI 自由归纳，不得因人数多/文字多倒戈某一方。",
	})
}

// GET /api/admin/sentiment/camp-target-setting?content_type=&content_id=
// 读取某评论区的阵营分析开关状态（含平台默认值与是否显式设置）。
func GetCampTargetSetting(c *gin.Context) {
	if !admincommon.EnsureElevated(c) {
		return
	}
	ct := c.Query("content_type")
	cid := parseUint(c.Query("content_id"))
	if ct == "" || cid == 0 {
		infra.BadRequest(c, "请指定 content_type 与 content_id")
		return
	}
	var s store.SentimentCampTargetSetting
	explicit := store.DB.Where("content_type = ? AND content_id = ?", ct, cid).First(&s).Error == nil
	def := true
	if raw := models.GetPlatformConfig(platformCampAnalysisEnabled); raw != "" {
		if b, e := strconv.ParseBool(raw); e == nil {
			def = b
		}
	}
	infra.Success(c, gin.H{
		"content_type":     ct,
		"content_id":       cid,
		"enabled":          campAnalysisEnabled(ct, cid),
		"explicit":         explicit,
		"platform_default": def,
	})
}

// POST /api/admin/sentiment/camp-target-setting
// 超级管理员按评论区开/关阵营分析（content_type + content_id + enabled）。
func SetCampTargetSetting(c *gin.Context) {
	if !admincommon.EnsureSuperAdmin(c) {
		return
	}
	var in struct {
		ContentType string `json:"content_type"`
		ContentID   uint   `json:"content_id"`
		Enabled     *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.Enabled == nil {
		infra.BadRequest(c, "参数错误：需 content_type、content_id、enabled")
		return
	}
	if in.ContentType == "" || in.ContentID == 0 {
		infra.BadRequest(c, "请指定 content_type 与 content_id")
		return
	}
	uid, name := admincommon.DescribeActor(c)
	var s store.SentimentCampTargetSetting
	err := store.DB.Where("content_type = ? AND content_id = ?", in.ContentType, in.ContentID).First(&s).Error
	s.Enabled = *in.Enabled
	s.OperatorID = uid
	s.Operator = name
	var dbErr error
	if err != nil {
		s.ContentType = in.ContentType
		s.ContentID = in.ContentID
		dbErr = store.DB.Create(&s).Error
	} else {
		dbErr = store.DB.Model(&store.SentimentCampTargetSetting{}).Where("id = ?", s.ID).
			Updates(map[string]interface{}{"enabled": s.Enabled, "operator_id": s.OperatorID, "operator": s.Operator}).Error
	}
	if dbErr != nil {
		infra.InternalError(c, "保存失败")
		return
	}
	msg := "已关闭该评论区阵营分析"
	if *in.Enabled {
		msg = "已开启该评论区阵营分析"
	}
	infra.SuccessMessage(c, msg, gin.H{"enabled": s.Enabled})
}

// GET /api/admin/sentiment/camp-analysis-global
// 读取平台级阵营分析默认开关。
func GetCampGlobalSetting(c *gin.Context) {
	if !admincommon.EnsureElevated(c) {
		return
	}
	def := true
	if raw := models.GetPlatformConfig(platformCampAnalysisEnabled); raw != "" {
		if b, e := strconv.ParseBool(raw); e == nil {
			def = b
		}
	}
	infra.Success(c, gin.H{"enabled": def})
}

// PUT /api/admin/sentiment/camp-analysis-global
// 超级管理员设置平台级阵营分析默认开关（新内容默认继承；已有显式设置的内容不受影响）。
func SetCampGlobalSetting(c *gin.Context) {
	if !admincommon.EnsureSuperAdmin(c) {
		return
	}
	var in struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.Enabled == nil {
		infra.BadRequest(c, "请指定 enabled")
		return
	}
	if err := models.SetPlatformConfig(platformCampAnalysisEnabled, strconv.FormatBool(*in.Enabled)); err != nil {
		infra.InternalError(c, "保存失败")
		return
	}
	msg := "已关闭阵营分析（平台默认）"
	if *in.Enabled {
		msg = "已开启阵营分析（平台默认）"
	}
	infra.SuccessMessage(c, msg, gin.H{"enabled": *in.Enabled})
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
	// 「审核后自动生成图谱」：普通管理员新增的标记先进 pending，由站长审核后才进图谱；
	// 超级管理员（站长）自己新增视为本人审核，直接 approved。
	status := store.CampMarkApproved
	if !admincommon.IsSuperAdmin(c) {
		status = store.CampMarkPending
	}
	m := store.CampMark{
		ContentType: in.ContentType, ContentID: in.ContentID, CommentID: in.CommentID,
		UserID: hctx.CurrentUserID(c),
		CampID: in.CampID, CampLabel: in.CampLabel, Position: in.Position,
		LangFormat: camp.NormalizeLangFormat(in.LangFormat), LangEvidence: in.LangEvidence,
		Confidence: in.Confidence, Note: in.Note, Source: "manual", Status: status,
	}
	if err := store.DB.Create(&m).Error; err != nil {
		infra.InternalError(c, "保存标记失败")
		return
	}
	if status == store.CampMarkPending {
		uid, name := admincommon.DescribeActor(c)
		admincommon.NotifySuperAdmin(uid, "sentiment_camp_pending",
			"管理员提交阵营标记待审核",
			fmt.Sprintf("管理员 %s 新增阵营标记「%s」，待站长审核后才会进入图谱。", name, campMarkLabel(m)),
			"/admin/sentiment")
		infra.SuccessMessage(c, "已提交阵营标记，待超级管理员（站长）审核后进入图谱", m)
		return
	}
	infra.SuccessMessage(c, "已标记（站长，直接生效）", m)
}

// GET /api/admin/sentiment/camp-marks?scope=all|content|topic&content_type=&content_id=&topic=
// 阵营标记原始列表（与图谱共用 scope 维度，便于「按评论区/议题」查看时列表与图一致）。
func ListCampMarks(c *gin.Context) {
	if !admincommon.EnsureElevated(c) {
		return
	}
	scope := c.DefaultQuery("scope", "all")
	ct := c.Query("content_type")
	cid := parseUint(c.Query("content_id"))
	tag := strings.TrimSpace(c.Query("topic"))
	statusFilter := strings.TrimSpace(c.Query("status"))

	var list []store.CampMark
	q := store.DB.Model(&store.CampMark{}).Order("id DESC").Limit(500)
	if statusFilter != "" {
		q = q.Where("status = ?", statusFilter)
	}
	if scope == "content" {
		if ct != "" {
			q = q.Where("content_type = ?", ct)
		}
		if cid != 0 {
			q = q.Where("content_id = ?", cid)
		}
	}
	if err := q.Find(&list).Error; err != nil {
		infra.InternalError(c, "读取标记失败")
		return
	}
	if scope == "topic" && tag != "" {
		keys := topicContentKeys(tag)
		filtered := list[:0]
		for _, m := range list {
			if keys[fmt.Sprintf("%s:%d", m.ContentType, m.ContentID)] {
				filtered = append(filtered, m)
			}
		}
		list = filtered
	}
	infra.Success(c, list)
}

// GET /api/admin/sentiment/topics/list
// 返回去重后的议题标签 + 每标签命中的内容数，供阵营图谱「按议题」维度下拉选择。
func ListTopicTags(c *gin.Context) {
	if !admincommon.EnsureElevated(c) {
		return
	}
	var topics []store.ContentTopic
	if err := store.DB.Model(&store.ContentTopic{}).Find(&topics).Error; err != nil {
		infra.InternalError(c, "读取议题失败")
		return
	}
	counts := map[string]int{}
	for _, t := range topics {
		for _, x := range topic.ParseTagsJSON(t.Tags) {
			if x != "" {
				counts[x]++
			}
		}
	}
	type tagCount struct {
		Tag   string `json:"tag"`
		Count int    `json:"count"`
	}
	items := make([]tagCount, 0, len(counts))
	for tag, n := range counts {
		items = append(items, tagCount{Tag: tag, Count: n})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Count > items[j].Count })
	infra.Success(c, gin.H{"items": items})
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
	// 评论区已关闭阵营分析 → 不允许对其运行分析（与分析/落库都无关，先挡在门口）
	if in.ContentID != 0 && !campAnalysisEnabled(ct, in.ContentID) {
		infra.BadRequest(c, "该评论区已关闭阵营分析，无法对其运行 AI 分析")
		return
	}
	// 与手动标记同一审核规则：超管自审直接 approved；普通管理员先 pending 待审。
	status := store.CampMarkApproved
	if !admincommon.IsSuperAdmin(c) {
		status = store.CampMarkPending
	}
	if doSave {
		marks, thinking, reasoning, err := camp.AnalyzeAndPersist(c.Request.Context(), in.Text, in.ModelID, ct, in.ContentID, in.CommentID, status)
		if err != nil {
			infra.BadRequest(c, err.Error())
			return
		}
		notice := "AI 只标记阵营与语言格式，不判断谁对谁错；阵营对抗属结构性不可裁决，不会对任何立场做处置。"
		msg := "AI 已完成阵营标记"
		if status == store.CampMarkPending {
			uid, name := admincommon.DescribeActor(c)
			admincommon.NotifySuperAdmin(uid, "sentiment_camp_pending",
				"AI 阵营标记待审核",
				fmt.Sprintf("管理员 %s 通过 AI 新增 %d 条阵营标记，待站长审核后进入图谱。", name, len(marks)),
				"/admin/sentiment")
			notice += "（当前为管理员，标记待站长审核后才进入图谱）"
			msg = "AI 已完成阵营标记，待站长审核后进入图谱"
		}
		infra.SuccessMessage(c, msg, gin.H{
			"marks": marks, "source": "ai", "status": status,
			"ai_thinking":  thinking,
			"ai_reasoning": reasoning,
			"notice":       notice,
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
		"ai_thinking":  a.Thinking,
		"ai_reasoning": a.Reasoning,
		"notice":       "仅预览，未写入标记表。",
	})
}

// GET /api/admin/sentiment/camp-graph?scope=all|content|topic&content_type=&content_id=&topic=
// 生成管理员台图谱数据：节点=阵营（大小=标记数，颜色=主导语言格式），
// 边=同一内容下共同出现的阵营（权重=共现次数）。
//
// scope 维度（用户要求「两者都要」）：
//   - all（默认）：全平台总览，不加任何过滤；
//   - content：按单条内容（评论区）划分，需 content_type + content_id；
//   - topic：按议题标签跨内容聚合——阵营标记本身不挂议题，需从 ContentTopic 反查
//     带该标签的内容集合，再筛出这些内容的阵营标记。
//
// ⚠️ 节点大小/边权重只是**规模**，不是「谁更有理」；前端必须显示这条免责说明。
func GetCampGraph(c *gin.Context) {
	if !admincommon.EnsureElevated(c) {
		return
	}
	scope := c.DefaultQuery("scope", "all")
	var marks []store.CampMark
	scopeLabel := "全局总览"
	// 已关闭阵营分析的评论区集合（平台默认开启，仅显式关闭的才在表里有记录）
	block := disabledCampTargets()
	switch scope {
	case "content":
		ct := c.Query("content_type")
		cid := parseUint(c.Query("content_id"))
		if ct == "" || cid == 0 {
			infra.BadRequest(c, "按评论区查看需提供 content_type 与 content_id")
			return
		}
		scopeLabel = fmt.Sprintf("评论区：%s #%d", ct, cid)
		// 该评论区已关闭阵营分析 → 图谱直接返回空，不展示其阵营标记
		if !campAnalysisEnabled(ct, cid) {
			infra.Success(c, gin.H{
				"nodes":   []interface{}{},
				"edges":   []interface{}{},
				"scope":   scopeLabel,
				"enabled": false,
				"notice":  "该评论区已关闭阵营分析（由超级管理员设置），图谱不展示其阵营标记。仅超级管理员可在上方开关重新开启。",
			})
			return
		}
		// 仅聚合已审核通过的标记（审核后自动生成图谱）
		if err := store.DB.Model(&store.CampMark{}).
			Where("content_type = ? AND content_id = ?", ct, cid).
			Where("status = ?", store.CampMarkApproved).Order("id ASC").Find(&marks).Error; err != nil {
			infra.InternalError(c, "读取标记失败")
			return
		}
	case "topic":
		tag := strings.TrimSpace(c.Query("topic"))
		if tag == "" {
			infra.BadRequest(c, "按议题查看需提供 topic 标签")
			return
		}
		keys := topicContentKeys(tag)
		var all []store.CampMark
		if err := store.DB.Model(&store.CampMark{}).Order("id ASC").Find(&all).Error; err != nil {
			infra.InternalError(c, "读取标记失败")
			return
		}
		for _, m := range all {
			// 仅聚合已审核通过的标记，并排除已关闭阵营分析的评论区
			if m.Status == store.CampMarkApproved && keys[fmt.Sprintf("%s:%d", m.ContentType, m.ContentID)] &&
				!block[fmt.Sprintf("%s:%d", m.ContentType, m.ContentID)] {
				marks = append(marks, m)
			}
		}
		scopeLabel = fmt.Sprintf("议题：%s（%d 个相关内容的阵营标记）", tag, len(keys))
	default: // all
		// 仅聚合已审核通过的标记（审核后自动生成图谱）
		if err := store.DB.Model(&store.CampMark{}).
			Where("status = ?", store.CampMarkApproved).Order("id ASC").Find(&marks).Error; err != nil {
			infra.InternalError(c, "读取标记失败")
			return
		}
		// 排除已关闭阵营分析的评论区（保持全局总览与按评论区开关一致）
		if len(block) > 0 {
			filtered := marks[:0]
			for _, m := range marks {
				if !block[fmt.Sprintf("%s:%d", m.ContentType, m.ContentID)] {
					filtered = append(filtered, m)
				}
			}
			marks = filtered
		}
	}
	respondCampGraph(c, marks, scopeLabel)
}

// topicContentKeys 返回带某议题标签的内容集合键（"type:id"）。
// 阵营标记本身不挂议题，按议题聚合时必须经 content 桥接：先查 ContentTopic 命中标签的内容，再回筛阵营标记。
func topicContentKeys(tag string) map[string]bool {
	keys := map[string]bool{}
	if tag == "" {
		return keys
	}
	var topics []store.ContentTopic
	if err := store.DB.Model(&store.ContentTopic{}).Find(&topics).Error; err != nil {
		return keys
	}
	for _, t := range topics {
		for _, x := range topic.ParseTagsJSON(t.Tags) {
			if x == tag {
				keys[fmt.Sprintf("%s:%d", t.ContentType, t.ContentID)] = true
				break
			}
		}
	}
	return keys
}

// respondCampGraph 把一批阵营标记聚合成图谱（节点=阵营，边=同内容共现），与 scope 无关。
func respondCampGraph(c *gin.Context, marks []store.CampMark, scopeLabel string) {
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
		"nodes": outNodes, "edges": outEdges, "scope": scopeLabel,
		"notice": "图谱只呈现【已审核通过】的阵营与语言格式的分布和共现（待审核的标记不进图谱）；节点大小与边权重仅代表规模，不代表谁对谁错。当前范围：" + scopeLabel,
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

// ─────────────────────────────────────────────────────────────
// 阵营标记的「新增」审核（与改/删同一审批人：唯一超级管理员 / 站长）。
// 普通管理员新增的标记先进 pending，由站长审核通过后才进入图谱（审核后自动生成图谱）。
// ─────────────────────────────────────────────────────────────

// POST /api/admin/sentiment/camp-marks/:id/approve
// 仅超级管理员（站长）可审核通过：pending → approved，标记随即进入图谱。
func ApproveCampMark(c *gin.Context) {
	if !admincommon.EnsureSuperAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var m store.CampMark
	if err := store.DB.First(&m, id).Error; err != nil {
		infra.NotFound(c, "标记不存在")
		return
	}
	if m.Status == store.CampMarkApproved {
		infra.SuccessMessage(c, "已是审核通过", gin.H{"status": m.Status})
		return
	}
	if err := store.DB.Model(&store.CampMark{}).Where("id = ?", id).Update("status", store.CampMarkApproved).Error; err != nil {
		infra.InternalError(c, "审核失败")
		return
	}
	// 通知提交人（仅当提交人不是审核人自己时）
	if m.UserID != 0 && m.UserID != hctx.CurrentUserID(c) {
		_ = models.PushNotification(hctx.CurrentUserID(c), m.UserID, "sentiment_camp_approved",
			"阵营标记已审核通过", fmt.Sprintf("你提交的阵营标记「%s」已由超级管理员（站长）审核通过，已进入图谱。", campMarkLabel(m)),
			"/admin/sentiment")
	}
	infra.SuccessMessage(c, "已审核通过，图谱已更新", gin.H{"status": store.CampMarkApproved})
}

// POST /api/admin/sentiment/camp-marks/:id/reject
// 仅超级管理员（站长）可驳回：pending → rejected，标记永不进入图谱。
func RejectCampMark(c *gin.Context) {
	if !admincommon.EnsureSuperAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var m store.CampMark
	if err := store.DB.First(&m, id).Error; err != nil {
		infra.NotFound(c, "标记不存在")
		return
	}
	if err := store.DB.Model(&store.CampMark{}).Where("id = ?", id).Update("status", store.CampMarkRejected).Error; err != nil {
		infra.InternalError(c, "操作失败")
		return
	}
	if m.UserID != 0 && m.UserID != hctx.CurrentUserID(c) {
		var body struct {
			Reason string `json:"reason"`
		}
		_ = c.ShouldBindJSON(&body)
		_ = models.PushNotification(hctx.CurrentUserID(c), m.UserID, "sentiment_camp_rejected",
			"阵营标记未通过审核", fmt.Sprintf("你提交的阵营标记「%s」未通过审核。原因：%s", campMarkLabel(m), strings.TrimSpace(body.Reason)),
			"/admin/sentiment")
	}
	infra.SuccessMessage(c, "已驳回", gin.H{"status": store.CampMarkRejected})
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

// ── 自动扫描：让 AI 自己读平台内容生成阵营标记（站长无需复制粘贴）──
//
// 过去阵营图谱靠「管理员复制争议文本 → 粘贴 → AI 分析」，内容来源完全依赖人工搬运；
// 这里改为按目标（评论 / 论坛回帖 / 博客）自动拉取新内容，AI 批量标记，落库为待审，
// 站长只需在审核台通过即可进图谱。幂等：同一内容已有任意阵营标记则跳过。

type scanCandidate struct {
	ContentType string
	ContentID   uint
	CommentID   uint
	Text        string
}

// POST /api/admin/sentiment/camp-scan
// body: { targets:["comment","forum","blog"], limit:10, since_hours:24, model_id:0, dry_run:false }
func ScanCampMarks(c *gin.Context) {
	if !admincommon.EnsureElevated(c) {
		return
	}
	var in struct {
		Targets    []string `json:"targets"`
		Limit      int      `json:"limit"`
		SinceHours int      `json:"since_hours"`
		ModelID    uint     `json:"model_id"`
		DryRun     bool     `json:"dry_run"`
	}
	_ = c.ShouldBindJSON(&in)
	if len(in.Targets) == 0 {
		in.Targets = []string{"comment", "forum", "blog"}
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}
	sinceHours := in.SinceHours
	if sinceHours <= 0 {
		sinceHours = 24
	}
	since := time.Now().Add(-time.Duration(sinceHours) * time.Hour)

	stat := struct {
		Scanned int `json:"scanned"`
		Created int `json:"created"`
		Skipped int `json:"skipped"`
		Failed  int `json:"failed"`
		NoModel int `json:"no_model"`
	}{}

	for _, target := range in.Targets {
		cands, err := scanCandidates(target, since, limit)
		if err != nil {
			stat.Failed += limit
			continue
		}
		// 幂等：已有任意阵营标记的内容不再重复分析
		keys := map[string]bool{}
		if len(cands) > 0 {
			ids := make([]uint, 0, len(cands))
			for _, x := range cands {
				ids = append(ids, x.ContentID)
			}
			var existing []store.CampMark
			store.DB.Select("content_type, content_id").Where("content_type = ? AND content_id IN ?", target, ids).
				Find(&existing)
			for _, e := range existing {
				keys[fmt.Sprintf("%s:%d", e.ContentType, e.ContentID)] = true
			}
		}
		for _, cand := range cands {
			stat.Scanned++
			if keys[fmt.Sprintf("%s:%d", cand.ContentType, cand.ContentID)] {
				stat.Skipped++
				continue
			}
			text := strings.TrimSpace(cand.Text)
			if text == "" || len([]rune(text)) < 8 {
				stat.Skipped++ // 太短没有阵营判定价值
				continue
			}
			if in.DryRun {
				stat.Created++
				continue
			}
			status := store.CampMarkPending
			marks, _, _, aerr := camp.AnalyzeAndPersist(c.Request.Context(), text, in.ModelID,
				cand.ContentType, cand.ContentID, cand.CommentID, status)
			if aerr != nil {
				stat.Failed++
				continue
			}
			stat.Created += len(marks)
		}
	}

	// 记录扫描状态（供前台展示「上次扫描」）
	saveCampScanStatus(stat.Scanned, stat.Created, stat.Skipped, stat.Failed)

	msg := fmt.Sprintf("扫描完成：检查 %d 条，生成 %d 条标记（跳过 %d，失败 %d），均进入待站长审核",
		stat.Scanned, stat.Created, stat.Skipped, stat.Failed)
	if in.DryRun {
		msg = fmt.Sprintf("试扫完成（未落库）：检查 %d 条，预计生成 %d 条标记（跳过 %d）",
			stat.Scanned, stat.Created, stat.Skipped)
	}
	infra.SuccessMessage(c, msg, gin.H{
		"scanned": stat.Scanned, "created": stat.Created,
		"skipped": stat.Skipped, "failed": stat.Failed, "dry_run": in.DryRun,
	})
}

// GET /api/admin/sentiment/camp-scan/status
func CampScanStatus(c *gin.Context) {
	if !admincommon.EnsureElevated(c) {
		return
	}
	type statusRow struct {
		At      time.Time `json:"at"`
		Scanned int       `json:"scanned"`
		Created int       `json:"created"`
		Skipped int       `json:"skipped"`
		Failed  int       `json:"failed"`
	}
	var row statusRow
	var raw models.PlatformConfig
	if err := models.DB.Where("key = ?", "camp_scan_last").First(&raw).Error; err == nil {
		_ = json.Unmarshal([]byte(raw.Value), &row)
	}
	infra.Success(c, gin.H{"last": row})
}

func saveCampScanStatus(scanned, created, skipped, failed int) {
	raw, _ := json.Marshal(map[string]interface{}{
		"at": time.Now(), "scanned": scanned, "created": created,
		"skipped": skipped, "failed": failed,
	})
	_ = models.SetPlatformConfig("camp_scan_last", string(raw))
}

// scanCandidates 拉取某类内容里「自 since 起、未删除」的候选文本
func scanCandidates(target string, since time.Time, limit int) ([]scanCandidate, error) {
	switch target {
	case "comment":
		var list []models.Comment
		err := models.DB.Select("id, content").Where("created_at > ?", since).
			Order("id DESC").Limit(limit).Find(&list).Error
		if err != nil {
			return nil, err
		}
		out := make([]scanCandidate, 0, len(list))
		for _, x := range list {
			out = append(out, scanCandidate{ContentType: "comment", ContentID: x.ID, Text: x.Content})
		}
		return out, nil
	case "forum":
		var list []models.Post
		err := models.DB.Select("id, content").Where("created_at > ?", since).
			Order("id DESC").Limit(limit).Find(&list).Error
		if err != nil {
			return nil, err
		}
		out := make([]scanCandidate, 0, len(list))
		for _, x := range list {
			out = append(out, scanCandidate{ContentType: "forum", ContentID: x.ID, Text: x.Content})
		}
		return out, nil
	case "blog":
		type blogRow struct {
			ID      uint
			Summary string
			Title   string
		}
		var list []blogRow
		err := models.DB.Table("author_blogs").Select("id, summary, title").
			Where("created_at > ?", since).Order("id DESC").Limit(limit).Find(&list).Error
		if err != nil {
			return nil, err
		}
		out := make([]scanCandidate, 0, len(list))
		for _, x := range list {
			t := x.Summary
			if t == "" {
				t = x.Title
			}
			out = append(out, scanCandidate{ContentType: "blog", ContentID: x.ID, Text: t})
		}
		return out, nil
	}
	return nil, fmt.Errorf("未知目标：%s", target)
}
