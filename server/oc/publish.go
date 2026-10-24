package oc

// ==================== 一键发布：对话流 → 话本小说 ====================
// 轮子复用：作品/章节发布链路、AI 标识、发文限额、审核、来源追溯全部沿用现有体系。
// 「一键」= 一键导入 + 一键提交审核，**不是一键绕过审核**。

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nvs-server/aiclient"
	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/security"
)

// PublishStyle 正文形态
const (
	StyleDialogue  = "dialogue"  // 对话体（= 话本气泡：角色：台词）
	StyleNarrative = "narrative" // 叙述体
)

// PublishInput 发布入参
type PublishInput struct {
	FlowID          uint
	NovelID         uint   // 0 = 新建作品
	Title           string // 新建作品时的标题
	Category        string
	UserID          uint   // 发起人（章节作者）
	FromSeq, ToSeq  int    // 轮次范围（0 = 全部）
	Style           string // dialogue / narrative
	IncludeSettings bool   // 是否生成「设定集」章节
	Polish          bool   // 是否 AI 润色（叙述体场景，消耗代币）
	ChapterPrefix   string // 章节标题前缀（默认「第N章」）
	PublishNow      bool   // 是否直接发布（否则草稿待提审）
}

// PublishResult 发布结果
type PublishResult struct {
	NovelID     uint   `json:"novel_id"`
	ChapterIDs  []uint `json:"chapter_ids"`
	ChapterCnt  int    `json:"chapter_count"`
	Words       int    `json:"words"`
	AIModelName string `json:"ai_model_name,omitempty"`
}

// PublishFlowToNovel 把对话流转成章节并落到作品里
func PublishFlowToNovel(in PublishInput) (*PublishResult, error) {
	f, err := GetFlow(in.FlowID)
	if err != nil {
		return nil, err
	}
	if in.UserID == 0 {
		return nil, errors.New("请先登录")
	}
	turns, err := listTurnsRange(f.ID, in.FromSeq, in.ToSeq)
	if err != nil {
		return nil, err
	}
	if len(turns) == 0 {
		return nil, errors.New("所选范围内没有发言")
	}

	novel, created, err := ensureNovel(in)
	if err != nil {
		return nil, err
	}
	_ = created

	style := StyleDialogue
	if strings.ToLower(strings.TrimSpace(in.Style)) == StyleNarrative {
		style = StyleNarrative
	}
	// 每 N 轮切成一章（默认 20 轮一章）
	const turnsPerChapter = 20
	ownerID := novel.AuthorID
	authorDir := filepath.Join(config.NovelDataDir, "authors", fmt.Sprintf("%d", ownerID), fmt.Sprintf("%d", novel.ID))
	if err := os.MkdirAll(authorDir, 0755); err != nil {
		return nil, errors.New("创建目录失败")
	}

	res := &PublishResult{NovelID: novel.ID}
	startNum, _ := models.GetMaxChapterNumber(novel.ID)
	if in.IncludeSettings && startNum == 0 {
		startNum = 0
	}
	num := startNum
	chapterIdx := 0
	modelName := ""
	hasAI := false

	// 设定集（可选）：角色卡 + 世界书条目
	if in.IncludeSettings {
		num++
		body := buildSettingsChapter(f, turns)
		path, cerr := writeChapterFile(authorDir, num, body)
		if cerr != nil {
			return nil, cerr
		}
		ch := newChapter(novel.ID, num, "设定集", body, in.UserID, chapterModeOf(style), in.PublishNow, f.ID)
		ch.ContentPath = path
		if err := models.CreateChapter(ch); err != nil {
			return nil, err
		}
		res.ChapterIDs = append(res.ChapterIDs, ch.ID)
		res.Words += len([]rune(body))
	}

	for i := 0; i < len(turns); i += turnsPerChapter {
		end := i + turnsPerChapter
		if end > len(turns) {
			end = len(turns)
		}
		chunk := turns[i:end]
		body := renderChunk(chunk, style)
		if in.Polish {
			body2, mn, perr := polishChunk(body, in.UserID, f.GroupID)
			if perr == nil {
				body = body2
				modelName = mn
			}
		}
		body = strings.TrimSpace(security.SanitizeUserContent(body))
		if body == "" {
			continue
		}
		// 输出侧审核：发布内容必须过违禁词
		body, _ = security.ApplyBadWords(body, "chapter")
		chapterIdx++
		num++
		title := fmt.Sprintf("第%d章", num)
		if in.ChapterPrefix != "" {
			title = fmt.Sprintf("%s%d", in.ChapterPrefix, num)
		}
		path, werr := writeChapterFile(authorDir, num, body)
		if werr != nil {
			return nil, werr
		}
		ch := newChapter(novel.ID, num, title, body, in.UserID, chapterModeOf(style), in.PublishNow, f.ID)
		ch.ContentPath = path
		for _, t := range chunk {
			if t.UserID == 0 {
				hasAI = true
				if modelName == "" {
					modelName = t.ModelName
				}
			}
		}
		if err := models.CreateChapter(ch); err != nil {
			return nil, err
		}
		if hasAI && modelName != "" {
			_ = models.MarkAiContent(models.AiMarkChapter, ch.ID, modelName)
		}
		res.ChapterIDs = append(res.ChapterIDs, ch.ID)
		res.Words += len([]rune(body))
	}
	res.ChapterCnt = len(res.ChapterIDs)
	res.AIModelName = modelName
	models.UpdateNovelStats(novel.ID)
	// AI 参与标识（作品级）
	if hasAI {
		_ = models.DB.Model(&models.Novel{}).Where("id = ?", novel.ID).
			Updates(map[string]interface{}{"creation_method": "human_ai_assisted"}).Error
	}
	return res, nil
}

