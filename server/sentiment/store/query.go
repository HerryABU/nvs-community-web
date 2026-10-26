package store

import (
	"encoding/json"
	"strings"
	"time"

	"nvs-server/sentiment/lexicon"
	"nvs-server/sentiment/topic"

	"gorm.io/gorm"
)

// DB 由 main 在启动时注入（与 aisafety / moderation 保持同样的显式注入风格，
// 避免包内直接依赖全局 models.DB 造成测试无法替换）。
var DB *gorm.DB

// SetDB 注入数据库句柄。
func SetDB(db *gorm.DB) { DB = db }

// TopicOf 取某内容的话题标注；不存在返回 (nil, nil)。
func TopicOf(contentType string, contentID uint) (*ContentTopic, error) {
	if DB == nil || contentID == 0 {
		return nil, nil
	}
	var ct ContentTopic
	err := DB.Where("content_type = ? AND content_id = ?", contentType, contentID).First(&ct).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ct, nil
}

// UpsertTopic 写入/更新话题标注（AI 审核或人工修订都走这里）。
func UpsertTopic(ct *ContentTopic) error {
	if DB == nil || ct == nil || ct.ContentID == 0 {
		return nil
	}
	ct.UpdatedAt = time.Now()
	var exist ContentTopic
	err := DB.Where("content_type = ? AND content_id = ?", ct.ContentType, ct.ContentID).First(&exist).Error
	if err == gorm.ErrRecordNotFound {
		return DB.Create(ct).Error
	}
	if err != nil {
		return err
	}
	// 用 map 更新：risk/score 可能是零值（low / 0），struct 更新会被 GORM 跳过
	return DB.Model(&ContentTopic{}).Where("id = ?", exist.ID).Updates(map[string]interface{}{
		"tags":                 ct.Tags,
		"groups":               ct.Groups,
		"base_risk":            ct.BaseRisk,
		"risk":                 ct.Risk,
		"public_opinion_score": ct.PublicOpinionScore,
		"public_opinion_high":  ct.PublicOpinionHigh,
		"source":               ct.Source,
		"confidence":           ct.Confidence,
		"review_status":        ct.ReviewStatus,
		"updated_at":           ct.UpdatedAt,
	}).Error
}

// ApplyAIAssessment AI 审核写标注。
//
// 三条硬规则：
//  1. **人工标注优先**：existing.Source == manual 时直接跳过，AI 不得覆盖人；
//  2. 置信度低于门槛 → review_status=needs_review（守卫据此只标注、不拦截）；
//  3. 不覆盖管理端已设的策略（policy）——那是站长的决定，不是模型的。
func ApplyAIAssessment(contentType string, contentID uint, tags []string, confidence int) error {
	if DB == nil || contentID == 0 || len(tags) == 0 {
		return nil
	}
	if existing, err := TopicOf(contentType, contentID); err == nil && existing != nil {
		if existing.Source == "manual" {
			return nil // 人工改过的标注，AI 只读不写
		}
	}
	minConf := 60
	if cfg, err := GetConfig(); err == nil && cfg != nil && cfg.MinConfidence > 0 {
		minConf = cfg.MinConfidence
	}
	a := topic.Assess(tags, "")
	tagsJSON, _ := json.Marshal(a.Tags)
	gs := make([]string, 0, len(a.Groups))
	for _, g := range a.Groups {
		gs = append(gs, string(g))
	}
	groupsJSON, _ := json.Marshal(gs)
	rs := ReviewAuto
	if confidence < minConf {
		rs = ReviewNeedsReview
	}
	return UpsertTopic(&ContentTopic{
		ContentType:        contentType,
		ContentID:          contentID,
		Tags:               string(tagsJSON),
		Groups:             string(groupsJSON),
		BaseRisk:           a.BaseRisk.String(),
		Risk:               a.Risk.String(),
		PublicOpinionScore: a.PublicOpinionScore,
		PublicOpinionHigh:  a.PublicOpinionHigh,
		Source:             SourceAI,
		Confidence:         confidence,
		ReviewStatus:       rs,
	})
}

