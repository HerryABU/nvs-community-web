//go:build gitvcs

package chaptervcs

// ==================== 真实 git 仓库后端（go-git） ====================
// 构建：`go build -tags gitvcs`；运行时：`NVS_VCS_BACKEND=git`
//
// 布局：<NovelDataDir>/authors/{主笔ID}/{作品ID}/.vcsrepo（git 仓库，工作区即该目录）
//       章节文件名 chapters/{章节号}.md，分支名 ch{章节号}/{分支名}
//
// 语义对齐：
//   commit = 一次章节保存（author = 提交人，message = 提交说明）
//   version = 该分支上第 N 个提交（从 1 开始，按提交顺序）
//   merge = 三方合并后生成双父提交
//   revert = 以历史内容再提交一次
//   delete = git 历史不可变 → 不支持（返回明确错误，引导用 revert）

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"nvs-server/config"
	"nvs-server/models"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type gitBackend struct{}

func newGitBackend() (Backend, error) { return gitBackend{}, nil }

func (gitBackend) Name() string { return "git" }

// ─── 仓库与文件 ───

func repoDir(ownerID, novelID uint) string {
	return filepath.Join(config.NovelDataDir, "authors", fmt.Sprintf("%d", ownerID), fmt.Sprintf("%d", novelID), ".vcsrepo")
}

func chapterRel(num int) string { return fmt.Sprintf("chapters/%d.md", num) }

func gitBranchName(chapterNum int, branch string) string {
	return fmt.Sprintf("ch%d/%s", chapterNum, NormalizeBranch(branch))
}

// openOrCreate 打开（或初始化）作品级仓库
func openOrCreate(ownerID, novelID uint) (*git.Repository, error) {
	dir := repoDir(ownerID, novelID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	repo, err := git.PlainOpen(dir)
	if err == git.ErrRepositoryNotExists {
		repo, err = git.PlainInit(dir, false)
		if err != nil {
			return nil, err
		}
		// 首次提交前先用空提交建立 HEAD，避免空仓库分支指针缺失
		if err := emptyInit(repo, dir); err != nil {
			return nil, err
		}
	}
	return repo, err
}

func emptyInit(repo *git.Repository, dir string) error {
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# NVS 章节版本仓库\n\n由 chaptervcs 的 git 后端维护，请勿手工修改。\n"), 0644); err != nil {
		return err
	}
	if _, err := wt.Add("README.md"); err != nil {
		return err
	}
	_, err = wt.Commit("init", &git.CommitOptions{
		Author: &object.Signature{Name: "nvs", Email: "nvs@local", When: time.Now()},
	})
	return err
}

// branchRef 章节分支的引用名
func branchRef(chapterNum int, branch string) plumbing.ReferenceName {
	return plumbing.NewBranchReferenceName(gitBranchName(chapterNum, branch))
}

// commitsOn 取该分支上的提交（旧 → 新）
func commitsOn(repo *git.Repository, chapterNum int, branch string) ([]*object.Commit, error) {
	ref, err := repo.Reference(branchRef(chapterNum, branch), false)
	if err != nil {
		return nil, ErrNoVersion
	}
	iter, err := repo.Log(&git.LogOptions{From: ref.Hash(), Order: git.LogOrderDFS})
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	var all []*object.Commit
	_ = iter.ForEach(func(c *object.Commit) error {
		// 只保留属于本分支的提交（含从分叉点继承的父链）
		all = append(all, c)
		return nil
	})
	// Log 默认新 → 旧，翻转成旧 → 新。
	// 注意：git 时间戳只到秒，同一秒内的多个提交不能用时间排序，必须依赖 Log 的拓扑顺序。
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	out := make([]*object.Commit, 0, len(all))
	for _, c := range all {
		if strings.HasPrefix(c.Message, "init\n") || c.Message == "init" {
			continue
		}
		// 只保留改动过本章文件的提交
		if fileChanged(repo, c, chapterNum) {
			out = append(out, c)
		}
	}
	return out, nil
}

