package oc

// ==================== 角色形象图库 ====================
// 角色卡本身只有一张头像（avatar_url）和「情绪→立绘」映射（emote_map），
// 而「样子」实际上是一组图：立绘、表情、设定参考（兽设三视图/服饰纹样）、场景同人。
//
// 这里给角色开一个独立图库：
//   - 每张图挂在**一张角色卡**下，世界（project_id）从角色身上取，不信任前端传值；
//   - 每张图可记录「这张图强调的外貌特征」（traits），配合角色的 appearance / persona，
//     世界页的图库就能同时看到「长相 + 特征 + 性格」；
//   - 图片本体仍走既有 `POST /api/upload`（已有大小/类型校验），本包只管注册表——
//     不重复实现「能传什么」的口径（与 emote.go 同样的取舍）。

import (
	"errors"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/security"

	"gorm.io/gorm"
)

// 图片分类
const (
	ArtPortrait = "portrait" // 立绘 / 全身
	ArtEmote    = "emote"    // 表情 / 半身
	ArtRef      = "ref"      // 设定参考（兽设三视图、服饰、纹样）
	ArtScene    = "scene"    // 场景 / 同人
)

const (
	MaxArtTitleRunes = 60
	MaxArtTraitsLen  = 240
	MaxArtNoteRunes  = 1000
	MaxArtsPerChar   = 60
)

