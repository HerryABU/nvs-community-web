package oc

// ==================== OC 立项制：项目 / 世界 ====================
// 亚文化创作区的「先立项、再创作」：
//   ① 先建项目（=一个世界观/企划），设定类别与可见性（私有 / 公开 / 密码公开）；
//   ② 成员在项目内创作角色卡与世界书（世界绑定 project_id）；
//   ③ 世界之间可互相链接（oc_world_links），形成跨世界网络；
//   ④ 密码公开的项目/世界需凭访问码进入（访问码存库，非 JWT、非加密存储）。
//
// 依赖方向：oc → models / security / utils/secure（不反向依赖 handlers）。

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils/secure"

	"gorm.io/gorm"
)

// 项目成员角色
const (
	ProjectRoleOwner  = "owner"  // 创建者：全权
	ProjectRoleEditor = "editor" // 可创作（角色/世界/链接），不可删项目
	ProjectRoleViewer = "viewer" // 只读
)

// 项目可见性（VisPrivate / VisPublic 复用 model.go 的定义，这里补一档密码公开）
const (
	VisPassword = "password" // 密码公开（凭访问码进入）
)

const (
	MaxProjectTitleRunes = 60
	MaxProjectSummary    = 2000
	ProjectAccessTTL     = 7 * 24 * time.Hour
)

// ─── 模型 ───

// OcProject 项目/世界立项（亚文化创作区的顶层容器）
type OcProject struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	UserID       uint   `gorm:"not null;index" json:"user_id"`
	Title        string `gorm:"size:64;not null;index" json:"title"`
	Summary      string `gorm:"type:text" json:"summary"`
	CoverURL     string `gorm:"size:512;default:''" json:"cover_url"`
	Category     string `gorm:"size:32;not null;default:''" json:"category"` // 类别：原创世界/furry兽人/现代都市…
	Visibility   string `gorm:"size:16;not null;default:private;index" json:"visibility"`
	PasswordHash string `gorm:"size:128;not null;default:''" json:"-"`
	WorldCount   int    `gorm:"not null;default:0" json:"world_count"`
	MemberCount  int    `gorm:"not null;default:0" json:"member_count"`
	Review       string `gorm:"size:16;not null;default:approved;index" json:"review"`
	Status       string `gorm:"size:16;not null;default:active" json:"status"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	OwnerName string `gorm:"-" json:"owner_name,omitempty"`
}

func (OcProject) TableName() string { return "oc_projects" }

// OcProjectMember 项目成员（owner/editor/viewer）
type OcProjectMember struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProjectID uint      `gorm:"not null;index:idx_ocpm" json:"project_id"`
	UserID    uint      `gorm:"not null;index:idx_ocpm" json:"user_id"`
	Role      string    `gorm:"size:16;not null;default:editor" json:"role"`
	CreatedAt time.Time `json:"created_at"`

	Username  string `gorm:"-" json:"username,omitempty"`
	Nickname  string `gorm:"-" json:"nickname,omitempty"`
	AvatarURL string `gorm:"-" json:"avatar_url,omitempty"`
}

func (OcProjectMember) TableName() string { return "oc_project_members" }

// OcProjectAccess 密码公开项目的访问码（随机 token，服务端存储；非 JWT）
type OcProjectAccess struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProjectID uint      `gorm:"not null;index" json:"project_id"`
	Token     string    `gorm:"size:64;not null;uniqueIndex" json:"token"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (OcProjectAccess) TableName() string { return "oc_project_access" }

// OcWorldLink 跨世界链接（双向可查）
type OcWorldLink struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	FromBookID uint      `gorm:"not null;index" json:"from_book_id"`
	ToBookID   uint      `gorm:"not null;index" json:"to_book_id"`
	Label      string    `gorm:"size:128;default:''" json:"label"`
	CreatedAt  time.Time `json:"created_at"`

	FromTitle string `gorm:"-" json:"from_title,omitempty"`
	ToTitle   string `gorm:"-" json:"to_title,omitempty"`
}

func (OcWorldLink) TableName() string { return "oc_world_links" }

// ─── 入参 ───

type ProjectInput struct {
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	CoverURL   string `json:"cover_url"`
	Category   string `json:"category"`
	Visibility string `json:"visibility"`
	Password   string `json:"password"`
}

