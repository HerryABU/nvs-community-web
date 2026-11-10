package authorai

import (
	"nvs-server/handlers/hctx"
	"nvs-server/prompttpl"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

// ==================== 文本 → 视图：格式化（推荐提示词路线） ====================
// 把作者随手写的自由文本（故事线 / 时间线 / 大纲）洗成约定结构，供前端视图模式渲染。
// 只返回结果、不落库——由作者确认后在前端保存（避免 AI 直接覆盖创作）。

const (
	storylineSchema = `目标：故事线（多条主线/支线并存）。
[LINE:线id|类型|标题] 开始一条线，类型只能是 主线 或 支线，线id 全局唯一；[/LINE] 结束。
线内每行一个事件：「序号 事件简述」。
标识符：[CROSS:另一条线id] 多线交汇；[INFLUENCE:目标线id|影响简述] 本线事件改变目标线走向（有方向）；[IMPACT:关键词] 线内影响；[A] 时间锚点；ret / returnA 回溯。`

	timelineSchema = `目标：时间线 / 大事年表。
基础事件：每行一个事件，格式「时间 事件简述」，按故事内时间顺序。
嵌套循环：[LOOP:loop_key] … [/LOOP]（loop_key 全局唯一，最多 3 层）。
时间锚点与回溯：[A] 声明锚点；ret 回到最近锚点；returnA / retA 回到名为 A 的锚点。`

	outlineSchema = `目标：大纲（分卷 / 分章层级）。
用 Markdown 层级表达：# 卷名；## 章名 — 核心事件；### 小节/节拍（可选）。
每章用一句话点明核心事件与人物弧光，保持节奏与伏笔。`
)

// FormatComponent POST /api/author-ai/projects/:id/format
func FormatComponent(c *gin.Context) {
	p, ok := requireOwner(c, atou(c.Param("id")))
	if !ok {
		return
	}
	var in struct {
		Component string `json:"component"`
		ModelID   uint   `json:"model_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	var existing, schema string
	switch in.Component {
	case CompStoryline:
		existing, schema = p.Storyline, storylineSchema
	case CompTimeline:
		existing, schema = p.Timeline, timelineSchema
	case CompOutline:
		existing, schema = p.Outline, outlineSchema
	default:
		infra.BadRequest(c, "不支持的组件")
		return
	}
	if len([]rune(existing)) < 10 {
		infra.BadRequest(c, "内容太少，无法格式化（请先写一点或 AI 生成）")
		return
	}
	t, terr := prompttpl.GetByKey("authorai-format")
	if terr != nil || t == nil {
		infra.BadRequest(c, "格式化模板未就绪")
		return
	}
	rendered := prompttpl.Render(t.Content, t.Vars, map[string]string{
		"Schema":          schema,
		"ExistingContent": existing,
	})
	content, gerr := genCall(hctx.CurrentUserID(c), in.ModelID, rendered.Text, p.ID)
	if gerr != nil {
		infra.InternalError(c, "AI 格式化失败: "+gerr.Error())
		return
	}
	infra.SuccessMessage(c, "格式化完成", gin.H{"component": in.Component, "content": content})
}
