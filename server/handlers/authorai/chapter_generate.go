package authorai

import (
	"fmt"
	"strings"

	"nvs-server/handlers/hctx"
	"nvs-server/models"
	authoraim "nvs-server/models/authorai"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

// ---------- 逐章生成（阶段流水线：组装上下文 → 生成 → 落盘） ----------

type generateChapterInput struct {
	Part    string `json:"part"` // outline / body
	ModelID uint   `json:"model_id"`
	Prompt  string `json:"prompt"`
}

// POST /api/author-ai/projects/:id/chapters/:cid/generate
// 逐章生成：把工程物料（设定/人物/大纲/时间线）+ 前序章节作为上下文打包，
// 对齐 AI Novel Writer 的 chapter-level context assembly；生成后落盘，可断点续跑。
func GenerateChapter(c *gin.Context) {
	id := atou(c.Param("id"))
	p, ok := requireOwner(c, id)
	if !ok {
		return
	}
	cid := atou(c.Param("cid"))
	var ch authoraim.Chapter
	if err := models.DB.Where("project_id = ? AND id = ?", p.ID, cid).First(&ch).Error; err != nil {
		infra.NotFound(c, "章节不存在")
		return
	}
	var in generateChapterInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	part := in.Part
	if part != "outline" && part != "body" {
		infra.BadRequest(c, "part 仅支持 outline / body")
		return
	}

	// 前序章节（提供衔接上下文，对齐阶段流水线的「前文即上下文」）
	var priors []authoraim.Chapter
	models.DB.Where("project_id = ? AND `order` < ?", p.ID, ch.Order).Order("`order` ASC, id ASC").Find(&priors)

	system, user := buildChapterPrompt(p, &ch, part, in.Prompt, priors)
	content, err := callAI(hctx.CurrentUserID(c), in.ModelID, system, user, p.ID)
	if err != nil {
		infra.InternalError(c, "AI 生成失败: "+err.Error())
		return
	}

	switch part {
	case "outline":
		ch.Outline = content
		if ch.Status == "" {
			ch.Status = "outline"
		}
	case "body":
		ch.Body = content
		ch.Status = "generated"
	}
	if err := models.DB.Save(&ch).Error; err != nil {
		infra.InternalError(c, "保存生成结果失败")
		return
	}
	infra.SuccessMessage(c, "已生成"+chapterPartLabel(part), gin.H{"part": part, "content": content, "chapter": ch})
}

func chapterPartLabel(part string) string {
	if part == "outline" {
		return "章纲"
	}
	return "正文"
}

// buildChapterPrompt 组装单章生成的 system/user 提示词（阶段流水线的上下文打包）。
// 把工程物料 + 前序章节（章纲 / 正文节选）+ 时间线循环结构一并注入，
// 使模型在续写时维持设定、人物与时间线（含 [LOOP] 嵌套）的一致性。
func buildChapterPrompt(p *authoraim.NovelProject, ch *authoraim.Chapter, part, extra string, priors []authoraim.Chapter) (system, user string) {
	style := styleDirective(p.StylePreset, p.StyleDetail)
	loopSummary := authoraim.SummarizeLoops(p.Timeline)

	var priorOutline, priorBody strings.Builder
	for _, pc := range priors {
		priorOutline.WriteString(fmt.Sprintf("\n· 第%d章《%s》：%s", pc.Order, pc.Title, clip(pc.Outline, 200)))
		if strings.TrimSpace(pc.Body) != "" {
			priorBody.WriteString(fmt.Sprintf("\n· 第%d章《%s》正文节选：%s", pc.Order, pc.Title, clip(pc.Body, 600)))
		}
	}

	switch part {
	case "outline":
		system = "你是一位擅长结构化的中文小说编剧。请为指定章节撰写详细章纲：本章核心事件、人物弧光、伏笔埋设与回收、与前后章的衔接。" + style
		user = fmt.Sprintf("作品标题：%s。\n整体世界观/设定：%s。\n人物设定：%s。\n全书大纲：%s。\n时间线：%s。%s\n当前要写的章节：第%d章《%s》。\n前文章节（用于衔接）：%s\n%s",
			p.Title, clip(p.Setting, 600), clip(p.Characters, 600), clip(p.Outline, 600), clip(p.Timeline, 400), loopSummary,
			ch.Order, ch.Title, priorOutline.String(), extra)
	case "body":
		system = "你是一位专业的中文小说写作助手。请基于给定设定、人物、大纲、时间线与本章章纲撰写本章正文，只输出正文，保持风格与设定一致。" + style
		user = fmt.Sprintf("作品标题：%s。\n世界观/设定：%s。\n人物设定：%s。\n时间线：%s。%s\n全书大纲：%s。\n前文章节正文摘要：%s\n本章章纲：%s\n当前章节：第%d章《%s》。\n%s",
			p.Title, clip(p.Setting, 400), clip(p.Characters, 400), clip(p.Timeline, 300), loopSummary, clip(p.Outline, 300),
			priorBody.String(), ch.Outline, ch.Order, ch.Title, extra)
	}
	return system, user
}

// ---------- 素材一键全生成（阶段流水线：设定→人物→大纲→时间线，逐字段落盘） ----------

type generateAllInput struct {
	ModelID uint   `json:"model_id"`
	Prompt  string `json:"prompt"`
}

// POST /api/author-ai/projects/:id/generate-all
// 依次生成 设定/人物/大纲/时间线 四类素材，每类生成后先落盘再进入下一项，
// 使后项能读到前项结果（与 buildGeneratePrompt 的字段注入一致）。
func GenerateAllMaterials(c *gin.Context) {
	id := atou(c.Param("id"))
	p, ok := requireOwner(c, id)
	if !ok {
		return
	}
	var in generateAllInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	steps := []string{CompSetting, CompCharacter, CompOutline, CompTimeline}
	done := []string{}
	for _, step := range steps {
		promptText, perr := generateViaTemplate(p, step, in.Prompt)
		if perr != nil {
			infra.InternalError(c, fmt.Sprintf("组装「%s」提示词失败: %s", step, perr.Error()))
			return
		}
		content, gerr := genCall(hctx.CurrentUserID(c), in.ModelID, promptText, p.ID)
		if gerr != nil {
			infra.InternalError(c, fmt.Sprintf("生成「%s」失败: %s（已完成：%s）", step, gerr.Error(), strings.Join(done, "、")))
			return
		}
		switch step {
		case CompSetting:
			p.Setting = content
		case CompCharacter:
			p.Characters = content
		case CompOutline:
			p.Outline = content
		case CompTimeline:
			p.Timeline = content
		}
		if err := models.DB.Save(p).Error; err != nil {
			infra.InternalError(c, "保存生成结果失败")
			return
		}
		done = append(done, step)
	}
	infra.SuccessMessage(c, "素材已全部生成并保存", gin.H{"done": done, "project": p})
}

// ---------- 时间线一致性校验（纯 Go，不依赖模型） ----------

// GET /api/author-ai/projects/:id/timeline-check
// 解析 [LOOP] 嵌套循环结构并做纯 Go 一致性校验（开闭平衡、ID 唯一、顺序匹配），
// 返回结构化的 loops / events / errors，供前端在提交前提示用户修正。
func TimelineCheck(c *gin.Context) {
	id := atou(c.Param("id"))
	p, ok := requireOwner(c, id)
	if !ok {
		return
	}
	res := authoraim.ParseLoops(p.Timeline)
	infra.Success(c, gin.H{
		"has_loop": res.HasLoop,
		"loops":    res.Loops,
		"events":   res.Events,
		"errors":   res.Errors,
		"ok":       len(res.Errors) == 0,
	})
}
