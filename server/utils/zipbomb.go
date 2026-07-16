package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ZipBombConfig ZIP 炸弹检测配置
type ZipBombConfig struct {
	MaxUncompressedSize     int64 // 最大解压后总大小（字节），默认 50MB
	MaxSingleFileSize       int64 // 单个文件最大解压后大小（字节），默认 20MB
	MaxCompressionRatio     int64 // 最大压缩比（分母为1），默认 100（即压缩率 > 100:1 视为炸弹）
	MaxFileCount            int   // 最大文件数，默认 500
}

// DefaultZipBombConfig 默认安全配置
var DefaultZipBombConfig = ZipBombConfig{
	MaxUncompressedSize: 50 * 1024 * 1024,  // 50 MB
	MaxSingleFileSize:   20 * 1024 * 1024,  // 20 MB
	MaxCompressionRatio: 100,               // 100:1
	MaxFileCount:        500,
}

// ZipEntry 解压后的文件条目
type ZipEntry struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Path string `json:"path"` // 解压后的磁盘路径
}

// ValidateZipBomb 检测 ZIP 是否为压缩包炸弹
// 返回 (是否安全, 错误信息)
func ValidateZipBomb(reader *zip.Reader, cfg *ZipBombConfig) (bool, string) {
	if cfg == nil {
		cfg = &DefaultZipBombConfig
	}

	var totalUncompressed int64
	fileCount := 0

	for _, f := range reader.File {
		fileCount++
		if fileCount > cfg.MaxFileCount {
			return false, fmt.Sprintf("ZIP 包含 %d 个文件，超过最大限制 %d", fileCount, cfg.MaxFileCount)
		}

		// 使用 FileInfo 获取未压缩大小
		uncompressed := f.UncompressedSize64
		if uncompressed == 0 && f.FileInfo().Size() > 0 {
			uncompressed = uint64(f.FileInfo().Size())
		}

		if uncompressed > uint64(cfg.MaxSingleFileSize) {
			return false, fmt.Sprintf("文件「%s」解压后大小 %s，超过单文件限制 %s",
				f.Name,
				formatSize(int64(uncompressed)),
				formatSize(cfg.MaxSingleFileSize))
		}

		// 检查压缩比
		compressed := f.CompressedSize64
		if compressed > 0 && uncompressed > 0 {
			ratio := int64(uncompressed / compressed)
			if ratio > cfg.MaxCompressionRatio {
				return false, fmt.Sprintf("文件「%s」压缩比 %d:1，超过最大压缩比 %d:1（疑似压缩炸弹）",
					f.Name, ratio, cfg.MaxCompressionRatio)
			}
		}

		totalUncompressed += int64(uncompressed)
		if totalUncompressed > cfg.MaxUncompressedSize {
			return false, fmt.Sprintf("ZIP 解压后总大小 %s，超过最大限制 %s",
				formatSize(totalUncompressed),
				formatSize(cfg.MaxUncompressedSize))
		}

		// 检测目录穿越攻击
		if containsPathTraversal(f.Name) {
			return false, fmt.Sprintf("文件路径包含非法字符: %s", f.Name)
		}
	}

	return true, ""
}

// ExtractZipSafe 安全解压 ZIP 到目标目录
// 先通过炸弹检测，然后逐文件解压
func ExtractZipSafe(reader *zip.Reader, destDir string, cfg *ZipBombConfig) ([]ZipEntry, error) {
	if cfg == nil {
		cfg = &DefaultZipBombConfig
	}

	// 先做炸弹检测
	safe, reason := ValidateZipBomb(reader, cfg)
	if !safe {
		return nil, fmt.Errorf("ZIP 安全检查失败: %s", reason)
	}

	var entries []ZipEntry
	var totalWritten int64

	for _, f := range reader.File {
		// 跳过目录
		if f.FileInfo().IsDir() {
			continue
		}

		// 安全检查：路径穿越
		if containsPathTraversal(f.Name) {
			return nil, fmt.Errorf("文件路径包含非法字符: %s", f.Name)
		}

		// 只允许安全扩展名
		if !isAllowedExtension(f.Name) {
			return nil, fmt.Errorf("不允许的文件类型: %s", f.Name)
		}

		// 构建安全路径
		destPath := safeJoin(destDir, f.Name)

		// 打开源文件
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("无法读取 %s: %w", f.Name, err)
		}

		// 限制读取大小（使用 io.LimitReader 防止解压炸弹）
		limitReader := io.LimitReader(rc, cfg.MaxSingleFileSize)
		written, err := writeFileSafe(destPath, limitReader)
		rc.Close()

		if err != nil {
			return nil, fmt.Errorf("写入 %s 失败: %w", f.Name, err)
		}

		totalWritten += written
		if totalWritten > cfg.MaxUncompressedSize {
			return nil, fmt.Errorf("解压总大小超过限制")
		}

		entries = append(entries, ZipEntry{
			Name: f.Name,
			Size: written,
			Path: destPath,
		})
	}

	return entries, nil
}

// ==================== 内部辅助 ====================

func containsPathTraversal(path string) bool {
	// 检测 .. / 和 \ 的越权模式
	if path == ".." || path == "." {
		return true
	}
	// 检测是否包含 ../ 或 ..\
	for i := 0; i < len(path)-2; i++ {
		if path[i] == '.' && path[i+1] == '.' && (path[i+2] == '/' || path[i+2] == '\\') {
			return true
		}
	}
	// 绝对路径
	if len(path) > 0 && (path[0] == '/' || path[0] == '\\') {
		return true
	}
	// Windows 盘符
	if len(path) >= 2 && path[1] == ':' {
		return true
	}
	return false
}

func isAllowedExtension(name string) bool {
	allowed := map[string]bool{
		".html": true, ".htm": true, ".css": true, ".js": true, ".wasm": true,
		".json": true, ".xml": true, ".svg": true, ".png": true, ".jpg": true,
		".jpeg": true, ".gif": true, ".webp": true, ".ico": true,
		".woff": true, ".woff2": true, ".ttf": true, ".otf": true,
		".txt": true, ".md": true, ".data": true, ".map": true,
	}
	for ext := range allowed {
		if len(name) >= len(ext) && name[len(name)-len(ext):] == ext {
			return true
		}
	}
	return false
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func safeJoin(base, name string) string {
	// 清理路径，防止路径穿越
	clean := filepath.Clean(name)
	// 确保结果在 base 目录内
	fullPath := filepath.Join(base, clean)
	absBase, _ := filepath.Abs(base)
	absPath, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absPath, absBase) {
		// 如果越权，回退到安全的文件名
		safeFallback := filepath.Base(clean)
		return filepath.Join(base, safeFallback)
	}
	return fullPath
}

func writeFileSafe(path string, reader io.Reader) (int64, error) {
	os.MkdirAll(filepath.Dir(path), 0755)
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(f, reader)
}