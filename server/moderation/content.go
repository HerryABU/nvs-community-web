package moderation

import (
	"os"
	"strings"

	"nvs-server/models"
	"nvs-server/scriptmode"
	"nvs-server/utils"
)

// ResolveContent 从内容源抽取待审纯文本。返回 (text, found)。
// 话本气泡体（script/mixed）会先解析再拼接可读文本，避免把「角色：台词」原样丢给 AI。
func ResolveContent(scope string, targetID uint) (string, bool) {
	switch scope {
	case ScopeComment:
		c, err := models.GetCommentByID(targetID)
		if err != nil || c == nil {
			return "", false
		}
		return c.Content, true
	case ScopePost:
		var p models.Post
		if err := models.DB.First(&p, targetID).Error; err != nil {
			return "", false
		}
		return p.Content, true
	case ScopeThread:
		t, err := models.GetThreadByID(targetID)
		if err != nil || t == nil {
			return "", false
		}
		return t.Title + "\n" + t.Content, true
	case ScopeBlog:
		b, err := models.GetBlogByID(targetID)
		if err != nil || b == nil || b.ContentPath == "" {
			return "", false
		}
		return readTextFile(b.ContentPath), true
	case ScopeChapter:
		return resolveChapterContent(targetID)
	case ScopeNovel:
		return resolveNovelContent(targetID)
	case ScopeForum:
		f, err := models.GetForumByID(targetID)
		if err != nil || f == nil {
			return "", false
		}
		return f.Name + "\n" + f.Description, true
	}
	return "", false
}

// resolveChapterContent 读取章节正文；script/mixed 模式解析为可读文本。
func resolveChapterContent(chapterID uint) (string, bool) {
	ch, err := models.GetChapterByID(chapterID)
	if err != nil || ch == nil || ch.ContentPath == "" {
		return "", false
	}
	raw := readTextFile(ch.ContentPath)
	if raw == "" {
		return "", false
	}
	mode := ch.Mode
	if mode == "" {
		if n, e := models.GetNovelByID(ch.NovelID); e == nil && n != nil {
			mode = n.Mode
		}
	}
	if mode == "script" || mode == "mixed" {
		var nodes []scriptmode.Node
		if mode == "mixed" {
			nodes = scriptmode.ParseMixed(raw)
		} else {
			nodes = scriptmode.Parse(raw)
		}
		var sb strings.Builder
		for _, n := range nodes {
			if n.Text == "" {
				continue
			}
			if n.Type == scriptmode.NodeDialogue && n.Speaker != "" {
				sb.WriteString(n.Speaker)
				sb.WriteString("：")
			}
			sb.WriteString(n.Text)
			sb.WriteString("\n")
		}
		return sb.String(), true
	}
	return raw, true
}

// resolveNovelContent 聚合小说各章正文（上限 20 章，避免单次过长）。
func resolveNovelContent(novelID uint) (string, bool) {
	var chapters []models.Chapter
	if err := models.DB.Where("novel_id = ?", novelID).Order("chapter_no ASC").Limit(20).Find(&chapters).Error; err != nil {
		return "", false
	}
	var sb strings.Builder
	for _, ch := range chapters {
		if ch.ContentPath == "" {
			continue
		}
		txt := readTextFile(ch.ContentPath)
		if txt == "" {
			continue
		}
		sb.WriteString("【")
		sb.WriteString(ch.Title)
		sb.WriteString("】\n")
		sb.WriteString(txt)
		sb.WriteString("\n")
	}
	if sb.Len() == 0 {
		return "", false
	}
	return sb.String(), true
}

// readTextFile 按内容路径读文件（兼容跨平台分隔符）。
func readTextFile(path string) string {
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(utils.ResolveContentPath(path))
	if err != nil {
		return ""
	}
	return string(b)
}
