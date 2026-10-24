package chaptervcs

// ==================== 分支与三方合并 ====================
// 语义对齐 git：分支是独立版本链；合并时以「分叉点」为 base 做三方合并
// （base / ours=目标分支 / theirs=来源分支）。
// 同一处两侧都改 → 冲突：不写库，回传冲突片段由作者并排确认（不做自动猜测）。

import (
	"errors"
	"strings"

	"nvs-server/models"
)

// MergeConflict 一处冲突（三段内容都给前端，便于并排展示）
type MergeConflict struct {
	Base   []string `json:"base"`
	Ours   []string `json:"ours"`
	Theirs []string `json:"theirs"`
}

// MergeResult 合并结果
type MergeResult struct {
	Conflict   bool            `json:"conflict"`
	Merged     string          `json:"merged,omitempty"`
	Conflicts  []MergeConflict `json:"conflicts,omitempty"`
	Version    int             `json:"version,omitempty"`
	FromVer    int             `json:"from_ver"`
	IntoVer    int             `json:"into_ver"`
	FromBranch string          `json:"from_branch"`
	IntoBranch string          `json:"into_branch"`
}

// CreateBranch 基于某分支的某版本开新分支：把该版本内容复制为新分支的 v1
func createBranchDefault(ownerID, novelID uint, chapter *models.Chapter, name, fromBranch string, fromVer int, authorID uint) (*ChapterVersion, error) {
	newName := NormalizeBranch(name)
	if newName == DefaultBranch {
		return nil, errors.New("不能创建与默认分支同名的分支")
	}
	src := NormalizeBranch(fromBranch)
	if src == "" {
		src = DefaultBranch
	}
	// 源版本：未指定则取源分支 head
	if fromVer <= 0 {
		fromVer = headDefault(chapter.ID, src)
	}
	content := ""
	title := chapter.Title
	if fromVer > 0 {
		if sv, err := getDefault(chapter.ID, src, fromVer); err == nil {
			content, _ = contentDefault(ownerID, novelID, sv)
			if sv.Title != "" {
				title = sv.Title
			}
		}
	}
	if _, err := EnsureBranch(chapter.ID, newName, authorID); err != nil {
		return nil, err
	}
	// 记录分叉点（三方合并的 base）
	_ = models.DB.Model(&ChapterBranch{}).Where("chapter_id = ? AND name = ?", chapter.ID, newName).
		Updates(map[string]interface{}{"from_ver": fromVer, "from_branch": src}).Error
	v, _, err := commitDefault(CommitInput{
		OwnerID:  ownerID,
		Chapter:  chapter,
		AuthorID: authorID,
		Title:    title,
		Content:  content,
		Branch:   newName,
		Message:  "从 " + src + " v" + itoa(fromVer) + " 开分支",
	})
	return v, err
}

// Merge 把 fromBranch 合并进 intoBranch（默认 main）
func mergeDefault(ownerID, novelID uint, chapter *models.Chapter, fromBranch, intoBranch string, authorID uint, message string) (*MergeResult, error) {
	from := NormalizeBranch(fromBranch)
	into := NormalizeBranch(intoBranch)
	if from == into {
		return nil, errors.New("源分支与目标分支相同")
	}
	var fb, ib ChapterBranch
	if err := models.DB.Where("chapter_id = ? AND name = ?", chapter.ID, from).First(&fb).Error; err != nil {
		return nil, errors.New("源分支不存在")
	}
	if err := models.DB.Where("chapter_id = ? AND name = ?", chapter.ID, into).First(&ib).Error; err != nil {
		return nil, errors.New("目标分支不存在")
	}
	res := &MergeResult{FromBranch: from, IntoBranch: into, FromVer: fb.HeadVer, IntoVer: ib.HeadVer}

	ours, _ := contentOfVer(ownerID, novelID, chapter.ID, into, ib.HeadVer)
	theirs, _ := contentOfVer(ownerID, novelID, chapter.ID, from, fb.HeadVer)
	base := ""
	if fb.FromVer > 0 {
		base, _ = contentOfVer(ownerID, novelID, chapter.ID, fb.FromBranch, fb.FromVer)
	}

	if ours == theirs {
		return res, nil // 内容已一致，无需合并
	}

	merged, conflicts := merge3(SplitLines(base), SplitLines(ours), SplitLines(theirs))
	if len(conflicts) > 0 {
		res.Conflict = true
		res.Conflicts = conflicts
		res.Merged = strings.Join(merged, "\n")
		return res, nil
	}
	title := chapter.Title
	if hv, err := headVersionDefault(chapter.ID, into); err == nil && hv.Title != "" {
		title = hv.Title
	}
	msg := message
	if strings.TrimSpace(msg) == "" {
		msg = "合并分支 " + from + " → " + into
	}
	v, _, err := commitDefault(CommitInput{
		OwnerID:      ownerID,
		Chapter:      chapter,
		AuthorID:     authorID,
		Title:        title,
		Content:      strings.Join(merged, "\n"),
		Branch:       into,
		BaseVer:      ib.HeadVer,
		Message:      msg,
		MergeFrom:    from,
		MergeFromVer: fb.HeadVer,
	})
	if err != nil {
		return nil, err
	}
	res.Version = v.Version
	res.Merged = strings.Join(merged, "\n")
	return res, nil
}

