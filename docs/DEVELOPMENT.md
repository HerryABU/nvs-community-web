# 源码开发

> 本地跑起来、构建矩阵、测试轨道、质量门禁、发版流程。
> 返回 [README](../README.md) · [更新日志](CHANGELOG.md) · [API](API.md) · [配置](CONFIG.md) · [架构](ARCHITECTURE.md) · [特性详解](FEATURES.md)

## 版本号维护

> 版本号**四处**必须一致：`server/version/version.go`、`web/package.json`、
> `web/package-lock.json`（root 与 `packages[""]`）、本页顶部徽章。
> 校验/一键同步只用一条命令（纯 bash + node，**不需要 make**）：`bash scripts/check-version.sh`
> ／改版本 `bash scripts/check-version.sh --set x.y.z`（有 make 时 `make version` 等价）。
> 发版时同步更新本徽章与下方「版本更新记录」。


### 环境要求

| 工具 | 版本 | 用途 |
|------|------|------|
| Go | ≥ 1.25（见 `server/go.mod`） | 后端编译 |
| Node.js | ≥ 20（CI 用 22） | 前端构建（Vite 5） |
| npm | ≥ 9 | 依赖管理 |
| Git | ≥ 2.9 | 须支持 `core.hooksPath`（仓库内 hooks 依赖它） |
| make | 可选 | `make check/dist/build` 等便捷目标；**Windows 的 Git Bash 通常不带 make**，等价命令见各小节 |

> 真浏览器验收脚本需要 `playwright-core` 与已下载的 Chromium（不在 `web/package.json` 里，
> 由运行环境提供；脚本默认从 `%LOCALAPPDATA%\ms-playwright` 找浏览器，可用 `NVS_CHROME` 指定）。

### 本地开发

```bash
# 终端 1：启动 Go 后端（监听 :8080）
cd server
go run .

# 终端 2：启动 Vue 前端（监听 :5173，API 自动代理到 8080）
cd web
npm run dev
```

浏览器打开 **http://localhost:5173** 进行前端开发（热重载）。Vite 自动将 `/api` 请求代理到 `localhost:8080`。

### Git hooks（提交门禁，新克隆必做一次）

hooks 随仓库走（`.githooks/`），不需要问"你本地装没装"：

```bash
bash .githooks/install.sh        # Linux / macOS / Git Bash / WSL
# 或 Windows PowerShell
powershell -ExecutionPolicy Bypass -File .githooks\install.ps1
# 或
make hooks
```

| hook | 检查内容 | 耗时 |
|---|---|---|
| `pre-commit` | 禁提交 `.env` / `data/` / `dist/` / `*.exe` / 证书、>2MB 大文件、冲突标记、`gofmt`、**前端硬编码 `/go-nvs`**、**疑似明文密钥**、`debugger` | < 2s |
| `commit-msg` | 约定式提交 `type(scope): 说明`，首行 ≤120 字符 | < 0.2s |
| `pre-push` | `go build` + `go vet` + `go test ./...`（前端由 CI 兜底，`CHECK_WEB=1` 可强制本地构建） | 1~3 min |

为什么 commit 阶段不跑编译：提交是高频动作，3 分钟的门会让人人 `--no-verify`，等于没有门。
提交只拦"事后无法挽回"的错（密钥进历史、二进制进历史、前缀硬编码），正确性验证放在 push 与 CI。

需要绕过时（请别养成习惯）：

```bash
SKIP_HOOKS=1 git commit -m "..."   # 全部跳过
SKIP_TESTS=1 git push              # push 只 build + vet
CHECK_WEB=1 git push               # push 额外构建前端
```

详见 `.githooks/README.md`。

### 已提供的构建目标

```bash
make check        # 本地等价 CI：SPA 白名单 + 组件审计 + 后端 build/vet/test（nodist 轨）+ 前端 build
make check-all    # check + 同步 embed + 产物自检 + 前端产物密钥扫描
make dist         # 只构建前端并同步到 server/dist（embed 源）
make dist-check   # 校验 server/dist 有 index.html 且资源数正常（防"空 embed 打进 exe"）
make version      # 校验版本号四处一致（等价 bash scripts/check-version.sh，本机无 make 时跑脚本）
make hooks        # 启用 .githooks
make test         # 后端单测（nodist 轨，**不需要**先构建前端）
make test-embed   # 带真实 embed 产物的单测（需先 make dist）
make build        # 单平台（当前系统）产出 nvs-server.exe
make build-gitvcs # 章节版本控制走真实 git 仓库（-tags gitvcs，运行时 NVS_VCS_BACKEND=git）
make build-gm     # 兼容别名：纯 Go 国密已默认编入主构建（无需此步）；Tongsuo CGO 走 `go build -tags tongsuo`
```

> **构建顺序不能反**：`npm run build`（产出 `dist/`）→ 同步到 `server/dist/` → `go build`。
> `//go:embed all:dist` 是**编译期**打包，顺序反了会把旧前端（或空目录）嵌进 exe。

