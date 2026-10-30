package models

import (
	"errors"
	"strings"
	"time"
)

// ==================== 首页内容形态（管理员可调） ====================
//
// 站点首页不一定非得是小说列表：博客为主的站点、论坛为主的社区都合理。
// 这里只存「用哪套视图当首页」，真实的视图组件在前端，后端只做配置与校验。
//
// 取值：
//   novels —— 作品首页（默认，保持历史行为）
//   blogs  —— 博客列表
//   forum  —— 论坛版块
//   feed   —— 混合信息流（一条龙：作品 / 博客 / 帖子混排）

const homeViewKey = "home_view"

// HomeViews 合法取值（顺序即后台下拉顺序）
var HomeViews = []string{"novels", "blogs", "forum", "feed"}

// HomeViewSetting 首页配置
type HomeViewSetting struct {
	View      string    `json:"view"`
	UpdatedAt time.Time `json:"updated_at"`
}

// validHomeView 校验取值，非法一律回退 novels（配置写坏不至于让首页 500）
func validHomeView(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	for _, x := range HomeViews {
		if x == v {
			return true
		}
	}
	return false
}

// GetHomeView 读取首页形态
func GetHomeView() string {
	v := strings.ToLower(strings.TrimSpace(GetPlatformConfig(homeViewKey)))
	if validHomeView(v) {
		return v
	}
	return "novels"
}

// SetHomeView 保存首页形态（非法值返回错误，不静默改写）
func SetHomeView(v string) error {
	v = strings.ToLower(strings.TrimSpace(v))
	if !validHomeView(v) {
		return errors.New("首页形态只能是 novels / blogs / forum / feed")
	}
	return SetPlatformConfig(homeViewKey, v)
}
