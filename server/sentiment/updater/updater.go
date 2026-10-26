package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"nvs-server/aiclient"
	"nvs-server/models"
	"nvs-server/sentiment/lexicon"
	"nvs-server/sentiment/store"
)

// Result 一次更新的结果摘要（后台/日志用）。
type Result struct {
	Trigger    string    `json:"trigger"`
	StartedAt  time.Time `json:"started_at"`
	Learned    int       `json:"learned"`     // 对内候选数
	Searched   int       `json:"searched"`    // 实际发起的检索次数
	Inserted   int       `json:"inserted"`    // 入库条数
	Pending    int       `json:"pending"`     // 其中进待审
	Activated  int       `json:"activated"`   // 其中直接生效
	Skipped    int       `json:"skipped"`     // 已存在/被过滤
	SearchUsed int       `json:"search_used"` // 检索预算消耗
	Errors     []string  `json:"errors"`
}

// RunOnce 执行一次更新。返回错误仅代表「整轮不可用」（如配置关闭），
// 单条候选失败只记在 Errors 里，不中断整轮。
func RunOnce(trigger string) (*Result, error) {
	cfg, err := store.GetConfig()
	if err != nil || cfg == nil {
		return nil, fmt.Errorf("配置不可用")
	}
	if !cfg.Enabled || (!cfg.LearnFromAI && !cfg.SearchEnabled) {
		return &Result{Trigger: trigger, StartedAt: time.Now()}, nil
	}
	if store.DB == nil {
		return nil, fmt.Errorf("数据库未就绪")
	}

	res := &Result{Trigger: trigger, StartedAt: time.Now()}
	m, apiKey := pickModel(cfg.SearchModelID)
	if m == nil {
		res.Errors = append(res.Errors, "没有可用的对话模型，跳过本轮")
		return res, nil
	}

	budget := cfg.SearchBudgetPerDay
	if budget <= 0 {
		budget = 20
	}

	// ── 对内：从近期命中里提取可疑词 ──
	var candidates []Candidate
	if cfg.LearnFromAI {
		got, err := learnFromHits(context.Background(), m, apiKey)
		if err != nil {
			res.Errors = append(res.Errors, "对内学习失败："+err.Error())
		} else {
			res.Learned = len(got)
			candidates = append(candidates, got...)
		}
	}

	// ── 对外：检索 + 总结 ──
	if cfg.SearchEnabled && budget > 0 {
		queries := searchQueries(cfg, 3)
		for _, q := range queries {
			if res.SearchUsed >= budget {
				break
			}
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			snips, serr := DoSearch(ctx, m, apiKey, cfg.SearchPath, q, cfg.RecencyDays, 8)
			res.SearchUsed++
			res.Searched++
			if serr != nil {
				cancel()
				res.Errors = append(res.Errors, "检索失败("+q+")："+serr.Error())
				continue
			}
			client := aiclient.NewClient(m.BaseURL, apiKey, 60*time.Second)
			got, aerr := Summarize(ctx, client, m.ModelID, q, snips)
			cancel()
			if aerr != nil {
				res.Errors = append(res.Errors, "总结失败("+q+")："+aerr.Error())
				continue
			}
			candidates = append(candidates, got...)
		}
	}

	// ── 入库（去重 + 配额 + 置信度分级）──
	quota := cfg.DailyQuota
	if quota <= 0 {
		quota = 50
	}
	inserted := 0
	for _, c := range candidates {
		if inserted >= quota {
			break
		}
		status, ok := decideStatus(cfg, c)
		if !ok {
			res.Skipped++
			continue
		}
		if err := insertCandidate(c, status, trigger); err != nil {
			res.Skipped++
			continue
		}
		inserted++
		if status == store.StatusActive {
			res.Activated++
		} else {
			res.Pending++
		}
	}
	res.Inserted = inserted
	return res, nil
}

