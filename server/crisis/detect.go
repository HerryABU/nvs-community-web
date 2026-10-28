// Package crisis 未成年人心理危机防护（「赛博守护神」方案）的独立领域模块。
//
// 设计立场：AI 最有价值的能力不是拦截，而是共情。同一件事做两遍：
//   - 对用户：不是「你的内容违规」，而是「我看见了你的求救，热线在这里」——
//     折叠/先审后发 + 抚慰私信 + 危机干预热线。
//   - 对站长：AI 把工作做到极致（识别、留痕、汇总成报告），
//     但最后「要不要报警」「要不要以官方身份发关怀邮件」必须由活人按。
//
// 本包不依赖 gin / handlers，可被评论、论坛、AI 对话等多个入口统一调用，
// 彻底从 handlers/comment_crisis.go 中抽离出来，避免评论区继续膨胀成上帝文件。
package crisis

import (
	"time"

	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/sentiment/store"
)

// Target 危机处置的载荷。
// 单独抽出来是为了异步路径安全：AI 判危在 goroutine 里跑，
// 不能直接共享调用方的 *models.Comment 指针（数据竞争）。
type Target struct {
	CommentID     uint
	UserID        uint
	NovelID       uint
	BlogID        uint
	ChapterNumber int
	Raw           string
}

// IsUserInObservation 用户是否处于高危观察期（绝对保护模式生效）。
func IsUserInObservation(u *models.User) bool {
	return u != nil && u.RiskYouthMental && u.RiskYouthMentalUntil != nil &&
		u.RiskYouthMentalUntil.After(time.Now())
}

// IsMinorUser 是否未成年（MinorProtected 标记或按生日推算）。
func IsMinorUser(u *models.User) bool {
	if u == nil {
		return false
	}
	if u.MinorProtected {
		return true
	}
	if u.BirthDate == nil {
		return false
	}
	age := time.Since(*u.BirthDate)
	return age < 18*365*24*time.Hour
}

// DetectSelfHarmSlang 内置黑话命中（零延迟、必中，规则层兜底）。
func DetectSelfHarmSlang(raw, scope string) []security.BadWordHit {
	return security.ScanSelfHarm(raw, scope)
}

// ScanDistressSignals 低落语义粗筛：用内置信号挑出值得送 AI 的评论，控制模型调用成本。
func ScanDistressSignals(raw string) []string {
	return security.ScanDistressSignals(raw)
}

// snippetOf 取文本前 200 字符（危机留痕/站长告警片段）。
func snippetOf(s string) string {
	runes := []rune(s)
	if len(runes) > 200 {
		return string(runes[:200])
	}
	return s
}

// contentRef 解析评论挂在什么内容上（绝对保护模式要用它取话题标注）。
func contentRef(novelID, blogID uint) (contentType string, contentID uint) {
	if novelID > 0 {
		return store.ContentNovel, novelID
	}
	if blogID > 0 {
		return store.ContentBlog, blogID
	}
	return "", 0
}