> 🏷 **两条编译轨（`nodist` 标签）**：`server/dist` 是构建产物、不入库，而 `//go:embed all:dist`
> 要求目录非空 —— 所以**全新 clone 里直接 `go build ./...` 会报 `pattern all:dist: no matching files found`**。
> - **发布轨**（默认，不带标签）：必须先 `make dist`，产出物才内含前端（`make build` / Release / 内嵌冒烟用它）
> - **业务轨**（`-tags nodist`）：嵌入一个提示页，只用于 `go build` / `go vet` / `go test`（CI 后端 job、
>   `make check` / `make test`、pre-push 钩子都用它）；该轨产物启动时会打印醒目警告，**不可作为发布物**
>
> 两条轨并存还配了一道防回归：CI 会额外用「最小 dist 占位 + 不带标签」编译一次，确保 nodist 开关没把发布轨弄坏。

### 质量门禁与验收脚本

`scripts/` 下是**可重复执行**的静态检查与真浏览器验收脚本（不是"我本地跑过一次"的口头结论）：

| 脚本 | 查什么 | 何时跑 |
|---|---|---|
| `scripts/check-version.sh` | 版本号**四处一致**（`version.go` / `package.json` / `package-lock.json` 两个 root 字段 / README 徽章）；`--tag` 连 tag 一起校验，`--set x.y.z` 一键同步（**不需要 make**，纯 bash + node） | `make check` / 改版本 / 打 tag 前 |
| `scripts/check-spa-roots.sh` | 前端**路由首段**是否全部登记到 `server/routes/routes.go` 的 `knownSPARoots`（漏登记 = 点链接能进、刷新 404） | `make check` / CI |
| `scripts/audit-unresolved-components.mjs` | 模板里用了、`<script setup>` 里没导入的组件（Element Plus 按需导入只解析 `El*`，`<ArrowDown />` 少了 import 会**静默不渲染**） | `make check` / CI |
| `scripts/audit-admin-pages.mjs` | 后台页四态体检（表格 / StateBlock / `#empty` / 骨架 / 分页 / 筛选 / 错误态），`--strict` 有表格无四态即退出码 1 | 改后台页后 |
| `scripts/fix-table-empty-slots.mjs` | 清理**列级 `#empty` 死插槽**（`el-table-column` 没有 empty 插槽，写了永远不渲染）；`--write` 才实改 | 需要时手动 |
| `scripts/scan-frontend-secrets.sh` | 前端产物里是否混入密钥/令牌 | `make check-all` / CI |
| `scripts/browser-smoke.mjs` | 真浏览器 **四档宽度（375/768/1024/1440）**回归：无横向溢出、关键元素存在且可见、`/api/*` 无失败、console 无错误；截图 + `report.json` 落盘 | 改前台页面后 |
| `scripts/browser-func.mjs` | **功能级**回归：保存资料并回读落库、作者主页四分栏可切、评论排序真带 `sort=` 参数、密度切换真改 DOM 高度、朗读真请求 `/api/ai/speech` 且失败有可读提示 | 改交互逻辑后 |
| `scripts/browser-me-center.mjs` | 个人中心专项：下拉只剩 5 项、13 个入口已迁出、`/me` 服务链接逐个直达不 404 不弹登录、头像上传落点 | 改导航/个人中心后 |

三个浏览器脚本都用 `playwright-core` + 已登录 cookie：

```bash
# 1) 登录拿 cookie（测试实例若开了滑块，需临时把 captcha_enabled 置 false，跑完改回）
curl -s --noproxy '*' -c /tmp/nvs_cookie.txt \
  -H 'Content-Type: application/json' -H 'X-Requested-With: XMLHttpRequest' \
  -d '{"email":"admin@nvs.local","password":"你的密码"}' \
  http://127.0.0.1:8000/api/auth/login

# 2) 跑验收（Windows 下 cookie 路径要传 Windows 绝对路径，Node 不认 MSYS 的 /tmp）
NVS_COOKIE="C:/Users/<你>/AppData/Local/Temp/nvs_cookie.txt" \
NVS_BASE=http://127.0.0.1:8000 node scripts/browser-smoke.mjs
```

> 为什么必须有真浏览器：`vite build` 成功 **≠** 页面能跑。本仓库踩过的坑里，
> 模板引用未定义变量导致整页白屏、图标漏 import 静默消失、`overflow` 加错容器导致头像被挤出视野，
> 全都是构建期无感的。另：非 `/api` 的 404 只会产生一条不带 URL 的 `Failed to load resource`，
> 静态检查与肉眼都定位不到。

### 自动构建（GitHub Actions）

| workflow | 触发 | 做什么 |
|---|---|---|
| `ci.yml` | push 到 `v4` / `v3` / `main` / `master`、PR、手动 | ① 后端 `build` + `vet` + `test` + `gitvcs` 标签轨（均走 `nodist` 轨，不依赖前端产物）+ 发布轨占位编译；② 前端 `npm ci` + `vite build`；③ **内嵌产物冒烟**（拼出单 exe，真启动，断言首页 200、off 档诱饵 404、日志无 panic）；④ 推主干产出 `linux-amd64` 快照 artifact |
| `release.yml` | push tag `v*` | 校验 **tag = version.go = package.json = package-lock = README 徽章**（`scripts/check-version.sh --tag`）；多平台交叉编译（Win x64/ARM64、Linux x64/ARM64/ARMv7、macOS Intel/ARM64）；打包 zip/tar.gz + `SHA256SUMS`；创建 GitHub Release |

