package selfhost

// 用户「需要的运行时与版本」申请：管理员查看后可以按原样添加，
// 也可以根据代码说明改成另一个语言/版本再落地。

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"nvs-server/models"
)

type RuntimeRequest struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	UserID        uint       `gorm:"not null;index" json:"user_id"`
	ProjectID     uint       `gorm:"default:0;index" json:"project_id"`
	Language      string     `gorm:"size:32;not null" json:"language"`
	Version       string     `gorm:"size:32;not null" json:"version"`
	Note          string     `gorm:"size:1000;default:''" json:"note"`
	Status        string     `gorm:"size:16;default:pending;index" json:"status"`
	FinalLanguage string     `gorm:"size:32;default:''" json:"final_language"`
	FinalVersion  string     `gorm:"size:32;default:''" json:"final_version"`
	AdminNote     string     `gorm:"size:512;default:''" json:"admin_note"`
	ReviewedBy    uint       `gorm:"default:0" json:"reviewed_by"`
	ReviewedAt    *time.Time `json:"reviewed_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (RuntimeRequest) TableName() string { return "selfhost_runtime_requests" }

func MigrateRuntimeRequests(db *gorm.DB) error { return db.AutoMigrate(&RuntimeRequest{}) }

func CreateRuntimeRequest(userID uint, in RuntimeRequest) (*RuntimeRequest, error) {
	in.ID, in.UserID = 0, userID
	in.Language = normalizeLang(in.Language)
	in.Version = strings.TrimSpace(in.Version)
	in.Note = strings.TrimSpace(in.Note)
	if !langRe.MatchString(in.Language) || !versionRe.MatchString(in.Version) {
		return nil, errors.New("语言或版本格式不合法")
	}
	if len([]rune(in.Note)) > 1000 {
		return nil, errors.New("说明过长")
	}
	var n int64
	models.DB.Model(&RuntimeRequest{}).Where("user_id = ? AND language = ? AND version = ? AND status = ?", userID, in.Language, in.Version, "pending").Count(&n)
	if n > 0 {
		return nil, errors.New("相同申请正在处理中")
	}
	in.Status = "pending"
	if err := models.DB.Create(&in).Error; err != nil {
		return nil, err
	}
	return &in, nil
}

func ListMyRuntimeRequests(userID uint) ([]RuntimeRequest, error) {
	var list []RuntimeRequest
	err := models.DB.Where("user_id = ?", userID).Order("id DESC").Limit(100).Find(&list).Error
	return list, err
}

func ListRuntimeRequests(status string) ([]RuntimeRequest, error) {
	var list []RuntimeRequest
	q := models.DB.Model(&RuntimeRequest{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Order("id DESC").Limit(200).Find(&list).Error
	return list, err
}

// ReviewRuntimeRequest 审批；approve 时可按管理员判定修改语言/版本并直接落地运行时。
func ReviewRuntimeRequest(id, operatorID uint, approve bool, language, version, execPath, note string) (*Runtime, error) {
	var req RuntimeRequest
	if err := models.DB.First(&req, id).Error; err != nil {
		return nil, errors.New("申请不存在")
	}
	if req.Status != "pending" {
		return nil, errors.New("该申请已处理")
	}
	req.AdminNote = strings.TrimSpace(note)
	req.ReviewedBy, req.ReviewedAt = operatorID, timePtr(time.Now())
	if !approve {
		req.Status = "rejected"
		if err := models.DB.Save(&req).Error; err != nil {
			return nil, err
		}
		return nil, nil
	}
	// 管理员可改语言/版本：这就是「根据代码修改对方需要的运行时版本」。
	finalLang := normalizeLang(language)
	if finalLang == "" {
		finalLang = req.Language
	}
	finalVersion := strings.TrimSpace(version)
	if finalVersion == "" {
		finalVersion = req.Version
	}
	req.Status, req.FinalLanguage, req.FinalVersion = "approved", finalLang, finalVersion
	if err := models.DB.Save(&req).Error; err != nil {
		return nil, err
	}
	var dup int64
	models.DB.Model(&Runtime{}).Where("language = ? AND version = ?", finalLang, finalVersion).Count(&dup)
	if dup > 0 {
		return nil, nil // 已存在，无需重复添加
	}
	kind := "interpreted"
	if IsCompiledLanguage(finalLang) {
		kind = "compiled"
	}
	return CreateRuntime(Runtime{
		Language: finalLang, Version: finalVersion, Label: req.Note,
		Kind: kind, ExecPath: strings.TrimSpace(execPath),
		InstallHint: fmt.Sprintf("由用户申请 #%d 添加；原说明：%s", id, req.Note),
		Enabled:     true,
	})
}
