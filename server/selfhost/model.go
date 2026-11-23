// Package selfhost 实现「自托管项目」平台能力：运行时目录、源码/编译产物托管、
// 提交审核、以及审核通过后的反向代理路由。
//
// 安全边界（本轮）：
//   - 平台**不在本机执行用户代码**；作者自带服务，平台只做文件托管、审核与转发。
//   - 本机执行（LocalExec）在模型里预留开关，但当前恒为 false、接口不接受置真。
//   - 代理路由只有 project.review_status=approved 且 route.enabled=true 才生效。
//
// 反代核心改写/跳数守卫/目标解析移植自同作者、同许可证（AGPL-3.0）的 gatewayhub v1.6.0。
package selfhost

import (
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"nvs-server/config"
	"nvs-server/models"
)

// ─── 常量 ───

const (
	ModeSource   = "source"   // 源码模式：只提交源码，作者自行编译运行
	ModeCompiled = "compiled" // 编译版模式：源码 + 可执行文件
)

const (
	ReviewDraft     = "draft"
	ReviewPending   = "pending"
	ReviewApproved  = "approved"
	ReviewRejected  = "rejected"
	ReviewSuspended = "suspended"
)

const (
	VisPrivate = "private"
	VisPublic  = "public"
)

const (
	RouteStopped = "stopped"
	RouteRunning = "running"
	RouteError   = "error"
)

// PlatformConfigKey 站长开关：编译型语言是否必须附源码。
// value 为 "1" 表示强制（默认），"0" 表示可豁免；逐项目豁免需管理员操作。
const SourceRequiredConfigKey = "selfhost_source_required"

// MaxSourceBytes / MaxBinaryBytes 单项目文件上限。
const (
	MaxSourceBytes = 50 << 20
	MaxBinaryBytes = 200 << 20
)

var (
	slugRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,63}$`)
	prefixRe   = regexp.MustCompile(`^/[a-z0-9][a-z0-9_/-]{0,127}$`)
	langRe     = regexp.MustCompile(`^[a-z0-9+#.-]{1,32}$`)
	versionRe  = regexp.MustCompile(`^[A-Za-z0-9._+-]{1,32}$`)
	trustedExt = map[string]bool{
		".zip": true, ".tar": true, ".gz": true, ".tgz": true, ".tar.gz": true,
	}
	binaryExt = map[string]bool{
		".jar": true, ".war": true, ".exe": true, ".bin": true, ".app": true,
		".dll": true, ".so": true, ".pyz": true, ".class": true, ".wasm": true,
	}
)

// ─── 模型 ───

