# Git hooks（仓库内版本化）

`core.hooksPath` 指向本目录，hooks 随仓库走——换机器 / 新克隆**只需执行一次安装命令**，
不用再问「本地有没有装 pre-commit」。

```bash
# Linux / macOS / Git Bash / WSL
bash .githooks/install.sh

# Windows PowerShell
powershell -ExecutionPolicy Bypass -File .githooks\install.ps1

# 或者
make hooks
```

关闭：`git config --unset core.hooksPath`

## 三道门的分工

| hook | 时机 | 检查什么 | 耗时 |
|---|---|---|---|
| `pre-commit` | 每次 commit | 禁提交路径（`.env` / `data/` / `dist/` / `*.exe` / 证书）、>2MB 大文件、冲突标记、`gofmt`、**前端硬编码 `/go-nvs`**、**疑似明文密钥**、**脚本类文件必须 LF**、前端 `debugger` | < 2s |
| `commit-msg` | 每次 commit | 约定式提交 `type(scope): 说明`，type 白名单，首行 ≤120 字符 | < 0.2s |
| `pre-push` | 每次 push | `go build ./...` + `go vet ./...` + `go test ./... -count=1` | 1~3 min |

### 为什么 pre-commit 不跑编译与单测

提交（commit）是高频动作，把 3 分钟的门放在这里，结果一定是人人 `--no-verify`，
等于没有门。所以：**提交时只拦"事后无法挽回"的错**（密钥进历史、二进制进历史、
前缀硬编码、语法级脏数据），**正确性验证放到 push 与 CI**。

### 为什么 pre-push 不默认构建前端

本地 `npm run build` 会覆盖 `dist/`，而这台机器上可能正跑着站点（内嵌产物来自 `server/dist`），
推送前顺手换掉产物属于意外副作用。前端门禁由 CI 的 `frontend` + `embed-smoke` 两个 job 承担；
需要本地验证时用 `CHECK_WEB=1 git push`。

## 绕过方式（都留了口子，但请别养成习惯）

```bash
SKIP_HOOKS=1 git commit -m "..."   # 全部跳过
SKIP_COMMITMSG=1 git commit ...    # 只跳提交信息格式
SKIP_TESTS=1 git push              # push 只 build + vet
CHECK_WEB=1 git push               # push 额外构建前端
```

## 项目铁律落在哪条规则上

- **前端严禁硬编码 `/go-nvs`**：NVS 部署在反向代理前缀下，前缀由 `@/utils/prefix` 的
  `appPrefix` 运行时解析。`pre-commit` 会拦「引号内以 `/go-nvs` 开头」的新增行
  （注释里的说明文字放过；`src/utils/prefix.ts` 本身是解析实现，允许出现字面量）。
- **密钥不进仓库**：历史一旦写入，`git rm` 也删不掉（只能改写历史 + 强制推送 + 全员重克隆）。
  扫描覆盖 `sk-` / `sk-ant-` / `AKIA` / `ghp_` / `xox[baprs]-` / `BEGIN ... PRIVATE KEY`，
  跳过 `_test.go` 与 `testdata/`（用例里的假密钥）。
- **产物不进仓库**：`dist/`、`server/dist/`、`release/`、`*.exe` 一律拒绝；
  发布产物走 GitHub Release（见 `.github/workflows/release.yml`）。

## 约定式提交

```
feat(oc): 角色卡支持酒馆 V2 导入
fix(honeypot): off 档不再返回 500
refactor(vcs)!: 移除独立蜜罐进程
docs: 补充部署前缀说明
```

type 白名单：`feat` `fix` `refactor` `perf` `docs` `test` `build` `ci` `chore` `security` `style`。
`Merge` / `Revert` / `fixup!` / `squash!` 与 git 自动生成信息自动放行。

## 排错

hook 不生效时按顺序排查：

```bash
git config --get core.hooksPath     # 应为 .githooks
ls -l .githooks/pre-commit          # 需可执行（Windows 下看 git ls-files -s，应为 100755）
git ls-files -s .githooks           # mode 必须是 100755，否则 Linux/macOS 上直接不执行
```

`pre-push` 报单测失败但满屏 `ok`：那是输出被截断，失败行在中间。hook 会把完整日志
落盘并高亮失败包（路径会打印出来），先看日志再改代码；Windows 本地偶发抖动时先 `for i in 1 2 3; do go test ./... -count=1; done` 确认。

Windows 上如果 hook 报 `bad interpreter`，是行尾被改成 CRLF 了：

```bash
git config core.autocrlf false   # 或
sed -i 's/\r$//' .githooks/pre-commit
```

仓库根有 `.gitattributes` 统一 `*.sh` / hooks 为 LF（见该文件），正常情况下不会踩到。
