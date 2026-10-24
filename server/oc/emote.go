package oc

// ==================== 气泡头像与自定义表情包（gap-audit #22）====================
// 已有：内置表情词典（scriptmode/emoji.go）+ `emote_map`（情绪→立绘 URL）。
// 缺的是「用户上传自己的表情/头像，并在气泡里用」。
//
// 设计取舍：
//   - 表情键**全局唯一**：气泡正文是服务端渲染的，没有「当前用户」上下文；
//     若允许同名键，A 的 `[笑]` 会渲染成 B 的图。冲突时直接拒绝并提示改名，
//     比「静默覆盖别人」诚实。
//   - 只接受站内 /uploads 或 http(s) 图片地址（拒绝 javascript:/data:，防 XSS）。
//   - 图片本体走既有 `POST /api/upload`（已有 10MB/类型/后缀校验），本包只管注册表，
//     不重复实现上传——避免出现第二套「能传什么」的口径。

import (
	"errors"
	"strings"
	"sync"
	"time"

	"nvs-server/models"

	"gorm.io/gorm"
)

// Emote 自定义表情（key → 图片地址）
type Emote struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Key       string    `gorm:"size:16;not null;uniqueIndex" json:"key"` // 全局唯一（渲染无用户上下文）
	URL       string    `gorm:"size:512;not null" json:"url"`
	Note      string    `gorm:"size:64;default:''" json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

func (Emote) TableName() string { return "oc_emotes" }

// ValidateEmoteKey 表情键：1~8 个非空白字符，不含方括号（否则与语法冲突）
func ValidateEmoteKey(k string) error {
	k = strings.TrimSpace(k)
	if k == "" {
		return errors.New("表情键不能为空")
	}
	if n := len([]rune(k)); n > 8 {
		return errors.New("表情键最长 8 个字")
	}
	if strings.ContainsAny(k, "[] \t\n") {
		return errors.New("表情键不能包含方括号或空白（与 [表情] 语法冲突）")
	}
	return nil
}

// ValidateEmoteURL 只允许站内上传路径或 http(s)
func ValidateEmoteURL(u string) error {
	u = strings.TrimSpace(u)
	if u == "" {
		return errors.New("图片地址不能为空")
	}
	low := strings.ToLower(u)
	for _, bad := range []string{"javascript:", "data:", "vbscript:", "file:", "blob:"} {
		if strings.HasPrefix(low, bad) {
			return errors.New("不支持的图片协议")
		}
	}
	if strings.HasPrefix(u, "/uploads/") || strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		return nil
	}
	return errors.New("图片需为站内上传地址（/uploads/…）或 http(s) 链接")
}

// CreateEmote 新增（键冲突直接拒绝，不覆盖他人）
func CreateEmote(userID uint, key, url, note string) (*Emote, error) {
	if userID == 0 {
		return nil, errors.New("请先登录")
	}
	key = strings.TrimSpace(key)
	if err := ValidateEmoteKey(key); err != nil {
		return nil, err
	}
	if err := ValidateEmoteURL(url); err != nil {
		return nil, err
	}
	var exist Emote
	if err := models.DB.Where("key = ?", key).First(&exist).Error; err == nil {
		if exist.UserID == userID {
			return nil, errors.New("你已经用这个表情键了，换个名字或先删除")
		}
		return nil, errors.New("该表情键已被其他用户占用（表情是全局的，请换个名字）")
	}
	e := &Emote{UserID: userID, Key: key, URL: strings.TrimSpace(url),
		Note: truncate(note, 64), CreatedAt: time.Now()}
	if err := models.DB.Create(e).Error; err != nil {
		return nil, err
	}
	ReloadCustomEmotes()
	return e, nil
}

// ListMyEmotes 我的表情
func ListMyEmotes(userID uint) ([]Emote, error) {
	var list []Emote
	err := models.DB.Where("user_id = ?", userID).Order("id DESC").Limit(200).Find(&list).Error
	return list, err
}

// DeleteEmote 删除自己的表情
func DeleteEmote(userID, id uint) error {
	res := models.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&Emote{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("表情不存在或不属于你")
	}
	ReloadCustomEmotes()
	return nil
}

// ─────────────── 全局注册表 ───────────────

var (
	emoteMu     sync.RWMutex
	emoteGlobal = map[string]string{} // key → url
	emoteLoaded time.Time
)

// ReloadCustomEmotes 从库重载到内存（写操作后调用；进程启动时调一次）
func ReloadCustomEmotes() {
	var list []Emote
	if err := models.DB.Find(&list).Error; err != nil {
		return
	}
	m := make(map[string]string, len(list))
	for _, e := range list {
		m[e.Key] = e.URL
	}
	emoteMu.Lock()
	emoteGlobal = m
	emoteLoaded = time.Now()
	emoteMu.Unlock()
}

// CustomEmoteMap 当前注册表快照（供 scriptmode 渲染）
func CustomEmoteMap() map[string]string {
	emoteMu.RLock()
	defer emoteMu.RUnlock()
	out := make(map[string]string, len(emoteGlobal))
	for k, v := range emoteGlobal {
		out[k] = v
	}
	return out
}

// ensureLoaded 懒加载（避免启动顺序依赖）
func ensureLoaded() {
	emoteMu.RLock()
	empty := len(emoteGlobal) == 0
	emoteMu.RUnlock()
	if empty {
		ReloadCustomEmotes()
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// MigrateEmote 建表
func MigrateEmote(d *gorm.DB) error { return d.AutoMigrate(&Emote{}) }
