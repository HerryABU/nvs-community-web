package chaptervcs

// ==================== 快照存储 ====================
// 目录结构：<NovelDataDir>/authors/{主笔ID}/{作品ID}/.versions/{章节号}/{分支}/{版本号}.md
// 与章节正文同属一个作品目录，备份/迁移时整目录带走即可。

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nvs-server/config"
)

// chapterDir 作品正文目录（与主笔 ID 绑定）
func chapterDir(ownerID, novelID uint) string {
	return filepath.Join(config.NovelDataDir, "authors", fmt.Sprintf("%d", ownerID), fmt.Sprintf("%d", novelID))
}

// snapshotRel 快照相对路径（库里存正斜杠，跨平台可移植）
func snapshotRel(num int, branch string, version int) string {
	return fmt.Sprintf(".versions/%d/%s/%d.md", num, NormalizeBranch(branch), version)
}

// snapshotAbs 快照绝对路径
func snapshotAbs(ownerID, novelID uint, num int, branch string, version int) string {
	return filepath.Join(chapterDir(ownerID, novelID), filepath.FromSlash(snapshotRel(num, branch, version)))
}

// writeSnapshot 写入快照文件，返回（相对路径, 哈希, 字数, error）
func writeSnapshot(ownerID, novelID uint, num int, branch string, version int, content string) (string, string, int, error) {
	abs := snapshotAbs(ownerID, novelID, num, branch, version)
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		return "", "", 0, err
	}
	if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
		return "", "", 0, err
	}
	sum := sha256.Sum256([]byte(content))
	return filepath.ToSlash(snapshotRel(num, branch, version)), hex.EncodeToString(sum[:]), len([]rune(content)), nil
}

// readSnapshot 读取快照正文（文件缺失返回空串，不报错——历史快照可被作者清理）
func readSnapshot(ownerID, novelID uint, v ChapterVersion) (string, error) {
	abs := filepath.Join(chapterDir(ownerID, novelID), filepath.FromSlash(v.ContentPath))
	data, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// deleteSnapshot 清理快照文件（元数据保留）
func deleteSnapshot(ownerID, novelID uint, v ChapterVersion) error {
	if strings.TrimSpace(v.ContentPath) == "" {
		return nil
	}
	abs := filepath.Join(chapterDir(ownerID, novelID), filepath.FromSlash(v.ContentPath))
	if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// hashOf 内容哈希（与章节正文哈希口径一致：SHA256 十六进制）
func hashOf(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
