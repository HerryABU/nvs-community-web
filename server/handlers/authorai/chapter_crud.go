package authorai

import (
	"fmt"
	"strings"

	"nvs-server/models"
	authoraim "nvs-server/models/authorai"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

// chapterInput 章节写操作的请求体（字段均可选，仅更新传入项）。
type chapterInput struct {
	Title   *string `json:"title"`
	Outline *string `json:"outline"`
	Body    *string `json:"body"`
	Order   *int    `json:"order"`
	Status  *string `json:"status"`
}

// GET /api/author-ai/projects/:id/chapters
// 列出工程的全部章节（按 order 升序），用于章节页渲染与逐章生成编排。
func ListChapters(c *gin.Context) {
	id := atou(c.Param("id"))
	p, ok := requireOwner(c, id)
	if !ok {
		return
	}
	var list []authoraim.Chapter
	if err := models.DB.Where("project_id = ?", p.ID).Order("`order` ASC, id ASC").Find(&list).Error; err != nil {
		infra.InternalError(c, "获取章节列表失败")
		return
	}
	infra.Success(c, list)
}

// POST /api/author-ai/projects/:id/chapters
// 新建章节：默认 order 为「现有最大 order + 1」，标题留空时按序号生成。
func CreateChapter(c *gin.Context) {
	id := atou(c.Param("id"))
	p, ok := requireOwner(c, id)
	if !ok {
		return
	}
	var in chapterInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	ch := &authoraim.Chapter{ProjectID: p.ID}
	var maxOrder int
	models.DB.Model(&authoraim.Chapter{}).Where("project_id = ?", p.ID).Select("COALESCE(MAX(`order`),0)").Scan(&maxOrder)
	ch.Order = maxOrder + 1
	if in.Order != nil {
		ch.Order = *in.Order
	}
	ch.Title = strings.TrimSpace(orNil(in.Title))
	if ch.Title == "" {
		ch.Title = fmt.Sprintf("第 %d 章", ch.Order)
	}
	if in.Outline != nil {
		ch.Outline = *in.Outline
	}
	if in.Body != nil {
		ch.Body = *in.Body
	}
	if in.Status != nil {
		ch.Status = *in.Status
	}
	if err := models.DB.Create(ch).Error; err != nil {
		infra.InternalError(c, "创建章节失败")
		return
	}
	infra.SuccessMessage(c, "已创建章节", ch)
}

// PUT /api/author-ai/projects/:id/chapters/:cid
// 更新章节字段（标题/章纲/正文/顺序/状态），仅覆盖传入项。
func UpdateChapter(c *gin.Context) {
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
	var in chapterInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if t != "" {
			ch.Title = t
		}
	}
	if in.Outline != nil {
		ch.Outline = *in.Outline
	}
	if in.Body != nil {
		ch.Body = *in.Body
	}
	if in.Order != nil {
		ch.Order = *in.Order
	}
	if in.Status != nil {
		ch.Status = *in.Status
	}
	if err := models.DB.Save(&ch).Error; err != nil {
		infra.InternalError(c, "保存章节失败")
		return
	}
	infra.SuccessMessage(c, "已保存章节", ch)
}

// DELETE /api/author-ai/projects/:id/chapters/:cid
func DeleteChapter(c *gin.Context) {
	id := atou(c.Param("id"))
	p, ok := requireOwner(c, id)
	if !ok {
		return
	}
	cid := atou(c.Param("cid"))
	if err := models.DB.Where("project_id = ? AND id = ?", p.ID, cid).Delete(&authoraim.Chapter{}).Error; err != nil {
		infra.InternalError(c, "删除章节失败")
		return
	}
	infra.SuccessMessage(c, "已删除章节", nil)
}

// orNil 辅助：*string → 字符串
func orNil(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
