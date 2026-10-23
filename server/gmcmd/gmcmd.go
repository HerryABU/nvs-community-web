// Package gmcmd 国密相关 CLI 子命令（需求文档 §8.1 / §6.3）。
//
// 用法（不启动 HTTP 服务）：
//
//	nvs-server gm-audit [--out report.md] [--lang zh|en]
//	nvs-server credential-age <文件路径>...
package gmcmd

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"nvs-server/audit"
	"nvs-server/config"
)

const usage = `国密 CLI 子命令：
  gm-audit [--out 文件] [--lang zh|en]   国密自检报告（Markdown，不构成密评结论）
  credential-age <路径>...               凭据文件年龄报告（轮换辅助）

说明：默认构建（国际轨）仅能报告配置与握手统计；SM2 证书详情需
以 make build-gm / make build-gm-cgo 构建的二进制执行。`

// Run 执行子命令，返回进程退出码。
// 调用方需已完成 config.Init()（main 负责）。
func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}
	switch args[0] {
	case "gm-audit":
		return runGMAudit(args[1:])
	case "credential-age":
		return runCredentialAge(args[1:])
	case "help", "-h", "--help":
		fmt.Println(usage)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n\n%s\n", args[0], usage)
		return 2
	}
}

func runGMAudit(args []string) int {
	fs := flag.NewFlagSet("gm-audit", flag.ContinueOnError)
	var out, lang string
	fs.StringVar(&out, "out", "", "输出文件路径（默认标准输出）")
	fs.StringVar(&lang, "lang", "zh", "报告语言：zh（默认）/ en")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if lang != "zh" && lang != "en" {
		fmt.Fprintf(os.Stderr, "--lang 仅支持 zh/en，收到 %q\n", lang)
		return 2
	}

	cfg := config.CryptoCfg
	if cfg == nil {
		cfg = &config.CryptoConfig{Track: config.TrackIntl}
	}
	rep := audit.BuildGMAuditReport(cfg)
	md := rep.RenderMarkdown(lang)

	if out == "" {
		fmt.Print(md)
		return 0
	}
	if err := os.WriteFile(out, []byte(md), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "写入报告失败: %v\n", err)
		return 1
	}
	fmt.Printf("报告已写入: %s\n", out)
	return 0
}

func runCredentialAge(args []string) int {
	fs := flag.NewFlagSet("credential-age", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	paths := fs.Args()
	if len(paths) == 0 {
		// 默认检查国密证书私钥（最常见凭据）
		if cfg := config.CryptoCfg; cfg != nil {
			paths = []string{cfg.SMSignKeyFile, cfg.SMEncKeyFile, cfg.IntlKeyFile}
		}
	}
	ages, err := audit.CredentialAgeReport(paths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "生成凭据年龄报告失败: %v\n", err)
		return 1
	}
	fmt.Printf("%-50s %-10s %-12s %s\n", "PATH", "STATUS", "AGE(days)", "MODIFIED")
	for _, a := range ages {
		status, age, mod := "ok", fmt.Sprintf("%d", a.AgeDays), a.ModTime.Format("2006-01-02T15:04:05")
		if a.Missing {
			status, age, mod = "missing", "—", "—"
		}
		fmt.Printf("%-50s %-10s %-12s %s\n", truncMid(a.Path, 50), status, age, mod)
	}
	return 0
}

func truncMid(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max < 5 {
		return s
	}
	head := (max - 3) / 2
	return s[:head] + "..." + s[len(s)-(max-3-head):]
}

var _ = strings.TrimSpace
