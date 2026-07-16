package security

import (
	"fmt"
	"regexp"
	"strings"
)

// ScanResult 扫描结果
type ScanResult struct {
	Passed  bool     `json:"passed"`
	Threats []Threat `json:"threats,omitempty"`
}

// Threat 检测到的威胁
type Threat struct {
	Category string `json:"category"` // privilege_escalation / trojan / exfiltration / shell_command / obfuscation
	Pattern  string `json:"pattern"`  // 匹配到的模式
	Severity string `json:"severity"` // critical / high / medium / low
	Detail   string `json:"detail"`   // 详细描述
}

// ==================== 恶意模式规则库 ====================

type scanRule struct {
	name     string
	category string
	severity string
	pattern  *regexp.Regexp
	detail   string
}

var maliciousRules = []scanRule{
	// ═══════ 提权命令 / 系统调用 ═══════
	{
		name:     "privilege_escalation_child_process",
		category: "privilege_escalation",
		severity: "critical",
		pattern:  regexp.MustCompile(`(?i)(child_process|require\s*\(\s*['"]child_process['"]|execSync|spawnSync|fork\s*\(|execFile)`),
		detail:   "检测到Node.js子进程调用，可能用于提权执行系统命令",
	},
	{
		name:     "privilege_escalation_system",
		category: "privilege_escalation",
		severity: "critical",
		pattern:  regexp.MustCompile(`(?i)(\bsudo\b|\bsu\b\s+-|setuid|setgid|chmod\s+[0-7]{3,4}\s+\/|chown\s+root)`),
		detail:   "检测到系统提权命令（sudo/su/setuid/chmod/chown）",
	},
	{
		name:     "privilege_escalation_passwd",
		category: "privilege_escalation",
		severity: "critical",
		pattern:  regexp.MustCompile(`(?i)(\/etc\/(passwd|shadow|sudoers|hosts)|C:\\\\Windows\\\\System32)`),
		detail:   "检测到访问系统敏感文件（passwd/shadow/sudoers）",
	},
	{
		name:     "privilege_escalation_rootkit",
		category: "privilege_escalation",
		severity: "critical",
		pattern:  regexp.MustCompile(`(?i)(insmod|modprobe|kldload|LoadLibrary|dlopen\s*\(|mmap\s*\(.*PROT_EXEC)`),
		detail:   "检测到内核模块加载/动态库注入，可能植入rootkit",
	},

	// ═══════ Shell命令注入 ═══════
	{
		name:     "shell_command_exec",
		category: "shell_command",
		severity: "critical",
		pattern:  regexp.MustCompile(`(?i)(\bexec\s*\(\s*['"](?:\/bin\/|\/usr\/bin\/|cmd\.exe|powershell\.exe|wscript|cscript))`),
		detail:   "检测到直接执行Shell命令，可能用于RCE攻击",
	},
	{
		name:     "shell_command_reverse",
		category: "shell_command",
		severity: "critical",
		pattern:  regexp.MustCompile(`(?i)(\bnc\s+-[eln]|netcat\s+-[eln]|bash\s+-i\s*>&|python\s+-c\s+['\"]import\s+socket|perl\s+-e\s+['\"]use\s+Socket)`),
		detail:   "检测到反弹Shell命令（nc/netcat/bash/python/perl反向连接）",
	},
	{
		name:     "shell_command_download",
		category: "shell_command",
		severity: "high",
		pattern:  regexp.MustCompile(`(?i)(\b(wget|curl)\s+.*\|\s*(bash|sh|python|perl)|certutil\s+-urlcache|bitsadmin\s+\/transfer)`),
		detail:   "检测到远程下载并执行命令（wget/curl pipe shell）",
	},

	// ═══════ 木马/病毒特征 ═══════
	{
		name:     "trojan_eval_chain",
		category: "trojan",
		severity: "critical",
		pattern:  regexp.MustCompile(`(?i)(eval\s*\(\s*atob\s*\(|eval\s*\(\s*unescape\s*\(|eval\s*\(\s*String\.fromCharCode|Function\s*\(\s*['"][^'"]*['"]\s*\)\s*\(\s*\))`),
		detail:   "检测到eval+解码链式调用，典型的混淆恶意代码执行模式",
	},
	{
		name:     "trojan_obfuscation",
		category: "obfuscation",
		severity: "high",
		pattern:  regexp.MustCompile(`(?i)(\\x[0-9a-fA-F]{2}){10,}`),
		detail:   "检测到大量十六进制转义序列，高度疑似混淆恶意代码",
	},
	{
		name:     "trojan_hidden_iframe",
		category: "trojan",
		severity: "high",
		pattern:  regexp.MustCompile(`(?i)(<iframe[^>]*(?:hidden|display\s*:\s*none|width\s*=\s*['"]\s*0|height\s*=\s*['"]\s*0|style\s*=\s*['"'][^'"]*(?:visibility\s*:\s*hidden|opacity\s*:\s*0)))`),
		detail:   "检测到隐藏iframe，常用于注入恶意广告/挖矿/钓鱼页面",
	},
	{
		name:     "trojan_document_write",
		category: "trojan",
		severity: "high",
		pattern:  regexp.MustCompile(`(?i)(document\.write\s*\(\s*atob\s*\(|document\.write\s*\(\s*unescape\s*\(|document\.write\s*\(\s*String\.fromCharCode)`),
		detail:   "检测到document.write动态注入解码内容，典型XSS/恶意注入",
	},
	{
		name:     "trojan_external_script",
		category: "trojan",
		severity: "medium",
		pattern:  regexp.MustCompile(`(?i)(<script[^>]*src\s*=\s*['"]https?:\/\/[^'"]*\.(?:ru|cn|top|xyz|tk|ml|ga|cf)\/[^'"]*['"])`),
		detail:   "检测到加载可疑顶级域名外部脚本（.ru/.cn/.top/.xyz/.tk等高风险域名）",
	},

	// ═══════ 僵尸网络 / C2 ═══════
	{
		name:     "botnet_websocket_c2",
		category: "botnet",
		severity: "critical",
		pattern:  regexp.MustCompile(`(?i)(new\s+WebSocket\s*\(\s*['"]wss?:\/\/[^'"]*['"]\s*\).*setInterval\s*\(|setInterval\s*\(\s*[^)]*new\s+WebSocket)`),
		detail:   "检测到WebSocket连接外部服务器+定时通信，疑似C2僵尸网络",
	},
	{
		name:     "botnet_long_poll",
		category: "botnet",
		severity: "high",
		pattern:  regexp.MustCompile(`(?i)(setInterval\s*\(\s*function\s*\(\s*\)\s*\{[^}]*fetch\s*\([^)]*beacon|setInterval\s*\(\s*function\s*\(\s*\)\s*\{[^}]*XMLHttpRequest)`),
		detail:   "检测到定时轮询远程服务器，疑似C2心跳/数据回传",
	},

	// ═══════ 数据窃取 ═══════
	{
		name:     "exfiltration_cookie",
		category: "exfiltration",
		severity: "critical",
		pattern:  regexp.MustCompile(`(?i)(document\.cookie.*(?:sendBeacon|fetch|XMLHttpRequest|Image\s*\(\s*['"])|navigator\.sendBeacon\s*\([^)]*document\.cookie)`),
		detail:   "检测到窃取Cookie并通过网络发送，典型的会话劫持攻击",
	},
	{
		name:     "exfiltration_localstorage",
		category: "exfiltration",
		severity: "high",
		pattern:  regexp.MustCompile(`(?i)(localStorage\.(?:getItem|key)\s*\(.*sendBeacon|localStorage\.(?:getItem|key)\s*\(.*WebSocket)`),
		detail:   "检测到窃取localStorage数据并外传",
	},
	{
		name:     "exfiltration_keylogger",
		category: "exfiltration",
		severity: "critical",
		pattern:  regexp.MustCompile(`(?i)(addEventListener\s*\(\s*['"]key(?:down|up|press)['"]\s*,.*sendBeacon|onkey(?:down|up|press)\s*=\s*function.*sendBeacon)`),
		detail:   "检测到键盘记录器（keylogger）+数据外传",
	},
	{
		name:     "exfiltration_clipboard",
		category: "exfiltration",
		severity: "high",
		pattern:  regexp.MustCompile(`(?i)(navigator\.clipboard\.read.*sendBeacon|navigator\.clipboard\.read.*fetch)`),
		detail:   "检测到窃取剪贴板内容并外传",
	},

	// ═══════ 挖矿脚本 ═══════
	{
		name:     "cryptominer",
		category: "trojan",
		severity: "critical",
		pattern:  regexp.MustCompile(`(?i)(CoinHive|coinhive|\.wasm.*miner|cryptonight|WebAssembly\.instantiate.*miner|new\s+Miner\s*\(|\.throttleMiner)`),
		detail:   "检测到浏览器挖矿脚本（CoinHive/CryptoNight等），盗用用户CPU资源",
	},

	// ═══════ 钓鱼/欺诈 ═══════
	{
		name:     "phishing_credential",
		category: "trojan",
		severity: "critical",
		pattern:  regexp.MustCompile(`(?i)(<form[^>]*action\s*=\s*['"]https?:\/\/[^'"]*['"][^>]*>.*<input[^>]*type\s*=\s*['"]password['"])`),
		detail:   "检测到向外部服务器提交密码的表单，疑似钓鱼攻击",
	},
}

