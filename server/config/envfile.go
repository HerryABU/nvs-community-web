package config

// envfile.go — .env 运行时同步写回（管理后台保存配置时自动落盘）。
//
// 设计要点：
//   - 数据库（platform_configs）是运行时唯一真相；.env 同步的目标是「重启后仍生效」
//     以及让外部工具/部署脚本读到一致值，因此写失败不阻断前台保存（仅告警）。
//   - 已存在的键原位替换（保留注释与其他行），不存在的键追加到文件末尾。
//   - 临时文件 + rename 原子替换，防止半写损坏；互斥锁保护并发保存。
//   - 兼容 CRLF/LF 与 export 前缀、引号包裹；加载端 loadEnvFile 按首个 '=' 分割
//     并去除首尾引号，写端 quoteEnvValue 与之对齐。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var (
	envSyncMu   sync.Mutex
	envSyncPath string // Init 时记录实际加载的 .env 路径（空则回退推断）
)

// SetEnvFilePath 记录实际加载的 .env 路径（Init 中调用）。
func SetEnvFilePath(p string) {
	envSyncMu.Lock()
	defer envSyncMu.Unlock()
	envSyncPath = p
}

// GetEnvFilePath 返回 .env 实际路径：优先启动时加载的文件；
// 未记录时按「exe 同级目录」→「当前工作目录」推断（都可能不存在，调用方按追加模式处理）。
// 注意：持锁调用方（SyncEnvFile）必须用 envFilePathLocked，否则自死锁。
func GetEnvFilePath() string {
	envSyncMu.Lock()
	defer envSyncMu.Unlock()
	return envFilePathLocked()
}

// envFilePathLocked 路径推断（调用方必须已持有 envSyncMu）
func envFilePathLocked() string {
	if envSyncPath != "" {
		return envSyncPath
	}
	if exe, err := os.Executable(); err == nil {
		cand := filepath.Join(filepath.Dir(exe), ".env")
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	return ".env"
}

// envKeyRe 合法的 .env 键名（防注入换行/等号破坏文件结构）
var envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// SyncEnvFile 将 updates 写回 .env，返回实际追加（原先不存在）的键名列表。
// 非法键名跳过；文件不存在时按纯追加创建。
func SyncEnvFile(updates map[string]string) (appended []string, err error) {
	envSyncMu.Lock()
	defer envSyncMu.Unlock()

	// 过滤非法键，防止注入
	clean := make(map[string]string, len(updates))
	for k, v := range updates {
		if envKeyRe.MatchString(k) {
			clean[k] = v
		}
	}
	if len(clean) == 0 {
		return nil, nil
	}

	path := envFilePathLocked()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	content := string(data)
	crlf := strings.Contains(content, "\r\n")
	if crlf {
		content = strings.ReplaceAll(content, "\r\n", "\n")
	}
	// 剥离结尾换行再 split，避免结尾空行被「吃」到中间导致输出丢失结尾换行
	hadTrailingNL := strings.HasSuffix(content, "\n")
	if hadTrailingNL {
		content = content[:len(content)-1]
	}
	lines := strings.Split(content, "\n")

	// 原位替换：保留注释与无关行；同键多次出现只改第一处（与加载端语义一致）
	seen := make(map[string]bool, len(clean))
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		eq := strings.Index(trimmed, "=")
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:eq])
		key = strings.TrimPrefix(key, "export ")
		key = strings.TrimSpace(key)
		if nv, ok := clean[key]; ok && !seen[key] {
			lines[i] = key + "=" + quoteEnvValue(nv)
			seen[key] = true
		}
	}

	// 缺失键追加到末尾（先补一个空行分隔，保持可读）
	var missing []string
	for k, v := range clean {
		if seen[k] {
			continue
		}
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, k+"="+quoteEnvValue(v))
		missing = append(missing, k)
	}
	sort.Strings(missing)

	out := strings.Join(lines, "\n")
	if hadTrailingNL || len(missing) > 0 || out != "" {
		out += "\n" // .env 惯例以换行结尾
	}
	if crlf {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}

	// 原子写：同目录临时文件 + rename
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	tmp, err := os.CreateTemp(dir, ".env-sync-*")
	if err != nil {
		return nil, err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(out); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return nil, err
	}
	if err := os.Chmod(tmpName, 0644); err != nil {
		os.Remove(tmpName)
		return nil, err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return nil, err
	}
	return missing, nil
}

// quoteEnvValue 值含空白/#/引号时用双引号包裹（加载端会去除首尾引号）。
func quoteEnvValue(v string) string {
	if v == "" {
		return ""
	}
	if strings.ContainsAny(v, " \t#\"'") {
		return "\"" + strings.ReplaceAll(v, "\"", "\\\"") + "\""
	}
	return v
}

// PlatformConfigEnvKeys 平台配置键 → .env 键的同步映射。
// 仅列出 .env 中存在对应语义的键；未列出项（如 novel_visitor_mode、home_module）
// 只落数据库，不写 .env。
var PlatformConfigEnvKeys = map[string]string{
	"site_name":          "SITE_NAME",
	"email_verify":       "EMAIL_VERIFY_ENABLED",
	"captcha_enabled":    "CAPTCHA_ENABLED",
	"smtp_host":          "SMTP_HOST",
	"smtp_port":          "SMTP_PORT",
	"smtp_user":          "SMTP_USER",
	"smtp_password":      "SMTP_PASSWORD",
	"smtp_from":          "SMTP_FROM",
	"log_redact_enabled": "LOG_REDACT_SENSITIVE",
	"log_redact_mask":    "LOG_REDACT_MASK",
	"log_redact_fields":  "LOG_SENSITIVE_FIELDS",
}

// SyncPlatformConfigEnv 把平台配置变更同步写回 .env（重启后仍生效）。
// kv 为「平台配置键 → 值」；仅映射表内的键被写入。
// 返回实际同步的 .env 键列表（排序），无映射键时返回 nil。
func SyncPlatformConfigEnv(kv map[string]string) ([]string, error) {
	updates := make(map[string]string, len(PlatformConfigEnvKeys))
	for pKey, envKey := range PlatformConfigEnvKeys {
		v, ok := kv[pKey]
		if !ok {
			continue
		}
		if pKey == "log_redact_fields" {
			v = jsonArrayToCSV(v)
		}
		updates[envKey] = v
	}
	if len(updates) == 0 {
		return nil, nil
	}
	if _, err := SyncEnvFile(updates); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(updates))
	for k := range updates {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

// jsonArrayToCSV 日志脱敏字段在 DB 中以 JSON 数组存储、.env 中以逗号分隔存储。
// 输入既可是 JSON 数组（`["a","b"]`）也已是 CSV（`a,b`），后者原样规范化返回。
func jsonArrayToCSV(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	var arr []string
	if err := json.Unmarshal([]byte(v), &arr); err == nil {
		out := make([]string, 0, len(arr))
		for _, s := range arr {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return strings.Join(out, ",")
	}
	// 已是 CSV / 其他文本：去除空白后返回
	return strings.Join(strings.FieldsFunc(v, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t'
	}), ",")
}
