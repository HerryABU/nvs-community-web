// Package novelindex 生成小说章节清单 index.json，并同步落到数据库。
//
// 历史上 index.json 只写文件、由 Nginx 静态直出，后端自己从不读它：
//   - 纯数据库存储模式下没有文件可读；
//   - 文件被误删/不同步时无法自查。
//
// 现在每次章节/元信息变动都同时写库，库里存的是**同一份 JSON 字节**，
// 文件与库不会各自 marshal 出不同格式。
//
// FileEnabled 是存储模式钩子：默认写文件；切到纯数据库模式后由存储层关掉。
package novelindex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/security"
)

type Entry struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Words  int    `json:"words"`
}

type Data struct {
	NovelID   uint    `json:"novel_id"`
	Title     string  `json:"title"`
	Chapters  []Entry `json:"chapters"`
	UpdatedAt string  `json:"updated_at"`
}

// FileEnabled 由存储模式在启动时设置；默认保持原有「文件+库」混合行为。
var FileEnabled = func() bool { return true }

// Write 从 DB 重建清单，写库并（按存储模式）写文件，返回最终 JSON。
// 章节数据来自 DB，因此纯数据库模式下也能生成完整清单。
func Write(authorID, novelID uint, title string) []byte {
	chapters, err := models.GetChaptersByNovel(novelID)
	if err != nil {
		return nil
	}
	if title == "" {
		var n models.Novel
		if models.DB != nil && models.DB.First(&n, novelID).Error == nil {
			title = n.Title
		}
	}
	entries := make([]Entry, len(chapters))
	for i, ch := range chapters {
		entries[i] = Entry{Number: ch.ChapterNumber, Title: ch.Title, Words: ch.WordCount}
	}
	data := Data{
		NovelID:   novelID,
		Title:     title,
		Chapters:  entries,
		UpdatedAt: security.NowString(),
	}
	buf, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil
	}
	_ = models.SaveNovelIndex(novelID, buf)
	if FileEnabled() && authorID > 0 {
		dir := filepath.Join(config.NovelDataDir, "authors", fmt.Sprintf("%d", authorID), fmt.Sprintf("%d", novelID))
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(filepath.Join(dir, "index.json"), buf, 0o644)
	}
	return buf
}

// Load 读库内副本；老数据没有副本时返回 false，调用方可以现场 Write 重建。
func Load(novelID uint) ([]byte, bool) { return models.GetNovelIndex(novelID) }

// Remove 删除整本小说时调用（文件保留，符合软删除历史行为，只清库内索引）
func Remove(novelID uint) { models.DeleteNovelIndex(novelID) }