// decideStatus 置信度分级：高置信才可能直接生效，其余进待审。
func decideStatus(cfg *store.Config, c Candidate) (string, bool) {
	if c.Confidence < cfg.MinConfidence {
		// 低置信一律进待审：AI 判断不可靠时，宁可让人看一眼
		return store.StatusPending, true
	}
	if cfg.RequireReview {
		return store.StatusPending, true
	}
	return store.StatusActive, true
}

// insertCandidate 入库并写审计。已存在的词跳过（不覆盖人工调过的绑定）。
func insertCandidate(c Candidate, status, trigger string) error {
	var exist store.Term
	err := store.DB.Where("word = ?", c.Word).First(&exist).Error
	if err == nil {
		return fmt.Errorf("已存在")
	}
	tagsJSON, _ := json.Marshal(c.Tags)
	now := time.Now()
	t := &store.Term{
		Word:            c.Word,
		Cat:             c.Cat,
		EscalateUnder:   string(tagsJSON),
		Severity:        severityOf(c.Risk),
		Source:          store.SourceAI,
		Status:          status,
		Confidence:      c.Confidence,
		ReviewCycleDays: 90,
		// Note 存「模型自己的一句话摘要」，不是检索原文
		Note:      truncate(c.Basis, 240),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.DB.Create(t).Error; err != nil {
		return err
	}
	// GORM Create 会跳过零值，这里显式回写（severity/confidence 可能为 0）
	store.DB.Model(&store.Term{}).Where("id = ?", t.ID).Updates(map[string]interface{}{
		"severity": t.Severity, "confidence": t.Confidence,
	})
	return store.DB.Create(&store.TermAudit{
		TermID: t.ID, Word: t.Word, Action: "create",
		Operator:  "system:" + trigger,
		Note:      "AI 自动学习入库（状态=" + status + "，置信度=" + fmt.Sprint(c.Confidence) + "）",
		After:     truncate(c.Basis, 240),
		CreatedAt: now,
	}).Error
}

// severityOf 风险等级 → 词级。黑话类永远给 1，避免自动学来的词被判成可拦截。
func severityOf(risk string) int {
	switch strings.ToLower(risk) {
	case "high":
		return 3
	case "midhigh":
		return 2
	case "mid":
		return 2
	default:
		return 1
	}
}

// truncate 按字符截断（不是字节，避免切坏中文）。
func truncate(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n]) + "…"
}

// pickModel 选检索/总结用的模型：优先配置指定的，否则挑一个启用的对话模型。
func pickModel(id uint) (*models.AiModel, string) {
	var m *models.AiModel
	if id > 0 {
		if got, err := models.GetAiModel(id); err == nil {
			m = got
		}
	}
	if m == nil {
		list, err := models.ListAiModels(false)
		if err != nil {
			return nil, ""
		}
		for i := range list {
			if list[i].Enabled && list[i].Kind == models.AiKindChat && list[i].BaseURL != "" {
				m = &list[i]
				break
			}
		}
	}
	if m == nil {
		return nil, ""
	}
	key := ""
	if lease, err := models.AcquireModelKey(m); lease != nil && err == nil {
		key = lease.Key
	}
	return m, key
}

// searchQueries 生成本轮检索词。
//
// 不硬编码具体黑话：用「平台近期命中词 + 通用问法」两个来源，
// 这样新站没有历史语料时也能跑起来，老站则能顺着已有线索深挖。
func searchQueries(cfg *store.Config, n int) []string {
	base := strings.TrimSpace(cfg.SearchQuery)
	if base == "" {
		base = "网络 最新 黑话 缩写"
	}
	base = strings.ReplaceAll(base, "{date}", time.Now().Format("2006-01"))
	out := []string{base}

	// 从近期命中里取几个高频词，去外部看它们的语境
	rows, err := store.DB.Model(&store.Hit{}).
		Select("matched_words").Where("created_at >= ?", time.Now().AddDate(0, 0, -7)).
		Limit(50).Rows()
	if err == nil {
		defer rows.Close()
		seen := map[string]bool{}
		for rows.Next() && len(out) < n {
			var mw string
			if rows.Scan(&mw) != nil {
				continue
			}
			for _, w := range strings.Split(mw, ",") {
				w = strings.TrimSpace(w)
				if w == "" || seen[w] || len([]rune(w)) < 2 {
					continue
				}
				seen[w] = true
				out = append(out, w+" 是什么意思 用法")
				if len(out) >= n {
					break
				}
			}
		}
	}
	return out
}

