// Package selfhosthandler 自托管项目的 HTTP 接口层：只做鉴权、参数校验与响应组装。
// 规则与数据在 server/selfhost。
package selfhosthandler

import (
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/selfhost"
	"nvs-server/utils/infra"
)

func parseUint(s string) uint {
	n, _ := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	return uint(n)
}

func isAdmin(c *gin.Context) bool {
	r := middleware.UserRole(c)
	return r == "admin" || r == "super"
}

// ─── 运行时目录（登录用户只读） ───

// GET /api/selfhost/runtimes
func ListRuntimes(c *gin.Context) {
	list, err := selfhost.ListRuntimes(true)
	if err != nil {
		infra.InternalError(c, "运行时目录读取失败")
		return
	}
	infra.Success(c, gin.H{"runtimes": list})
}

// ─── 我的项目 ───

// GET /api/selfhost/projects
func ListMyProjects(c *gin.Context) {
	list, total, err := selfhost.ListMyProjects(middleware.UserID(c), c.Query("keyword"))
	if err != nil {
		infra.InternalError(c, "项目列表读取失败")
		return
	}
	infra.Success(c, gin.H{"projects": list, "total": total})
}

// POST /api/selfhost/projects
func CreateProject(c *gin.Context) {
	var in selfhost.Project
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	p, err := selfhost.CreateProject(middleware.UserID(c), in)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "项目已创建，请上传源码并填写服务地址", p)
}

// PUT /api/selfhost/projects/:id
func UpdateProject(c *gin.Context) {
	var in selfhost.Project
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	p, err := selfhost.UpdateProject(middleware.UserID(c), parseUint(c.Param("id")), in, isAdmin(c))
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已保存", p)
}

// GET /api/selfhost/projects/:id
func GetProject(c *gin.Context) {
	id := parseUint(c.Param("id"))
	p, err := selfhost.GetProject(id)
	if err != nil {
		infra.NotFound(c, "项目不存在")
		return
	}
	if !isAdmin(c) && p.UserID != middleware.UserID(c) && p.ReviewStatus != selfhost.ReviewApproved {
		infra.Forbidden(c, "无权查看该项目")
		return
	}
	logs, _ := selfhost.ListReviewLogs(id)
	infra.Success(c, gin.H{"project": p, "logs": logs, "config": selfhost.ConfigSummary()})
}

// DELETE /api/selfhost/projects/:id
func DeleteProject(c *gin.Context) {
	if err := selfhost.DeleteProject(middleware.UserID(c), parseUint(c.Param("id")), isAdmin(c)); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "项目已删除", nil)
}

// POST /api/selfhost/projects/:id/submit
func SubmitForReview(c *gin.Context) {
	if err := selfhost.SubmitForReview(parseUint(c.Param("id")), middleware.UserID(c)); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已提交审核，通过后才能对外", nil)
}

