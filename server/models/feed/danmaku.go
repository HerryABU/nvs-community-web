package feed

// ==================== 视频弹幕 ====================
// 弹幕与普通评论是两种东西：
//   - 弹幕绑定到视频时间轴（time_ms），播放到点才出现，量大、读多写少；
//   - 只用于 video 类内容，且展示在播放器内部，不进评论区列表。
// 因此单独一张轻表，不参与 feed_comments 的计数与楼中楼逻辑。

import (
	"errors"
	"strings"
	"time"

	"nvs-server/models"
)

const (
	DanmakuScroll = "scroll" // 滚动弹幕（默认）
	DanmakuTop    = "top"    // 顶部固定
	DanmakuBottom = "bottom" // 底部固定

	DanmakuPublished = "published"
	DanmakuHidden    = "hidden"
)

// Danmaku 一条弹幕
type Danmaku struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	PostID uint `gorm:"not null;index:idx_danmaku_post_time,priority:1" json:"post_id"`
	UserID uint `gorm:"not null;index" json:"user_id"`
	// Content 只存纯文本：渲染端用 textContent 输出，不解析 HTML
	Content string `gorm:"size:255;not null" json:"content"`
	// TimeMs 弹幕出现的视频时间点（毫秒）。整型存储，前端算 秒*1000 即可
	TimeMs int `gorm:"not null;default:0;index:idx_danmaku_post_time,priority:2" json:"time_ms"`
	// Mode scroll / top / bottom
	Mode string `gorm:"size:12;not null;default:scroll" json:"mode"`
	// Color #rrggbb，空串表示默认白色
	Color     string    `gorm:"size:16;default:''" json:"color"`
	Status    string    `gorm:"size:16;not null;default:published;index" json:"status"`
	CreatedAt time.Time `json:"created_at"`

	AuthorName string `gorm:"-" json:"author_name,omitempty"`
}

func (Danmaku) TableName() string { return "feed_danmaku" }

const (
	danmakuMaxLen = 80   // 单条最多 80 字，超过 255 字节列上限也放不下
	danmakuMaxPer = 3000 // 单次拉取上限，防止超长视频一次拉爆
)

// ValidateDanmaku 输入校验（handler 与模型共用，保证口径一致）
func ValidateDanmaku(content, mode, color string, timeMs int) (string, string, string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", "", "", errors.New("弹幕内容不能为空")
	}
	if len([]rune(content)) > danmakuMaxLen {
		return "", "", "", errors.New("弹幕最多 80 个字")
	}
	// 弹幕是单行盖在画面上，换行会破坏排版
	content = strings.ReplaceAll(content, "\r", " ")
	content = strings.ReplaceAll(content, "\n", " ")

	switch mode {
	case DanmakuTop, DanmakuBottom, DanmakuScroll:
	case "":
		mode = DanmakuScroll
	default:
		return "", "", "", errors.New("弹幕模式不合法")
	}

	color = strings.ToLower(strings.TrimSpace(color))
	if color != "" && !isHexColor(color) {
		return "", "", "", errors.New("弹幕颜色需为 #rrggbb")
	}
	if timeMs < 0 {
		timeMs = 0
	}
	return content, mode, color, nil
}

func isHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// CreateDanmaku 发一条弹幕
func CreateDanmaku(postID, userID uint, content string, timeMs int, mode, color string) (*Danmaku, error) {
	content, mode, color, err := ValidateDanmaku(content, mode, color, timeMs)
	if err != nil {
		return nil, err
	}
	if models.DB == nil {
		return nil, errors.New("数据库不可用")
	}
	var p Post
	if err := models.DB.Where("id = ? AND status = ? AND kind = ?", postID, StatusPublished, KindVideo).
		First(&p).Error; err != nil {
		return nil, errors.New("视频不存在或已下架")
	}
	// 弹幕时间点不能超出视频时长（时长未知时放行，前端可能先发后取元数据）
	if p.Duration > 0 && timeMs > (p.Duration+5)*1000 {
		return nil, errors.New("弹幕时间点超出视频时长")
	}
	d := &Danmaku{
		PostID: postID, UserID: userID, Content: content, TimeMs: timeMs,
		Mode: mode, Color: color, Status: DanmakuPublished, CreatedAt: time.Now(),
	}
	if err := models.DB.Create(d).Error; err != nil {
		return nil, err
	}
	return d, nil
}

// ListDanmaku 按时间窗拉取（from/to 为毫秒，闭区间；to<=0 表示不设上限）
func ListDanmaku(postID uint, fromMs, toMs, limit int) ([]Danmaku, error) {
	if models.DB == nil {
		return nil, errors.New("数据库不可用")
	}
	if fromMs < 0 {
		fromMs = 0
	}
	if limit <= 0 || limit > danmakuMaxPer {
		limit = danmakuMaxPer
	}
	q := models.DB.Where("post_id = ? AND status = ? AND time_ms >= ?", postID, DanmakuPublished, fromMs)
	if toMs > 0 {
		q = q.Where("time_ms <= ?", toMs)
	}
	var list []Danmaku
	if err := q.Order("time_ms ASC, id ASC").Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	fillDanmakuAuthors(list)
	return list, nil
}

// CountDanmaku 弹幕总数（播放器上显示「弹幕 N」）
func CountDanmaku(postID uint) int64 {
	if models.DB == nil {
		return 0
	}
	var n int64
	_ = models.DB.Model(&Danmaku{}).Where("post_id = ? AND status = ?", postID, DanmakuPublished).Count(&n).Error
	return n
}

// DeleteDanmaku 删自己的弹幕；管理员/作者删他人由上层判断后再调 ForceDeleteDanmaku
func DeleteDanmaku(userID, id uint) error {
	if models.DB == nil {
		return errors.New("数据库不可用")
	}
	res := models.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&Danmaku{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("弹幕不存在或无权删除")
	}
	return nil
}

// ForceDeleteDanmaku 管理员/作者删任意弹幕
func ForceDeleteDanmaku(id uint) error {
	if models.DB == nil {
		return errors.New("数据库不可用")
	}
	return models.DB.Where("id = ?", id).Delete(&Danmaku{}).Error
}

func fillDanmakuAuthors(list []Danmaku) {
	if len(list) == 0 {
		return
	}
	ids := make([]uint, 0, len(list))
	seen := map[uint]bool{}
	for _, d := range list {
		if d.UserID > 0 && !seen[d.UserID] {
			seen[d.UserID] = true
			ids = append(ids, d.UserID)
		}
	}
	if len(ids) == 0 {
		return
	}
	type row struct {
		ID       uint
		Nickname string
		Username string
	}
	var rows []row
	_ = models.DB.Model(&models.User{}).Select("id, nickname, username").Where("id IN ?", ids).Scan(&rows).Error
	names := make(map[uint]string, len(rows))
	for _, r := range rows {
		if r.Nickname != "" {
			names[r.ID] = r.Nickname
		} else {
			names[r.ID] = r.Username
		}
	}
	for i := range list {
		list[i].AuthorName = names[list[i].UserID]
	}
}
