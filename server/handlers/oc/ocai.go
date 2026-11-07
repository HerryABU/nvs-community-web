package ochandler

// ==================== OC 角色卡 AI 生成 / 补全 接口层 ====================
// 借鉴作者工作台 AI 写作流，但数据隔离在 oc_characters；兼容 furry。

import (
	"strings"

	"nvs-server/middleware"
	ai "nvs-server/models/ai"
	"nvs-server/oc"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

// AiGenerateCharacter POST /api/oc/ai-character
// 从灵感生成一张完整 OC 角色卡（落库，归当前用户）。furry 时传入 oc_kind=species/traits 展开兽人设定。
func AiGenerateCharacter(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	if !ai.IsAiEnabled() {
		infra.BadRequest(c, "平台 AI 未开启")
		return
	}
	var in struct {
		Concept           string `json:"concept"`
		StyleDirective    string `json:"style_directive"`
		OcKind            string `json:"oc_kind"`
		Species           string `json:"species"`
		SpeciesTraits     string `json:"species_traits"`
		AnthroType        string `json:"anthro_type"`
		ExtraRequirements string `json:"extra_requirements"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if strings.TrimSpace(in.Concept) == "" {
		infra.BadRequest(c, "请填写核心概念/灵感")
		return
	}
	ch, err := oc.GenerateOCCharacter(uid, oc.OcGenSeed{
		Concept:           in.Concept,
		StyleDirective:    in.StyleDirective,
		OcKind:            in.OcKind,
		Species:           in.Species,
		SpeciesTraits:     in.SpeciesTraits,
		AnthroType:        in.AnthroType,
		ExtraRequirements: in.ExtraRequirements,
	})
	if err != nil {
		infra.InternalError(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "AI 已生成 OC 角色卡", ch)
}

// AiFillCharacter POST /api/oc/characters/:cid/ai-fill
// 在已有角色卡基础上补全/润色（落库更新）。
func AiFillCharacter(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	if !ai.IsAiEnabled() {
		infra.BadRequest(c, "平台 AI 未开启")
		return
	}
	cid := parseUint(c.Param("cid"))
	var in struct {
		Focus             string `json:"focus"`
		ExtraRequirements string `json:"extra_requirements"`
		OcKind            string `json:"oc_kind"`
		Species           string `json:"species"`
		SpeciesTraits     string `json:"species_traits"`
		AnthroType        string `json:"anthro_type"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	ch, err := oc.FillOCCharacter(uid, cid, oc.OcFillSeed{
		Focus:             in.Focus,
		ExtraRequirements: in.ExtraRequirements,
		OcKind:            in.OcKind,
		Species:           in.Species,
		SpeciesTraits:     in.SpeciesTraits,
		AnthroType:        in.AnthroType,
	})
	if err != nil {
		infra.InternalError(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "AI 已补全角色卡", ch)
}