// ApplyAssessment 把 topic.Assessment 落成 ContentTopic 并写入（人工/系统路径）。
func ApplyAssessment(contentType string, contentID uint, a topic.Assessment, source string) error {
	gs := make([]string, 0, len(a.Groups))
	for _, g := range a.Groups {
		gs = append(gs, string(g))
	}
	tags, _ := json.Marshal(a.Tags)
	groups, _ := json.Marshal(gs)
	return UpsertTopic(&ContentTopic{
		ContentType:        contentType,
		ContentID:          contentID,
		Tags:               string(tags),
		Groups:             string(groups),
		BaseRisk:           a.BaseRisk.String(),
		Risk:               a.Risk.String(),
		PublicOpinionScore: a.PublicOpinionScore,
		PublicOpinionHigh:  a.PublicOpinionHigh,
		Source:             source,
		Confidence:         100,
		ReviewStatus:       ReviewConfirmed,
	})
}

// AssessmentOf 读出内容的话题评估结果（供守卫使用，不重算文本升档）。
func AssessmentOf(contentType string, contentID uint) topic.Assessment {
	ct, err := TopicOf(contentType, contentID)
	if err != nil || ct == nil {
		return topic.Assessment{Tags: nil, Groups: nil, Risk: topic.RiskLow}
	}
	var tags []string
	_ = json.Unmarshal([]byte(ct.Tags), &tags)
	var gsRaw []string
	_ = json.Unmarshal([]byte(ct.Groups), &gsRaw)
	gs := make([]topic.Group, 0, len(gsRaw))
	for _, g := range gsRaw {
		gs = append(gs, topic.Group(g))
	}
	return topic.Assessment{
		Tags:               tags,
		Groups:             gs,
		BaseRisk:           topic.ParseRisk(ct.BaseRisk),
		Risk:               topic.ParseRisk(ct.Risk),
		PublicOpinionScore: ct.PublicOpinionScore,
		PublicOpinionHigh:  ct.PublicOpinionHigh,
	}
}

// TopicState 话题标注的完整状态（评估 + 置信度 + 策略覆盖 + 复核状态）。
//
// 为什么要单独包一层：判定不只需要风险等级，还需要「这份标注可不可信」
// 与「本站对这个话题的策略覆盖」。只传 topic.Assessment 会丢掉这两个关键前提。
type TopicState struct {
	Assessment   topic.Assessment
	Confidence   int
	ReviewStatus string
	Policy       string
	Exists       bool
}

// StateOf 读取内容的话题状态；无标注时返回 Exists=false（守卫据此决定是否放行）。
func StateOf(contentType string, contentID uint) TopicState {
	ct, err := TopicOf(contentType, contentID)
	if err != nil || ct == nil {
		return TopicState{Exists: false}
	}
	a := topic.Assessment{
		Tags:               topic.ParseTagsJSON(ct.Tags),
		BaseRisk:           topic.ParseRisk(ct.BaseRisk),
		Risk:               topic.ParseRisk(ct.Risk),
		PublicOpinionScore: ct.PublicOpinionScore,
		PublicOpinionHigh:  ct.PublicOpinionHigh,
	}
	var gsRaw []string
	_ = json.Unmarshal([]byte(ct.Groups), &gsRaw)
	for _, g := range gsRaw {
		a.Groups = append(a.Groups, topic.Group(g))
	}
	return TopicState{
		Assessment:   a,
		Confidence:   ct.Confidence,
		ReviewStatus: ct.ReviewStatus,
		Policy:       ct.Policy,
		Exists:       true,
	}
}

// ListTerms 列出词条。includePending=false 时只返回生效的。
func ListTerms(includePending bool) ([]Term, error) {
	if DB == nil {
		return nil, nil
	}
	q := DB.Model(&Term{})
	if !includePending {
		q = q.Where("status = ?", StatusActive)
	}
	var out []Term
	err := q.Order("cat asc, severity desc, id asc").Find(&out).Error
	return out, err
}

// ActiveLexicon 合并「内置基线 + 库内生效词条」，供扫描使用。
//
// 合并规则：同词以库内为准（站长可改 severity / 升级话题 / 强制拦截），
// 这样既保住新站开箱即用的基线，又不会让站长改不动基线词。
func ActiveLexicon() ([]lexicon.Term, error) {
	base := make([]lexicon.Term, 0, len(lexicon.Baseline)+32)
	base = append(base, lexicon.Baseline...)

	rows, err := ListTerms(false)
	if err != nil {
		return base, err
	}
	idx := map[string]int{}
	for i, t := range base {
		idx[t.Word] = i
	}
	for _, r := range rows {
		esc := parseGroups(r.EscalateUnder)
		lt := lexicon.Term{
			Word: r.Word, Cat: r.Cat, Severity: r.Severity,
			EscalateUnder: esc,
			ForceBlock:    r.ActionOverride == ActionBlock,
		}
		if i, ok := idx[r.Word]; ok {
			base[i] = lt
			continue
		}
		base = append(base, lt)
	}
	return base, nil
}