// ScanContent 扫描内容是否包含恶意代码
// 返回 ScanResult，Passed=true 表示通过安全检查
func ScanContent(content string, filename string) ScanResult {
	result := ScanResult{Passed: true}

	// 跳过纯数据文件和图片文件
	ext := strings.ToLower(filename)
	for i := len(ext) - 1; i >= 0; i-- {
		if ext[i] == '.' {
			ext = ext[i:]
			break
		}
	}
	skipExtensions := map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
		".svg": true, ".webp": true, ".ico": true,
		".woff": true, ".woff2": true, ".ttf": true, ".otf": true,
	}
	if skipExtensions[ext] {
		return result
	}

	// 预归一化：移除注释中的干扰
	normalized := removeComments(content)

	for _, rule := range maliciousRules {
		if rule.pattern.MatchString(normalized) {
			result.Passed = false
			result.Threats = append(result.Threats, Threat{
				Category: rule.category,
				Pattern:  rule.name,
				Severity: rule.severity,
				Detail:   rule.detail,
			})
		}
	}

	return result
}

// ScanContentStrict 严格模式扫描——任何威胁都不放过
func ScanContentStrict(content string, filename string) (bool, string) {
	result := ScanContent(content, filename)
	if !result.Passed {
		// 列出所有检测到的威胁
		var threats []string
		for _, t := range result.Threats {
			threats = append(threats, fmt.Sprintf("[%s/%s] %s", t.Category, t.Severity, t.Detail))
		}
		return false, fmt.Sprintf("内容安全扫描未通过，检测到 %d 个威胁:\n%s",
			len(result.Threats), strings.Join(threats, "\n"))
	}
	return true, ""
}

// removeComments 移除JS/HTML注释以减少干扰
func removeComments(s string) string {
	// 移除 JS 单行注释
	s = regexp.MustCompile(`//.*$`).ReplaceAllString(s, "")
	// 移除 JS 多行注释
	s = regexp.MustCompile(`/\*[\s\S]*?\*/`).ReplaceAllString(s, "")
	return s
}
