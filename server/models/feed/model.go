package feed

// ==================== 短视频 / 微文（视频号 + 贴图号） ====================
// 一套内容模型，两种形态：
//   - video：短视频，前端沉浸式下滑播放（类视频号）
//   - micro：微文，可附多图（类公众号/贴图号）
// 评论与点赞/收藏使用本域独立轻量表，不触碰高耦合的 novel/blog 评论链路。

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"nvs-server/models"

	"gorm.io/gorm"
)

const (
	KindVideo = "video"
	KindMicro = "micro"

	StatusDraft     = "draft"
	StatusPending   = "pending"
	StatusPublished = "published"
	StatusRemoved   = "removed"
)

// Post 短视频 / 微文
type Post struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	UserID    uint   `gorm:"not null;index:idx_feed_user" json:"user_id"`
	Kind      string `gorm:"size:16;not null;index:idx_feed_kind" json:"kind"`       // video / micro
	Title     string `gorm:"size:128;default:''" json:"title"`                       // 微文标题 / 视频标题
	Content   string `gorm:"type:text" json:"content"`                               // 正文 / 视频描述
	VideoURL  string `gorm:"size:512;default:''" json:"video_url"`                   // video：站内 /uploads 或外链
	CoverURL  string `gorm:"size:512;default:''" json:"cover_url"`                   // 封面（视频/微文头图）
	Images    string `gorm:"type:text;default:''" json:"images_raw,omitempty"`       // JSON 数组字符串（微文多图）
	Tags      string `gorm:"size:256;default:''" json:"tags"`                        // 逗号分隔
	Duration  int    `gorm:"not null;default:0" json:"duration"`                     // 视频秒数（可选）
	Status    string `gorm:"size:16;not null;default:published;index" json:"status"` // published 即公开
	Latitude  string `gorm:"size:32;default:''" json:"-"`
	Longitude string `gorm:"size:32;default:''" json:"-"`

	LikeCount     int `gorm:"not null;default:0" json:"like_count"`
	CommentCount  int `gorm:"not null;default:0" json:"comment_count"`
	FavoriteCount int `gorm:"not null;default:0" json:"favorite_count"`
	ShareCount    int `gorm:"not null;default:0" json:"share_count"`
	ViewCount     int `gorm:"not null;default:0" json:"view_count"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 运行时
	AuthorName string   `gorm:"-" json:"author_name,omitempty"`
	AvatarURL  string   `gorm:"-" json:"avatar_url,omitempty"`
	ImagesList []string `gorm:"-" json:"images,omitempty"`
	Liked      bool     `gorm:"-" json:"liked,omitempty"`
	Favorited  bool     `gorm:"-" json:"favorited,omitempty"`
}

func (Post) TableName() string { return "feed_posts" }

// Like 点赞
type Like struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PostID    uint      `gorm:"not null;uniqueIndex:idx_feed_like" json:"post_id"`
	UserID    uint      `gorm:"not null;uniqueIndex:idx_feed_like" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (Like) TableName() string { return "feed_likes" }

// Favorite 收藏
type Favorite struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PostID    uint      `gorm:"not null;uniqueIndex:idx_feed_fav" json:"post_id"`
	UserID    uint      `gorm:"not null;uniqueIndex:idx_feed_fav" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (Favorite) TableName() string { return "feed_favorites" }

// Comment 轻量评论（支持楼中楼一层）
type Comment struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PostID    uint      `gorm:"not null;index:idx_feed_comment" json:"post_id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	ParentID  uint      `gorm:"not null;default:0;index" json:"parent_id"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	LikeCount int       `gorm:"not null;default:0" json:"like_count"`
	CreatedAt time.Time `json:"created_at"`

	AuthorName string `gorm:"-" json:"author_name,omitempty"`
	AvatarURL  string `gorm:"-" json:"avatar_url,omitempty"`
}

func (Comment) TableName() string { return "feed_comments" }