func fileChanged(repo *git.Repository, c *object.Commit, chapterNum int) bool {
	if c.NumParents() == 0 {
		// 首次提交：直接看树里有没有目标文件
		if _, err := c.File(chapterRel(chapterNum)); err == nil {
			return true
		}
		return false
	}
	parent, err := c.Parent(0)
	if err != nil {
		return false
	}
	patch, err := c.Patch(parent)
	if err != nil {
		return false
	}
	target := chapterRel(chapterNum)
	for _, fp := range patch.FilePatches() {
		from, to := fp.Files()
		if (from != nil && from.Path() == target) || (to != nil && to.Path() == target) {
			return true
		}
	}
	return false
}

// fileAt 取某提交里本章文件内容
func fileAt(c *object.Commit, chapterNum int) (string, error) {
	f, err := c.File(chapterRel(chapterNum))
	if err != nil {
		return "", err
	}
	return f.Contents()
}

func authorNameOf(c *object.Commit) string { return c.Author.Name }

func userDisplay(uid uint) string {
	if u, err := models.GetUserByID(uid); err == nil && u != nil {
		if u.Nickname != "" {
			return u.Nickname
		}
		return u.Username
	}
	return fmt.Sprintf("%d", uid)
}

// ─── Backend 实现 ───

func (g gitBackend) Commit(in CommitInput) (*ChapterVersion, *Conflict, error) {
	if in.Chapter == nil || in.Chapter.ID == 0 {
		return nil, nil, errors.New("章节不存在")
	}
	branch := NormalizeBranch(in.Branch)
	num := in.Chapter.ChapterNumber
	repo, err := openOrCreate(in.OwnerID, in.Chapter.NovelID)
	if err != nil {
		return nil, nil, err
	}
	commits, _ := commitsOn(repo, num, branch)
	head := len(commits)

	// 冲突检测：与默认后端同语义
	if in.BaseVer > 0 && head > 0 && head != in.BaseVer && !in.Force {
		server := ""
		if head <= len(commits) {
			server, _ = fileAt(commits[head-1], num)
		}
		base := ""
		if in.BaseVer <= len(commits) {
			base, _ = fileAt(commits[in.BaseVer-1], num)
		}
		return nil, &Conflict{
			BaseVersion:   in.BaseVer,
			ServerVersion: head,
			ServerTitle:   in.Title,
			ServerContent: server,
			BaseContent:   base,
			MineContent:   in.Content,
		}, nil
	}
	// 内容未变 → 不产生新提交
	if head > 0 {
		if cur, cerr := fileAt(commits[head-1], num); cerr == nil && cur == in.Content {
			v, verr := g.Get(in.Chapter.ID, branch, head)
			if verr == nil {
				return v, nil, nil
			}
		}
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, nil, err
	}
	// 切到目标分支（不存在则基于当前 HEAD 新建）
	ref := branchRef(num, branch)
	if _, err := repo.Reference(ref, false); err != nil {
		if err := wt.Checkout(&git.CheckoutOptions{Branch: ref, Create: true}); err != nil {
			return nil, nil, err
		}
	} else if err := wt.Checkout(&git.CheckoutOptions{Branch: ref}); err != nil {
		return nil, nil, err
	}
	abs := filepath.Join(repoDir(in.OwnerID, in.Chapter.NovelID), filepath.FromSlash(chapterRel(num)))
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(abs, []byte(in.Content), 0644); err != nil {
		return nil, nil, err
	}
	if _, err := wt.Add(chapterRel(num)); err != nil {
		return nil, nil, err
	}
	msg := in.Message
	if strings.TrimSpace(msg) == "" {
		msg = "更新章节"
	}
	hash, err := wt.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{Name: userDisplay(in.AuthorID), Email: fmt.Sprintf("%d@nvs.local", in.AuthorID), When: time.Now()},
	})
	if err != nil {
		return nil, nil, err
	}
	// 同步 DB 分支指针（供前端列表与权限侧使用）
	bp, _ := EnsureBranch(in.Chapter.ID, branch, in.AuthorID)
	version := head + 1
	_ = models.DB.Model(&ChapterBranch{}).Where("id = ?", bp.ID).
		Updates(map[string]interface{}{"head_ver": version, "updated_at": time.Now()}).Error
	return &ChapterVersion{
		ChapterID:   in.Chapter.ID,
		NovelID:     in.Chapter.NovelID,
		ChapterNum:  num,
		Branch:      branch,
		Version:     version,
		ParentVer:   head,
		Title:       in.Title,
		ContentHash: hash.String(),
		WordCount:   len([]rune(in.Content)),
		Message:     msg,
		AuthorID:    in.AuthorID,
		Force:       in.Force,
		CreatedAt:   time.Now(),
	}, nil, nil
}