// PUT /api/selfhost/projects/:id/visibility
func SetVisibility(c *gin.Context) {
	var in struct {
		Visibility string `json:"visibility"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if err := selfhost.SetVisibility(parseUint(c.Param("id")), middleware.UserID(c), in.Visibility, isAdmin(c)); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "可见性已更新", nil)
}

// POST /api/selfhost/projects/:id/files/:kind   kind = source | binary
func UploadFile(c *gin.Context) {
	id := parseUint(c.Param("id"))
	kind := c.Param("kind")
	p, err := selfhost.GetProjectRaw(id)
	if err != nil {
		infra.NotFound(c, "项目不存在")
		return
	}
	if !isAdmin(c) && p.UserID != middleware.UserID(c) {
		infra.Forbidden(c, "只能上传自己的项目文件")
		return
	}
	if kind != "source" && kind != "binary" {
		infra.BadRequest(c, "文件类型不合法")
		return
	}
	if kind == "source" {
		if err := selfhost.CheckSourceName(c.Query("name")); err != nil {
			infra.BadRequest(c, err.Error())
			return
		}
	} else if err := selfhost.CheckBinaryName(c.Query("name")); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	if p.ReviewStatus == selfhost.ReviewPending {
		infra.BadRequest(c, "审核中的项目不能替换文件；请先撤回或等待审核")
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		infra.BadRequest(c, "请选择文件")
		return
	}
	limit := int64(selfhost.MaxSourceBytes)
	if kind == "binary" {
		limit = selfhost.MaxBinaryBytes
	}
	if fh.Size > limit {
		infra.BadRequest(c, "文件超过大小上限（源码 50MB / 可执行文件 200MB）")
		return
	}
	if _, err := selfhost.ProjectDir(p.UserID, p.ID); err != nil {
		infra.InternalError(c, "存储目录创建失败")
		return
	}
	rel, err := selfhost.SafeRelPath(p.UserID, p.ID, c.Query("name"), kind)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	abs, err := selfhost.AbsStoredPath(rel)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	if err := c.SaveUploadedFile(fh, abs); err != nil {
		infra.InternalError(c, "文件保存失败")
		return
	}
	if err := selfhost.AttachFile(p, kind, rel, c.Query("name")); err != nil {
		infra.InternalError(c, "项目更新失败")
		return
	}
	infra.SuccessMessage(c, "文件已上传，重新提交审核后生效", gin.H{"file": rel})
}

// GET /api/selfhost/projects/:id/files/:kind  仅本人/管理员/已通过公开项目可下载
func DownloadFile(c *gin.Context) {
	id := parseUint(c.Param("id"))
	kind := c.Param("kind")
	p, err := selfhost.GetProjectRaw(id)
	if err != nil {
		infra.NotFound(c, "项目不存在")
		return
	}
	allowed := isAdmin(c) || (middleware.UserID(c) != 0 && p.UserID == middleware.UserID(c)) ||
		(p.ReviewStatus == selfhost.ReviewApproved && p.Visibility == selfhost.VisPublic)
	if !allowed {
		infra.Forbidden(c, "该项目文件尚未公开")
		return
	}
	rel := p.SourceFile
	name := p.SourceName
	if kind == "binary" {
		rel, name = p.BinaryFile, p.BinaryName
	}
	if kind != "source" && kind != "binary" {
		infra.BadRequest(c, "文件类型不合法")
		return
	}
	abs, err := selfhost.AbsStoredPath(rel)
	if err != nil {
		infra.NotFound(c, "文件不存在")
		return
	}
	if _, err := os.Stat(abs); err != nil {
		infra.NotFound(c, "文件不存在")
		return
	}
	c.FileAttachment(abs, name)
}

// ─── 广场 ───

// GET /api/selfhost/plaza  （公开）
func Plaza(c *gin.Context) {
	list, total, err := selfhost.ListPublic(c.Query("keyword"), c.Query("lang"))
	if err != nil {
		infra.InternalError(c, "广场读取失败")
		return
	}
	infra.Success(c, gin.H{"projects": list, "total": total})
}

// ─── 管理端：运行时 ───

func AdminListRuntimes(c *gin.Context) {
	list, err := selfhost.ListRuntimes(false)
	if err != nil {
		infra.InternalError(c, "运行时目录读取失败")
		return
	}
	infra.Success(c, gin.H{"runtimes": list})
}

func AdminCreateRuntime(c *gin.Context) {
	var in selfhost.Runtime
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	r, err := selfhost.CreateRuntime(in)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已添加", r)
}

func AdminUpdateRuntime(c *gin.Context) {
	var in selfhost.Runtime
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	r, err := selfhost.UpdateRuntime(parseUint(c.Param("id")), in)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已保存", r)
}

func AdminDeleteRuntime(c *gin.Context) {
	if err := selfhost.DeleteRuntime(parseUint(c.Param("id"))); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已删除", nil)
}

// ─── 管理端：项目审核 ───

func AdminListProjects(c *gin.Context) {
	var list []selfhost.Project
	q := models.DB.Model(&selfhost.Project{})
	if s := c.Query("status"); s != "" {
		q = q.Where("review_status = ?", s)
	}
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		q = q.Where("name LIKE ? OR slug LIKE ?", "%"+kw+"%", "%"+kw+"%")
	}
	var total int64
	_ = q.Count(&total)
	if err := q.Order("id DESC").Find(&list).Error; err != nil {
		infra.InternalError(c, "项目列表读取失败")
		return
	}
	infra.Success(c, gin.H{"projects": list, "total": total})
}

type reviewInput struct {
	Approve     bool   `json:"approve"`
	Note        string `json:"note"`
	WaiveSource bool   `json:"waive_source"`
}

func AdminReview(c *gin.Context) {
	var in reviewInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if err := selfhost.ReviewProject(parseUint(c.Param("id")), middleware.UserID(c), in.Approve, in.Note, in.WaiveSource); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "审核已处理", nil)
}

type suspendInput struct {
	Suspend bool   `json:"suspend"`
	Note    string `json:"note"`
}

func AdminSuspend(c *gin.Context) {
	var in suspendInput
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if err := selfhost.SuspendProject(parseUint(c.Param("id")), middleware.UserID(c), in.Suspend, in.Note); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已更新项目状态", nil)
}

// ─── 管理端：反向代理 ───

func AdminListRoutes(c *gin.Context) {
	list, err := selfhost.ListRoutes(c.Query("keyword"))
	if err != nil {
		infra.InternalError(c, "路由列表读取失败")
		return
	}
	total, active, _ := selfhost.RouteCount()
	infra.Success(c, gin.H{"routes": list, "total": total, "active": active, "base": ""})
}

func AdminSaveRoute(c *gin.Context) {
	var in selfhost.Route
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	r, err := selfhost.UpsertRoute(in)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "路由已保存并热加载", r)
}

func AdminToggleRoute(c *gin.Context) {
	var in struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	r, err := selfhost.SetRouteEnabled(parseUint(c.Param("id")), in.Enabled)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "路由状态已更新", r)
}

func AdminDeleteRoute(c *gin.Context) {
	if err := selfhost.DeleteRoute(parseUint(c.Param("id"))); err != nil {
		infra.InternalError(c, "删除失败")
		return
	}
	infra.SuccessMessage(c, "路由已删除", nil)
}

func AdminConfig(c *gin.Context) {
	infra.Success(c, selfhost.ConfigSummary())
}

func AdminSaveConfig(c *gin.Context) {
	var in struct {
		SourceRequired *bool `json:"source_required"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if in.SourceRequired == nil {
		infra.BadRequest(c, "缺少 source_required")
		return
	}
	if err := selfhost.SaveSourceRequiredConfig(*in.SourceRequired); err != nil {
		infra.InternalError(c, "配置保存失败")
		return
	}
	infra.SuccessMessage(c, "配置已保存", selfhost.ConfigSummary())
}