// parseGroups 解析 JSON 数组；容错为空。
func parseGroups(raw string) []topic.Group {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var strs []string
	if err := json.Unmarshal([]byte(raw), &strs); err != nil {
		// 兼容手工填的逗号分隔
		for _, s := range strings.Split(raw, ",") {
			if s = strings.TrimSpace(s); s != "" {
				strs = append(strs, s)
			}
		}
	}
	out := make([]topic.Group, 0, len(strs))
	for _, s := range strs {
		out = append(out, topic.Group(s))
	}
	return out
}

// RecordHit 记录一次裁决。
func RecordHit(h *Hit) error {
	if DB == nil || h == nil {
		return nil
	}
	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now()
	}
	return DB.Create(h).Error
}

// CountItem 统计用（词 / 话题组）。
type CountItem struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

// Stats 舆情概览（警方台顶部用）。
type Stats struct {
	Total       int64       `json:"total"`
	Blocked     int64       `json:"blocked"`
	Flagged     int64       `json:"flagged"`
	HighRisk    int64       `json:"high_risk"`
	Last24h     int64       `json:"last_24h"`
	Last7d      int64       `json:"last_7d"`
	TopGroups   []CountItem `json:"top_groups"`
	TopWords    []CountItem `json:"top_words"`
	UpdatedAt   time.Time   `json:"updated_at"`
	PendingTerm int64       `json:"pending_terms"`
}

// LoadStats 汇总统计。days 控制 Top 榜的时间窗（默认 30 天）。
func LoadStats(days int) (*Stats, error) {
	s := &Stats{UpdatedAt: time.Now()}
	if DB == nil {
		return s, nil
	}
	if days <= 0 {
		days = 30
	}
	since := time.Now().AddDate(0, 0, -days)

	DB.Model(&Hit{}).Count(&s.Total)
	DB.Model(&Hit{}).Where("decision = ?", ActionBlock).Count(&s.Blocked)
	DB.Model(&Hit{}).Where("decision = ?", ActionFlag).Count(&s.Flagged)
	DB.Model(&Hit{}).Where("risk = ?", topic.RiskHigh.String()).Count(&s.HighRisk)
	DB.Model(&Hit{}).Where("created_at >= ?", time.Now().Add(-24*time.Hour)).Count(&s.Last24h)
	DB.Model(&Hit{}).Where("created_at >= ?", time.Now().AddDate(0, 0, -7)).Count(&s.Last7d)
	DB.Model(&Term{}).Where("status = ?", StatusPending).Count(&s.PendingTerm)

	// 按话题组统计：groups 存成逗号分隔（单条可能多组），这里在 Go 侧展开
	var rows []struct {
		Groups string
		Reason string
		N      int64
	}
	DB.Model(&Hit{}).Select("groups, reason, count(*) as n").
		Where("created_at >= ?", since).Group("groups, reason").Find(&rows)
	gc := map[string]int64{}
	for _, r := range rows {
		for _, g := range strings.Split(r.Groups, ",") {
			if g = strings.TrimSpace(g); g != "" {
				gc[g] += r.N
			}
		}
	}
	s.TopGroups = topN(gc, 6)

	// 命中词榜
	var wordRows []struct {
		MatchedWords string
		N            int64
	}
	DB.Model(&Hit{}).Select("matched_words, count(*) as n").
		Where("created_at >= ?", since).Group("matched_words").Find(&wordRows)
	wc := map[string]int64{}
	for _, r := range wordRows {
		for _, w := range strings.Split(r.MatchedWords, ",") {
			if w = strings.TrimSpace(w); w != "" {
				wc[w] += r.N
			}
		}
	}
	s.TopWords = topN(wc, 10)
	return s, nil
}

// topN 取计数前 n，按次数降序。
func topN(m map[string]int64, n int) []CountItem {
	out := make([]CountItem, 0, len(m))
	for k, v := range m {
		out = append(out, CountItem{Key: k, Count: v})
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Count > out[i].Count {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	if len(out) > n {
		out = out[:n]
	}
	return out
}
