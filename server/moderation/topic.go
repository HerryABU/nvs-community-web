package moderation

import (
	"nvs-server/models"
	sentimentstore "nvs-server/sentiment/store"
)

// AI 审核产出的话题标注落库。
//
// 为什么单独一个文件：话题标注是「舆情守卫」的输入，不是审核结论的一部分。
// 但它只能由审核流程产出（AI 已经在读全文，顺带判断语境不额外花钱），
// 所以放在 moderation 里写入、由 sentimentstore 存储，职责清晰且不成环：
// moderation → sentiment/store（单向）。

// topicRef 把审核 scope 映射成「舆情守卫认的内容类型 + 内容 id」。
//
// 评论守卫只关心它挂在的作品/博客上，所以：
//   - 章节审完了要归到**所属作品**（评论区挂在作品下，不挂章节）
//   - 帖子/评论/论坛版块本身不作为话题载体，返回空表示不写
func topicRef(scope string, targetID uint) (string, uint) {
	switch scope {
	case ScopeNovel:
		return sentimentstore.ContentNovel, targetID
	case ScopeBlog:
		return sentimentstore.ContentBlog, targetID
	case ScopeChapter:
		// 章节 → 所属作品：评论区的上下文是作品，不是单章
		var ch models.Chapter
		if err := models.DB.Select("id", "novel_id").First(&ch, targetID).Error; err != nil {
			return "", 0
		}
		if ch.NovelID == 0 {
			return "", 0
		}
		return sentimentstore.ContentNovel, ch.NovelID
	default:
		return "", 0
	}
}

// persistTopic 写入 AI 敲定的话题标注（人工标注优先，不会被 AI 覆盖）。
//
// 没有任何标签时不写：宁可这条内容「无话题」（守卫只受强制词/刷屏约束），
// 也不要写一条空标注——空标注会让后台看起来「已审核」而实际没信息。
func persistTopic(scope string, targetID uint, res *ReviewResult) {
	if res == nil || len(res.Topics) == 0 {
		return
	}
	ct, id := topicRef(scope, targetID)
	if ct == "" || id == 0 {
		return
	}
	_ = sentimentstore.ApplyAIAssessment(ct, id, res.Topics, res.TopicConfidence)
}

// getContentTopic 读回话题标注（供审核记录页展示「这条内容被判成什么话题」）。
// 没有则返回 nil，调用方自行处理空态。
func getContentTopic(scope string, targetID uint) *sentimentstore.ContentTopic {
	ct, id := topicRef(scope, targetID)
	if ct == "" || id == 0 {
		return nil
	}
	t, err := sentimentstore.TopicOf(ct, id)
	if err != nil {
		return nil
	}
	return t
}
