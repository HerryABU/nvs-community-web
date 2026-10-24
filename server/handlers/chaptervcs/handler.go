package chaptervcshandler

// ==================== 章节版本控制：接口层 ====================
// 只做入参校验 + 权限判定 + 响应组装；规则一律走 chaptervcs / collab，前后端不各写一套。

import (
	"strconv"

	"nvs-server/chaptervcs"
	"nvs-server/collab"
	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// loadChapter 载入作品/章节并校验「可编辑本章」（复用协作权限矩阵）
func loadChapter(c *gin.Context) (*models.Novel, *models.Chapter, bool) {
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	num, _ := strconv.Atoi(c.Param("num"))
	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil {
		utils.NotFound(c, "作品不存在")
		return nil, nil, false
	}
	chapter, err := models.GetChapterByNovelAndNumber(uint(novelID), num)
	if err != nil {
		utils.NotFound(c, "章节不存在")
		return nil, nil, false
	}
	actor := collab.ActorOf(novel.ID, middleware.UserID(c), middleware.UserRole(c))
	if !actor.CanEditChapter(chapter) {
		utils.Forbidden(c, "无权操作本章版本（需主笔 / 本章作者 / 有改他人章节权限）")
		return nil, nil, false
	}
	return novel, chapter, true
}

// canDeleteHistory 删历史：主笔、本章作者、平台（避免协作者互删对方稿件）
func canDeleteHistory(c *gin.Context, novel *models.Novel, chapter *models.Chapter) bool {
	role := middleware.UserRole(c)
	if role == models.RoleAdmin || role == models.RolePolice {
		return true
	}
	uid := middleware.UserID(c)
	return uid != 0 && (novel.AuthorID == uid || chapter.AuthorID == uid)
}

// ListVersions GET /api/novels/:id/chapters/:num/versions?branch=main
func ListVersions(c *gin.Context) {
	novel, chapter, ok := loadChapter(c)
	if !ok {
		return
	}
	branch := chaptervcs.NormalizeBranch(c.Query("branch"))
	list, err := chaptervcs.List(chapter.ID, branch)
	if err != nil {
		utils.InternalError(c, "查询版本失败")
		return
	}
	branches, _ := chaptervcs.Branches(chapter.ID)
	utils.Success(c, gin.H{
		"list":     list,
		"branch":   branch,
		"branches": branches,
		"head":     chaptervcs.Head(chapter.ID, branch),
		"novel_id": novel.ID,
	})
}

// GetVersion GET /api/novels/:id/chapters/:num/versions/:ver
func GetVersion(c *gin.Context) {
	novel, chapter, ok := loadChapter(c)
	if !ok {
		return
	}
	ver, _ := strconv.Atoi(c.Param("ver"))
	branch := chaptervcs.NormalizeBranch(c.Query("branch"))
	v, err := chaptervcs.Get(chapter.ID, branch, ver)
	if err != nil {
		utils.NotFound(c, "版本不存在")
		return
	}
	content, _ := chaptervcs.Content(novel.AuthorID, novel.ID, v)
	utils.Success(c, gin.H{"version": v, "content": content})
}

// DiffVersions GET /api/novels/:id/chapters/:num/versions/:ver/diff?to=2&branch=main
// 未传 to 时与分支 head 对比
func DiffVersions(c *gin.Context) {
	novel, chapter, ok := loadChapter(c)
	if !ok {
		return
	}
	from, _ := strconv.Atoi(c.Param("ver"))
	to, _ := strconv.Atoi(c.Query("to"))
	branch := chaptervcs.NormalizeBranch(c.Query("branch"))
	if to <= 0 {
		to = chaptervcs.Head(chapter.ID, branch)
	}
	a, errA := chaptervcs.Get(chapter.ID, branch, from)
	b, errB := chaptervcs.Get(chapter.ID, branch, to)
	if errA != nil || errB != nil {
		utils.NotFound(c, "版本不存在")
		return
	}
	ca, _ := chaptervcs.Content(novel.AuthorID, novel.ID, a)
	cb, _ := chaptervcs.Content(novel.AuthorID, novel.ID, b)
	utils.Success(c, gin.H{
		"from": gin.H{"version": from, "author_name": a.AuthorName, "message": a.Message, "created_at": a.CreatedAt},
		"to":   gin.H{"version": to, "author_name": b.AuthorName, "message": b.Message, "created_at": b.CreatedAt},
		"diff": chaptervcs.DiffLines(ca, cb),
	})
}

// RevertVersion POST /api/novels/:id/chapters/:num/versions/:ver/revert
// 以历史版本内容再提交一次（历史不被抹掉）
func RevertVersion(c *gin.Context) {
	novel, chapter, ok := loadChapter(c)
	if !ok {
		return
	}
	ver, _ := strconv.Atoi(c.Param("ver"))
	branch := chaptervcs.NormalizeBranch(c.Query("branch"))
	target, err := chaptervcs.Get(chapter.ID, branch, ver)
	if err != nil {
		utils.NotFound(c, "版本不存在")
		return
	}
	v, err := chaptervcs.Revert(novel.AuthorID, novel.ID, chapter, target, middleware.UserID(c))
	if err != nil {
		utils.InternalError(c, "回滚失败")
		return
	}
	collab.LogChapterAction(novel.ID, middleware.UserID(c), "revert", chapter.ID, "回滚到 v"+strconv.Itoa(ver))
	utils.SuccessMessage(c, "已回滚并生成新版本 v"+strconv.Itoa(v.Version), v)
}

// DeleteVersion DELETE /api/novels/:id/chapters/:num/versions/:ver
// 只清正文快照，元数据保留；不能删分支 head
func DeleteVersion(c *gin.Context) {
	novel, chapter, ok := loadChapter(c)
	if !ok {
		return
	}
	if !canDeleteHistory(c, novel, chapter) {
		utils.Forbidden(c, "只有主笔或本章作者可删除历史版本")
		return
	}
	ver, _ := strconv.Atoi(c.Param("ver"))
	branch := chaptervcs.NormalizeBranch(c.Query("branch"))
	if err := chaptervcs.DeleteVersion(novel.AuthorID, novel.ID, chapter.ID, branch, ver); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	collab.LogChapterAction(novel.ID, middleware.UserID(c), "del_version", chapter.ID, branch+" v"+strconv.Itoa(ver))
	utils.SuccessMessage(c, "已删除该历史快照（作者与提交记录保留）", nil)
}

// ListBranches GET /api/novels/:id/chapters/:num/branches
func ListBranches(c *gin.Context) {
	_, chapter, ok := loadChapter(c)
	if !ok {
		return
	}
	list, err := chaptervcs.Branches(chapter.ID)
	if err != nil {
		utils.InternalError(c, "查询分支失败")
		return
	}
	utils.Success(c, gin.H{"list": list})
}

// CreateBranch POST /api/novels/:id/chapters/:num/branches { name, from_branch, from_ver }
func CreateBranch(c *gin.Context) {
	novel, chapter, ok := loadChapter(c)
	if !ok {
		return
	}
	var req struct {
		Name       string `json:"name"`
		FromBranch string `json:"from_branch"`
		FromVer    int    `json:"from_ver"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	v, err := chaptervcs.CreateBranch(novel.AuthorID, novel.ID, chapter, req.Name, req.FromBranch, req.FromVer, middleware.UserID(c))
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	collab.LogChapterAction(novel.ID, middleware.UserID(c), "branch", chapter.ID, v.Branch)
	utils.SuccessMessage(c, "已创建分支 "+v.Branch, v)
}

// MergeBranch POST /api/novels/:id/chapters/:num/merge { from_branch, into_branch, message }
func MergeBranch(c *gin.Context) {
	novel, chapter, ok := loadChapter(c)
	if !ok {
		return
	}
	var req struct {
		FromBranch string `json:"from_branch"`
		IntoBranch string `json:"into_branch"`
		Message    string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	res, err := chaptervcs.Merge(novel.AuthorID, novel.ID, chapter, req.FromBranch, req.IntoBranch, middleware.UserID(c), req.Message)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if res.Conflict {
		utils.Conflict(c, "合并存在冲突，请人工确认", res)
		return
	}
	collab.LogChapterAction(novel.ID, middleware.UserID(c), "merge", chapter.ID,
		res.FromBranch+" → "+res.IntoBranch)
	utils.SuccessMessage(c, "已合并并生成新版本 v"+strconv.Itoa(res.Version), res)
}
