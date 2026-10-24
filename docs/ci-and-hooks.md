# Git hooks 与自动构建

> 本文说明两道门禁的**设计取舍**与**排错方法**。速查用法在 `README.md`「源码开发」；
> hooks 的逐条规则在 `.githooks/README.md`。

## 0. 一句话

- **本地**（`.githooks/`）：提交时拦"事后无法挽回"的错，推送时跑正确性验证。
- **CI**（`.github/workflows/ci.yml`）：权威门——本地可以被绕过，这里不能；并且做一件本地很难做对的事：
  **拼出单 exe 真启动**。
- **Release**（`.github/workflows/release.yml`）：打 tag 出多平台包，且强制 `tag = version.go = package.json`。

## 1. 为什么要有 hooks（而不是只靠 CI）

只靠 CI 的问题不是"漏检"，而是**反馈太晚**：密钥提交进历史后，CI 红了也删不掉——
只能改写历史 + 强制推送 + 全员重克隆。这类错误的特征是「一次就永久」，
所以必须在 `commit` 这一层拦。

反过来，把编译与单测放进 `commit` 也是错的：3 分钟的门，人一定 `--no-verify`，
门就形同虚设。所以分工是明确的：

| 检查 | 放在哪 | 理由 |
|---|---|---|
| 禁提交路径、大文件、冲突标记、`gofmt`、硬编码前缀、明文密钥、`debugger` | `pre-commit` | 秒级可判、后果不可逆 |
| 提交信息格式 | `commit-msg` | 版本记录与 Release Notes 靠它归类 |
| `go build` / `go vet` / `go test` | `pre-push` | 分钟级，但推送频率低 |
| 前端构建、embed 拼装、单 exe 真启动、产物密钥扫描 | CI | 需要干净环境；本地构建会覆盖正在用的 `dist/` |

## 2. pre-commit 的六条规则与理由

### 2.1 禁提交路径

`.env`、`data/`、`dist/`、`server/dist/`、`release/`、`*.exe`、`*.pem|p12|pfx|keystore`。

`.gitignore` 已经挡了一层，hook 挡的是 `git add -f` 与「换机器后忽略规则失效」。
`data/` 尤其重要：它是**本地站点真实数据**（`nvs.db`、`.jwt_secret`、`uploads/`、作品正文），
一旦入库既泄露又污染仓库。

### 2.2 大文件 >2MB

二进制进 git 历史后无法真正删除（只能改写历史）。>2MB 的文件应放 Release 或对象存储。

### 2.3 冲突标记

只拦 `<<<<<<<` / `>>>>>>>` / `=======` 整行。

**刻意不用 `git diff --cached --check`**：它把「行尾空白」也算错误，历史上遗留文件一多就整片红，
门禁第一天就开始被绕过，比没有还糟。

### 2.4 gofmt

对暂存的 `.go` 文件跑 `gofmt -l`，命中则拒绝，并打印修复命令。
hook **不自动改文件**——静默改代码会让人以为是 IDE 干的，掩盖真实问题。

### 2.5 前端硬编码 `/go-nvs`

这是本项目最容易复发的一类缺陷：NVS 部署在反向代理前缀下（如 `/go-nvs/`），
前缀必须由 `@/utils/prefix` 的 `appPrefix` 运行时解析，**硬编码会在换前缀部署时全线 404**。

匹配规则只针对「引号内以 `/go-nvs` 开头」的新增行：

```
'...'  "/go-nvs/api"  `...`
```

并排除注释行（代码里大量「严禁硬编码 /go-nvs」的说明文字本身就是字面量）。
`src/utils/prefix.ts` 是解析实现，允许出现真值。

### 2.6 疑似明文密钥

`sk-` / `sk-ant-` / `AKIA` / `ghp_` / `xox[baprs]-` / `-----BEGIN ... PRIVATE KEY-----`。
跳过 `_test.go` 与 `testdata/`（用例里的 `sk-aaaa...` 是假密钥，否则天天误报）。
文档（`.md`）也跳过——示例常需要写 `sk-xxx` 这种占位。

## 3. pre-push 为什么默认不构建前端

本地 `npm run build` 会覆盖 `dist/`，而这台机器上可能正跑着站点（内嵌产物来自 `server/dist`），
推送前顺手换掉产物属于意外副作用。前端门禁交给 CI 的 `frontend` + `embed-smoke` 两个 job；
需要本地验证用 `CHECK_WEB=1 git push`。

## 4. CI 的四个 job

```
backend      go build / go vet / go test / go build -tags gitvcs
frontend     npm ci / vite build → 上传 web-dist artifact
embed-smoke  下载 web-dist → 拼 server/dist → 密钥扫描 → go build → 真启动断言
snapshot     推主干时产出 linux-amd64 单文件（站长可直接下）
```

### 4.1 embed-smoke 为什么必须存在

后端用 `//go:embed all:dist` 把前端打进二进制，而 `dist/` 是 gitignore 的。
这意味着 **`go test ./...` 全绿也可能产出一个打不开的 exe**：

- `server/dist/index.html` 缺失 → 编译直接失败（还算好）
- 目录陈旧 / 只同步了一半 → 编译成功，但页面 404 或白屏（最难查）
- 前端产物里混进密钥 → 单测完全发现不了

所以 CI 里必须「真的拼一次、真的起一次」，断言：