// chapterModeOf 对话体 → 话本气泡（script）；叙述体 → 散文（prose）
func chapterModeOf(style string) string {
	if style == StyleNarrative {
		return "prose"
	}
	return "script"
}

func ensureNovel(in PublishInput) (*models.Novel, bool, error) {
	if in.NovelID > 0 {
		n, err := models.GetNovelByID(in.NovelID)
		if err != nil {
			return nil, false, errors.New("目标作品不存在")
		}
		// 只有主笔或协作者可追加章节（沿用协作规则）
		if n.AuthorID != in.UserID {
			return nil, false, errors.New("只有作品主笔可以从对话流导入章节")
		}
		return n, false, nil
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = "来自 OC 群组的作品"
	}
	cat := strings.TrimSpace(in.Category)
	if cat == "" {
		cat = "其他"
	}
	n := &models.Novel{
		AuthorID: in.UserID, Title: title, Category: cat,
		Status: "draft", Mode: "script",
		CreationMethod: "human_ai_assisted",
		CreatedAt:      time.Now(), UpdatedAt: time.Now(),
	}
	if err := models.CreateNovel(n); err != nil {
		return nil, false, err
	}
	return n, true, nil
}

func listTurnsRange(flowID uint, from, to int) ([]Turn, error) {
	q := models.DB.Where("flow_id = ?", flowID)
	if from > 0 {
		q = q.Where("seq >= ?", from)
	}
	if to > 0 {
		q = q.Where("seq <= ?", to)
	}
	var list []Turn
	if err := q.Order("seq ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	ids := map[uint]bool{}
	for _, t := range list {
		if t.CharacterID > 0 {
			ids[t.CharacterID] = true
		}
	}
	nameOf := map[uint]string{}
	if len(ids) > 0 {
		keys := make([]uint, 0, len(ids))
		for id := range ids {
			keys = append(keys, id)
		}
		var chars []Character
		models.DB.Select("id, name").Where("id IN ?", keys).Find(&chars)
		for _, c := range chars {
			nameOf[c.ID] = c.Name
		}
	}
	for i := range list {
		list[i].CharacterName = nameOf[list[i].CharacterID]
		list[i].IsAI = list[i].UserID == 0
	}
	return list, nil
}

// renderChunk 把一批轮次渲染为章节正文
func renderChunk(chunk []Turn, style string) string {
	var b strings.Builder
	for _, t := range chunk {
		name := t.CharacterName
		if name == "" {
			name = "旁白"
		}
		if style == StyleNarrative {
			b.WriteString(fmt.Sprintf("%s说：“%s”\n\n", name, singleLine(t.Content)))
			continue
		}
		// 对话体 = 话本语法：角色：台词（同角色连句自然成气泡）
		for _, line := range strings.Split(strings.ReplaceAll(t.Content, "\r\n", "\n"), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			b.WriteString(name + "：" + line + "\n")
		}
	}
	return b.String()
}

func singleLine(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
}

// buildSettingsChapter 设定集：角色卡 + 世界书条目
func buildSettingsChapter(f *Flow, turns []Turn) string {
	var b strings.Builder
	b.WriteString("【角色】\n\n")
	seen := map[uint]bool{}
	var chars []Character
	ids := make([]uint, 0, 8)
	for _, t := range turns {
		if t.CharacterID > 0 && !seen[t.CharacterID] {
			seen[t.CharacterID] = true
			ids = append(ids, t.CharacterID)
		}
	}
	if len(ids) > 0 {
		models.DB.Where("id IN ?", ids).Find(&chars)
	}
	for _, c := range chars {
		b.WriteString("### " + c.Name + "\n")
		if c.Appearance != "" {
			b.WriteString("- 外貌：" + c.Appearance + "\n")
		}
		if c.Persona != "" {
			b.WriteString("- 人设：" + c.Persona + "\n")
		}
		if c.Speech != "" {
			b.WriteString("- 说话风格：" + c.Speech + "\n")
		}
		b.WriteString("\n")
	}
	if f.BookID > 0 {
		var book WorldBook
		if err := models.DB.Where("id = ?", f.BookID).First(&book).Error; err == nil {
			b.WriteString("\n【世界书】" + book.Title + "\n\n")
			entries, _ := ListEntries(book.ID)
			for _, e := range entries {
				b.WriteString(fmt.Sprintf("- （%s）%s：%s\n", EntryKindLabel(e.Kind), e.Key, e.Content))
			}
		}
	}
	return b.String()
}

func writeChapterFile(authorDir string, num int, body string) (string, error) {
	p := filepath.Join(authorDir, fmt.Sprintf("%d.html", num))
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		return "", err
	}
	return filepath.ToSlash(p), nil
}