发布流程（版本号四处同步后打 tag）：

```bash
# 1) 改版本 —— 一条命令搞定四处（含 package-lock 的两个 root 字段，不碰依赖自己的 version）
bash scripts/check-version.sh --set x.y.z
#    等价的手工改动位置：
#    server/version/version.go  var AppVersion = "x.y.z"
#    web/package.json           "version": "x.y.z"
#    web/package-lock.json      顶层 version 与 packages[""].version
#    README.md                  顶部 version 徽章 + 版本更新记录新增段落

# 2) 本地先过一遍（make version 就是脚本的壳，没有 make 直接跑脚本）
bash scripts/check-version.sh && make check-all

# 3) 提交 + 打 tag（tag 必须与版本一致，否则 release 直接失败）
git add -A && git commit -m "chore(release): x.y.z <摘要>"
git tag vx.y.z && git push origin v3 --tags
#   tag 命名两种都认：v3.20.1（推荐）或 3.20.1（兼容本仓库历史 tag）
```

Release 里的二进制**已内嵌前端**，解压即用，无需 Node / Go 环境；Windows 包为 zip，其余为 tar.gz，附 `SHA256SUMS` 与各自 `.sha256`。

### 全平台交叉编译

项目提供 `build-all.ps1` 脚本，自动为 **13 个目标平台** 生成预编译二进制文件（本地发布用；CI 的 Release 覆盖 7 个主流平台）：

```bash
# Windows（需 PowerShell）
./build-all.ps1
```

输出到 `release/` 目录：

```
release/
├── nvs-win-x64.exe          # Windows 64-bit (x86_64)
├── nvs-win-arm64.exe        # Windows ARM64
├── nvs-linux-x32            # Linux 32-bit
├── nvs-linux-x64            # Linux 64-bit
├── nvs-linux-arm32          # Linux ARM32 (v7)
├── nvs-linux-armv6          # Linux ARM32 (v6 — 树莓派1/Zero)
├── nvs-linux-arm64          # Linux ARM64 (树莓派4/5)
├── nvs-linux-riscv64        # Linux RISC-V 64
├── nvs-mac-x64              # macOS Intel
└── nvs-mac-arm64            # macOS Apple Silicon
```

手动单目标编译：

```bash
cd server
GOOS=linux   GOARCH=arm64  go build -ldflags="-s -w" -o nvs-linux-arm64 .
GOOS=windows GOARCH=amd64  go build -ldflags="-s -w" -o nvs-win-x64.exe .
GOOS=darwin  GOARCH=arm64  go build -ldflags="-s -w" -o nvs-mac-arm64 .
GOOS=linux   GOARCH=riscv64 go build -ldflags="-s -w" -o nvs-linux-riscv64 .
```

> **关于 `embed.go`**：`//go:embed all:dist` 将 `server/dist/` 全部嵌入二进制。**前端必须先 `npm run build`，否则编译失败。**

### 发布产物体积

发布包体积主要由三块决定，前两块纯属浪费：

1. **预压缩副本（.gz / .br）** —— 已默认关闭。
   `vite-plugin-compression2` 会给每个 js/css 再生成一份压缩版，但本项目是单 exe 部署，
   压缩由 `gin-contrib/gzip` 在响应时实时完成，这些副本**永远不会被读到**；
   而 `go:embed` 会照单全收。实测 642 个文件、8.5 MB。
   只有在「nginx / CDN 直服 dist 静态目录」时才需要，那种场景显式开启：
   `NVS_PRECOMPRESS=1 npm run build`（CI 里另有 `find -delete` 兜底）。
2. **符号表与调试信息** —— 发布构建统一加 `-ldflags "-s -w"`（去符号表 + DWARF），
   配合 `-trimpath` 去掉本机绝对路径；`-buildvcs=false` 不把 VCS 信息写进二进制。
3. **sourcemap** —— `build.sourcemap` 显式 `false`。产物要 embed 发布，
   `.map` 既让体积近乎翻倍，也等于把完整源码随安装包发给任何人。

实测（Windows amd64，含内嵌前端）：

| 构建配置 | 可执行文件 |
|---|---|
| 嵌入 .gz/.br 副本 | 73.6 MB |
| 排除 .gz/.br（当前默认） | **65.1 MB** |

顺带：`reportCompressedSize: false` 让 vite 不再为每个 chunk 压一遍只为打印一列，
构建时间从 1m41s 降到 1m14s。

**没有启用 UPX**：它能再压掉一半左右，但会拖慢冷启动，且极易触发 Windows SmartScreen
与杀软误报 —— 对「下载即用」的单文件分发，体验损失大于收益。需要极限体积时自行 UPX 后签名。
