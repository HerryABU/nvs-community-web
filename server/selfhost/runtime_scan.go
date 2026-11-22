package selfhost

// 扫描 PATH 中已安装的运行时。仅管理员可触发；命令表固定，不拼接用户输入，
// 不使用 shell，带超时与输出截断，避免被恶意 PATH 中的同名程序长时间阻塞。

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type RuntimeCandidate struct {
	Language string `json:"language"`
	Version  string `json:"version"`
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	ExecPath string `json:"exec_path"`
	Note     string `json:"note"`
}

type probe struct {
	language string
	command  string
	args     []string
	kind     string
	label    string
}

var runtimeProbes = []probe{
	{"java", "java", []string{"-version"}, "compiled", "Java"},
	{"go", "go", []string{"version"}, "compiled", "Go"},
	{"node", "node", []string{"--version"}, "interpreted", "Node.js"},
	{"python", "python3", []string{"--version"}, "interpreted", "Python"},
	{"python", "python", []string{"--version"}, "interpreted", "Python"},
	{"php", "php", []string{"-v"}, "interpreted", "PHP"},
	{"dotnet", "dotnet", []string{"--version"}, "compiled", ".NET"},
	{"rust", "rustc", []string{"--version"}, "compiled", "Rust"},
	{"gcc", "gcc", []string{"--version"}, "compiled", "GCC"},
	{"maven", "mvn", []string{"-v"}, "compiled", "Maven"},
}

var scanVersionRe = regexp.MustCompile(`(\d+(?:\.\d+){0,3}(?:[-+][0-9A-Za-z._]+)?)`)

// ScanPathRuntimes 返回 PATH 中可执行的运行时及探测到的版本。
func ScanPathRuntimes() []RuntimeCandidate {
	out := make([]RuntimeCandidate, 0, len(runtimeProbes))
	seen := map[string]bool{}
	for _, p := range runtimeProbes {
		path, err := exec.LookPath(p.command)
		if err != nil || path == "" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		cmd := exec.CommandContext(ctx, path, p.args...)
		raw, err := cmd.CombinedOutput()
		cancel()
		text := string(raw)
		if len(text) > 4096 {
			text = text[:4096]
		}
		version := ""
		if m := scanVersionRe.FindString(text); m != "" {
			version = m
		}
		note := ""
		if err != nil && version == "" {
			note = "版本探测失败：" + err.Error()
		}
		if version == "" {
			version = "unknown"
		}
		key := p.language + "@" + version
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, RuntimeCandidate{
			Language: p.language, Version: strings.TrimSpace(version), Label: p.label,
			Kind: p.kind, ExecPath: path, Note: note,
		})
	}
	return out
}