func contentOfVer(ownerID, novelID, chapterID uint, branch string, ver int) (string, error) {
	if ver <= 0 {
		return "", nil
	}
	v, err := getDefault(chapterID, NormalizeBranch(branch), ver)
	if err != nil {
		return "", err
	}
	return contentDefault(ownerID, novelID, v)
}

// ─── 三方合并（hunk 级） ───

type hunk struct {
	start int      // base 起始下标（含）
	end   int      // base 结束下标（不含）
	lines []string // 替换内容
}

// hunksOf 把 base→other 的差异压成若干「替换区间」
func hunksOf(base, other []string) []hunk {
	dr := diffSlices(base, other)
	out := make([]hunk, 0, 8)
	cur := hunk{start: -1}
	bi := 0
	flush := func() {
		if cur.start >= 0 {
			cur.end = bi
			out = append(out, cur)
			cur = hunk{start: -1}
		}
	}
	for _, l := range dr.Lines {
		switch l.Type {
		case DiffEqual:
			flush()
			bi++
		case DiffDel:
			if cur.start < 0 {
				cur.start = bi
			}
			bi++
			cur.end = bi
		case DiffAdd:
			if cur.start < 0 {
				cur.start = bi
			}
			cur.end = bi
			cur.lines = append(cur.lines, l.Text)
		}
	}
	flush()
	return out
}

// merge3 三方合并：返回合并后的行与冲突片段
func merge3(base, ours, theirs []string) ([]string, []MergeConflict) {
	oh := hunksOf(base, ours)
	th := hunksOf(base, theirs)
	out := make([]string, 0, len(base)+8)
	conflicts := make([]MergeConflict, 0, 2)
	pos := 0
	i, j := 0, 0

	for i < len(oh) || j < len(th) {
		start := -1
		if i < len(oh) {
			start = oh[i].start
		}
		if j < len(th) && (start < 0 || th[j].start < start) {
			start = th[j].start
		}
		end := start
		oRep := []string(nil)
		tRep := []string(nil)
		hasO, hasT := false, false
		// 吸收所有与该区间相交的 hunk（含纯插入：start==end）
		for changed := true; changed; {
			changed = false
			if i < len(oh) && oh[i].start <= end {
				oRep = append(oRep, oh[i].lines...)
				hasO = true
				if oh[i].end > end {
					end = oh[i].end
				}
				i++
				changed = true
			}
			if j < len(th) && th[j].start <= end {
				tRep = append(tRep, th[j].lines...)
				hasT = true
				if th[j].end > end {
					end = th[j].end
				}
				j++
				changed = true
			}
		}
		if start > pos {
			out = append(out, base[pos:start]...)
		}
		switch {
		case hasO && !hasT:
			out = append(out, oRep...)
		case !hasO && hasT:
			out = append(out, tRep...)
		default:
			if sameLines(oRep, tRep) {
				out = append(out, oRep...)
			} else {
				// 两侧都改了同一处 → 冲突（保留 ours，交由作者确认）
				conflicts = append(conflicts, MergeConflict{
					Base:   baseSlice(base, start, end),
					Ours:   oRep,
					Theirs: tRep,
				})
				out = append(out, oRep...)
			}
		}
		pos = end
	}
	if pos < len(base) {
		out = append(out, base[pos:]...)
	}
	return out, conflicts
}

func baseSlice(base []string, s, e int) []string {
	if s < 0 {
		s = 0
	}
	if e > len(base) {
		e = len(base)
	}
	if s >= e {
		return []string{}
	}
	out := make([]string, e-s)
	copy(out, base[s:e])
	return out
}

func sameLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
