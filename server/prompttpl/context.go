package prompttpl

import (
	"fmt"
	"os"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"
)

// ==================== 模板上下文自动注入（§13.2） ====================
// 目标：把「作品上下文 / 角色信息 / 前文摘要 / 作者样本」自动填进模板变量，
// 而不是让作者手工复制粘贴。
//
// 数据来源（全部是作者自己在站内已有的数据，不额外采集）：
//   作品名/类型/简介  ← novels
//   大纲              ← ai_novel_memories(kind=outline)
//   角色信息          ← ai_novel_memories(kind=characters)（角色设定）
//   前文摘要          ← ai_novel_memories(kind=chapter_summary) 最近 N 章
//   前文              ← 上一章正文尾部
//   原文/正文         ← 当前章正文（编辑器传入的内容优先，不覆盖）
//   作者样本          ← 同作品其它章节正文片段（风格参考，不是训练）
//
// 安全：只读作者本人作品（handler 校验归属）；正文按 rune 截断，避免超长撑爆上下文。

// contextTailRunes 单段正文默认注入窗口
const contextTailRunes = 3000

// ContextResult 自动注入结果
type ContextResult struct {
	Vars         map[string]string `json:"vars"`
	Sources      map[string]string `json:"sources"`     // 变量名 → 来源标识
	AutoFilled   []string          `json:"auto_filled"` // 自动填充的变量名
	NovelTitle   string            `json:"novel_title"`
	ChapterTitle string            `json:"chapter_title"`
}

// BuildNovelVars 组装作品上下文变量。
// provided 中的键值优先级最高（例如编辑器里正在写的正文不能被旧章节覆盖）。
func BuildNovelVars(novelID, chapterID uint, provided map[string]string, tailRunes int) (*ContextResult, error) {
	novel, err := models.GetNovelByID(novelID)
	if err != nil || novel == nil {
		return nil, fmt.Errorf("作品不存在")
	}
	if tailRunes <= 0 {
		tailRunes = contextTailRunes
	}
	res := &ContextResult{
		Vars:       map[string]string{},
		Sources:    map[string]string{},
		NovelTitle: novel.Title,
	}

	// 基础信息
	set := func(value, source string, aliases ...string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, name := range aliases {
			// 客户端已提供则不覆盖
			if v, ok := provided[name]; ok && strings.TrimSpace(v) != "" {
				continue
			}
			res.Vars[name] = value
			res.Sources[name] = source
		}
	}

	set(novel.Title, "novel.title", "作品名", "作品", "书名")
	set(novel.Category, "novel.category", "类型", "题材", "分类")
	set(novel.Summary, "novel.summary", "作品简介", "简介")
	set(models.GetNovelOutline(novel.ID), "memory.outline", "大纲", "作品大纲")
	set(models.GetNovelCharacters(novel.ID), "memory.characters", "角色信息", "角色", "人物", "角色库", "人物设定")

	// 前文摘要：最近 5 章（不含当前章）
	if summaries, serr := models.ListRecentChapterSummaries(novel.ID, chapterID, 5); serr == nil && len(summaries) > 0 {
		var lines []string
		for i := len(summaries) - 1; i >= 0; i-- { // 转成时间正序
			lines = append(lines, "- "+strings.TrimSpace(summaries[i].Content))
		}
		set(strings.Join(lines, "\n"), "memory.summaries", "前文摘要", "摘要", "章节摘要")
	}

	// 当前章 / 上一章正文
	var cur *models.Chapter
	if chapterID > 0 {
		cur, _ = models.GetChapterByID(chapterID)
	} else if all, cerr := models.GetChaptersByNovel(novel.ID); cerr == nil && len(all) > 0 {
		cur = &all[len(all)-1] // GetChaptersByNovel 按章节号升序
	}
	if cur != nil {
		res.ChapterTitle = cur.Title
		set(cur.Title, "chapter.title", "章节名", "章节", "当前章节")
		if body := readChapterBody(cur, tailRunes); body != "" {
			set(body, "chapter.current", "原文", "正文", "待审文本", "本章内容", "文本")
		}
		if prev, perr := models.GetChapterByNovelAndNumber(novel.ID, cur.ChapterNumber-1); perr == nil && prev != nil {
			if body := readChapterBody(prev, tailRunes); body != "" {
				set(body, "chapter.prev", "前文")
			}
		}
	}

	// 作者样本：同作品其它章节各取一段（最多 2 段 × 500 字）
	if sample := buildAuthorSample(novel.ID, cur); sample != "" {
		set(sample, "author.sample", "作者样本", "风格样本", "作者文风")
	}

	// 客户端提供的变量计入结果与来源（保持同口径）
	for k, v := range provided {
		if strings.TrimSpace(v) == "" {
			continue
		}
		res.Vars[k] = v
		res.Sources[k] = "client"
	}

	// 只把「有值」的变量列入 auto_filled（client 提供的不算自动）
	for name, src := range res.Sources {
		if src != "client" {
			res.AutoFilled = append(res.AutoFilled, name)
		}
	}
	sortStrings(res.AutoFilled)
	return res, nil
}

// readChapterBody 读取章节正文并截取尾部窗口（rune 截断）
func readChapterBody(ch *models.Chapter, tailRunes int) string {
	if ch == nil || strings.TrimSpace(ch.ContentPath) == "" {
		return ""
	}
	b, err := os.ReadFile(utils.ResolveContentPath(ch.ContentPath))
	if err != nil {
		return ""
	}
	r := []rune(strings.TrimSpace(string(b)))
	if len(r) > tailRunes {
		r = r[len(r)-tailRunes:]
	}
	return string(r)
}

// buildAuthorSample 取作者同作品其它章节的片段作为风格样本
func buildAuthorSample(novelID uint, exclude *models.Chapter) string {
	chapters, err := models.GetChaptersByNovel(novelID)
	if err != nil || len(chapters) == 0 {
		return ""
	}
	var parts []string
	// 从最近的往前找，最多 2 段
	for i := len(chapters) - 1; i >= 0 && len(parts) < 2; i-- {
		ch := chapters[i]
		if exclude != nil && ch.ID == exclude.ID {
			continue
		}
		b := readChapterBody(&ch, 500)
		if b == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("【示例片段：第%d章 %s】\n%s", ch.ChapterNumber, ch.Title, b))
	}
	return strings.Join(parts, "\n\n")
}

func sortStrings(list []string) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && list[j] < list[j-1]; j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}

// Placeholders 提取模板中用到的占位名（去重，保持出现顺序）
func Placeholders(content string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range placeholderRe.FindAllStringSubmatch(content, -1) {
		if len(m) != 2 || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		out = append(out, m[1])
	}
	return out
}

// FilterVars 只保留模板实际用到的变量（避免 unused 噪音）
func FilterVars(content string, vars map[string]string) map[string]string {
	keep := Placeholders(content)
	if len(keep) == 0 {
		return map[string]string{}
	}
	out := map[string]string{}
	for _, name := range keep {
		if v, ok := vars[name]; ok {
			out[name] = v
		}
	}
	return out
}
