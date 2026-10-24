package chaptervcs

// ==================== 提交（commit）与冲突检测 ====================
// 语义对齐 git：
//   - 每次保存 = 一次提交（父版本 + 作者 + 说明 + 快照 + 哈希）
//   - 保存携带 base_version；若服务端 head 已经不是 base_version → 冲突（拒绝覆盖）
//   - force=true 表示作者确认「以我的为准」，仍然生成一个新版本，冲突现场留痕

import (
	"errors"
	"time"

	"nvs-server/models"
)

// CommitInput 提交入参
type CommitInput struct {
	OwnerID      uint            // 主笔 ID（快照目录归属）
	Chapter      *models.Chapter // 章节（提供 ID / NovelID / ChapterNumber）
	AuthorID     uint            // 实际提交人（协作者为其本人）
	Title        string
	Content      string
	Branch       string // 空 = main
	BaseVer      int    // 客户端基于的版本号；0 = 不校验
	Force        bool   // 冲突后强制覆盖
	Message      string // 提交说明
	MergeFrom    string // 合并来源分支（合并提交时填）
	MergeFromVer int    // 合并来源版本
}

// Conflict 冲突现场：给前端做「我的版本 vs 服务端最新版」并排对比
type Conflict struct {
	BaseVersion   int    `json:"base_version"`
	ServerVersion int    `json:"server_version"`
	ServerTitle   string `json:"server_title"`
	ServerContent string `json:"server_content"`
	BaseContent   string `json:"base_content"`
	MineContent   string `json:"mine_content"`
}

// Commit 产生一次提交；发生冲突且未强制时返回 Conflict（不写库）
func Commit(in CommitInput) (*ChapterVersion, *Conflict, error) {
	if in.Chapter == nil || in.Chapter.ID == 0 {
		return nil, nil, errors.New("章节不存在")
	}
	branch := NormalizeBranch(in.Branch)
	bp, err := EnsureBranch(in.Chapter.ID, branch, in.AuthorID)
	if err != nil {
		return nil, nil, err
	}
	head := bp.HeadVer
	ownerID := in.OwnerID
	novelID := in.Chapter.NovelID

	// 冲突检测：客户端基于的版本 ≠ 服务端 head
	if in.BaseVer > 0 && head > 0 && head != in.BaseVer && !in.Force {
		headV, herr := Get(in.Chapter.ID, branch, head)
		baseV, berr := Get(in.Chapter.ID, branch, in.BaseVer)
		if herr == nil {
			serverContent := ""
			if c, cerr := Content(ownerID, novelID, headV); cerr == nil {
				serverContent = c
			}
			baseContent := ""
			if berr == nil {
				if c, cerr := Content(ownerID, novelID, baseV); cerr == nil {
					baseContent = c
				}
			}
			return nil, &Conflict{
				BaseVersion:   in.BaseVer,
				ServerVersion: head,
				ServerTitle:   headV.Title,
				ServerContent: serverContent,
				BaseContent:   baseContent,
				MineContent:   in.Content,
			}, nil
		}
	}

	// 内容未变（哈希一致）→ 不产生新版本（与 git 一致，避免刷历史）
	if head > 0 {
		if hv, herr := Get(in.Chapter.ID, branch, head); herr == nil && hv.ContentHash == hashOf(in.Content) && hv.Title == in.Title {
			hv.AuthorName = ""
			return hv, nil, nil
		}
	}

	version := head + 1
	rel, hash, words, err := writeSnapshot(ownerID, novelID, in.Chapter.ChapterNumber, branch, version, in.Content)
	if err != nil {
		return nil, nil, err
	}
	msg := in.Message
	if len([]rune(msg)) > MaxMessageLen {
		msg = string([]rune(msg)[:MaxMessageLen])
	}
	v := &ChapterVersion{
		ChapterID:    in.Chapter.ID,
		NovelID:      novelID,
		ChapterNum:   in.Chapter.ChapterNumber,
		Branch:       branch,
		Version:      version,
		ParentVer:    head,
		MergeFrom:    NormalizeBranch(in.MergeFrom),
		MergeFromVer: in.MergeFromVer,
		Title:        in.Title,
		ContentPath:  rel,
		ContentHash:  hash,
		WordCount:    words,
		Message:      msg,
		AuthorID:     in.AuthorID,
		Force:        in.Force,
		CreatedAt:    time.Now(),
	}
	if v.MergeFrom == "" {
		v.MergeFrom = ""
	}
	if err := models.DB.Create(v).Error; err != nil {
		return nil, nil, err
	}
	// 推进分支指针
	_ = models.DB.Model(&ChapterBranch{}).Where("id = ?", bp.ID).
		Updates(map[string]interface{}{"head_ver": version, "updated_at": time.Now()}).Error

	// 快照数量超限 → 清理最老的正文（元数据保留）
	PruneOld(ownerID, novelID, in.Chapter.ID, branch)
	return v, nil, nil
}