// polishChunk AI 润色：把对话体片段整理成叙述体正文（可选，消耗代币，受预算约束）
func polishChunk(body string, userID, groupID uint) (string, string, error) {
	if berr := CheckBudget(groupID, 0, 0, 0); berr != nil {
		return body, "", nil // 预算已达上限：直接用未润色版本，不阻断发布
	}
	list, err := models.ListAiModelsByKind(models.AiKindChat, true)
	if err != nil || len(list) == 0 {
		return body, "", nil
	}
	m := &list[0]
	prompt := "请把下面的角色扮演对话整理成小说叙述段落：保留人物性格与信息量，" +
		"补上必要的动作与环境描写，不改情节，不新增设定，输出纯正文。\n\n" + body
	estTokens := models.EstimateTokens(prompt) + 1024
	freeOK, _, _ := models.CheckAiFreeQuota(userID, estTokens)
	if !freeOK {
		if !models.IsTokenEnabled() {
			return body, "", nil
		}
		if _, ok := models.PrecheckAiBalance(userID, m, estTokens, m.MaxTokens); !ok {
			return body, "", nil
		}
	}
	lease, kerr := models.AcquireModelKey(m)
	if kerr != nil {
		return body, "", nil
	}
	apiKey := ""
	if lease != nil {
		apiKey = lease.Key
	}
	callLog, lerr := models.CreateAiCallLog(userID, m, "oc_publish", "oc_flow", 0,
		models.EstimateTokens(prompt), m.MaxTokens, freeOK)
	if lerr != nil {
		return body, "", nil
	}
	client := aiclient.NewClient(m.BaseURL, apiKey, 120*time.Second)
	out, realIn, realOut, cerr := client.Complete(aiclient.ChatRequest{
		Model: m.ModelID,
		Messages: []aiclient.Message{
			{Role: "system", Content: prompt},
			{Role: "user", Content: body},
		},
		MaxTokens: m.MaxTokens, Temperature: 0.6,
	})
	if cerr != nil || strings.TrimSpace(out) == "" {
		lease.Fail()
		if callLog != nil {
			models.FailAiCallLog(callLog.ID, "润色失败")
		}
		return body, "", nil
	}
	if realIn <= 0 {
		realIn = estTokens
	}
	if realOut <= 0 {
		realOut = models.EstimateTokens(out)
	}
	cost := models.EstimateAiCost(m, realIn, realOut)
	if freeOK {
		_ = models.RecordAiFreeUsage(userID, 1, realIn+realOut)
	} else if _, serr := models.SettleAiCallLog(callLog.ID, realIn, realOut); serr != nil {
		return body, "", nil
	}
	lease.OK()
	AddUsage(groupID, int(cost))
	return strings.TrimSpace(out), m.Name, nil
}

func newChapter(novelID uint, num int, title, body string, authorID uint, mode string, publish bool, flowID uint) *models.Chapter {
	status := models.ChapterStatusDraft
	if publish {
		status = models.ChapterStatusPublished
	}
	ch := &models.Chapter{
		NovelID: novelID, ChapterNumber: num, Title: title,
		ContentPath: "", Status: status, AuthorID: authorID, Mode: mode,
		SourceFlowID: flowID, // 来源追溯：来自哪个 OC 对话流
		WordCount: len([]rune(body)), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	return ch
}
