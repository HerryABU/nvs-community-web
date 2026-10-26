package handlers

import (
	"nvs-server/models"
	"nvs-server/sentiment"
	"nvs-server/sentiment/store"
	"nvs-server/sentiment/topic"

	"github.com/gin-gonic/gin"
)

// 评论区舆情守卫的接入层。
//
// 单独成文件而不是塞进 comment.go：comment.go 已经背着楼层、楼加密、@提及、
// 黑名单等一堆职责，再塞审核会变成新的上帝函数。

// contentRef 解析评论挂在什么内容上（守卫要用它取话题标注）。
func contentRef(novelID, blogID uint) (contentType string, contentID uint) {
	if novelID > 0 {
		return store.ContentNovel, novelID
	}
	if blogID > 0 {
		return store.ContentBlog, blogID
	}
	return "", 0
}

// guardComment 对一条待发布评论做舆情裁决。
//
// 返回 blocked=true 时调用方必须直接拒绝、不再落库。
// 话题标注由 AI 审核写入（sentiment_content_topics）；没有标注时按低风险处理，
// 只有「强制拦截词」与「刷屏出征行为」生效——不会凭猜测去删评论。
func guardComment(c *gin.Context, novelID, blogID, userID uint, text string) (blocked bool, reason string) {
	contentType, contentID := contentRef(novelID, blogID)
	if contentType == "" || contentID == 0 {
		return false, ""
	}

	state := store.StateOf(contentType, contentID)
	// 文本自身的升档信号（出征 / 挂人 / 群体蔑称）要叠到内容话题上：
	// 同一话题下，平铺直叙与组织化围攻必须区别对待。
	if state.Exists {
		state.Assessment = sentiment.Reassess(state.Assessment, text)
	}

	d := sentiment.Evaluate(state, text)

	username := ""
	if u, err := models.GetUserByID(userID); err == nil && u != nil {
		username = u.Username
	}
	sentiment.Record(d, sentiment.Meta{
		ContentType: contentType, ContentID: contentID,
		UserID: userID, Username: username,
	})

	if d.Action == sentiment.ActionBlock {
		return true, d.Reason
	}
	return false, ""
}

// reassessContent 把评论文本的升档信号并回话题评估（供 AI 审核写标注时复用）。
func reassessContent(a topic.Assessment, text string) topic.Assessment {
	return sentiment.Reassess(a, text)
}
