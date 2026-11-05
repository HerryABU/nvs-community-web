package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"nvs-server/config"
	"nvs-server/models"
	aiops "nvs-server/models/aiops"
	siteops "nvs-server/models/siteops"
	"nvs-server/utils"
	"nvs-server/version"
)

// cli.go —— 单文件命令行工具（--version / --pwd / --help）
//
// 这些命令只做一次性维护动作，处理完立即退出，**不启动 HTTP 服务**。
//
// 为什么要有它：站长把密码忘了、又进不去后台时，唯一的自救通道就是命令行。
// 所以它必须能在「服务没跑起来」的情况下工作 —— 自己连库、自己改密码。
//
// 安全取向：
//   - 密码可以从命令行参数给，也可以从标准输入读（后者不进进程列表、不留 shell 历史）；
//   - 每次重置都写 emergency_audit_logs 留痕（operator 记 cli，IP 记 local）；
//   - 站点未完成建站向导初始化时直接拒绝（那时根本没有管理员可改）。

const (
	cliScopeAll       = "all"       // 管理员密码 + 二级密码
	cliScopeAdmin     = "admin"     // 仅管理员登录密码
	cliScopeEmergency = "emergency" // 仅内容抢救台二级密码（= 双重验证密码）
	cliScopeToken     = "token"     // 仅令牌加密密码（上游 API Key 落库加密用，与 JWT 无关）
)

// emergencyPasswordKey 二级密码在 platform_configs 里的键（与后台设置接口同键）
const emergencyPasswordKey = "emergency_password_hash"

// runReadOnlyCLI 处理不依赖配置与数据库的命令（--version / --help）。
//
// ⚠️ 这里额外做一次「近似开关」识别：站长手滑写成 -pwd / --pwd=xxx / --pasword 时，
// 旧实现会**静默忽略**（hasPwd 仍为 false）→ 直接启动 HTTP 服务 → 终端卡在服务日志里，
// 看起来就像「命令没反应」。现在遇到这类写法直接给出明确报错 + 用法，不再沉默启动。
func runReadOnlyCLI(args []string) (handled bool, exitCode int) {
	// 1) 近似开关检查（不认识的密码类开关直接报错，不静默启动服务）
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			continue // 非开关：交给子命令分支（gm-audit 等）
		}
		name, _, _ := normalizeFlag(a)
		switch name {
		case "--version", "--help", "--pwd", "--scope", "--user":
			continue // 已知开关，交给后面的解析
		}
		// 只对「明显是想做密码维护」的写法报错，避免误伤将来新增的其它开关
		low := strings.ToLower(name)
		nearMiss := false
		for _, kw := range []string{"pwd", "pw", "pass", "pasw", "scope", "user", "version", "help"} {
			if strings.Contains(low, kw) {
				nearMiss = true
				break
			}
		}
		if nearMiss {
			fmt.Fprintf(os.Stderr, "✗ 无法识别的命令行参数：%s\n", a)
			fmt.Fprintln(os.Stderr, "  提示：密码重置的写法是 --pwd <新密码>（两个短横线），")
			fmt.Fprintln(os.Stderr, "        也支持 --pwd=<新密码> 与 -pwd <新密码>。")
			printCLIUsage()
			return true, 2
		}
	}

	for _, a := range args {
		switch a {
		case "--version", "-v", "version":
			printVersionInfo()
			return true, 0
		case "--help", "-h", "help":
			printCLIUsage()
			return true, 0
		}
	}
	return false, 0
}

// normalizeFlag 把各种开关写法归一：
//
//	--pwd      → name="--pwd"
//	--pwd=abc  → name="--pwd", value="abc", hasValue=true
//	-pwd=abc   → name="--pwd", value="abc", hasValue=true
//	-pwd       → name="--pwd"
func normalizeFlag(arg string) (name string, value string, hasValue bool) {
	s := strings.TrimLeft(arg, "-")
	if s == "" {
		return "", "", false
	}
	if i := strings.IndexByte(s, '='); i >= 0 {
		return "--" + s[:i], s[i+1:], true
	}
	return "--" + s, "", false
}