func (g gitBackend) List(chapterID uint, branch string) ([]ChapterVersion, error) {
	novel, chapter, err := loadRefs(chapterID)
	if err != nil {
		return nil, err
	}
	repo, err := openOrCreate(novel.AuthorID, novel.ID)
	if err != nil {
		return nil, err
	}
	name := NormalizeBranch(branch)
	commits, err := commitsOn(repo, chapter.ChapterNumber, name)
	if err != nil {
		return []ChapterVersion{}, nil
	}
	out := make([]ChapterVersion, 0, len(commits))
	for i, c := range commits {
		content, _ := fileAt(c, chapter.ChapterNumber)
		out = append(out, ChapterVersion{
			ChapterID:   chapterID,
			NovelID:     novel.ID,
			ChapterNum:  chapter.ChapterNumber,
			Branch:      name,
			Version:     i + 1,
			ParentVer:   i,
			Title:       chapter.Title,
			ContentHash: c.Hash.String(),
			WordCount:   len([]rune(content)),
			Message:     strings.TrimSpace(c.Message),
			AuthorID:    0,
			AuthorName:  authorNameOf(c),
			CreatedAt:   c.Committer.When,
		})
	}
	// 新 → 旧
	sort.Slice(out, func(i, j int) bool { return out[i].Version > out[j].Version })
	if len(out) > 0 {
		out[0].IsHead = true
	}
	return out, nil
}

func (g gitBackend) Get(chapterID uint, branch string, version int) (*ChapterVersion, error) {
	list, err := g.List(chapterID, branch)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].Version == version {
			return &list[i], nil
		}
	}
	return nil, ErrNoVersion
}

func (g gitBackend) Head(chapterID uint, branch string) int {
	novel, chapter, err := loadRefs(chapterID)
	if err != nil {
		return 0
	}
	repo, err := openOrCreate(novel.AuthorID, novel.ID)
	if err != nil {
		return 0
	}
	commits, err := commitsOn(repo, chapter.ChapterNumber, NormalizeBranch(branch))
	if err != nil {
		return 0
	}
	return len(commits)
}

func (g gitBackend) Content(ownerID, novelID uint, v *ChapterVersion) (string, error) {
	if v == nil {
		return "", errors.New("版本不存在")
	}
	repo, err := openOrCreate(ownerID, novelID)
	if err != nil {
		return "", err
	}
	commits, err := commitsOn(repo, v.ChapterNum, v.Branch)
	if err != nil || v.Version <= 0 || v.Version > len(commits) {
		return "", nil
	}
	return fileAt(commits[v.Version-1], v.ChapterNum)
}

func (g gitBackend) Revert(ownerID, novelID uint, chapter *models.Chapter, target *ChapterVersion, authorID uint) (*ChapterVersion, error) {
	content, err := g.Content(ownerID, novelID, target)
	if err != nil {
		return nil, err
	}
	v, _, err := g.Commit(CommitInput{
		OwnerID:  ownerID,
		Chapter:  chapter,
		AuthorID: authorID,
		Title:    target.Title,
		Content:  content,
		Branch:   target.Branch,
		Message:  "回滚到 v" + itoa(target.Version),
	})
	return v, err
}

// DeleteVersion git 历史不可变 → 不支持删除（引导用回滚）
func (g gitBackend) DeleteVersion(ownerID, novelID, chapterID uint, branch string, version int) error {
	return errors.New("git 后端的历史不可删除（提交即留痕），如需撤销请用「回滚」生成反向版本")
}

func (g gitBackend) Branches(chapterID uint) ([]ChapterBranch, error) {
	return branchesDefault(chapterID)
}

