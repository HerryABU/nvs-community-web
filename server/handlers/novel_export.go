package handlers

import (
	"archive/zip"
	"fmt"
	"os"
	"strconv"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

func ExportNovel(c *gin.Context) {
	userID := c.GetUint("userID")
	userRole := c.GetString("userRole")

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "无效的作品ID")
		return
	}

	novel, err := models.GetNovelByID(uint(id))
	if err != nil {
		utils.NotFound(c, "作品不存在")
		return
	}

	if novel.AuthorID != userID && userRole != "admin" {
		utils.Forbidden(c, "无权操作此作品")
		return
	}

	chapters, err := models.GetChaptersByNovel(uint(id))
	if err != nil {
		utils.InternalError(c, "获取章节失败")
		return
	}

	// 创建临时 ZIP 文件
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("novel_%d_*.zip", id))
	if err != nil {
		utils.InternalError(c, "创建临时文件失败")
		return
	}
	defer os.Remove(tmpFile.Name())

	zipWriter := zip.NewWriter(tmpFile)

	// 添加作品信息
	infoContent := fmt.Sprintf("标题: %s\n作者ID: %d\n分类: %s\n标签: %s\n简介: %s\n", novel.Title, novel.AuthorID, novel.Category, novel.Tags, novel.Summary)
	w, _ := zipWriter.Create("novel_info.txt")
	w.Write([]byte(infoContent))

	// 添加每个章节
	for _, ch := range chapters {
		// 读取章节内容文件
		if ch.ContentPath != "" {
			content, err := os.ReadFile(ch.ContentPath)
			if err == nil {
				fileName := fmt.Sprintf("chapter_%04d.html", ch.ChapterNumber)
				w, _ := zipWriter.Create(fileName)
				w.Write(content)
			}
		}
	}

	zipWriter.Close()
	tmpFile.Close()

	// 设置下载响应头
	fileName := fmt.Sprintf("%s.zip", novel.Title)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	c.Header("Content-Type", "application/zip")
	c.File(tmpFile.Name())
}