// isKnownFlagToken 判断 token 是「另一个开关」而不是密码值。
// （--pwd -scope admin 这种写法不能把 -scope 当成密码吞掉）
func isKnownFlagToken(token string) bool {
	if strings.HasPrefix(token, "--") {
		return true
	}
	name, _, _ := normalizeFlag(token)
	switch name {
	case "--pwd", "--scope", "--user", "--version", "--help":
		return true
	}
	return false
}

// runMaintenanceCLI 处理需要数据库的维护命令（--pwd）。
// 在数据目录创建之后、HTTP 服务启动之前调用。
func runMaintenanceCLI(args []string) (handled bool, exitCode int) {
	scope := cliScopeAll
	userRef := ""
	pwd := ""
	hasPwd := false

	for i := 0; i < len(args); i++ {
		// 归一化：--pwd / -pwd / --pwd=xxx / -pwd=xxx 一视同仁
		name, val, hasVal := normalizeFlag(args[i])
		switch name {
		case "--pwd":
			hasPwd = true
			if hasVal { // --pwd=xxx
				pwd = val
				continue
			}
			// 下一个 token 若不是开关，就当它是密码值
			if i+1 < len(args) && !isKnownFlagToken(args[i+1]) {
				pwd = args[i+1]
				i++
			}
		case "--scope":
			if hasVal {
				scope = strings.ToLower(strings.TrimSpace(val))
				continue
			}
			if i+1 < len(args) {
				scope = strings.ToLower(strings.TrimSpace(args[i+1]))
				i++
			}
		case "--user":
			if hasVal {
				userRef = strings.TrimSpace(val)
				continue
			}
			if i+1 < len(args) {
				userRef = strings.TrimSpace(args[i+1])
				i++
			}
		}
	}
	if !hasPwd {
		return false, 0
	}

	switch scope {
	case cliScopeAll, cliScopeAdmin, cliScopeEmergency, cliScopeToken:
	default:
		fmt.Fprintln(os.Stderr, "✗ --scope 只能是 all / admin / emergency(second) / token")
		return true, 2
	}

	// 密码来源：命令行参数，或从标准输入读一行
	if pwd == "" {
		fmt.Print("请输入新密码（建议用管道传入，避免留在 shell 历史与进程列表）：")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		pwd = strings.TrimSpace(line)
	}
	if len(pwd) < 8 {
		fmt.Fprintln(os.Stderr, "✗ 密码至少 8 位")
		return true, 2
	}

	if !config.IsInstalled() {
		fmt.Fprintln(os.Stderr, "✗ 站点尚未完成建站向导初始化（data/secure-config.json 不存在），没有可重置的管理员")
		return true, 3
	}
	if err := initDatabase(); err != nil {
		fmt.Fprintf(os.Stderr, "✗ 数据库连接失败: %v\n", err)
		return true, 4
	}

	hash, err := utils.HashPassword(pwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 密码哈希失败: %v\n", err)
		return true, 5
	}

	detail := fmt.Sprintf("命令行重置密码（scope=%s）", scope)

	if scope == cliScopeAll || scope == cliScopeAdmin {
		admins, err := resolveAdminTargets(userRef)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ 查询管理员失败: %v\n", err)
			return true, 6
		}
		if len(admins) == 0 {
			fmt.Fprintln(os.Stderr, "✗ 没找到匹配的管理员（用 --user <id|邮箱|用户名> 指定）")
			return true, 7
		}
		for _, a := range admins {
			// 顺手解锁：改完密码还锁着的话，站长会以为「密码没生效」
			if err := models.DB.Model(&models.User{}).Where("id = ?", a.ID).Updates(map[string]any{
				"password_hash":    hash,
				"login_fail_count": 0,
				"locked_until":     nil,
			}).Error; err != nil {
				fmt.Fprintf(os.Stderr, "✗ 重置管理员 #%d 失败: %v\n", a.ID, err)
				return true, 8
			}
			fmt.Printf("✓ 管理员密码已重置：#%d %s <%s>\n", a.ID, a.Username, a.Email)
		}
		detail += fmt.Sprintf("，管理员 %d 个", len(admins))
	}

	if scope == cliScopeAll || scope == cliScopeEmergency {
		if err := models.SetPlatformConfig(emergencyPasswordKey, hash); err != nil {
			fmt.Fprintf(os.Stderr, "✗ 重置二级密码失败: %v\n", err)
			return true, 9
		}
		fmt.Println("✓ 二级密码（内容抢救台）已重置")
	}

	if scope == cliScopeAll || scope == cliScopeToken {
		// 令牌加密密码：派生独立 AES 密钥（PBKDF2），不依赖 JWT，
		// 并把现有上游 API Key 用旧钥匙解开、用新钥匙重新加密。
		moved, err := utils.SetTokenVaultPassphrase(pwd, aiops.ReencryptAiModelKeys)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ 设置令牌加密密码失败: %v\n", err)
			return true, 10
		}
		fmt.Printf("✓ 令牌加密密码已设置（独立于 JWT），已迁移模型密钥 %d 条\n", moved)
		detail += "，令牌加密密码"
	}

	// 留痕：高危操作必须有审计，operator 记 cli 以便与真人操作区分
	siteops.RecordEmergencyAudit(&siteops.EmergencyAuditLog{
		OperatorID: 0,
		Operator:   "cli",
		IP:         "local",
		Action:     siteops.AuditSetPassword,
		TargetType: "password",
		Detail:     detail,
	})
	fmt.Println("✓ 审计已记录：emergency_audit_logs")
	return true, 0
}