func normalizeProjectInput(in *ProjectInput) {
	in.Title = strings.TrimSpace(security.SanitizeUserContent(in.Title))
	in.Summary = strings.TrimSpace(security.SanitizeUserContent(in.Summary))
	in.CoverURL = strings.TrimSpace(in.CoverURL)
	in.Category = strings.TrimSpace(security.SanitizeUserContent(in.Category))
	in.Visibility = NormalizeProjectVisibility(in.Visibility)
	in.Password = strings.TrimSpace(in.Password)
}

// NormalizeProjectVisibility 归一化项目可见性（多一档 password）
func NormalizeProjectVisibility(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case VisPublic:
		return VisPublic
	case VisPassword:
		return VisPassword
	default:
		return VisPrivate
	}
}

// ─── 项目 CRUD ───

// CreateProject 立项
func CreateProject(userID uint, in ProjectInput) (*OcProject, error) {
	normalizeProjectInput(&in)
	if in.Title == "" {
		return nil, errors.New("项目名称不能为空")
	}
	if len([]rune(in.Title)) > MaxProjectTitleRunes {
		return nil, errors.New("项目名称过长")
	}
	if len([]rune(in.Summary)) > MaxProjectSummary {
		return nil, errors.New("项目简介过长")
	}
	review := ReviewApproved
	if in.Visibility == VisPublic {
		review = ReviewPending
	}
	p := &OcProject{
		UserID: userID, Title: in.Title, Summary: in.Summary, CoverURL: in.CoverURL,
		Category: in.Category, Visibility: in.Visibility, Review: review,
		Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	// 密码公开：必须设置密码（bcrypt，独立于 JWT/字段加密）
	if in.Visibility == VisPassword {
		if len(in.Password) < 6 {
			return nil, errors.New("密码公开的访问密码至少 6 位")
		}
		h, err := secure.HashPassword(in.Password)
		if err != nil {
			return nil, errors.New("访问密码处理失败")
		}
		p.PasswordHash = h
	}
	err := models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(p).Error; err != nil {
			return err
		}
		return tx.Create(&OcProjectMember{
			ProjectID: p.ID, UserID: userID, Role: ProjectRoleOwner, CreatedAt: time.Now(),
		}).Error
	})
	if err != nil {
		return nil, err
	}
	p.MemberCount = 1
	fillProjectOwnerNames([]*OcProject{p})
	return p, nil
}

// UpdateProject 更新项目（仅 owner）
func UpdateProject(userID, id uint, in ProjectInput) (*OcProject, error) {
	var p OcProject
	if err := models.DB.Where("id = ? AND status = ?", id, "active").First(&p).Error; err != nil {
		return nil, errors.New("项目不存在")
	}
	// 仅 owner 可改（editor 只能创作内容）
	if p.UserID != userID {
		return nil, errors.New("只有项目负责人可修改项目设置")
	}
	normalizeProjectInput(&in)
	if in.Title == "" {
		return nil, errors.New("项目名称不能为空")
	}
	if in.Visibility == VisPublic && p.Visibility != VisPublic {
		p.Review = ReviewPending
	}
	p.Title = in.Title
	p.Summary = in.Summary
	p.CoverURL = in.CoverURL
	p.Category = in.Category
	// 可见性切换
	if in.Visibility != p.Visibility {
		p.Visibility = in.Visibility
		if in.Visibility != VisPassword {
			p.PasswordHash = ""
			// 清掉历史访问码
			_ = models.DB.Where("project_id = ?", p.ID).Delete(&OcProjectAccess{}).Error
		}
	}
	if in.Visibility == VisPassword {
		if p.PasswordHash == "" || in.Password != "" {
			if len(in.Password) < 6 {
				return nil, errors.New("访问密码至少 6 位")
			}
			h, err := secure.HashPassword(in.Password)
			if err != nil {
				return nil, errors.New("访问密码处理失败")
			}
			p.PasswordHash = h
			_ = models.DB.Where("project_id = ?", p.ID).Delete(&OcProjectAccess{}).Error
		}
	}
	p.UpdatedAt = time.Now()
	if err := models.DB.Save(&p).Error; err != nil {
		return nil, err
	}
	fillProjectOwnerNames([]*OcProject{&p})
	return &p, nil
}