// CommentLike 评论点赞
type CommentLike struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CommentID uint      `gorm:"not null;uniqueIndex:idx_feed_clike" json:"comment_id"`
	UserID    uint      `gorm:"not null;uniqueIndex:idx_feed_clike" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (CommentLike) TableName() string { return "feed_comment_likes" }

// Models 注册本域全部表
func Models() []interface{} {
	return []interface{}{&Post{}, &Like{}, &Favorite{}, &Comment{}, &CommentLike{}}
}

func init() {
	models.RegisterDomainModels(Models)
}

func NormalizeKind(k string) string {
	if strings.EqualFold(strings.TrimSpace(k), KindVideo) {
		return KindVideo
	}
	return KindMicro
}

// PostInput 发布入参
type PostInput struct {
	Kind     string
	Title    string
	Content  string
	VideoURL string
	CoverURL string
	Images   []string
	Tags     string
	Duration int
	Status   string
}

func CreatePost(userID uint, in PostInput) (*Post, error) {
	kind := NormalizeKind(in.Kind)
	content := strings.TrimSpace(in.Content)
	video := strings.TrimSpace(in.VideoURL)
	if kind == KindVideo && video == "" {
		return nil, errors.New("请上传视频")
	}
	if kind == KindMicro && content == "" && len(in.Images) == 0 {
		return nil, errors.New("微文需要正文或图片")
	}
	status := strings.TrimSpace(in.Status)
	switch status {
	case StatusDraft, StatusPending, StatusRemoved:
	default:
		status = StatusPublished
	}
	images := ""
	if len(in.Images) > 0 {
		b, _ := json.Marshal(in.Images)
		images = string(b)
	}
	p := &Post{
		UserID: userID, Kind: kind, Title: strings.TrimSpace(in.Title), Content: content,
		VideoURL: video, CoverURL: strings.TrimSpace(in.CoverURL), Images: images,
		Tags: strings.TrimSpace(in.Tags), Duration: in.Duration, Status: status,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := models.DB.Create(p).Error; err != nil {
		return nil, err
	}
	p.ImagesList = in.Images
	return p, nil
}

func UpdatePost(userID uint, id uint, in PostInput) (*Post, error) {
	var p Post
	if err := models.DB.Where("id = ? AND user_id = ?", id, userID).First(&p).Error; err != nil {
		return nil, errors.New("内容不存在或无权编辑")
	}
	p.Kind = NormalizeKind(in.Kind)
	p.Title = strings.TrimSpace(in.Title)
	p.Content = strings.TrimSpace(in.Content)
	if in.VideoURL != "" {
		p.VideoURL = strings.TrimSpace(in.VideoURL)
	}
	if in.CoverURL != "" {
		p.CoverURL = strings.TrimSpace(in.CoverURL)
	}
	if len(in.Images) > 0 {
		b, _ := json.Marshal(in.Images)
		p.Images = string(b)
	}
	p.Tags = strings.TrimSpace(in.Tags)
	p.Duration = in.Duration
	if in.Status != "" {
		p.Status = in.Status
	}
	p.UpdatedAt = time.Now()
	if err := models.DB.Save(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// DeletePost 软删除（保留记录）
func DeletePost(userID uint, id uint) error {
	return models.DB.Model(&Post{}).Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{"status": StatusRemoved, "updated_at": time.Now()}).Error
}

// GetPost 读取（公开内容；作者可读自己的草稿）
func GetPost(id uint, viewerID uint) (*Post, error) {
	var p Post
	if err := models.DB.Where("id = ?", id).First(&p).Error; err != nil {
		return nil, errors.New("内容不存在")
	}
	if p.Status != StatusPublished && p.UserID != viewerID {
		return nil, errors.New("内容不可见")
	}
	fillAuthors([]*Post{&p})
	return &p, nil
}

// ListPosts 列表（kind=video|micro，scope=all|following|mine，author_id 可选）
func ListPosts(kind, scope string, authorID, viewerID uint, page, pageSize int) ([]Post, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	q := models.DB.Model(&Post{}).Where("status = ?", StatusPublished)
	if kind == KindVideo || kind == KindMicro {
		q = q.Where("kind = ?", kind)
	}
	switch scope {
	case "mine":
		q = q.Where("user_id = ?", viewerID)
	case "following":
		q = q.Where("user_id IN (?)", models.DB.Table("follows").
			Select("followed_id").Where("follower_id = ?", viewerID))
	}
	if authorID > 0 {
		q = q.Where("user_id = ?", authorID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []Post
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	fillAuthors(listToPtrs(list))
	fillViewerFlags(listToPtrs(list), viewerID)
	return list, total, nil
}

func listToPtrs(list []Post) []*Post {
	out := make([]*Post, len(list))
	for i := range list {
		out[i] = &list[i]
	}
	return out
}

// IncrView 浏览量 +1（best effort）
func IncrView(id uint) {
	_ = models.DB.Model(&Post{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

// ToggleLike 点赞/取消，返回点赞后状态
func ToggleLike(postID, userID uint) (bool, error) {
	var like Like
	err := models.DB.Where("post_id = ? AND user_id = ?", postID, userID).First(&like).Error
	if err == nil {
		if e := models.DB.Delete(&like).Error; e != nil {
			return false, e
		}
		_ = models.DB.Model(&Post{}).Where("id = ?", postID).
			UpdateColumn("like_count", gorm.Expr("CASE WHEN like_count > 0 THEN like_count - 1 ELSE 0 END")).Error
		return false, nil
	}
	if err := models.DB.Create(&Like{PostID: postID, UserID: userID, CreatedAt: time.Now()}).Error; err != nil {
		return false, err
	}
	_ = models.DB.Model(&Post{}).Where("id = ?", postID).
		UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error
	return true, nil
}

// ToggleFavorite 收藏/取消
func ToggleFavorite(postID, userID uint) (bool, error) {
	var f Favorite
	err := models.DB.Where("post_id = ? AND user_id = ?", postID, userID).First(&f).Error
	if err == nil {
		if e := models.DB.Delete(&f).Error; e != nil {
			return false, e
		}
		_ = models.DB.Model(&Post{}).Where("id = ?", postID).
			UpdateColumn("favorite_count", gorm.Expr("CASE WHEN favorite_count > 0 THEN favorite_count - 1 ELSE 0 END")).Error
		return false, nil
	}
	if err := models.DB.Create(&Favorite{PostID: postID, UserID: userID, CreatedAt: time.Now()}).Error; err != nil {
		return false, err
	}
	_ = models.DB.Model(&Post{}).Where("id = ?", postID).
		UpdateColumn("favorite_count", gorm.Expr("favorite_count + 1")).Error
	return true, nil
}

// IncrShare 分享 +1
func IncrShare(id uint) {
	_ = models.DB.Model(&Post{}).Where("id = ?", id).
		UpdateColumn("share_count", gorm.Expr("share_count + 1")).Error
}

func fillViewerFlags(list []*Post, viewerID uint) {
	if len(list) == 0 || viewerID == 0 {
		return
	}
	ids := make([]uint, 0, len(list))
	for _, p := range list {
		ids = append(ids, p.ID)
	}
	var likes []Like
	var favs []Favorite
	models.DB.Select("post_id").Where("user_id = ? AND post_id IN ?", viewerID, ids).Find(&likes)
	models.DB.Select("post_id").Where("user_id = ? AND post_id IN ?", viewerID, ids).Find(&favs)
	liked := map[uint]bool{}
	faved := map[uint]bool{}
	for _, x := range likes {
		liked[x.PostID] = true
	}
	for _, x := range favs {
		faved[x.PostID] = true
	}
	for _, p := range list {
		p.Liked = liked[p.ID]
		p.Favorited = faved[p.ID]
	}
}

func fillAuthors(list []*Post) {
	if len(list) == 0 {
		return
	}
	ids := map[uint]bool{}
	for _, p := range list {
		ids[p.UserID] = true
	}
	keys := make([]uint, 0, len(ids))
	for id := range ids {
		keys = append(keys, id)
	}
	var users []models.User
	models.DB.Select("id, username, nickname, avatar_url").Where("id IN ?", keys).Find(&users)
	type uInfo struct{ name, avatar string }
	byID := map[uint]uInfo{}
	for _, u := range users {
		n := u.Nickname
		if n == "" {
			n = u.Username
		}
		byID[u.ID] = uInfo{n, u.AvatarURL}
	}
	for _, p := range list {
		if u, ok := byID[p.UserID]; ok {
			p.AuthorName = u.name
			p.AvatarURL = u.avatar
		}
		if p.Images != "" {
			var imgs []string
			if json.Unmarshal([]byte(p.Images), &imgs) == nil {
				p.ImagesList = imgs
			}
		}
	}
}

// ─── 轻量评论（类微信：点赞 / 回复一层） ───

// CreateComment 发布评论（parentID>0 = 回复某条评论）
func CreateComment(postID, userID, parentID uint, content string) (*Comment, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, errors.New("评论内容不能为空")
	}
	if len([]rune(content)) > 1000 {
		return nil, errors.New("评论内容过长")
	}
	var p Post
	if err := models.DB.Where("id = ? AND status = ?", postID, StatusPublished).First(&p).Error; err != nil {
		return nil, errors.New("内容不存在")
	}
	if parentID > 0 {
		var parent Comment
		if err := models.DB.Where("id = ? AND post_id = ?", parentID, postID).First(&parent).Error; err != nil {
			return nil, errors.New("被回复的评论不存在")
		}
	}
	c := &Comment{PostID: postID, UserID: userID, ParentID: parentID, Content: content, CreatedAt: time.Now()}
	if err := models.DB.Create(c).Error; err != nil {
		return nil, err
	}
	_ = models.DB.Model(&Post{}).Where("id = ?", postID).
		UpdateColumn("comment_count", gorm.Expr("comment_count + 1")).Error
	// 通知作者（best effort，自己评自己不通知由 PushNotification 内部处理）
	link := "/shorts/" + fmt.Sprint(postID)
	if p.Kind == KindMicro {
		link = "/micro/" + fmt.Sprint(postID)
	}
	_ = models.PushNotification(userID, p.UserID, "feed_comment",
		"你的内容收到新评论", truncate(content, 60), link)
	fillCommentAuthors([]*Comment{c})
	return c, nil
}

// ListComments 按时间倒序返回评论（含一层回复）
func ListComments(postID uint, page, pageSize int) ([]Comment, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}
	q := models.DB.Model(&Comment{}).Where("post_id = ?", postID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []Comment
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	ptrs := make([]*Comment, len(list))
	for i := range list {
		ptrs[i] = &list[i]
	}
	fillCommentAuthors(ptrs)
	return list, total, nil
}

// DeleteComment 作者或内容作者可删
func DeleteComment(userID, commentID uint) error {
	var c Comment
	if err := models.DB.Where("id = ?", commentID).First(&c).Error; err != nil {
		return errors.New("评论不存在")
	}
	var p Post
	if err := models.DB.Select("user_id").Where("id = ?", c.PostID).First(&p).Error; err != nil {
		return errors.New("内容不存在")
	}
	if c.UserID != userID && p.UserID != userID {
		return errors.New("无权删除该评论")
	}
	if err := models.DB.Delete(&c).Error; err != nil {
		return err
	}
	// 一并删除回复
	_ = models.DB.Where("parent_id = ?", commentID).Delete(&Comment{}).Error
	_ = models.DB.Model(&Post{}).Where("id = ?", c.PostID).
		UpdateColumn("comment_count", gorm.Expr("CASE WHEN comment_count > 0 THEN comment_count - 1 ELSE 0 END")).Error
	return nil
}

// ToggleCommentLike 评论点赞
func ToggleCommentLike(commentID, userID uint) (bool, error) {
	var l CommentLike
	err := models.DB.Where("comment_id = ? AND user_id = ?", commentID, userID).First(&l).Error
	if err == nil {
		if e := models.DB.Delete(&l).Error; e != nil {
			return false, e
		}
		_ = models.DB.Model(&Comment{}).Where("id = ?", commentID).
			UpdateColumn("like_count", gorm.Expr("CASE WHEN like_count > 0 THEN like_count - 1 ELSE 0 END")).Error
		return false, nil
	}
	if err := models.DB.Create(&CommentLike{CommentID: commentID, UserID: userID, CreatedAt: time.Now()}).Error; err != nil {
		return false, err
	}
	_ = models.DB.Model(&Comment{}).Where("id = ?", commentID).
		UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error
	return true, nil
}

func fillCommentAuthors(list []*Comment) {
	if len(list) == 0 {
		return
	}
	ids := map[uint]bool{}
	for _, c := range list {
		ids[c.UserID] = true
	}
	keys := make([]uint, 0, len(ids))
	for id := range ids {
		keys = append(keys, id)
	}
	var users []models.User
	models.DB.Select("id, username, nickname, avatar_url").Where("id IN ?", keys).Find(&users)
	type uInfo struct{ name, avatar string }
	byID := map[uint]uInfo{}
	for _, u := range users {
		n := u.Nickname
		if n == "" {
			n = u.Username
		}
		byID[u.ID] = uInfo{n, u.AvatarURL}
	}
	for _, c := range list {
		if u, ok := byID[c.UserID]; ok {
			c.AuthorName = u.name
			c.AvatarURL = u.avatar
		}
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
