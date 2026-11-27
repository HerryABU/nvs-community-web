// Package selfhosthandler 自托管项目的 HTTP 接口层：只做鉴权、参数校验与响应组装。
// 规则与数据在 server/selfhost。
package selfhosthandler

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"nvs-server/middleware"
	"nvs-server/models"
	softdel "nvs-server/models/softdel"
	"nvs-server/selfhost"
	"nvs-server/selfhost/isolation/probe"
	"nvs-server/selfhost/target"
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

// projectView 在前端需要的权限位；嵌套结构体在 JSON 里会展开成同层字段。
type projectView struct {
	selfhost.Project
	CanSource    bool  `json:"can_source"`
	CanBinary    bool  `json:"can_binary"`
	StorageBytes int64 `json:"storage_bytes"`
}

// decorate 给列表补权限位。共享开关注入到 Project 本身，逐用户授权从库里批量取。
func decorate(c *gin.Context, list []selfhost.Project) []projectView {
	out := make([]projectView, 0, len(list))
	ids := make([]uint, 0, len(list))
	for _, p := range list {
		ids = append(ids, p.ID)
	}
	grants := selfhost.ApprovedGrantKinds(middleware.UserID(c), ids)
	admin := isAdmin(c)
	viewer := middleware.UserID(c)
	for _, p := range list {
		canSource, canBinary := selfhost.PermissionsFor(&p, viewer, admin)
		if g := grants[p.ID]; g != nil {
			if g["source"] || g["all"] {
				canSource = true
			}
			if g["binary"] || g["all"] {
				canBinary = true
			}
		}
		out = append(out, projectView{Project: p, CanSource: canSource, CanBinary: canBinary, StorageBytes: selfhost.StorageBytes(&p)})
	}
	return out
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
	infra.Success(c, gin.H{"projects": decorate(c, list), "total": total})
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
	viewer := middleware.UserID(c)
	canSource, canBinary := selfhost.PermissionsFor(p, viewer, isAdmin(c))
	data := gin.H{
		"project": p, "logs": logs, "config": selfhost.ConfigSummary(),
		"can_source": canSource, "can_binary": canBinary,
		"usage":     selfhost.ProjectUsage(id),
		"my_grants": selfhost.GrantStatusFor(id, viewer),
	}
	if isAdmin(c) || viewer == p.UserID {
		grants, _ := selfhost.ListProjectGrants(id)
		data["grants"] = grants
	}
	infra.Success(c, data)
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

// GET /api/selfhost/projects/:id/files/:kind
// 默认拒绝：只有所有者/管理员，或显式共享/逐用户获批才能下载。
func DownloadFile(c *gin.Context) {
	id := parseUint(c.Param("id"))
	kind := c.Param("kind")
	if kind != "source" && kind != "binary" {
		infra.BadRequest(c, "文件类型不合法")
		return
	}
	p, err := selfhost.GetProjectRaw(id)
	if err != nil {
		infra.NotFound(c, "项目不存在")
		return
	}
	if !selfhost.CanDownload(p, middleware.UserID(c), isAdmin(c), kind) {
		infra.Forbidden(c, "无权下载该项目的文件；请先向所有者申请授权")
		return
	}
	rel, name := p.SourceFile, p.SourceName
	if kind == "binary" {
		rel, name = p.BinaryFile, p.BinaryName
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
	infra.Success(c, gin.H{"projects": decorate(c, list), "total": total})
}

// ─── 文件访问授权 ───

// POST /api/selfhost/projects/:id/access  {kind, note}
func RequestAccess(c *gin.Context) {
	var in struct {
		Kind string `json:"kind"`
		Note string `json:"note"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	g, err := selfhost.RequestGrant(parseUint(c.Param("id")), middleware.UserID(c), in.Kind, in.Note)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "申请已提交，等待项目所有者审批", g)
}

// GET /api/selfhost/access-requests  我的申请
func MyGrants(c *gin.Context) {
	list, err := selfhost.ListMyGrants(middleware.UserID(c))
	if err != nil {
		infra.InternalError(c, "申请列表读取失败")
		return
	}
	infra.Success(c, gin.H{"grants": list})
}

// POST /api/selfhost/grants/:id/review  {approve}
func ReviewAccess(c *gin.Context) {
	var in struct {
		Approve bool `json:"approve"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if err := selfhost.ReviewGrant(parseUint(c.Param("id")), middleware.UserID(c), in.Approve); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已处理", nil)
}

// DELETE /api/selfhost/grants/:id
func RevokeAccess(c *gin.Context) {
	if err := selfhost.RevokeGrant(parseUint(c.Param("id")), middleware.UserID(c)); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "授权已撤销", nil)
}

// PUT /api/selfhost/projects/:id/sharing  {source_shared, binary_shared}
func SetSharing(c *gin.Context) {
	var in struct {
		SourceShared bool `json:"source_shared"`
		BinaryShared bool `json:"binary_shared"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if err := selfhost.SetSharing(parseUint(c.Param("id")), middleware.UserID(c), in.SourceShared, in.BinaryShared, isAdmin(c)); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "共享设置已保存", nil)
}

// PlazaAll 统一广场：自托管项目 + 已公开扩展应用（扩展应用归并入口）。
func PlazaAll(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	kind := c.Query("type") // all | selfhost | html

	entries := make([]gin.H, 0)
	if kind == "" || kind == "all" || kind == "selfhost" {
		projects, _, err := selfhost.ListPublic(keyword, "")
		if err == nil {
			for _, v := range decorate(c, projects) {
				url := ""
				if v.Route != nil && v.Route.Enabled {
					url = v.Route.Prefix + "/"
				}
				version := ""
				if v.Runtime != nil {
					version = v.Runtime.Version
				}
				author := ""
				if v.User != nil {
					author = v.User.Username
				}
				entries = append(entries, gin.H{
					"type": "selfhost", "id": v.ID, "title": v.Name,
					"description": v.Description, "language": v.Language,
					"version": version, "author": author, "url": url,
					"created_at": v.CreatedAt, "updated_at": v.UpdatedAt,
					"can_source": v.CanSource, "can_binary": v.CanBinary,
					"source_name": v.SourceName, "binary_name": v.BinaryName,
				})
			}
		}
	}
	if kind == "" || kind == "all" || kind == "html" {
		var htmls []softdel.UserHTML
		q := models.DB.Model(&softdel.UserHTML{}).Where("is_public = ? AND is_active = ?", true, true)
		if keyword != "" {
			q = q.Where("name LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
		}
		q.Preload("User").Order("updated_at DESC").Limit(100).Find(&htmls)
		for _, h := range htmls {
			author := ""
			if h.User != nil {
				author = h.User.Username
			}
			entries = append(entries, gin.H{
				"type": "html", "id": h.ID, "title": h.Name,
				"description": h.Description, "language": "HTML", "version": "ZIP",
				"author": author, "url": "/app/" + strconv.FormatUint(uint64(h.ID), 10),
				"created_at": h.CreatedAt, "updated_at": h.UpdatedAt,
				"can_source": false, "can_binary": false,
			})
		}
	}
	infra.Success(c, gin.H{"entries": entries, "total": len(entries)})
}

// ─── 运行时申请 ───

func CreateRuntimeRequest(c *gin.Context) {
	var in selfhost.RuntimeRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	r, err := selfhost.CreateRuntimeRequest(middleware.UserID(c), in)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已提交给管理员", r)
}

func MyRuntimeRequests(c *gin.Context) {
	list, err := selfhost.ListMyRuntimeRequests(middleware.UserID(c))
	if err != nil {
		infra.InternalError(c, "申请列表读取失败")
		return
	}
	infra.Success(c, gin.H{"requests": list})
}

func AdminListRuntimeRequests(c *gin.Context) {
	list, err := selfhost.ListRuntimeRequests(c.Query("status"))
	if err != nil {
		infra.InternalError(c, "申请列表读取失败")
		return
	}
	infra.Success(c, gin.H{"requests": list})
}

func AdminReviewRuntimeRequest(c *gin.Context) {
	var in struct {
		Approve  bool   `json:"approve"`
		Language string `json:"language"`
		Version  string `json:"version"`
		ExecPath string `json:"exec_path"`
		Note     string `json:"note"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	r, err := selfhost.ReviewRuntimeRequest(parseUint(c.Param("id")), middleware.UserID(c), in.Approve, in.Language, in.Version, in.ExecPath, in.Note)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已处理", r)
}

func AdminScanRuntimes(c *gin.Context) {
	infra.Success(c, gin.H{"candidates": selfhost.ScanPathRuntimes()})
}

// ─── 管理端：三种代理类型的隔离服务 ───

func AdminListIsoServices(c *gin.Context) {
	list, err := selfhost.ListIsoServices()
	if err != nil {
		infra.InternalError(c, "隔离服务读取失败")
		return
	}
	e := selfhost.IsoEngine()
	tbl := e.Table()
	warnings := e.Warnings()
	if warnings == nil {
		warnings = []string{}
	}
	infra.Success(c, gin.H{
		"services":   list,
		"snapshot":   e.Snapshot(), // GWh 同款：配置 + 熔断 + 指标
		"enabled":    e.Active(),
		"count":      e.Count(),
		"source":     tbl.Source,
		"hash":       tbl.Hash,
		"loaded_at":  tbl.LoadedAt,
		"warnings":   warnings,
		"audit_file": e.AuditPath(),
	})
}

// GET /api/admin/selfhost/iso-metrics  隔离链路指标（GWh 同款）
func AdminIsoMetrics(c *gin.Context) {
	infra.Success(c, gin.H{"metrics": selfhost.IsoEngine().Metrics()})
}

// GET /api/admin/selfhost/iso-audit  最近审计记录（GWh 同款，默认 200 条）
func AdminIsoAudit(c *gin.Context) {
	n := 200
	if v := c.Query("limit"); v != "" {
		if x, err := strconv.Atoi(v); err == nil && x > 0 && x <= 2000 {
			n = x
		}
	}
	infra.Success(c, gin.H{"entries": selfhost.IsoEngine().AuditRecent(n)})
}

// POST /api/admin/selfhost/iso-probe  智能识别后端该用哪种去头模式。
func AdminProbeIsoMode(c *gin.Context) {
	var in struct {
		Target  string `json:"target" binding:"required"`
		Service string `json:"service"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "请填写目标地址")
		return
	}
	if _, err := target.ParseBackend(in.Target); err != nil {
		infra.BadRequest(c, "目标地址必须是 http/https URL")
		return
	}
	res, err := probe.Detect(in.Target, in.Service, 5*time.Second)
	if err != nil {
		infra.BadRequest(c, "探测失败："+err.Error())
		return
	}
	infra.Success(c, res)
}

func AdminSaveIsoService(c *gin.Context) {
	var in selfhost.IsoService
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	s, err := selfhost.SaveIsoService(in)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已保存并热加载", s)
}

func AdminToggleIsoService(c *gin.Context) {
	var in struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if err := selfhost.ToggleIsoService(parseUint(c.Param("id")), in.Enabled); err != nil {
		infra.InternalError(c, "操作失败")
		return
	}
	infra.SuccessMessage(c, "状态已更新", nil)
}

func AdminDeleteIsoService(c *gin.Context) {
	if err := selfhost.DeleteIsoService(parseUint(c.Param("id"))); err != nil {
		infra.InternalError(c, "删除失败")
		return
	}
	infra.SuccessMessage(c, "已删除", nil)
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
	infra.Success(c, gin.H{"projects": decorate(c, list), "total": total})
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

// GET /api/admin/selfhost/isolation  空间隔离自检
func AdminIsolation(c *gin.Context) {
	infra.Success(c, gin.H{"issues": selfhost.CheckIsolation()})
}

// GET /api/admin/selfhost/usage  全站项目流量与体积
func AdminUsage(c *gin.Context) {
	list, err := selfhost.ListUsage()
	if err != nil {
		infra.InternalError(c, "用量统计失败")
		return
	}
	infra.Success(c, gin.H{"usage": list})
}

// ─── 管理端：TCP 端口映射 ───

func AdminListPortMaps(c *gin.Context) {
	list, err := selfhost.ListPortMaps()
	if err != nil {
		infra.InternalError(c, "端口映射读取失败")
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, p := range list {
		out = append(out, gin.H{
			"id": p.ID, "listen_port": p.ListenPort, "target_host": p.TargetHost,
			"target_port": p.TargetPort, "enabled": p.Enabled, "description": p.Description,
			"last_error": p.LastError, "running": selfhost.PortMapRunning(p.ID),
			"created_at": p.CreatedAt, "updated_at": p.UpdatedAt,
		})
	}
	infra.Success(c, gin.H{"maps": out})
}

func AdminSavePortMap(c *gin.Context) {
	var in selfhost.PortMap
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	p, err := selfhost.SavePortMap(in)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "映射已保存并生效", p)
}

func AdminTogglePortMap(c *gin.Context) {
	var in struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if err := selfhost.TogglePortMap(parseUint(c.Param("id")), in.Enabled); err != nil {
		infra.InternalError(c, "操作失败")
		return
	}
	infra.SuccessMessage(c, "状态已更新", nil)
}

func AdminDeletePortMap(c *gin.Context) {
	if err := selfhost.DeletePortMap(parseUint(c.Param("id"))); err != nil {
		infra.InternalError(c, "删除失败")
		return
	}
	infra.SuccessMessage(c, "映射已删除", nil)
}

// ─── 管理端：端口直派 ───

func AdminListDirectRoutes(c *gin.Context) {
	list, err := selfhost.ListDirectRoutes()
	if err != nil {
		infra.InternalError(c, "直派路由读取失败")
		return
	}
	infra.Success(c, gin.H{"routes": list, "base": ""})
}

func AdminSaveDirectRoute(c *gin.Context) {
	var in selfhost.DirectRoute
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	r, err := selfhost.SaveDirectRoute(in)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已指派并热加载", r)
}

func AdminToggleDirectRoute(c *gin.Context) {
	var in struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if err := selfhost.ToggleDirectRoute(parseUint(c.Param("id")), in.Enabled); err != nil {
		infra.InternalError(c, "操作失败")
		return
	}
	infra.SuccessMessage(c, "状态已更新", nil)
}

func AdminDeleteDirectRoute(c *gin.Context) {
	if err := selfhost.DeleteDirectRoute(parseUint(c.Param("id"))); err != nil {
		infra.InternalError(c, "删除失败")
		return
	}
	infra.SuccessMessage(c, "已删除", nil)
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
