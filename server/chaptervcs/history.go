package chaptervcs

// ==================== 版本历史：head / 列表 / 读取 / 回滚 / 删除 ====================

import (
	"errors"
	"time"

	"nvs-server/models"
)

// ErrNoVersion 该分支还没有任何提交
var ErrNoVersion = errors.New("该分支暂无版本")

// EnsureBranch 保证分支指针存在（幂等）
func EnsureBranch(chapterID uint, branch string, authorID uint) (*ChapterBranch, error) {
	name := NormalizeBranch(branch)
	var b ChapterBranch
	err := models.DB.Where("chapter_id = ? AND name = ?", chapterID, name).First(&b).Error
	if err == nil {
		return &b, nil
	}
	b = ChapterBranch{
		ChapterID: chapterID, Name: name, HeadVer: 0,
		AuthorID: authorID, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := models.DB.Create(&b).Error; err != nil {
		// 并发创建：再查一次
		var exist ChapterBranch
		if e2 := models.DB.Where("chapter_id = ? AND name = ?", chapterID, name).First(&exist).Error; e2 == nil {
			return &exist, nil
		}
		return nil, err
	}
	return &b, nil
}

// Head 分支头版本号（无提交返回 0）
func Head(chapterID uint, branch string) int {
	var b ChapterBranch
	if err := models.DB.Where("chapter_id = ? AND name = ?", chapterID, NormalizeBranch(branch)).First(&b).Error; err != nil {
		return 0
	}
	return b.HeadVer
}

// HeadVersion 分支头版本对象
func HeadVersion(chapterID uint, branch string) (*ChapterVersion, error) {
	ver := Head(chapterID, branch)
	if ver <= 0 {
		return nil, ErrNoVersion
	}
	return Get(chapterID, NormalizeBranch(branch), ver)
}

// Get 取指定版本（按 章节 + 分支 + 版本号）
func Get(chapterID uint, branch string, version int) (*ChapterVersion, error) {
	var v ChapterVersion
	err := models.DB.Where("chapter_id = ? AND branch = ? AND version = ?", chapterID, NormalizeBranch(branch), version).First(&v).Error
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// List 分支上的版本列表（倒序：新 → 旧）
func List(chapterID uint, branch string) ([]ChapterVersion, error) {
	name := NormalizeBranch(branch)
	var list []ChapterVersion
	if err := models.DB.Where("chapter_id = ? AND branch = ?", chapterID, name).
		Order("version DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	fillAuthorNames(list)
	head := Head(chapterID, name)
	for i := range list {
		if list[i].Version == head {
			list[i].IsHead = true
		}
	}
	return list, nil
}

// Branches 章节的分支列表（含 head 版本号）
func Branches(chapterID uint) ([]ChapterBranch, error) {
	var list []ChapterBranch
	if err := models.DB.Where("chapter_id = ?", chapterID).Order("id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// fillAuthorNames 批量填作者展示名
func fillAuthorNames(list []ChapterVersion) {
	if len(list) == 0 {
		return
	}
	ids := map[uint]bool{}
	for _, v := range list {
		ids[v.AuthorID] = true
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
	for i := range list {
		list[i].AuthorName = nameOf[list[i].AuthorID]
	}
}

// Content 读取某版本的正文快照（快照被清理时返回空串）
func Content(ownerID, novelID uint, v *ChapterVersion) (string, error) {
	if v == nil {
		return "", errors.New("版本不存在")
	}
	if v.Deleted {
		return "", nil
	}
	return readSnapshot(ownerID, novelID, *v)
}

// Revert 回滚：以历史版本内容再提交一次（不删历史，保持可追溯）
func Revert(ownerID uint, novelID uint, chapter *models.Chapter, target *ChapterVersion, authorID uint) (*ChapterVersion, error) {
	content, err := Content(ownerID, novelID, target)
	if err != nil {
		return nil, err
	}
	v, _, err := Commit(CommitInput{
		Chapter:  chapter,
		AuthorID: authorID,
		Title:    target.Title,
		Content:  content,
		Branch:   target.Branch,
		Message:  "回滚到 v" + itoa(target.Version),
		OwnerID:  ownerID,
	})
	return v, err
}

// DeleteVersion 作者删除历史快照：只清正文文件，保留元数据（作者/时间/哈希/说明）以便追溯。
// 约束：不能删除分支头（需先回滚或切分支），避免正文出现「有章节无内容」的空洞。
func DeleteVersion(ownerID, novelID, chapterID uint, branch string, version int) error {
	name := NormalizeBranch(branch)
	if version == Head(chapterID, name) {
		return errors.New("不能删除分支最新版本，请先回滚到其他版本")
	}
	v, err := Get(chapterID, name, version)
	if err != nil {
		return errors.New("版本不存在")
	}
	if v.Deleted {
		return nil
	}
	if err := deleteSnapshot(ownerID, novelID, *v); err != nil {
		return err
	}
	return models.DB.Model(&ChapterVersion{}).Where("id = ?", v.ID).
		Updates(map[string]interface{}{"deleted": true, "content_path": ""}).Error
}

// PruneOld 快照数量超过上限时清理最老的（仅清正文，元数据保留）
func PruneOld(ownerID, novelID, chapterID uint, branch string) {
	name := NormalizeBranch(branch)
	var list []ChapterVersion
	if err := models.DB.Where("chapter_id = ? AND branch = ? AND deleted = ?", chapterID, name, false).
		Order("version ASC").Find(&list).Error; err != nil || len(list) <= MaxSnapshotsPerChapter {
		return
	}
	excess := len(list) - MaxSnapshotsPerChapter
	for i := 0; i < excess; i++ {
		_ = deleteSnapshot(ownerID, novelID, list[i])
		_ = models.DB.Model(&ChapterVersion{}).Where("id = ?", list[i].ID).
			Updates(map[string]interface{}{"deleted": true, "content_path": ""}).Error
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		b[p] = '-'
	}
	return string(b[p:])
}