// Runtime 语言/运行时版本目录（站长维护，平台不负责安装）。
type Runtime struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Language    string    `gorm:"size:32;not null;index" json:"language"` // java / go / node / python / dotnet ...
	Version     string    `gorm:"size:32;not null" json:"version"`        // 21 / 1.26 / 22 ...
	Label       string    `gorm:"size:64;not null;default:''" json:"label"`
	Kind        string    `gorm:"size:16;not null;default:interpreted" json:"kind"` // compiled / interpreted
	InstallHint string    `gorm:"type:text" json:"install_hint"`
	DownloadURL string    `gorm:"size:512;default:''" json:"download_url"`
	Checksum    string    `gorm:"size:128;default:''" json:"checksum"`
	ExecPath    string    `gorm:"size:512;default:''" json:"exec_path"` // 管理员登记的可执行文件位置（扫描所得或手动填写）
	Enabled     bool      `gorm:"default:true;index" json:"enabled"`
	Sort        int       `gorm:"default:0" json:"sort"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Runtime) TableName() string { return "selfhost_runtimes" }

// Project 自托管项目。source_file / binary_file 是相对 SelfHostDir 的安全路径，
// 由后端生成，绝不接受客户端传入。
type Project struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	UserID       uint       `gorm:"not null;index" json:"user_id"`
	Name         string     `gorm:"size:128;not null" json:"name"`
	Slug         string     `gorm:"size:64;not null;uniqueIndex" json:"slug"`
	Description  string     `gorm:"size:512;default:''" json:"description"`
	Language     string     `gorm:"size:32;not null;index" json:"language"`
	RuntimeID    uint       `gorm:"not null;index;default:0" json:"runtime_id"`
	Mode         string     `gorm:"size:16;not null;default:source;index" json:"mode"`
	SourceFile   string     `gorm:"size:512;default:''" json:"source_file"`
	BinaryFile   string     `gorm:"size:512;default:''" json:"binary_file"`
	SourceName   string     `gorm:"size:256;default:''" json:"source_name"` // 原始文件名（仅展示）
	BinaryName   string     `gorm:"size:256;default:''" json:"binary_name"`
	BuildCommand string     `gorm:"size:512;default:''" json:"build_command"`
	StartCommand string     `gorm:"size:512;default:''" json:"start_command"`
	Upstream     string     `gorm:"size:512;default:''" json:"upstream"` // 作者自带服务的 http(s)://host:port/root
	Visibility   string     `gorm:"size:16;default:private;index" json:"visibility"`
	ReviewStatus string     `gorm:"size:16;default:draft;index" json:"review_status"`
	ReviewNote   string     `gorm:"size:512;default:''" json:"review_note"`
	ReviewedBy   uint       `gorm:"default:0" json:"reviewed_by"`
	ReviewedAt   *time.Time `json:"reviewed_at"`
	SourceWaived bool       `gorm:"default:false" json:"source_waived"` // 管理员逐项目豁免源码要求
	LocalExec    bool       `gorm:"default:false" json:"local_exec"`    // 预留，当前恒 false
	// 文件共享开关：默认 false。公开项目 ≠ 公开源码，下载必须另有授权。
	SourceShared bool           `gorm:"default:false" json:"source_shared"`
	BinaryShared bool           `gorm:"default:false" json:"binary_shared"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	User    *models.User `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Runtime *Runtime     `gorm:"foreignKey:RuntimeID" json:"runtime,omitempty"`
	Route   *Route       `gorm:"foreignKey:ProjectID" json:"route,omitempty"`
}

func (Project) TableName() string { return "selfhost_projects" }

// Route 反向代理路由（一个项目一条，后台独立管理页维护）。
type Route struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ProjectID   uint      `gorm:"not null;uniqueIndex" json:"project_id"`
	Prefix      string    `gorm:"size:128;not null;uniqueIndex" json:"prefix"` // /demo、/a/b
	Target      string    `gorm:"size:512;not null" json:"target"`
	StripPrefix bool      `gorm:"default:true" json:"strip_prefix"`
	Timeout     int       `gorm:"default:10" json:"timeout"` // 秒，1..120
	Enabled     bool      `gorm:"default:false;index" json:"enabled"`
	Status      string    `gorm:"size:16;default:stopped;index" json:"status"`
	LastError   string    `gorm:"size:512;default:''" json:"last_error"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Route) TableName() string { return "selfhost_routes" }

// ReviewLog 审核留痕。
type ReviewLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ProjectID  uint      `gorm:"not null;index" json:"project_id"`
	OperatorID uint      `gorm:"not null;index" json:"operator_id"`
	Action     string    `gorm:"size:32;not null" json:"action"`
	Note       string    `gorm:"size:512;default:''" json:"note"`
	CreatedAt  time.Time `json:"created_at"`
}

func (ReviewLog) TableName() string { return "selfhost_review_logs" }

