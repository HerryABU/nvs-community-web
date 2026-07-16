package security

import (
	"os"
	"path/filepath"
	"strings"
)

// BlockedScriptExtensions 禁止在沙盒目录中创建/修改的脚本类扩展名
// 防止侧链payload攻击：攻击者通过已运行的代码在目录中植入新的脚本文件
var BlockedScriptExtensions = []string{
	".js", ".mjs", ".cjs", ".jsx", ".ts", ".tsx",
	".wasm", ".wat",
	".html", ".htm", ".shtml", ".xhtml",
	".css", ".scss", ".less",
	".php", ".phtml", ".php3", ".php4", ".php5",
	".asp", ".aspx", ".jsp", ".jspx",
	".py", ".pyc", ".pyo", ".pyd",
	".rb", ".erb",
	".pl", ".pm",
	".sh", ".bash", ".zsh", ".fish",
	".bat", ".cmd", ".ps1", ".psm1", ".psd1",
	".exe", ".dll", ".so", ".dylib",
	".vbs", ".vbe", ".wsf", ".wsc",
	".svg", // SVG可嵌入脚本
}

// LockDirectory 锁定沙盒目录：将所有文件设为只读，目录设为只读
// 在Windows上使用文件属性，在Unix上使用chmod
func LockDirectory(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}

		// 跳过 _original.zip（保留原始ZIP备份的可写权限）
		if strings.HasSuffix(path, "_original.zip") {
			return nil
		}

		// 跳过已有的锁标记文件
		if info.Name() == ".nvs_sandbox_lock" {
			return nil
		}

		// 文件→只读 0444（禁止修改已提取的文件）
		// 目录→保持可写 0755（允许创建数据文件，脚本文件由中间件拦截）
		if info.IsDir() {
			os.Chmod(path, 0755) // rwxr-xr-x — 允许在目录中创建数据文件
		} else {
			os.Chmod(path, 0444) // r--r--r-- — 已提取文件不可修改
		}

		return nil
	})
}

// IsPathAllowed 验证给定路径是否在允许的基础目录内
// 防止目录穿越攻击
func IsPathAllowed(baseDir, targetPath string) bool {
	// 清理并转为绝对路径
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return false
	}

	// 确保目标路径在基础目录内
	return strings.HasPrefix(absTarget, absBase+string(filepath.Separator)) ||
		absTarget == absBase
}

// IsBlockedExtension 检查文件扩展名是否在被禁止列表中
func IsBlockedExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	for _, blocked := range BlockedScriptExtensions {
		if ext == blocked {
			return true
		}
	}
	return false
}

// GetSandboxLockFile 获取沙盒目录的锁标记文件路径
func GetSandboxLockFile(dir string) string {
	return filepath.Join(dir, ".nvs_sandbox_lock")
}

// IsSandboxLocked 检查沙盒目录是否已锁定
func IsSandboxLocked(dir string) bool {
	_, err := os.Stat(GetSandboxLockFile(dir))
	return err == nil
}

// MarkSandboxLocked 在沙盒目录写入锁标记，表明已完成安全加固
func MarkSandboxLocked(dir string) error {
	return os.WriteFile(GetSandboxLockFile(dir), []byte(`NVS Sandbox Lock - Do not remove
This directory is locked for security.
No new script files may be created.
`), 0444)
}
