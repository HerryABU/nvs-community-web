package ochandler

// ==================== OC 模式：接口层 ====================
// 只做参数校验、权限判定与响应组装；规则与数据在 server/oc。
// 权限：群主/管理员管配置与 AI 角色；成员可建角色卡/世界书、参与对话流；
// 一键发布限群主/管理员/流发起人（受未成年与限额约束）。

import (
	"strconv"

	"nvs-server/middleware"
	"nvs-server/oc"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

func isAdmin(c *gin.Context) bool {
	r := middleware.UserRole(c)
	return r == "admin" || r == "police"
}

// loadGroupForMember 群可读校验（复用群组包）
func groupCanManage(c *gin.Context, groupID uint) bool {
	return oc.CanManageGroup(groupID, middleware.UserID(c), isAdmin(c))
}

// ─── 群配置 ───

// GetSetting GET /api/groups/:id/oc
func GetSetting(c *gin.Context) {
	gid := parseUint(c.Param("id"))
	if gid == 0 {
		utils.BadRequest(c, "群组 ID 无效")
		return
	}
	set := oc.GetGroupSetting(gid)
	agents, _ := oc.ListAgents(gid)
	utils.Success(c, gin.H{
		"setting":   set,
		"agents":    agents,
		"can_manage": groupCanManage(c, gid),
		"daily_used": oc.DailyUsed(gid),
	})
}

// SaveSetting PUT /api/groups/:id/oc
func SaveSetting(c *gin.Context) {
	gid := parseUint(c.Param("id"))
	if !groupCanManage(c, gid) {
		utils.Forbidden(c, "仅群主/管理员可配置 OC 模式")
		return
	}
	var in oc.GroupSetting
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	// 红队模式必须显式授权说明（隔离 + 授权 + 留痕）
	if in.AuditLevel == oc.AuditRedTeam && in.RedTeamEnabled && len([]rune(in.RedTeamNote)) < 4 {
		utils.BadRequest(c, "开启红队模式需填写授权说明（范围/责任人）")
		return
	}
	set, err := oc.SaveGroupSetting(gid, in)
	if err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.SuccessMessage(c, "已保存", set)
}

// ─── 角色卡 ───

// ListCharacters GET /api/oc/characters?scope=mine|public|usable
func ListCharacters(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	ps, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := oc.ListCharacters(middleware.UserID(c), c.Query("scope"), c.Query("keyword"), page, ps)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list, "total": total})
}

// GetCharacter GET /api/oc/characters/:cid
func GetCharacter(c *gin.Context) {
	id := parseUint(c.Param("cid"))
	ch, err := oc.GetCharacter(id, middleware.UserID(c), isAdmin(c))
	if err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.Success(c, ch)
}

// CreateCharacter POST /api/oc/characters
func CreateCharacter(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	var in oc.CharacterInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	ch, err := oc.CreateCharacter(uid, in)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已创建角色卡", ch)
}

// UpdateCharacter PUT /api/oc/characters/:cid
func UpdateCharacter(c *gin.Context) {
	uid := middleware.UserID(c)
	id := parseUint(c.Param("cid"))
	var in oc.CharacterInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	ch, err := oc.UpdateCharacter(uid, id, in)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已保存", ch)
}

// DeleteCharacter DELETE /api/oc/characters/:cid —— 软删除留证
func DeleteCharacter(c *gin.Context) {
	if err := oc.DeleteCharacter(middleware.UserID(c), parseUint(c.Param("cid"))); err != nil {
		utils.InternalError(c, "删除失败")
		return
	}
	utils.SuccessMessage(c, "已删除（保留记录以备追溯）", nil)
}

// ─── 世界书 ───

// ListBooks GET /api/oc/books?scope=mine|public|group&group_id=
func ListBooks(c *gin.Context) {
	gid := parseUint(c.Query("group_id"))
	list, err := oc.ListWorldBooks(middleware.UserID(c), c.Query("scope"), gid)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list})
}

// GetBook GET /api/oc/books/:bid
func GetBook(c *gin.Context) {
	b, err := oc.GetWorldBook(parseUint(c.Param("bid")), middleware.UserID(c), isAdmin(c))
	if err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.Success(c, b)
}