// FileGrant 单个用户对单个项目某类文件的访问授权。
// 一次性授权（approved）长期有效，可由所有者/管理员撤销；默认没有任何授权。
type FileGrant struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	ProjectID  uint       `gorm:"not null;index" json:"project_id"`
	UserID     uint       `gorm:"not null;index" json:"user_id"`
	Kind       string     `gorm:"size:16;not null" json:"kind"` // source | binary | all
	Status     string     `gorm:"size:16;default:pending;index" json:"status"`
	Note       string     `gorm:"size:512;default:''" json:"note"`
	ReviewedBy uint       `gorm:"default:0" json:"reviewed_by"`
	ReviewedAt *time.Time `json:"reviewed_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (FileGrant) TableName() string { return "selfhost_file_grants" }

// ─── 迁移 ───

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&Runtime{}, &Project{}, &Route{}, &ReviewLog{}, &FileGrant{}); err != nil {
		return err
	}
	// 默认强制提供源码。必须在 DB 就绪后写，不能放 init()（那时 models.DB 还是 nil）。
	if models.GetPlatformConfig(SourceRequiredConfigKey) == "" {
		_ = models.SetPlatformConfig(SourceRequiredConfigKey, "1")
	}
	if err := MigrateDirect(db); err != nil {
		return err
	}
	if err := MigrateUsage(db); err != nil {
		return err
	}
	if err := MigrateRuntimeRequests(db); err != nil {
		return err
	}
	return MigrateIsoServices(db)
}

// ─── 规则 ───

func isAdmin(role string) bool { return role == "admin" || role == "super" }

func normalizeLang(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func ValidateRuntime(in *Runtime) error {
	in.Language = normalizeLang(in.Language)
	in.Version = strings.TrimSpace(in.Version)
	in.Label = strings.TrimSpace(in.Label)
	in.Kind = strings.ToLower(strings.TrimSpace(in.Kind))
	if in.Kind != "compiled" && in.Kind != "interpreted" {
		return errors.New("运行时类型只能是 compiled 或 interpreted")
	}
	if !langRe.MatchString(in.Language) {
		return errors.New("语言标识不合法")
	}
	if !versionRe.MatchString(in.Version) {
		return errors.New("版本号不合法")
	}
	if len([]rune(in.InstallHint)) > 2000 {
		return errors.New("安装说明过长")
	}
	return nil
}

// IsCompiledLanguage 判断语言是否需要编译（可由运行时目录的 kind 覆盖）。
func IsCompiledLanguage(lang string) bool {
	switch normalizeLang(lang) {
	case "java", "go", "dotnet", "csharp", "rust", "c", "cpp", "kotlin", "swift":
		return true
	}
	return false
}

// SourceRequired 判断该项目是否必须提供源码。
func SourceRequired(lang string, waived bool) bool {
	if waived {
		return false
	}
	return IsCompiledLanguage(lang)
}

func ValidateSlug(s string) error {
	if !slugRe.MatchString(s) {
		return errors.New("项目标识只能使用小写字母、数字、连字符和下划线（2-64 位）")
	}
	return nil
}

// reservedProxyPrefixes 不能被代理路由占用：与 Gin 显式路由、静态资源或前端 SPA 根段冲突时，
// NoRoute 会被代理抢先命中，导致后台/API/前端页面被劫持。新增前端根段时也要同步这里。
var reservedProxyPrefixes = map[string]bool{
	"api": true, "uploads": true, "sandbox": true, "health": true, "assets": true, "hp": true,
	"admin": true, "author": true, "blog": true, "blogs": true, "bookshelf": true,
	"category": true, "comment-network": true, "docs": true, "forum": true, "forums": true,
	"frames": true, "htmls": true, "install": true, "login": true, "novel": true,
	"novels": true, "plaza": true, "register": true, "selfhost": true, "reports": true,
	"run": true, "settings": true, "setup": true, "thread": true, "wasm": true, "pay": true,
	"groups": true, "group": true, "about": true, "ai": true, "api-spider": true,
	"feed": true, "me": true, "messages": true, "my-keys": true, "police-console": true,
	"police": true, "reading": true, "topics": true, "user": true, "wallet": true,
	"membership": true, "collab": true, "live": true, "oc": true, "ai-write": true,
	"subculture": true, "federation": true, "shorts": true, "micro": true,
	"blog-theme": true, "discover": true,
}

func ValidatePrefix(s string) error {
	if !prefixRe.MatchString(s) || strings.HasSuffix(s, "/") {
		return errors.New("代理前缀格式不合法（示例 /demo 或 /team/app）")
	}
	if strings.Contains(s, "//") || strings.Contains(s, "..") {
		return errors.New("代理前缀不能包含 // 或 ..")
	}
	first := strings.SplitN(strings.TrimPrefix(s, "/"), "/", 2)[0]
	if reservedProxyPrefixes[first] {
		return errors.New("该首段属于平台保留路径（API / 静态资源 / 前端页面），不能用作代理前缀")
	}
	return nil
}

// SafeRelPath 把客户端提供的相对路径收敛为 SelfHostDir 下的安全相对路径。
func SafeRelPath(userID, projectID uint, name, suffix string) (string, error) {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == ".." || name == "" || strings.ContainsAny(name, `/\:`) {
		return "", errors.New("文件名不合法")
	}
	rel := filepath.Join(strconv.FormatUint(uint64(userID), 10), strconv.FormatUint(uint64(projectID), 10), suffix+"-"+name)
	return filepath.ToSlash(rel), nil
}

func AbsStoredPath(rel string) (string, error) {
	if rel == "" {
		return "", errors.New("文件不存在")
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", errors.New("文件路径不合法")
	}
	return filepath.Join(config.SelfHostDir, clean), nil
}

func ProjectDir(userID, projectID uint) (string, error) {
	dir := filepath.Join(config.SelfHostDir, strconv.FormatUint(uint64(userID), 10), strconv.FormatUint(uint64(projectID), 10))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	return dir, nil
}

func allowedExt(name, suffix string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if suffix == "source" {
		return trustedExt[ext]
	}
	return binaryExt[ext]
}

func CheckSourceName(name string) error {
	if !allowedExt(name, "source") {
		return errors.New("源码包只支持 zip/tar/gz/tgz")
	}
	return nil
}

func CheckBinaryName(name string) error {
	if !allowedExt(name, "binary") {
		return errors.New("可执行文件后缀不在白名单（jar/war/exe/bin/app/dll/so/pyz/class/wasm）")
	}
	return nil
}

// ValidateUpstream 校验作者自带服务地址，拦掉明显指向本机的 SSRF 目标。
// 接受 :8080、:8080/root、http(s)://host:port/root；拒绝相对路径与协议头注入。
func ValidateUpstream(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("请填写服务地址")
	}
	if strings.HasPrefix(raw, "//") {
		return "", errors.New("服务地址不合法")
	}
	if !strings.Contains(raw, "://") {
		if strings.HasPrefix(raw, ":") {
			raw = "http://127.0.0.1" + raw
		} else {
			raw = "http://" + raw
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", errors.New("服务地址不合法")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("服务地址只支持 http/https")
	}
	host := u.Hostname()
	if host == "" || strings.EqualFold(host, "localhost") {
		return "", errors.New("服务地址不能指向本机 localhost")
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return "", errors.New("服务地址不能指向本机或链路本地地址")
		}
	}
	return u.String(), nil
}

// ─── 查询 ───

func GetProject(id uint) (*Project, error) {
	var p Project
	err := models.DB.Preload("Runtime").Preload("Route").First(&p, id).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func GetProjectRaw(id uint) (*Project, error) {
	var p Project
	err := models.DB.First(&p, id).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func ListMyProjects(userID uint, keyword string) ([]Project, int64, error) {
	var list []Project
	var total int64
	q := models.DB.Model(&Project{}).Where("user_id = ?", userID)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		q = q.Where("name LIKE ? OR slug LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("updated_at DESC").Find(&list).Error
	return list, total, err
}

// ListPublic 广场只展示审核通过、公开、且已配置可用路由的项目。
func ListPublic(keyword, lang string) ([]Project, int64, error) {
	var list []Project
	var total int64
	q := models.DB.Model(&Project{}).
		Where("review_status = ? AND visibility = ?", ReviewApproved, VisPublic)
	if lang = normalizeLang(lang); lang != "" {
		q = q.Where("language = ?", lang)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		q = q.Where("name LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("updated_at DESC").Find(&list).Error
	return list, total, err
}

func ListRuntimes(enabledOnly bool) ([]Runtime, error) {
	var list []Runtime
	q := models.DB.Model(&Runtime{})
	if enabledOnly {
		q = q.Where("enabled = ?", true)
	}
	err := q.Order("language ASC, sort ASC, id ASC").Find(&list).Error
	return list, err
}

func FindRuntime(id uint) (*Runtime, error) {
	var r Runtime
	if err := models.DB.First(&r, id).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

func ListReviewLogs(projectID uint) ([]ReviewLog, error) {
	var list []ReviewLog
	err := models.DB.Where("project_id = ?", projectID).Order("id DESC").Limit(50).Find(&list).Error
	return list, err
}

func SourceRequiredByConfig() bool {
	return models.GetPlatformConfig(SourceRequiredConfigKey) != "0"
}

// ─── 写操作 ───

func CreateRuntime(in Runtime) (*Runtime, error) {
	if err := ValidateRuntime(&in); err != nil {
		return nil, err
	}
	var dup int64
	models.DB.Model(&Runtime{}).Where("language = ? AND version = ?", in.Language, in.Version).Count(&dup)
	if dup > 0 {
		return nil, errors.New("该语言版本已存在")
	}
	if err := models.DB.Create(&in).Error; err != nil {
		return nil, err
	}
	return &in, nil
}

func UpdateRuntime(id uint, in Runtime) (*Runtime, error) {
	r, err := FindRuntime(id)
	if err != nil {
		return nil, err
	}
	if err := ValidateRuntime(&in); err != nil {
		return nil, err
	}
	r.Language, r.Version, r.Label, r.Kind = in.Language, in.Version, in.Label, in.Kind
	r.InstallHint, r.DownloadURL, r.Checksum = in.InstallHint, in.DownloadURL, in.Checksum
	r.ExecPath = in.ExecPath
	r.Enabled, r.Sort = in.Enabled, in.Sort
	if err := models.DB.Save(r).Error; err != nil {
		return nil, err
	}
	// 运行时目录变更可能改变「是否必须附源码」的口径，立即刷新代理表。
	InvalidateRoutes()
	return r, nil
}

func DeleteRuntime(id uint) error {
	var used int64
	models.DB.Model(&Project{}).Where("runtime_id = ?", id).Count(&used)
	if used > 0 {
		return errors.New("仍有项目绑定该运行时，不能删除")
	}
	return models.DB.Delete(&Runtime{}, id).Error
}

// CreateProject 建项目。初始状态 draft，绝不直接对外。
func CreateProject(userID uint, in Project) (*Project, error) {
	in.ID = 0
	in.UserID = userID
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.Slug = strings.ToLower(strings.TrimSpace(in.Slug))
	in.Language = normalizeLang(in.Language)
	in.Mode = strings.ToLower(strings.TrimSpace(in.Mode))
	if err := ValidateSlug(in.Slug); err != nil {
		return nil, err
	}
	if len([]rune(in.Name)) < 2 || len([]rune(in.Name)) > 128 {
		return nil, errors.New("项目名称长度应为 2-128 字")
	}
	if !langRe.MatchString(in.Language) {
		return nil, errors.New("语言标识不合法")
	}
	if in.Mode != ModeSource && in.Mode != ModeCompiled {
		return nil, errors.New("项目模式只能是 source 或 compiled")
	}
	if in.RuntimeID == 0 {
		return nil, errors.New("请选择运行时版本")
	}
	in.ReviewStatus = ReviewDraft
	in.ReviewNote = ""
	in.ReviewerFieldsClear()
	in.Visibility = VisPrivate
	in.LocalExec = false // 预留开关：本轮不接受置真
	if err := models.DB.Create(&in).Error; err != nil {
		return nil, err
	}
	return &in, nil
}

func (p *Project) ReviewerFieldsClear() {
	p.ReviewedBy = 0
	p.ReviewedAt = nil
}

func UpdateProject(userID uint, id uint, in Project, isAdminUser bool) (*Project, error) {
	p, err := GetProjectRaw(id)
	if err != nil {
		return nil, err
	}
	if !isAdminUser && p.UserID != userID {
		return nil, errors.New("无权修改该项目")
	}
	if p.ReviewStatus == ReviewApproved || p.ReviewStatus == ReviewPending {
		// 已过审/待审内容改动后必须重新审核，避免「先审后改」绕过门禁。
		p.ReviewStatus = ReviewDraft
		p.ReviewNote = "内容已修改，需重新提交审核"
		p.ReviewedBy, p.ReviewedAt = 0, nil
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.Language = normalizeLang(in.Language)
	p.Name, p.Description = in.Name, in.Description
	p.Language = in.Language
	p.RuntimeID = in.RuntimeID
	p.BuildCommand, p.StartCommand = in.BuildCommand, in.StartCommand
	if in.Upstream != "" {
		up, err := ValidateUpstream(in.Upstream)
		if err != nil {
			return nil, err
		}
		p.Upstream = up
	}
	if err := models.DB.Save(p).Error; err != nil {
		return nil, err
	}
	InvalidateRoutes()
	return p, nil
}

// DeleteProject 同时清理落盘文件与路由。
func DeleteProject(userID, id uint, isAdminUser bool) error {
	p, err := GetProjectRaw(id)
	if err != nil {
		return err
	}
	if !isAdminUser && p.UserID != userID {
		return errors.New("无权删除该项目")
	}
	models.DB.Where("project_id = ?", id).Delete(&Route{})
	models.DB.Where("project_id = ?", id).Delete(&ReviewLog{})
	if err := models.DB.Delete(&Project{}, id).Error; err != nil {
		return err
	}
	dir := filepath.Join(config.SelfHostDir, strconv.FormatUint(uint64(userID), 10), strconv.FormatUint(uint64(id), 10))
	_ = os.RemoveAll(dir)
	InvalidateRoutes()
	return nil
}

// AttachFile 保存上传文件并更新项目字段（调用方已做权限校验）。
func AttachFile(p *Project, suffix string, relPath, origName string) error {
	if suffix == "source" {
		p.SourceFile, p.SourceName = relPath, origName
	} else {
		p.BinaryFile, p.BinaryName = relPath, origName
	}
	// 换文件即需重审：只更新白名单字段，避免整结构体 Updates 误写审核状态。
	fields := map[string]interface{}{
		"review_status": ReviewDraft,
		"review_note":   "",
	}
	if suffix == "source" {
		fields["source_file"], fields["source_name"] = relPath, origName
	} else {
		fields["binary_file"], fields["binary_name"] = relPath, origName
	}
	if err := models.DB.Model(&Project{}).Where("id = ?", p.ID).Updates(fields).Error; err != nil {
		return err
	}
	if suffix == "source" {
		p.SourceFile, p.SourceName = relPath, origName
	} else {
		p.BinaryFile, p.BinaryName = relPath, origName
	}
	p.ReviewStatus, p.ReviewNote = ReviewDraft, ""
	return nil
}

// SubmitForReview 提交审核（源码门禁在这里再验一次，防止旧项目绕过）。
func SubmitForReview(id, userID uint) error {
	p, err := GetProjectRaw(id)
	if err != nil {
		return err
	}
	if p.UserID != userID {
		return errors.New("只能提交自己的项目")
	}
	if p.ReviewStatus == ReviewPending {
		return errors.New("项目正在审核中")
	}
	if p.SourceFile == "" {
		return errors.New("请先上传源码包")
	}
	if p.Mode == ModeCompiled && p.BinaryFile == "" && SourceRequired(p.Language, p.SourceWaived) {
		return errors.New("编译版项目需同时提供可执行文件")
	}
	// 可执行文件必须有启动参数/命令，否则运维与隔离检测都无从下手。
	if p.BinaryFile != "" && strings.TrimSpace(p.StartCommand) == "" {
		return errors.New("上传可执行文件的项目必须填写启动参数（启动命令）")
	}
	if p.Upstream == "" {
		return errors.New("请先填写作者自带服务地址")
	}
	if err := isolationBlocks(p.ID); err != nil {
		return err
	}
	p.ReviewStatus = ReviewPending
	p.ReviewNote = ""
	if err := models.DB.Model(&Project{}).Where("id = ?", p.ID).Select("review_status", "review_note", "updated_at").Updates(p).Error; err != nil {
		return err
	}
	return addLog(id, userID, "submit", "")
}

func ReviewProject(id, operatorID uint, approve bool, note string, waiveSource bool) error {
	p, err := GetProjectRaw(id)
	if err != nil {
		return err
	}
	if p.ReviewStatus != ReviewPending {
		return errors.New("只有待审项目可审核")
	}
	action := ReviewApproved
	if !approve {
		action = ReviewRejected
	} else if waiveSource {
		p.SourceWaived = true
	}
	p.ReviewStatus = action
	p.ReviewNote = strings.TrimSpace(note)
	p.ReviewedBy, p.ReviewedAt = operatorID, timePtr(time.Now())
	if err := models.DB.Model(&Project{}).Where("id = ?", p.ID).Select("review_status", "review_note", "reviewed_by", "reviewed_at", "source_waived", "updated_at").
		Updates(p).Error; err != nil {
		return err
	}
	if addErr := addLog(id, operatorID, action, p.ReviewNote); addErr != nil {
		return addErr
	}
	InvalidateRoutes()
	return nil
}

func SuspendProject(id, operatorID uint, suspend bool, note string) error {
	p, err := GetProjectRaw(id)
	if err != nil {
		return err
	}
	action := ReviewApproved
	verb := "restore"
	if suspend {
		action, verb = ReviewSuspended, "suspend"
	}
	p.ReviewStatus = action
	p.ReviewNote = strings.TrimSpace(note)
	p.ReviewedBy, p.ReviewedAt = operatorID, timePtr(time.Now())
	if err := models.DB.Model(&Project{}).Where("id = ?", p.ID).Select("review_status", "review_note", "reviewed_by", "reviewed_at", "updated_at").Updates(p).Error; err != nil {
		return err
	}
	InvalidateRoutes()
	return addLog(id, operatorID, verb, p.ReviewNote)
}

func SetVisibility(id, userID uint, vis string, isAdminUser bool) error {
	p, err := GetProjectRaw(id)
	if err != nil {
		return err
	}
	if !isAdminUser && p.UserID != userID {
		return errors.New("无权修改该项目")
	}
	if vis != VisPrivate && vis != VisPublic {
		return errors.New("可见性只能是 private 或 public")
	}
	if vis == VisPublic && p.ReviewStatus != ReviewApproved {
		return errors.New("项目审核通过后才能设为公开")
	}
	p.Visibility = vis
	return models.DB.Model(&Project{}).Where("id = ?", p.ID).Update("visibility", vis).Error
}

func addLog(projectID, operatorID uint, action, note string) error {
	return models.DB.Create(&ReviewLog{ProjectID: projectID, OperatorID: operatorID, Action: action, Note: note}).Error
}

func timePtr(t time.Time) *time.Time { return &t }

// ─── 路由写操作（热加载由 InvalidateRoutes 完成） ───

func UpsertRoute(in Route) (*Route, error) {
	in.ID = 0
	if err := ValidatePrefix(in.Prefix); err != nil {
		return nil, err
	}
	target, err := ValidateUpstream(in.Target)
	if err != nil {
		return nil, err
	}
	in.Target = target
	p, err := GetProjectRaw(in.ProjectID)
	if err != nil {
		return nil, errors.New("项目不存在")
	}
	if p.ReviewStatus != ReviewApproved {
		return nil, errors.New("项目尚未审核通过，不能建立对外路由")
	}
	if err := isolationBlocks(in.ProjectID); err != nil {
		return nil, err
	}
	if in.Timeout < 1 || in.Timeout > 120 {
		in.Timeout = 10
	}
	var old Route
	notFound := models.DB.Where("project_id = ?", in.ProjectID).First(&old).Error != nil
	if !notFound {
		in.ID = old.ID
		in.CreatedAt = old.CreatedAt
		if err := models.DB.Save(&in).Error; err != nil {
			return nil, err
		}
	} else if err := models.DB.Create(&in).Error; err != nil {
		return nil, err
	}
	InvalidateRoutes()
	return &in, nil
}

func SetRouteEnabled(id uint, enabled bool) (*Route, error) {
	var r Route
	if err := models.DB.First(&r, id).Error; err != nil {
		return nil, err
	}
	p, err := GetProjectRaw(r.ProjectID)
	if err != nil {
		return nil, err
	}
	if enabled && p.ReviewStatus != ReviewApproved {
		return nil, errors.New("项目未审核通过或已被停止，不能启用路由")
	}
	if enabled {
		if err := isolationBlocks(r.ProjectID); err != nil {
			return nil, err
		}
	}
	r.Enabled = enabled
	r.Status = RouteStopped
	if enabled {
		r.Status = RouteRunning
	}
	r.LastError = ""
	if err := models.DB.Save(&r).Error; err != nil {
		return nil, err
	}
	InvalidateRoutes()
	return &r, nil
}

func DeleteRoute(id uint) error {
	if err := models.DB.Delete(&Route{}, id).Error; err != nil {
		return err
	}
	InvalidateRoutes()
	return nil
}

func ListRoutes(keyword string) ([]Route, error) {
	var list []Route
	q := models.DB.Model(&Route{})
	if kw := strings.TrimSpace(keyword); kw != "" {
		q = q.Where("prefix LIKE ? OR target LIKE ?", "%"+kw+"%", "%"+kw+"%")
	}
	err := q.Order("prefix ASC").Find(&list).Error
	return list, err
}

// SortedEnabledRoutes 供代理表热加载：只返回已审核通过且启用的路由。
func SortedEnabledRoutes() ([]Route, error) {
	var list []Route
	err := models.DB.Table("selfhost_routes").
		Select("selfhost_routes.*").
		Joins("JOIN selfhost_projects ON selfhost_projects.id = selfhost_routes.project_id").
		Where("selfhost_routes.enabled = ? AND selfhost_projects.review_status = ? AND selfhost_projects.deleted_at IS NULL", true, ReviewApproved).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	// 纵深防御：历史数据里若有保留段前缀（ValidatePrefix 上线前写入的），加载时再挡一次，
	// 避免 NoRoute 抢走 API/静态资源/前端页面。
	out := list[:0]
	for _, r := range list {
		first := strings.SplitN(strings.TrimPrefix(r.Prefix, "/"), "/", 2)[0]
		if reservedProxyPrefixes[first] {
			continue
		}
		out = append(out, r)
	}
	// 最长前缀优先；proxycore 内部也不依赖此顺序（matchPath 取最长）。
	sort.Slice(out, func(i, j int) bool { return len(out[i].Prefix) > len(out[j].Prefix) })
	return out, nil
}

func RouteCount() (int64, int64, error) {
	var total, active int64
	if err := models.DB.Model(&Route{}).Count(&total).Error; err != nil {
		return 0, 0, err
	}
	err := models.DB.Model(&Route{}).Where("enabled = ?", true).Count(&active).Error
	return total, active, err
}

func ConfigSummary() map[string]interface{} {
	return map[string]interface{}{
		"source_required": SourceRequiredByConfig(),
		"local_exec":      false,
		"note":            "本机执行为预留能力，当前不接受启用",
	}
}

func SaveSourceRequiredConfig(v bool) error {
	if v {
		return models.SetPlatformConfig(SourceRequiredConfigKey, "1")
	}
	return models.SetPlatformConfig(SourceRequiredConfigKey, "0")
}