// DeleteProject 软删除项目（连带成员/访问码清退；世界解绑但保留）
func DeleteProject(userID, id uint) error {
	var p OcProject
	if err := models.DB.Where("id = ? AND status = ?", id, "active").First(&p).Error; err != nil {
		return errors.New("项目不存在")
	}
	if p.UserID != userID {
		return errors.New("只有项目负责人可删除项目")
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project_id = ?", id).Delete(&OcProjectMember{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", id).Delete(&OcProjectAccess{}).Error; err != nil {
			return err
		}
		// 世界解绑（保留世界本身）
		if err := tx.Model(&WorldBook{}).Where("project_id = ?", id).
			Update("project_id", 0).Error; err != nil {
			return err
		}
		if err := tx.Model(&Character{}).Where("project_id = ?", id).
			Update("project_id", 0).Error; err != nil {
			return err
		}
		return tx.Model(&OcProject{}).Where("id = ?", id).
			Updates(map[string]interface{}{"status": "deleted", "visibility": VisPrivate}).Error
	})
}

// GetProject 读取（成员/公开过审/密码访问码）
func GetProject(id, viewerID uint, isAdmin bool, token string) (*OcProject, error) {
	var p OcProject
	if err := models.DB.Where("id = ?", id).First(&p).Error; err != nil {
		return nil, errors.New("项目不存在")
	}
	if p.Status != "active" && !isAdmin {
		return nil, errors.New("项目已删除")
	}
	if !CanViewProject(&p, viewerID, isAdmin, token) {
		return nil, errors.New("项目不可见")
	}
	fillProjectOwnerNames([]*OcProject{&p})
	return &p, nil
}