// CreateBook POST /api/oc/books
func CreateBook(c *gin.Context) {
	var in struct {
		Title, Summary, Visibility string
		GroupID                    uint
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	b, err := oc.CreateWorldBook(middleware.UserID(c), in.GroupID, in.Title, in.Summary, in.Visibility)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已创建世界书", b)
}

// UpdateBook PUT /api/oc/books/:bid
func UpdateBook(c *gin.Context) {
	var in struct{ Title, Summary, Visibility string }
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	b, err := oc.UpdateWorldBook(middleware.UserID(c), parseUint(c.Param("bid")), in.Title, in.Summary, in.Visibility)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已保存", b)
}

// DeleteBook DELETE /api/oc/books/:bid
func DeleteBook(c *gin.Context) {
	if err := oc.DeleteWorldBook(middleware.UserID(c), parseUint(c.Param("bid"))); err != nil {
		utils.InternalError(c, "删除失败")
		return
	}
	utils.SuccessMessage(c, "已删除（保留记录以备追溯）", nil)
}

// SaveEntry POST /api/oc/books/:bid/entries
func SaveEntry(c *gin.Context) {
	var in struct {
		ID            uint
		Kind          string
		Key           string
		Content       string
		SecondaryKeys string
		Selective     bool
		Logic         int
		Constant      bool
		Enabled       *bool
		Probability   int
		CaseSensitive bool
		WholeWord     bool
		InsertionOrder int
		Priority      int
		Position      int
		Depth         int
		Group         string
		SortOrder     int
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	e, err := oc.SaveEntryFull(middleware.UserID(c), parseUint(c.Param("bid")), in.ID, oc.EntryInput{
		Kind: in.Kind, Key: in.Key, Content: in.Content,
		SecondaryKeys: in.SecondaryKeys, Selective: in.Selective, Logic: in.Logic,
		Constant: in.Constant, Enabled: in.Enabled, Probability: in.Probability,
		CaseSensitive: in.CaseSensitive, WholeWord: in.WholeWord,
		InsertionOrder: in.InsertionOrder, Priority: in.Priority,
		Position: in.Position, Depth: in.Depth, Group: in.Group, SortOrder: in.SortOrder,
	})
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已保存条目", e)
}

// DeleteEntry DELETE /api/oc/books/:bid/entries/:eid
func DeleteEntry(c *gin.Context) {
	if err := oc.DeleteEntry(middleware.UserID(c), parseUint(c.Param("bid")), parseUint(c.Param("eid"))); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已删除条目", nil)
}

// ─── 群内 AI 角色 ───

// AddAgent POST /api/groups/:id/oc/agents
func AddAgent(c *gin.Context) {
	gid := parseUint(c.Param("id"))
	if !groupCanManage(c, gid) {
		utils.Forbidden(c, "仅群主/管理员可配置 AI 角色")
		return
	}
	var in struct {
		CharacterID uint `json:"character_id"`
		ModelID     uint `json:"model_id"`
		TemplateID  uint `json:"template_id"`
		Strategy    string `json:"strategy"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	a, err := oc.AddAgent(gid, in.CharacterID, in.ModelID, in.TemplateID, in.Strategy)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已加入群内 AI 角色", a)
}

// UpdateAgent PUT /api/groups/:id/oc/agents/:aid
func UpdateAgent(c *gin.Context) {
	gid := parseUint(c.Param("id"))
	if !groupCanManage(c, gid) {
		utils.Forbidden(c, "仅群主/管理员可配置 AI 角色")
		return
	}
	var in map[string]interface{}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := oc.UpdateAgent(gid, parseUint(c.Param("aid")), in); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已保存", nil)
}

// RemoveAgent DELETE /api/groups/:id/oc/agents/:aid
func RemoveAgent(c *gin.Context) {
	gid := parseUint(c.Param("id"))
	if !groupCanManage(c, gid) {
		utils.Forbidden(c, "仅群主/管理员可配置 AI 角色")
		return
	}
	if err := oc.RemoveAgent(gid, parseUint(c.Param("aid"))); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已移除", nil)
}

// ─── 对话流 ───

// ListFlows GET /api/groups/:id/oc/flows
func ListFlows(c *gin.Context) {
	list, err := oc.ListFlows(parseUint(c.Param("id")), 20)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list})
}

// CreateFlow POST /api/groups/:id/oc/flows
func CreateFlow(c *gin.Context) {
	gid := parseUint(c.Param("id"))
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	var in struct {
		Title    string `json:"title"`
		Schedule string `json:"schedule"`
		BookID   uint   `json:"book_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	f, err := oc.CreateFlow(gid, uid, in.BookID, in.Title, in.Schedule)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "对话流已开启", f)
}

// ListTurns GET /api/oc/flows/:fid/turns?after_seq=&limit=
func ListTurns(c *gin.Context) {
	after, _ := strconv.Atoi(c.Query("after_seq"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	list, err := oc.ListTurns(parseUint(c.Param("fid")), after, limit)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list})
}

// StepFlow POST /api/oc/flows/:fid/step —— 推进一轮（单步）
func StepFlow(c *gin.Context) {
	fid := parseUint(c.Param("fid"))
	f, err := oc.GetFlow(fid)
	if err != nil {
		utils.NotFound(c, "对话流不存在")
		return
	}
	if !oc.CanOperateFlow(f, middleware.UserID(c), isAdmin(c)) {
		utils.Forbidden(c, "仅群主/管理员或流的发起人可推进")
		return
	}
	res, serr := oc.Step(fid, middleware.UserID(c))
	if serr != nil {
		utils.BadRequest(c, serr.Error())
		return
	}
	utils.Success(c, res)
}

// SetFlowStatus POST /api/oc/flows/:fid/status { status, reason }
func SetFlowStatus(c *gin.Context) {
	fid := parseUint(c.Param("fid"))
	f, err := oc.GetFlow(fid)
	if err != nil {
		utils.NotFound(c, "对话流不存在")
		return
	}
	if !oc.CanOperateFlow(f, middleware.UserID(c), isAdmin(c)) {
		utils.Forbidden(c, "无权操作该对话流")
		return
	}
	var in struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := oc.SetFlowStatus(fid, in.Status, in.Reason); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已更新", nil)
}

// HumanTurn POST /api/oc/flows/:fid/turns { character_id, content } —— 真人以角色身份发言
func HumanTurn(c *gin.Context) {
	fid := parseUint(c.Param("fid"))
	f, err := oc.GetFlow(fid)
	if err != nil {
		utils.NotFound(c, "对话流不存在")
		return
	}
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	var in struct {
		CharacterID uint   `json:"character_id"`
		Content     string `json:"content"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	// 只能用自己的角色发言
	if in.CharacterID > 0 {
		if ch, cerr := oc.GetCharacter(in.CharacterID, uid, isAdmin(c)); cerr != nil || ch.UserID != uid {
			utils.Forbidden(c, "只能以本人角色身份发言")
			return
		}
	}
	t, err := oc.AddHumanTurn(f.ID, uid, in.CharacterID, in.Content)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, t)
}

// AuditFlow POST /api/oc/flows/:fid/audit { to_seq } —— 手动结算一个审核窗口
func AuditFlow(c *gin.Context) {
	fid := parseUint(c.Param("fid"))
	f, err := oc.GetFlow(fid)
	if err != nil {
		utils.NotFound(c, "对话流不存在")
		return
	}
	if !oc.CanOperateFlow(f, middleware.UserID(c), isAdmin(c)) {
		utils.Forbidden(c, "无权操作该对话流")
		return
	}
	var in struct {
		ToSeq int `json:"to_seq"`
	}
	_ = c.ShouldBindJSON(&in)
	w, err := oc.AuditWindow(fid, in.ToSeq)
	if err != nil {
		utils.InternalError(c, "审核失败")
		return
	}
	utils.Success(c, gin.H{"window": w})
}

// Windows GET /api/oc/flows/:fid/windows —— 审核窗口记录（可导出报告）
func Windows(c *gin.Context) {
	list, err := oc.ListWindows(parseUint(c.Param("fid")), 50)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list})
}

// Report GET /api/oc/flows/:fid/report —— 红队/审计报告
func Report(c *gin.Context) {
	rep, err := oc.BuildReport(parseUint(c.Param("fid")))
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, rep)
}

// ─── 一键发布 ───

// Publish POST /api/oc/flows/:fid/publish
func Publish(c *gin.Context) {
	fid := parseUint(c.Param("fid"))
	f, err := oc.GetFlow(fid)
	if err != nil {
		utils.NotFound(c, "对话流不存在")
		return
	}
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	if !oc.CanOperateFlow(f, uid, isAdmin(c)) {
		utils.Forbidden(c, "仅群主/管理员或流的发起人可发布")
		return
	}
	var in struct {
		NovelID         uint   `json:"novel_id"`
		Title           string `json:"title"`
		Category        string `json:"category"`
		FromSeq         int    `json:"from_seq"`
		ToSeq           int    `json:"to_seq"`
		Style           string `json:"style"`
		IncludeSettings bool   `json:"include_settings"`
		Polish          bool   `json:"polish"`
		PublishNow      bool   `json:"publish_now"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	res, perr := oc.PublishFlowToNovel(oc.PublishInput{
		FlowID: fid, NovelID: in.NovelID, Title: in.Title, Category: in.Category,
		UserID: uid, FromSeq: in.FromSeq, ToSeq: in.ToSeq, Style: in.Style,
		IncludeSettings: in.IncludeSettings, Polish: in.Polish, PublishNow: in.PublishNow,
	})
	if perr != nil {
		utils.BadRequest(c, perr.Error())
		return
	}
	utils.SuccessMessage(c, "已发布为作品", res)
}

// ─── 工具 ───

func parseUint(s string) uint {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return uint(v)
}
