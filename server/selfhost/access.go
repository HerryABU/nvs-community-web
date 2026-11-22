package selfhost

// 源码与可执行文件的访问授权。
// 铁律：公开项目 ≠ 公开源码。默认任何非所有者/非管理员都不能下载，
// 必须显式开启共享开关或逐用户授权。

import (
	"errors"
	"strings"
	"time"

	"nvs-server/models"
)

func normalizeGrantKind(kind string) (string, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind != "source" && kind != "binary" && kind != "all" {
		return "", errors.New("授权类型只能是 source / binary / all")
	}
	return kind, nil
}

// CanDownload 下载总闸。匿名一律拒绝；所有者与管理员不受限。
func CanDownload(p *Project, userID uint, isAdminUser bool, kind string) bool {
	if p == nil || (kind != "source" && kind != "binary") {
		return false
	}
	if isAdminUser || (userID > 0 && p.UserID == userID) {
		return true
	}
	// 匿名用户即使项目公开也不能拿源码/产物。
	if userID == 0 || p.ReviewStatus != ReviewApproved {
		return false
	}
	if (kind == "source" && p.SourceShared) || (kind == "binary" && p.BinaryShared) {
		return true
	}
	var n int64
	models.DB.Model(&FileGrant{}).
		Where("project_id = ? AND user_id = ? AND status = ? AND kind IN ?", p.ID, userID, "approved", []string{kind, "all"}).
		Count(&n)
	return n > 0
}

// PermissionsFor 一次算出某用户对某项目两类文件的权限。
func PermissionsFor(p *Project, userID uint, isAdminUser bool) (bool, bool) {
	return CanDownload(p, userID, isAdminUser, "source"), CanDownload(p, userID, isAdminUser, "binary")
}

// ApprovedGrantKinds 批量取某用户在一批项目上已获批的授权类型，避免列表页 N+1 查询。
func ApprovedGrantKinds(userID uint, projectIDs []uint) map[uint]map[string]bool {
	out := make(map[uint]map[string]bool, len(projectIDs))
	if userID == 0 || len(projectIDs) == 0 {
		return out
	}
	var grants []FileGrant
	models.DB.Where("user_id = ? AND status = ? AND project_id IN ?", userID, "approved", projectIDs).Find(&grants)
	for _, g := range grants {
		if out[g.ProjectID] == nil {
			out[g.ProjectID] = map[string]bool{}
		}
		out[g.ProjectID][g.Kind] = true
	}
	return out
}

// GrantStatusFor 取某用户在某项目上的授权记录（用于前端显示「申请中 / 已获批 / 被驳回」）。
func GrantStatusFor(projectID, userID uint) []FileGrant {
	var list []FileGrant
	if userID > 0 {
		models.DB.Where("project_id = ? AND user_id = ?", projectID, userID).Order("id DESC").Find(&list)
	}
	return list
}

func RequestGrant(projectID, userID uint, kind, note string) (*FileGrant, error) {
	if userID == 0 {
		return nil, errors.New("请先登录")
	}
	kind, err := normalizeGrantKind(kind)
	if err != nil {
		return nil, err
	}
	p, err := GetProjectRaw(projectID)
	if err != nil {
		return nil, errors.New("项目不存在")
	}
	if p.UserID == userID {
		return nil, errors.New("这是你自己的项目，无需申请")
	}
	if p.ReviewStatus != ReviewApproved {
		return nil, errors.New("项目尚未审核通过")
	}
	if len([]rune(note)) > 500 {
		return nil, errors.New("说明过长")
	}
	// 同一用户同一类型只保留一条：重复申请改为重新提交（重置审批状态）。
	var g FileGrant
	err = models.DB.Where("project_id = ? AND user_id = ? AND kind = ?", projectID, userID, kind).First(&g).Error
	if err == nil {
		if g.Status == "pending" {
			return nil, errors.New("申请正在处理中")
		}
		g.Status, g.Note, g.ReviewedBy, g.ReviewedAt = "pending", note, 0, nil
		if err := models.DB.Save(&g).Error; err != nil {
			return nil, err
		}
		return &g, nil
	}
	g = FileGrant{ProjectID: projectID, UserID: userID, Kind: kind, Status: "pending", Note: strings.TrimSpace(note)}
	if err := models.DB.Create(&g).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

func ListMyGrants(userID uint) ([]FileGrant, error) {
	var list []FileGrant
	err := models.DB.Where("user_id = ?", userID).Order("id DESC").Limit(100).Find(&list).Error
	return list, err
}

func ListProjectGrants(projectID uint) ([]FileGrant, error) {
	var list []FileGrant
	err := models.DB.Where("project_id = ?", projectID).Order("id DESC").Find(&list).Error
	return list, err
}

func ReviewGrant(id, operatorID uint, approve bool) error {
	var g FileGrant
	if err := models.DB.First(&g, id).Error; err != nil {
		return errors.New("申请不存在")
	}
	if g.Status != "pending" {
		return errors.New("该申请已处理")
	}
	p, err := GetProjectRaw(g.ProjectID)
	if err != nil {
		return err
	}
	if p.UserID != operatorID {
		return errors.New("只有项目所有者可以审批")
	}
	g.Status = "rejected"
	if approve {
		g.Status = "approved"
	}
	g.ReviewedBy, g.ReviewedAt = operatorID, timePtr(time.Now())
	return models.DB.Save(&g).Error
}

func RevokeGrant(id, operatorID uint) error {
	var g FileGrant
	if err := models.DB.First(&g, id).Error; err != nil {
		return errors.New("授权不存在")
	}
	p, err := GetProjectRaw(g.ProjectID)
	if err != nil {
		return err
	}
	if p.UserID != operatorID {
		return errors.New("只有项目所有者可以撤销")
	}
	return models.DB.Delete(&FileGrant{}, id).Error
}

func SetSharing(id, operatorID uint, source, binary bool, isAdminUser bool) error {
	p, err := GetProjectRaw(id)
	if err != nil {
		return err
	}
	if !isAdminUser && p.UserID != operatorID {
		return errors.New("只有项目所有者可以设置共享")
	}
	p.SourceShared, p.BinaryShared = source, binary
	return models.DB.Model(&Project{}).Where("id = ?", id).
		Select("source_shared", "binary_shared").Updates(p).Error
}