func (g gitBackend) CreateBranch(ownerID, novelID uint, chapter *models.Chapter, name, fromBranch string, fromVer int, authorID uint) (*ChapterVersion, error) {
	newName := NormalizeBranch(name)
	if newName == DefaultBranch {
		return nil, errors.New("不能创建与默认分支同名的分支")
	}
	src := NormalizeBranch(fromBranch)
	if src == "" {
		src = DefaultBranch
	}
	content := ""
	title := chapter.Title
	if fromVer > 0 {
		if v, err := g.Get(chapter.ID, src, fromVer); err == nil {
			content, _ = g.Content(ownerID, novelID, v)
			if v.Title != "" {
				title = v.Title
			}
		}
	}
	_, _ = EnsureBranch(chapter.ID, newName, authorID)
	_ = models.DB.Model(&ChapterBranch{}).Where("chapter_id = ? AND name = ?", chapter.ID, newName).
		Updates(map[string]interface{}{"from_ver": fromVer, "from_branch": src}).Error
	return g.mustCommit(ownerID, chapter, authorID, title, content, newName, "从 "+src+" v"+itoa(fromVer)+" 开分支")
}

func (g gitBackend) mustCommit(ownerID uint, chapter *models.Chapter, authorID uint, title, content, branch, msg string) (*ChapterVersion, error) {
	v, _, err := g.Commit(CommitInput{
		OwnerID: ownerID, Chapter: chapter, AuthorID: authorID,
		Title: title, Content: content, Branch: branch, Message: msg,
	})
	return v, err
}

func (g gitBackend) Merge(ownerID, novelID uint, chapter *models.Chapter, fromBranch, intoBranch string, authorID uint, message string) (*MergeResult, error) {
	from, into := NormalizeBranch(fromBranch), NormalizeBranch(intoBranch)
	if from == into {
		return nil, errors.New("源分支与目标分支相同")
	}
	res := &MergeResult{FromBranch: from, IntoBranch: into}
	res.FromVer = g.Head(chapter.ID, from)
	res.IntoVer = g.Head(chapter.ID, into)
	if res.FromVer == 0 || res.IntoVer == 0 {
		return nil, errors.New("分支上没有可合并的提交")
	}
	ours, _ := contentAtVer(g, ownerID, novelID, chapter, into, res.IntoVer)
	theirs, _ := contentAtVer(g, ownerID, novelID, chapter, from, res.FromVer)
	base := ""
	if bp, err := branchRow(chapter.ID, from); err == nil && bp.FromVer > 0 {
		base, _ = contentAtVer(g, ownerID, novelID, chapter, bp.FromBranch, bp.FromVer)
	}
	if ours == theirs {
		return res, nil
	}
	merged, conflicts := merge3(SplitLines(base), SplitLines(ours), SplitLines(theirs))
	if len(conflicts) > 0 {
		res.Conflict = true
		res.Conflicts = conflicts
		res.Merged = strings.Join(merged, "\n")
		return res, nil
	}
	msg := message
	if strings.TrimSpace(msg) == "" {
		msg = "合并分支 " + from + " → " + into
	}
	v, _, err := g.Commit(CommitInput{
		OwnerID: ownerID, Chapter: chapter, AuthorID: authorID,
		Title: chapter.Title, Content: strings.Join(merged, "\n"),
		Branch: into, BaseVer: res.IntoVer, Message: msg,
		MergeFrom: from, MergeFromVer: res.FromVer,
	})
	if err != nil {
		return nil, err
	}
	res.Version = v.Version
	res.Merged = strings.Join(merged, "\n")
	return res, nil
}

func contentAtVer(g gitBackend, ownerID, novelID uint, chapter *models.Chapter, branch string, ver int) (string, error) {
	v, err := g.Get(chapter.ID, branch, ver)
	if err != nil {
		return "", err
	}
	return g.Content(ownerID, novelID, v)
}

// loadRefs 由 chapterID 反查作品与章节（git 后端需要作品目录与章节号）
func loadRefs(chapterID uint) (*models.Novel, *models.Chapter, error) {
	var ch models.Chapter
	if err := models.DB.Where("id = ?", chapterID).First(&ch).Error; err != nil {
		return nil, nil, errors.New("章节不存在")
	}
	novel, err := models.GetNovelByID(ch.NovelID)
	if err != nil {
		return nil, nil, errors.New("作品不存在")
	}
	return novel, &ch, nil
}

func branchRow(chapterID uint, name string) (*ChapterBranch, error) {
	var b ChapterBranch
	err := models.DB.Where("chapter_id = ? AND name = ?", chapterID, NormalizeBranch(name)).First(&b).Error
	if err != nil {
		return nil, err
	}
	return &b, nil
}
