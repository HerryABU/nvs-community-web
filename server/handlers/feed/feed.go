package feed

// ==================== 短视频 / 微文接口 ====================
// 视频号式沉浸下滑（video）+ 贴图号式图文（micro），共用一套 feed 模型。

import (
	"strconv"
	"strings"

	"nvs-server/handlers/admincommon"
	"nvs-server/handlers/badword"
	"nvs-server/handlers/hctx"
	"nvs-server/models/feed"
	"nvs-server/security"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

// GET /api/feed?kind=video|micro&scope=all|following|mine&author_id=&page=
func List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	ps, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := feed.ListPosts(
		strings.TrimSpace(c.Query("kind")),
		strings.TrimSpace(c.DefaultQuery("scope", "all")),
		parseUint(c.Query("author_id")),
		hctx.CurrentUserID(c), page, ps,
	)
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	infra.Success(c, gin.H{"list": list, "total": total})
}

// GET /api/feed/:id
func Detail(c *gin.Context) {
	id := parseUint(c.Param("id"))
	p, err := feed.GetPost(id, hctx.CurrentUserID(c))
	if err != nil {
		infra.NotFound(c, err.Error())
		return
	}
	feed.IncrView(id)
	infra.Success(c, p)
}

// POST /api/feed
func Create(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	var in struct {
		Kind     string   `json:"kind"`
		Title    string   `json:"title"`
		Content  string   `json:"content"`
		VideoURL string   `json:"video_url"`
		CoverURL string   `json:"cover_url"`
		Images   []string `json:"images"`
		Tags     string   `json:"tags"`
		Duration int      `json:"duration"`
		Status   string   `json:"status"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	in.Content = sanitize(in.Content)
	in.Title = sanitize(in.Title)
	in.Tags = sanitize(in.Tags)
	p, err := feed.CreatePost(uid, feed.PostInput{
		Kind: in.Kind, Title: in.Title, Content: in.Content,
		VideoURL: normalizeUploadURL(in.VideoURL), CoverURL: normalizeUploadURL(in.CoverURL),
		Images: normalizeImages(in.Images), Tags: in.Tags, Duration: in.Duration, Status: in.Status,
	})
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "发布成功", p)
}

// PUT /api/feed/:id
func Update(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	var in struct {
		Kind     string   `json:"kind"`
		Title    string   `json:"title"`
		Content  string   `json:"content"`
		VideoURL string   `json:"video_url"`
		CoverURL string   `json:"cover_url"`
		Images   []string `json:"images"`
		Tags     string   `json:"tags"`
		Duration int      `json:"duration"`
		Status   string   `json:"status"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	p, err := feed.UpdatePost(uid, parseUint(c.Param("id")), feed.PostInput{
		Kind: in.Kind, Title: sanitize(in.Title), Content: sanitize(in.Content),
		VideoURL: normalizeUploadURL(in.VideoURL), CoverURL: normalizeUploadURL(in.CoverURL),
		Images: normalizeImages(in.Images), Tags: sanitize(in.Tags), Duration: in.Duration, Status: in.Status,
	})
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已保存", p)
}

// DELETE /api/feed/:id
func Delete(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	if err := feed.DeletePost(uid, parseUint(c.Param("id"))); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已删除", nil)
}

// POST /api/feed/:id/like
func Like(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	liked, err := feed.ToggleLike(parseUint(c.Param("id")), uid)
	if err != nil {
		infra.InternalError(c, "操作失败")
		return
	}
	infra.Success(c, gin.H{"liked": liked})
}

// POST /api/feed/:id/favorite
func Favorite(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	faved, err := feed.ToggleFavorite(parseUint(c.Param("id")), uid)
	if err != nil {
		infra.InternalError(c, "操作失败")
		return
	}
	infra.Success(c, gin.H{"favorited": faved})
}

// POST /api/feed/:id/share
func Share(c *gin.Context) {
	feed.IncrShare(parseUint(c.Param("id")))
	infra.Success(c, gin.H{"ok": true})
}

// GET /api/feed/:id/comments
func ListComments(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	ps, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	list, total, err := feed.ListComments(parseUint(c.Param("id")), page, ps)
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	infra.Success(c, gin.H{"list": list, "total": total})
}

// POST /api/feed/:id/comments
func CreateComment(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	var in struct {
		Content  string `json:"content"`
		ParentID uint   `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	cm, err := feed.CreateComment(parseUint(c.Param("id")), uid, in.ParentID, sanitize(in.Content))
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "评论成功", cm)
}

// DELETE /api/feed/comments/:cid
func DeleteComment(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	if err := feed.DeleteComment(uid, parseUint(c.Param("cid"))); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.SuccessMessage(c, "已删除评论", nil)
}

// POST /api/feed/comments/:cid/like
func LikeComment(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	liked, err := feed.ToggleCommentLike(parseUint(c.Param("cid")), uid)
	if err != nil {
		infra.InternalError(c, "操作失败")
		return
	}
	infra.Success(c, gin.H{"liked": liked})
}

// GET /api/feed-config 前端发布页读取当前图片上限（独立路径，避免与 /feed/:id 同级通配冲突）
func Config(c *gin.Context) {
	infra.Success(c, gin.H{"max_micro_images": feed.MaxMicroImages()})
}

// PUT /api/admin/feed-config 后台调节单篇微文图片上限（1~50）
func SetConfig(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var in struct {
		MaxMicroImages int `json:"max_micro_images"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.MaxMicroImages <= 0 {
		infra.BadRequest(c, "参数错误：请给出 1~50 的整数")
		return
	}
	if err := feed.SetMaxMicroImages(in.MaxMicroImages); err != nil {
		infra.InternalError(c, "保存失败")
		return
	}
	infra.SuccessMessage(c, "已保存", gin.H{"max_micro_images": feed.MaxMicroImages()})
}

// ── 工具 ──

func parseUint(s string) uint {
	n, _ := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	return uint(n)
}

func sanitize(s string) string {
	s = strings.TrimSpace(security.SanitizeUserContent(s))
	s, _ = badword.ProcessBadWords(s, "feed")
	return s
}

// normalizeUploadURL 允许站内 /uploads/... 与 http(s) 外链，其余按站内路径拼
func normalizeUploadURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return s
	}
	return "/uploads/" + strings.TrimPrefix(s, "/")
}

func normalizeImages(list []string) []string {
	out := make([]string, 0, len(list))
	for _, u := range list {
		if u = normalizeUploadURL(u); u != "" {
			out = append(out, u)
			if len(out) >= feed.MaxMicroImages() {
				break
			}
		}
	}
	return out
}