// resolveAdminTargets 解析要改密码的管理员。
//
// userRef 为空时取全部管理员（站长可能不记得是哪个，多改几个比报错强，
// 站点里管理员本就不该有多个）；给了 id / 邮箱 / 用户名则只改那一个。
func resolveAdminTargets(userRef string) ([]models.User, error) {
	var admins []models.User
	q := models.DB.Where("role = ?", "admin")
	if ref := strings.TrimSpace(userRef); ref != "" {
		if id, err := strconv.ParseUint(ref, 10, 64); err == nil {
			q = q.Where("id = ?", uint(id))
		} else {
			q = q.Where("email = ? OR username = ?", ref, ref)
		}
	}
	if err := q.Order("id asc").Find(&admins).Error; err != nil {
		return nil, err
	}
	return admins, nil
}

// printVersionInfo 打印版本信息（--version）。
func printVersionInfo() {
	fmt.Printf("NVS Server v%s\n", version.AppVersion)
	fmt.Printf("构建：%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	if nodistBuild {
		fmt.Print("（不含前端产物）")
	}
	fmt.Println()
	if nodistBuild {
		fmt.Println("前端：未内嵌（该二进制为 nodist 编译轨，仅供开发/测试）")
	} else {
		fmt.Println("前端：已内嵌（单文件部署，解压即用，无需 Node / Go 环境）")
	}
}

// printCLIUsage 打印命令行用法（--help）。
func printCLIUsage() {
	fmt.Println(`NVS 命令行工具

用法：
  nvs-server.exe --version                        打印版本信息后退出
  nvs-server.exe --pwd <新密码>                   重置管理员登录密码 + 内容抢救台二级密码
  nvs-server.exe --pwd=<新密码>                   同上（等号写法）
  nvs-server.exe --pwd <新密码> --scope admin     只重置管理员登录密码
  nvs-server.exe --pwd <新密码> --scope emergency 只重置内容抢救台二级密码
  nvs-server.exe --pwd                             新密码从标准输入读取（不进进程列表）

选项：
  --scope <all|admin|emergency|token>   改哪些，默认 all
                                  · admin     管理员登录密码
                                  · emergency 双重验证密码（内容抢救台二级密码）
                                  · token     令牌加密密码（上游 API Key 落库加密，独立于 JWT）
  --user <id|邮箱|用户名>         指定管理员（默认改全部管理员账号）

说明：
  · 这些命令只做一次性维护动作，**不会启动 HTTP 服务**
  · 密码至少 8 位
  · 站点未完成建站向导初始化时无法重置（那时还没有管理员）
  · 每次重置都会写 emergency_audit_logs 留痕（operator=cli）
  · 管理员密码重置会顺带解锁账号（清零失败次数与锁定时间）
  · 开关写法：--pwd / --pwd=值 / -pwd / -pwd=值 都认；
    写成不认识的开关（如 --pasword）会直接报错提示，不会沉默地启动服务`)
}