// CharacterArt 角色形象图（一张角色卡下可有多张）
type CharacterArt struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	CharacterID uint   `gorm:"not null;index" json:"character_id"`
	ProjectID   uint   `gorm:"not null;default:0;index" json:"project_id"` // 冗余自角色，便于世界页一次查全
	UserID      uint   `gorm:"not null;index" json:"user_id"`              // 上传者
	URL         string `gorm:"size:512;not null" json:"url"`
	Title       string `gorm:"size:64;default:''" json:"title"` // 图片标题（如「夏季常服」）
	Kind        string `gorm:"size:16;not null;default:portrait" json:"kind"`
	Traits      string `gorm:"size:256;default:''" json:"traits"` // 本图强调的外貌特征
	Note        string `gorm:"type:text" json:"note"`             // 画师 / 授权 / 备注
	Sort        int    `gorm:"not null;default:0" json:"sort"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (CharacterArt) TableName() string { return "oc_character_arts" }

// ArtInput 图库写入入参
type ArtInput struct {
	URL    string `json:"url"`
	Title  string `json:"title"`
	Kind   string `json:"kind"`
	Traits string `json:"traits"`
	Note   string `json:"note"`
	Sort   int    `json:"sort"`
}

// NormalizeArtKind 分类白名单（缺省立绘）
func NormalizeArtKind(k string) string {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case ArtEmote:
		return ArtEmote
	case ArtRef:
		return ArtRef
	case ArtScene:
		return ArtScene
	default:
		return ArtPortrait
	}
}

func normalizeArtInput(in *ArtInput) {
	in.URL = strings.TrimSpace(in.URL)
	in.Title = strings.TrimSpace(security.SanitizeUserContent(in.Title))
	in.Traits = strings.TrimSpace(security.SanitizeUserContent(in.Traits))
	in.Note = strings.TrimSpace(security.SanitizeUserContent(in.Note))
	in.Kind = NormalizeArtKind(in.Kind)
}

func validateArtInput(in ArtInput) error {
	if err := ValidateEmoteURL(in.URL); err != nil {
		return err // 复用同一套协议白名单（拒绝 javascript:/data: 等）
	}
	if n := len([]rune(in.Title)); n > MaxArtTitleRunes {
		return errors.New("图片标题最长 60 个字")
	}
	if n := len([]rune(in.Traits)); n > MaxArtTraitsLen {
		return errors.New("特征描述最长 240 个字")
	}
	if n := len([]rune(in.Note)); n > MaxArtNoteRunes {
		return errors.New("备注最长 1000 个字")
	}
	return nil
}

// canEditArt 图库写权限：角色所有者 / 项目 owner·editor / 平台管理员
func canEditArt(c *Character, userID uint, isAdmin bool) bool {
	if isAdmin || c.UserID == userID {
		return true
	}
	if c.ProjectID > 0 {
		var p OcProject
		if err := models.DB.Where("id = ? AND status = ?", c.ProjectID, "active").First(&p).Error; err == nil {
			return CanEditProject(&p, userID, isAdmin)
		}
	}
	return false
}

// ListCharacterArts 某张角色卡的全部形象图（按 sort、创建时间升序）
func ListCharacterArts(characterID uint) ([]CharacterArt, error) {
	var list []CharacterArt
	if err := models.DB.Where("character_id = ?", characterID).
		Order("sort ASC, id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ListProjectArts 某个世界下的全部形象图（世界页一次拉全，避免按角色 N 次请求）
func ListProjectArts(projectID uint) ([]CharacterArt, error) {
	var list []CharacterArt
	if err := models.DB.Where("project_id = ?", projectID).
		Order("character_id ASC, sort ASC, id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// CreateCharacterArt 新增一张形象图
func CreateCharacterArt(operatorID uint, characterID uint, isAdmin bool, in ArtInput) (*CharacterArt, error) {
	if operatorID == 0 {
		return nil, errors.New("请先登录")
	}
	c, err := GetCharacterByID(characterID)
	if err != nil {
		return nil, err
	}
	if !canEditArt(c, operatorID, isAdmin) {
		return nil, errors.New("无权为该角色添加形象图")
	}
	normalizeArtInput(&in)
	if err := validateArtInput(in); err != nil {
		return nil, err
	}
	var count int64
	if err := models.DB.Model(&CharacterArt{}).Where("character_id = ?", characterID).Count(&count).Error; err != nil {
		return nil, err
	}
	if count >= MaxArtsPerChar {
		return nil, errors.New("该角色的形象图已达上限（60 张）")
	}
	a := CharacterArt{
		CharacterID: c.ID,
		ProjectID:   c.ProjectID,
		UserID:      operatorID,
		URL:         in.URL,
		Title:       in.Title,
		Kind:        in.Kind,
		Traits:      in.Traits,
		Note:        in.Note,
		Sort:        in.Sort,
	}
	if err := models.DB.Create(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

// UpdateCharacterArt 修改图（只改元信息与地址，不改归属）
func UpdateCharacterArt(operatorID, id uint, isAdmin bool, in ArtInput) (*CharacterArt, error) {
	var a CharacterArt
	if err := models.DB.Where("id = ?", id).First(&a).Error; err != nil {
		return nil, errors.New("形象图不存在")
	}
	c, err := GetCharacterByID(a.CharacterID)
	if err != nil {
		return nil, err
	}
	if !canEditArt(c, operatorID, isAdmin) {
		return nil, errors.New("无权修改该形象图")
	}
	normalizeArtInput(&in)
	if err := validateArtInput(in); err != nil {
		return nil, err
	}
	a.URL = in.URL
	a.Title = in.Title
	a.Kind = in.Kind
	a.Traits = in.Traits
	a.Note = in.Note
	a.Sort = in.Sort
	a.UpdatedAt = time.Now()
	if err := models.DB.Save(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

// DeleteCharacterArt 删除（物理删除：图库是纯展示资产，不像角色卡需要留证）
func DeleteCharacterArt(operatorID, id uint, isAdmin bool) error {
	var a CharacterArt
	if err := models.DB.Where("id = ?", id).First(&a).Error; err != nil {
		return errors.New("形象图不存在")
	}
	c, err := GetCharacterByID(a.CharacterID)
	if err != nil {
		return err
	}
	if !canEditArt(c, operatorID, isAdmin) {
		return errors.New("无权删除该形象图")
	}
	return models.DB.Delete(&CharacterArt{}, id).Error
}

// MigrateArtModels 建表（AutoMigrate 幂等）
func MigrateArtModels(db *gorm.DB) error {
	return db.AutoMigrate(&CharacterArt{})
}