| 断言 | 含义 |
|---|---|
| `GET /` = 200 | 内嵌资源可用 |
| `GET /.env` = 404 | 默认蜜罐等级 `off`，诱饵必须与普通 404 无差别（否则等于自报家门） |
| `GET /api/v2/user/me` = 404 | 同上，影子认证在 off 档不暴露 |
| 日志无 `panic` / `conflicts with existing wildcard` | gin 路由冲突会在启动时 panic，这是历史高发问题 |
| `scan-frontend-secrets.sh` 通过 | 前端产物不许带密钥 |

### 4.2 为什么用 `npm ci` 而不是 `npm install`

`npm install` 可能改写 `package-lock.json`，在 CI 里表现为「本地能过、CI 依赖版本不同」。
`npm ci` 严格按 lock 安装，装完还会校验 lock 与 `package.json` 是否一致。

### 4.3 版本一致性：一个被自动构建揪出来的真 bug

`release.yml` 里做了 `tag = version.go = package.json` 的强制校验。写这个校验时发现：

```
$ grep -oE '[0-9]+\.[0-9]+\.[0-9]+' server/version/version.go | head -1
2.3.1
```

`version.go` 的**注释**里写着 `-X nvs-server/version.AppVersion=v2.3.1`，而 `grep | head -1`
取的是第一处匹配——**不是真实的 `var AppVersion = "3.20.1"`**。

这直接导致：`Makefile` 的 `VERSION` 变量一直在取错值；任何"按注释取版本"的脚本都会
发出一个版本号错误的包（且不会报错）。已修：

- `version.go` 注释里不再出现具体版本号，并加注「本行必须是文件里唯一的 `var AppVersion`」；
- 所有取版本的地方统一为 `grep -E '^var AppVersion' ... | grep -oE ...`（Makefile / CI / Release）。

> 教训：**用正则从源码里取常量，必须锚定声明行**，否则注释、示例、文档字符串都会成为假阳性来源。

## 5. Release 的打包规则

| 平台 | 包 |
|---|---|
| Windows x64 / ARM64 | `.zip`（含 `nvs-server.exe` + README + LICENSE + SECURITY） |
| Linux x64 / ARM64 / ARMv7 | `.tar.gz`（tar 保留可执行位，zip 不保留） |
| macOS Intel / Apple Silicon | `.tar.gz` |

- 每个包单独 `.sha256`，汇总一份 `SHA256SUMS`
- `fail_on_unmatched_files: true`：少一个产物就让发布失败，避免发出缺平台的 Release
- Release 正文自动附「下载表 + 校验方式 + 部署命令」，`generate_release_notes: true` 追加提交级变更
- 二进制**未做代码签名**（Windows SmartScreen 可能提示），正文给出源码构建命令作为替代

## 6. 排错

### hook 不执行

```bash
git config --get core.hooksPath     # 应为 .githooks
git ls-files -s .githooks           # mode 必须是 100755（Windows 上 chmod 不进索引）
```

若显示 `100644`，修复：

```bash
git update-index --chmod=+x .githooks/pre-commit .githooks/commit-msg .githooks/pre-push
```

### `bad interpreter: /usr/bin/env bash^M`

行尾被 CRLF 污染。仓库根 `.gitattributes` 已声明 `.githooks/* text eol=lf`，
若仍出现：`git config core.autocrlf false` 后重新 checkout。

注意：本项目 `core.autocrlf=true`，**不加 `.gitattributes` 时 hooks 必然被 checkout 成 CRLF**，
在 Linux / macOS 上直接不可执行。

### commit-msg 拒绝了合理的提交信息

检查首行是否 ≤120 字符（超了把细节放正文）；`Merge` / `Revert` / `fixup!` / `squash!` 自动放行。

### CI 的 go 版本报错

`server/go.mod` 是唯一事实来源（`go-version-file: server/go.mod`）。本地升级 Go 后记得提交 `go.mod`。

## 7. 新增一条检查怎么加

`.githooks/pre-commit` 的结构是「累积 `fail`，最后统一退出」，加检查只需三样：

```bash
# 1) 打印失败（红色 + 修复提示）
bad "你的错误描述"
hint "怎么修"

# 2) 不要 exit 1（那会跳过后面的检查，一次只能看到一个错）
```

CI 侧同理：新增断言直接加在 `embed-smoke` 的 `run:` 里，用 `test "$(...)" = "..."`；
新增平台只改 `release.yml` 的 `matrix.include`。

## 8. 明确不做

| 不做 | 原因 |
|---|---|
| `commit` 阶段跑编译/单测 | 会让人人 `--no-verify`，门禁失效 |
| `vue-tsc` 作为门禁 | 当前 `tsconfig.node.json` 未配 `composite`，`vue-tsc --noEmit` 直接报 TS6306/TS6310；配好之前引入等于第一天全红 |
| 自动 `gofmt -w` / 自动改代码 | hook 静默改文件会掩盖真实问题 |
| 自动打 tag / 自动 bump 版本 | 版本三处同步涉及 README 叙述，机器猜不准；由人决定后打 tag |
| 二进制代码签名 | 需要证书与私钥托管，超出仓库能力；Release 正文给出源码构建替代 |

---

**版本**：v1（2026-10-24）　**对应**：v3.20.1