// ListProjects 项目列表（mine / public / joined）
func ListProjects(userID uint, scope, category, keyword string, page, pageSize int) ([]OcProject, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	q := models.DB.Model(&OcProject{}).Where("status = ?", "active")
	switch scope {
	case "public":
		q = q.Where("visibility = ? AND review = ?", VisPublic, ReviewApproved)
	case "mine":
		q = q.Where("user_id = ?", userID)
	case "joined":
		q = q.Where("id IN (?)", models.DB.Model(&OcProjectMember{}).
			Select("project_id").Where("user_id = ?", userID))
	default: // usable：我的 + 已加入 + 公开过审
		q = q.Where("user_id = ? OR id IN (?) OR (visibility = ? AND review = ?)",
			userID,
			models.DB.Model(&OcProjectMember{}).Select("project_id").Where("user_id = ?", userID),
			VisPublic, ReviewApproved)
	}
	if c := strings.TrimSpace(category); c != "" {
		q = q.Where("category = ?", c)
	}
	if kw := strings.TrimSpace(keyword); kw != "" {
		q = q.Where("title LIKE ?", "%"+kw+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []OcProject
	if err := q.Order("updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	ptrs := make([]*OcProject, len(list))
	for i := range list {
		ptrs[i] = &list[i]
	}
	fillProjectOwnerNames(ptrs)
	return list, total, nil
}

// ─── 权限 ───

// IsProjectMember 是否项目成员
func IsProjectMember(projectID, userID uint) bool {
	if userID == 0 {
		return false
	}
	var cnt int64
	models.DB.Model(&OcProjectMember{}).Where("project_id = ? AND user_id = ?", projectID, userID).Count(&cnt)
	return cnt > 0
}

// ProjectRole 成员角色（非成员返回空）
func ProjectRole(projectID, userID uint) string {
	var m OcProjectMember
	if err := models.DB.Where("project_id = ? AND user_id = ?", projectID, userID).First(&m).Error; err != nil {
		return ""
	}
	return m.Role
}

// CanEditProject 是否可在项目内创作（owner/editor 或平台）
func CanEditProject(p *OcProject, userID uint, isAdmin bool) bool {
	if p == nil || userID == 0 {
		return false
	}
	if isAdmin || p.UserID == userID {
		return true
	}
	r := ProjectRole(p.ID, userID)
	return r == ProjectRoleOwner || r == ProjectRoleEditor
}

// CanViewProject 是否可见（成员 / 公开过审 / 有效访问码 / 平台）
func CanViewProject(p *OcProject, userID uint, isAdmin bool, token string) bool {
	if p == nil {
		return false
	}
	if isAdmin || p.UserID == userID {
		return true
	}
	if IsProjectMember(p.ID, userID) {
		return true
	}
	switch p.Visibility {
	case VisPublic:
		return p.Review == ReviewApproved
	case VisPassword:
		return CheckProjectAccess(p.ID, token)
	default:
		return false
	}
}

// ─── 成员管理 ───

// AddProjectMember 添加成员（按用户名或邮箱；仅 owner）
func AddProjectMember(projectID, operatorID uint, targetUsername, role string) (*OcProjectMember, error) {
	var p OcProject
	if err := models.DB.Where("id = ? AND status = ?", projectID, "active").First(&p).Error; err != nil {
		return nil, errors.New("项目不存在")
	}
	if p.UserID != operatorID {
		return nil, errors.New("只有项目负责人可添加成员")
	}
	targetUsername = strings.TrimSpace(targetUsername)
	if targetUsername == "" {
		return nil, errors.New("请输入成员用户名或邮箱")
	}
	var u models.User
	if err := models.DB.Where("username = ? OR email = ?", targetUsername, targetUsername).First(&u).Error; err != nil {
		return nil, errors.New("用户不存在")
	}
	if u.ID == operatorID {
		return nil, errors.New("不能添加自己")
	}
	role = normalizeProjectRole(role)
	var cnt int64
	models.DB.Model(&OcProjectMember{}).Where("project_id = ? AND user_id = ?", projectID, u.ID).Count(&cnt)
	if cnt > 0 {
		return nil, errors.New("该用户已是项目成员")
	}
	m := &OcProjectMember{ProjectID: projectID, UserID: u.ID, Role: role, CreatedAt: time.Now()}
	if err := models.DB.Create(m).Error; err != nil {
		return nil, err
	}
	recalcProjectMemberCount(projectID)
	return m, nil
}

// UpdateProjectMemberRole 修改成员角色
func UpdateProjectMemberRole(projectID, operatorID, targetUserID uint, role string) error {
	var p OcProject
	if err := models.DB.Where("id = ? AND status = ?", projectID, "active").First(&p).Error; err != nil {
		return errors.New("项目不存在")
	}
	if p.UserID != operatorID {
		return errors.New("只有项目负责人可管理成员")
	}
	if targetUserID == operatorID {
		return errors.New("不能修改负责人自己的角色")
	}
	role = normalizeProjectRole(role)
	return models.DB.Model(&OcProjectMember{}).
		Where("project_id = ? AND user_id = ?", projectID, targetUserID).
		Update("role", role).Error
}

// RemoveProjectMember 移除成员
func RemoveProjectMember(projectID, operatorID, targetUserID uint) error {
	var p OcProject
	if err := models.DB.Where("id = ? AND status = ?", projectID, "active").First(&p).Error; err != nil {
		return errors.New("项目不存在")
	}
	if p.UserID != operatorID {
		return errors.New("只有项目负责人可移除成员")
	}
	if targetUserID == operatorID {
		return errors.New("不能移除负责人")
	}
	if err := models.DB.Where("project_id = ? AND user_id = ?", projectID, targetUserID).
		Delete(&OcProjectMember{}).Error; err != nil {
		return err
	}
	recalcProjectMemberCount(projectID)
	return nil
}

// ListProjectMembers 成员列表
func ListProjectMembers(projectID uint) ([]OcProjectMember, error) {
	var list []OcProjectMember
	if err := models.DB.Where("project_id = ?", projectID).Order("role ASC, id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return list, nil
	}
	ids := make([]uint, 0, len(list))
	for _, m := range list {
		ids = append(ids, m.UserID)
	}
	var users []models.User
	models.DB.Select("id, username, nickname, avatar_url").Where("id IN ?", ids).Find(&users)
	byID := map[uint]models.User{}
	for _, u := range users {
		byID[u.ID] = u
	}
	for i := range list {
		if u, ok := byID[list[i].UserID]; ok {
			list[i].Username = u.Username
			list[i].Nickname = u.Nickname
			list[i].AvatarURL = u.AvatarURL
		}
	}
	return list, nil
}

func normalizeProjectRole(r string) string {
	switch strings.ToLower(strings.TrimSpace(r)) {
	case ProjectRoleEditor:
		return ProjectRoleEditor
	case ProjectRoleViewer:
		return ProjectRoleViewer
	default:
		return ProjectRoleEditor
	}
}

func recalcProjectMemberCount(projectID uint) {
	var cnt int64
	models.DB.Model(&OcProjectMember{}).Where("project_id = ?", projectID).Count(&cnt)
	models.DB.Model(&OcProject{}).Where("id = ?", projectID).Update("member_count", cnt)
}

func fillProjectOwnerNames(list []*OcProject) {
	if len(list) == 0 {
		return
	}
	ids := map[uint]bool{}
	for _, p := range list {
		ids[p.UserID] = true
	}
	keys := make([]uint, 0, len(ids))
	for id := range ids {
		keys = append(keys, id)
	}
	var users []models.User
	models.DB.Select("id, username, nickname").Where("id IN ?", keys).Find(&users)
	nameOf := map[uint]string{}
	for _, u := range users {
		if u.Nickname != "" {
			nameOf[u.ID] = u.Nickname
		} else {
			nameOf[u.ID] = u.Username
		}
	}
	for _, p := range list {
		p.OwnerName = nameOf[p.UserID]
	}
}

// ─── 密码访问码 ───

// VerifyProjectPassword 校验项目密码并发放访问码（随机 token，存库；7 天有效）
func VerifyProjectPassword(projectID uint, password string) (string, error) {
	var p OcProject
	if err := models.DB.Where("id = ? AND status = ?", projectID, "active").First(&p).Error; err != nil {
		return "", errors.New("项目不存在")
	}
	if p.Visibility != VisPassword {
		return "", errors.New("该项目未开启密码访问")
	}
	if !secure.CheckPassword(password, p.PasswordHash) {
		return "", errors.New("访问密码错误")
	}
	token, err := randomToken()
	if err != nil {
		return "", errors.New("发放访问码失败")
	}
	acc := &OcProjectAccess{
		ProjectID: projectID, Token: token,
		ExpiresAt: time.Now().Add(ProjectAccessTTL), CreatedAt: time.Now(),
	}
	if err := models.DB.Create(acc).Error; err != nil {
		return "", errors.New("发放访问码失败")
	}
	return token, nil
}

// CheckProjectAccess 校验访问码（未过期）
func CheckProjectAccess(projectID uint, token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	var acc OcProjectAccess
	if err := models.DB.Where("project_id = ? AND token = ?", projectID, token).First(&acc).Error; err != nil {
		return false
	}
	if time.Now().After(acc.ExpiresAt) {
		return false
	}
	return true
}

// CleanupProjectAccess 清理过期访问码（定时/启动调用）
func CleanupProjectAccess() {
	_ = models.DB.Where("expires_at < ?", time.Now()).Delete(&OcProjectAccess{}).Error
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ─── 项目 ↔ 世界/角色 归属 ───

// AttachWorldToProject 把世界书归入项目（需项目编辑权；世界本身保持原 owner）
func AttachWorldToProject(projectID, bookID, operatorID uint, isAdmin bool) error {
	var p OcProject
	if err := models.DB.Where("id = ? AND status = ?", projectID, "active").First(&p).Error; err != nil {
		return errors.New("项目不存在")
	}
	if !CanEditProject(&p, operatorID, isAdmin) {
		return errors.New("无权把世界归入该项目")
	}
	var b WorldBook
	if err := models.DB.Where("id = ? AND status = ?", bookID, "active").First(&b).Error; err != nil {
		return errors.New("世界书不存在")
	}
	if b.ProjectID == projectID {
		return nil
	}
	if err := models.DB.Model(&WorldBook{}).Where("id = ?", bookID).
		Update("project_id", projectID).Error; err != nil {
		return err
	}
	recalcProjectWorldCount(projectID)
	return nil
}

// DetachWorldFromProject 把世界移出项目
func DetachWorldFromProject(projectID, bookID, operatorID uint, isAdmin bool) error {
	var p OcProject
	if err := models.DB.Where("id = ? AND status = ?", projectID, "active").First(&p).Error; err != nil {
		return errors.New("项目不存在")
	}
	if !CanEditProject(&p, operatorID, isAdmin) {
		return errors.New("无权把世界移出该项目")
	}
	if err := models.DB.Model(&WorldBook{}).Where("id = ? AND project_id = ?", bookID, projectID).
		Update("project_id", 0).Error; err != nil {
		return err
	}
	recalcProjectWorldCount(projectID)
	return nil
}

// ProjectWorlds 项目下的世界（含跨世界链接信息由调用方拼装）
func ProjectWorlds(projectID uint) ([]WorldBook, error) {
	var list []WorldBook
	if err := models.DB.Where("project_id = ? AND status = ?", projectID, "active").
		Order("updated_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func recalcProjectWorldCount(projectID uint) {
	var cnt int64
	models.DB.Model(&WorldBook{}).Where("project_id = ? AND status = ?", projectID, "active").Count(&cnt)
	models.DB.Model(&OcProject{}).Where("id = ?", projectID).Update("world_count", cnt)
}

// SetCharacterProject 把角色卡归入项目（需项目编辑权 + 角色属于本人）
func SetCharacterProject(projectID, characterID, operatorID uint, isAdmin bool) error {
	var p OcProject
	if err := models.DB.Where("id = ? AND status = ?", projectID, "active").First(&p).Error; err != nil {
		return errors.New("项目不存在")
	}
	if !CanEditProject(&p, operatorID, isAdmin) {
		return errors.New("无权把角色归入该项目")
	}
	var c Character
	if err := models.DB.Where("id = ? AND status = ? AND user_id = ?", characterID, "active", operatorID).
		First(&c).Error; err != nil {
		return errors.New("角色卡不存在或不属于你")
	}
	return models.DB.Model(&Character{}).Where("id = ?", characterID).
		Update("project_id", projectID).Error
}

// ─── 跨世界链接 ───

// AddWorldLink 建立跨世界链接（需对来源世界有编辑权：owner/项目成员/平台）
func AddWorldLink(fromBookID, toBookID uint, label string, operatorID uint, isAdmin bool) (*OcWorldLink, error) {
	if fromBookID == 0 || toBookID == 0 || fromBookID == toBookID {
		return nil, errors.New("链接的世界无效")
	}
	var from, to WorldBook
	if err := models.DB.Where("id = ? AND status = ?", fromBookID, "active").First(&from).Error; err != nil {
		return nil, errors.New("来源世界不存在")
	}
	if err := models.DB.Where("id = ? AND status = ?", toBookID, "active").First(&to).Error; err != nil {
		return nil, errors.New("目标世界不存在")
	}
	if !canEditWorld(&from, operatorID, isAdmin) {
		return nil, errors.New("无权链接该世界")
	}
	label = strings.TrimSpace(security.SanitizeUserContent(label))
	if len([]rune(label)) > 128 {
		label = string([]rune(label)[:128])
	}
	var cnt int64
	models.DB.Model(&OcWorldLink{}).
		Where("from_book_id = ? AND to_book_id = ?", fromBookID, toBookID).Count(&cnt)
	if cnt > 0 {
		return nil, errors.New("该链接已存在")
	}
	l := &OcWorldLink{FromBookID: fromBookID, ToBookID: toBookID, Label: label, CreatedAt: time.Now()}
	if err := models.DB.Create(l).Error; err != nil {
		return nil, err
	}
	l.FromTitle, l.ToTitle = from.Title, to.Title
	return l, nil
}

// RemoveWorldLink 删除链接（需对来源世界有编辑权）
func RemoveWorldLink(linkID, operatorID uint, isAdmin bool) error {
	var l OcWorldLink
	if err := models.DB.Where("id = ?", linkID).First(&l).Error; err != nil {
		return errors.New("链接不存在")
	}
	var from WorldBook
	if err := models.DB.Where("id = ? AND status = ?", l.FromBookID, "active").First(&from).Error; err != nil {
		return errors.New("来源世界不存在")
	}
	if !canEditWorld(&from, operatorID, isAdmin) {
		return errors.New("无权删除该链接")
	}
	return models.DB.Delete(&OcWorldLink{}, "id = ?", linkID).Error
}

// ListWorldLinks 某世界的全部链接（出 + 入）
func ListWorldLinks(bookID uint) ([]OcWorldLink, error) {
	var list []OcWorldLink
	if err := models.DB.Where("from_book_id = ? OR to_book_id = ?", bookID, bookID).
		Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	titles := map[uint]string{}
	fill := func(id uint) string {
		if t, ok := titles[id]; ok {
			return t
		}
		var b WorldBook
		if err := models.DB.Select("id, title").Where("id = ?", id).First(&b).Error; err == nil {
			titles[id] = b.Title
			return b.Title
		}
		return ""
	}
	for i := range list {
		list[i].FromTitle = fill(list[i].FromBookID)
		list[i].ToTitle = fill(list[i].ToBookID)
	}
	return list, nil
}

// canEditWorld 世界编辑权：世界 owner / 所属项目编辑权 / 平台
func canEditWorld(b *WorldBook, userID uint, isAdmin bool) bool {
	if b == nil || userID == 0 {
		return false
	}
	if isAdmin || b.UserID == userID {
		return true
	}
	if b.ProjectID > 0 {
		return CanEditProject(&OcProject{ID: b.ProjectID, UserID: 0}, userID, false) ||
			IsProjectMember(b.ProjectID, userID) && ProjectRole(b.ProjectID, userID) == ProjectRoleEditor
	}
	return false
}

// ─── 迁移补充（在 Migrate 中注册） ───

// MigrateProjectModels 注册立项制相关表（由 Migrate 调用）
func MigrateProjectModels(db *gorm.DB) error {
	return db.AutoMigrate(&OcProject{}, &OcProjectMember{}, &OcProjectAccess{}, &OcWorldLink{})
}
