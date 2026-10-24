package groups

import (
	"nvs-server/models"

	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ==================== 群文件 ====================
// 安全约束（需求 §10）：
//   - 存储隔离：data/groups/<group_id>/，下载必须走鉴权路由，禁止直链静态目录；
//   - 上传校验：扩展名白名单 + 单文件 50MB + 单群总量 512MB；
//   - ZIP 炸弹检测：解压后总量 > 500MB 或解压比 > 100 → 拒绝（只读中央目录，不真解压）；
//   - 群解散/删除文件 → 软删除，7 天后由清理任务物理删除（审计留痕）。

const (
	GroupFileActive  = "active"
	GroupFileRemoved = "removed"

	// MaxGroupFileSize 单文件上限 50MB
	MaxGroupFileSize = 50 << 20
	// MaxGroupTotalSize 单群文件总量上限 512MB
	MaxGroupTotalSize = 512 << 20
	// zipBombRatio 解压比阈值
	zipBombRatio = 100
	// zipBombMaxUncompressed 解压后总量上限 500MB
	zipBombMaxUncompressed = 500 << 20
)

// GroupFile 群文件
type GroupFile struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	GroupID     uint           `gorm:"not null;index" json:"group_id"`
	UploaderID  uint           `gorm:"not null;index" json:"uploader_id"`
	Name        string         `gorm:"size:255;not null" json:"name"`  // 原始文件名（已净化）
	Size        int64          `gorm:"not null;default:0" json:"size"` // 字节
	Mime        string         `gorm:"size:128;default:''" json:"mime"`
	SHA256      string         `gorm:"size:64;default:'';index" json:"sha256"`
	StoragePath string         `gorm:"size:512;not null" json:"-"` // 相对 data 目录，绝不下发
	Status      string         `gorm:"size:16;not null;default:active;index" json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	UploaderName string `gorm:"-" json:"uploader_name,omitempty"`
}

func (GroupFile) TableName() string { return "group_files" }

// 群文件扩展名白名单（比编辑器上传宽：允许文档/压缩包，但不允许可执行/脚本）
var allowedGroupFileExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true,
	".pdf": true, ".txt": true, ".md": true, ".csv": true,
	".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
	".zip": true, ".7z": true, ".rar": true,
	".mp3": true, ".wav": true, ".m4a": true, ".mp4": true, ".webm": true,
}

// GroupFileExtAllowed 扩展名是否允许
func GroupFileExtAllowed(ext string) bool {
	return allowedGroupFileExt[strings.ToLower(ext)]
}

// SanitizeGroupFileName 文件名净化：去路径分隔符/控制字符，限长 120 rune
func SanitizeGroupFileName(name string) string {
	name = filepath.Base(strings.ReplaceAll(strings.ReplaceAll(name, "\\", "/"), "/", "_"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		name = "file"
	}
	return truncateRunes(name, 120)
}

// CheckArchiveSafety ZIP 炸弹检测（只读中央目录，不解压）
// 返回错误表示拒绝：条目数过多、解压后总量过大、解压比异常。
func CheckArchiveSafety(path string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("压缩包无法解析（可能已损坏）")
	}
	defer r.Close()

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("读取文件失败")
	}
	var total uint64
	for _, f := range r.File {
		total += f.UncompressedSize64
		if total > zipBombMaxUncompressed {
			return fmt.Errorf("压缩包解压后超过 %d MB，已拒绝（疑似 ZIP 炸弹）", zipBombMaxUncompressed>>20)
		}
		// 单条目也做保护：单文件解压后不能超上限
		if f.UncompressedSize64 > zipBombMaxUncompressed {
			return fmt.Errorf("压缩包内单个文件过大，已拒绝")
		}
	}
	if len(r.File) > 10000 {
		return fmt.Errorf("压缩包内文件数过多（%d），已拒绝", len(r.File))
	}
	if info.Size() > 0 {
		ratio := int64(total) / info.Size()
		if ratio > zipBombRatio {
			return fmt.Errorf("压缩包解压比异常（%dx），已拒绝（疑似 ZIP 炸弹）", ratio)
		}
	}
	return nil
}

// CreateGroupFile 落库（调用方已完成存储与安全校验）
func CreateGroupFile(groupID, uploaderID uint, name string, size int64, mime, sha, storagePath string) (*GroupFile, error) {
	if groupID == 0 || uploaderID == 0 {
		return nil, fmt.Errorf("参数错误")
	}
	f := &GroupFile{
		GroupID: groupID, UploaderID: uploaderID,
		Name: SanitizeGroupFileName(name), Size: size,
		Mime: mime, SHA256: sha, StoragePath: storagePath,
		Status: GroupFileActive, CreatedAt: time.Now(),
	}
	if err := models.DB.Create(f).Error; err != nil {
		return nil, fmt.Errorf("保存文件记录失败")
	}
	if err := models.DB.Model(&GroupFile{}).Where("id = ?", f.ID).Update("status", GroupFileActive).Error; err != nil {
		return nil, err
	}
	return f, nil
}

// ListGroupFiles 群文件列表
func ListGroupFiles(groupID uint, page, size int) ([]GroupFile, int64, error) {
	q := models.DB.Model(&GroupFile{}).Where("group_id = ? AND status = ?", groupID, GroupFileActive)
	var total int64
	q.Count(&total)
	var list []GroupFile
	if err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	if len(list) > 0 {
		ids := make([]uint, 0, len(list))
		for _, f := range list {
			ids = append(ids, f.UploaderID)
		}
		var users []models.User
		models.DB.Select("id, username, nickname").Where("id IN ?", ids).Find(&users)
		byID := map[uint]models.User{}
		for _, u := range users {
			byID[u.ID] = u
		}
		for i := range list {
			if u, ok := byID[list[i].UploaderID]; ok {
				list[i].UploaderName = displayName(u)
			}
		}
	}
	return list, total, nil
}

// GetGroupFile 取单个文件记录
func GetGroupFile(groupID, fileID uint) (*GroupFile, error) {
	var f GroupFile
	if err := models.DB.Where("id = ? AND group_id = ? AND status = ?", fileID, groupID, GroupFileActive).First(&f).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

// GroupFileUsedBytes 单群已用空间
func GroupFileUsedBytes(groupID uint) int64 {
	var used int64
	models.DB.Model(&GroupFile{}).Select("COALESCE(SUM(size),0)").
		Where("group_id = ? AND status = ?", groupID, GroupFileActive).Scan(&used)
	return used
}

// DeleteGroupFile 软删除（物理清理走 TTL 任务）
func DeleteGroupFile(groupID, fileID, operatorID uint, isGroupAdmin bool) error {
	f, err := GetGroupFile(groupID, fileID)
	if err != nil {
		return fmt.Errorf("文件不存在")
	}
	if f.UploaderID != operatorID && !isGroupAdmin {
		return fmt.Errorf("无权删除该文件")
	}
	return models.DB.Model(&GroupFile{}).Where("id = ?", f.ID).Update("status", GroupFileRemoved).Error
}