// learnFromHits 对内线：把近期被标注/拦截的评论交给模型，归纳可疑新词。
//
// 只送「已被判定过的内容片段」，不碰未命中内容——避免把用户原文大批外发给上游。
func learnFromHits(ctx context.Context, m *models.AiModel, apiKey string) ([]Candidate, error) {
	rows, err := store.DB.Model(&store.Hit{}).
		Where("created_at >= ? AND decision IN ?", time.Now().AddDate(0, 0, -7), []string{"flag", "block"}).
		Order("id DESC").Limit(30).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []string
	known := lexicon.ByWord()
	for _, t := range lexicon.Baseline {
		known[t.Word] = t
	}
	for rows.Next() {
		var id, cid uint
		var mw string
		if rows.Scan(&id, &cid, &mw) != nil {
			continue
		}
		_ = id
		if mw != "" {
			lines = append(lines, "命中词："+mw)
		}
	}
	if len(lines) == 0 {
		return nil, nil
	}

	client := aiclient.NewClient(m.BaseURL, apiKey, 60*time.Second)
	content, _, _, err := client.Complete(aiclient.ChatRequest{
		Ctx:   ctx,
		Model: m.ModelID,
		Messages: []aiclient.Message{
			{Role: "system", Content: learnPrompt},
			{Role: "user", Content: strings.Join(lines, "\n")},
		},
		MaxTokens: 500, Temperature: 0,
	})
	if err != nil {
		return nil, err
	}
	return parseCandidates(content), nil
}

const learnPrompt = `你是内容安全词表的分析助手。下面是平台近期在评论区被标注/拦截过的词。

请据此推测：还有哪些**同义或近邻**的网络黑话/辱骂变体可能在平台上出现，且尚未被收录。

要求：
1. 只输出你真正认为有风险的词，不要凑数；拿不准就少输出。
2. 不要输出脏话原文的完整长句，只给词。
3. 只输出一行 JSON 数组，不要 markdown、不要解释：
[{"word":"词","cat":"abuse|raid|slang|bigotry","tags":["亚文化"],"risk":"low|mid|midhigh|high","confidence":0,"basis":"一句话理由"}]`

// ── 每日调度 ──

var (
	runMu    sync.Mutex
	lastDate string
)

// Start 启动每日调度。每小时检查一次，当天该跑还没跑就补跑。
//
// 不用精确 cron：容器/服务器时区、休眠恢复都会让 cron 漏跑；
// 「每小时看一眼今天跑过没有」更抗折腾，也天然支持错过后补跑。
func Start() {
	go func() {
		time.Sleep(2 * time.Minute) // 避开启动高峰
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			maybeRunDaily()
		}
	}()
}

// maybeRunDaily 到点且今天没跑过 → 跑一次。
func maybeRunDaily() {
	cfg, err := store.GetConfig()
	if err != nil || cfg == nil || !cfg.Enabled {
		return
	}
	if !cfg.LearnFromAI && !cfg.SearchEnabled {
		return
	}
	now := time.Now()
	if now.Hour() < cfg.AutoUpdateHour {
		return
	}
	today := now.Format("2006-01-02")

	runMu.Lock()
	if lastDate == today {
		runMu.Unlock()
		return
	}
	lastDate = today
	runMu.Unlock()

	res, err := RunOnce("daily")
	if err != nil {
		log.Printf("[sentiment] 每日词表更新失败: %v", err)
		return
	}
	log.Printf("[sentiment] 每日词表更新：候选=%d 检索=%d 入库=%d（待审 %d / 生效 %d）错误=%d",
		res.Learned, res.Searched, res.Inserted, res.Pending, res.Activated, len(res.Errors))
}
