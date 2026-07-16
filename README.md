<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/Vue-3.4-4FC08D?style=flat&logo=vuedotjs" alt="Vue">
  <img src="https://img.shields.io/badge/SQLite-3-003B57?style=flat&logo=sqlite" alt="SQLite">
  <img src="https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-blue?style=flat" alt="Platform">
  <img src="https://img.shields.io/badge/CPU-x86%20%7C%20ARM%20%7C%20MIPS64%20%7C%20RISC--V-orange?style=flat" alt="CPU">
  <img src="https://img.shields.io/badge/license-AGPL%20v3-blue.svg" alt="License">
</p>

<h1 align="center">📚 NVS — 去中心化网络小说平台</h1>

<p align="center">
  <strong>全平台 · 全CPU架构 · 下载即用 · 默认端口 8080</strong><br>
  聚焦高质量类型文学 · 作者自主定价 · 版权归属作者 · 文件系统存储
</p>

---

## ⚡ 为什么选 NVS？

**一个二进制文件，下载即运行。** 不需要装 MySQL，不需要配 Nginx，不需要 npm install —— 前端、后端、数据库全部内嵌。

> 首次启动自动创建 `.env` 配置文件（含数据库、端口、IPv4/IPv6），按需修改后重启即可。

| 对比 | 传统小说站点 | NVS |
|------|------------|-----|
| 部署 | 服务器 + 数据库 + 缓存 + 前端 | 一个二进制文件，下载即用 |
| 平台 | 通常仅 x86 Linux | **Windows / Linux / macOS 全平台** |
| CPU | 仅 x86_64 | **x86 · ARM · ARM64 · MIPS64 · RISC-V** |
| 默认端口 | 各组件端口不一 | **统一 8080** |
| 前端 | 需额外部署和配置 | **内嵌在二进制中** |
| 数据库 | 必须安装 MySQL/PostgreSQL | SQLite 自动创建，零配置 |
| 许可证 | 通常闭源 | **AGPL v3 —— 永远开源** |

---

## ✨ 特性

### 部署与运维
- 🌍 **全平台全CPU支持** — Windows / Linux / macOS × x86 / x64 / ARM / ARM64 / MIPS64 / RISC-V（10 个预编译目标；`windows/386`、`windows/arm`、`linux/mips64` 受限于底层 SQLite CGo 库暂不支持）
- 🚀 **下载即用** — 选择对应平台二进制文件，直接运行，零依赖安装（31.6 MB）
- ⚙️ **自动创建 .env** — 首次启动自动生成配置文件，自动检测本机 IPv4/IPv6 地址填入
- 🔌 **配置化绑定** — 支持配置 `BIND_IPV4` / `BIND_IPV6` / `ENABLE_IPV6`，灵活控制监听网卡
- 🌐 **启动时列出所有访问地址** — 扫描所有网卡，输出 `localhost` + 局域网 IP 的完整 URL 列表
- 🛡️ **Windows 防火墙自动配置** — 尝试自动添加 `netsh` 入站规则，管理员权限不足时输出命令行供复制
- 📦 **前端内嵌** — Vue 3 前端编译进 Go 二进制，单文件部署
- 🗄️ **SQLite 零配置** — 首次运行时自动创建数据库，无需安装或配置

### 用户系统
- 🔐 **JWT HttpOnly Cookie 认证** — `SameSite=Lax`，72h 过期
- 🔑 **bcrypt 密码哈希**
- 📧 **邮箱验证码** — 支持 QQ邮箱/Gmail/163 SMTP，6位数字验证码，10分钟有效
- 🧩 **滑块验证码** — `ENABLE_CAPTCHA` 开关控制
- 👥 **用户分级** — 普通用户 / 作者 / VIP作者 / 管理员 / 仲裁员

### 作品创作
- 📝 **12 大分类** — 硬科幻 / 奇幻 / 推演文学 / 架空历史 / 现实主义 / 悬疑推理 / 实验文学 / 同人区 / 政治区 / 讽刺文学 / 泛二次元区 / 其他
- 🎨 **Cherry Markdown 编辑器** — 统一渲染引擎（编辑器 = 阅读器），支持 Markdown / KaTeX 公式（含 physics 物理宏包 + mhchem 化学式）/ Mermaid 图表 / chemfig 化学结构图（SMILES）/ 颜色标记 · 一键切换富文本模式
- 📂 **文件系统存储** — 小说正文以 HTML 存于文件系统，数据库仅存路径索引
- 🔗 **内容可验证** — 每章自动计算 SHA256 哈希 + HMAC-SHA256 作者签名，前端可验证章节完整性
- 📤 **多格式导出** — ZIP / Markdown / EPUB / TXT 一键导出
- 📥 **多格式导入** — 支持 ZIP / Markdown / TXT 导入，自动解析章节结构，支持预览确认

### 社区互动
- 💬 **评论系统** — 支持章评和书评，IP 限流保护
- ⭐ **多维度评分** — 类型完成度 / 叙事质量 / 思想深度 / 社区口碑 / 更新稳定性
- 📚 **书架** — 收藏追读，自动记录阅读进度
- 🏠 **论坛双轨制** — 公共论坛广场 + 作品专属子论坛 + 作者专属论坛
- 👑 **作者工作台** — 仪表盘（多指标可切换图表：章节增长/字数增长/评论趋势/收藏/阅读·折线⇄柱状二选一）、作品管理、收益统计、提现申请
- 📝 **作者博客** — 认证作者可发布博客文章，支持置顶，独立博客列表页（`/author/:id/blogs`）
- 🛡️ **管理员面板** — 数据大屏（2×2 ECharts 图表）、社区动态仪表盘、用户管理、分类管理、举报处理、站长配置
- 🏘️ **社区动态** — 管理员可查看最近注册用户、最新作品、最近评论、论坛新帖

### 安全与合规
- 🧱 **敏感区三级隔离墙** — 系统级（管理员配置敏感分区）+ 论坛级（敏感论坛自动触发）+ 作者级（自定义警告语），3~5步确认弹窗含手动输入，动态配置步骤数/警告语/跨域额外步数
- 🚫 **IP 黑名单** — 管理员可封禁 IP，支持过期时间
- 🚦 **IP 限流** — 滑动窗口算法，内存存储，登录 5次/60s，评论 20次/60s
- 🧹 **XSS 防护** — bluemonday 白名单净化 + 正则移除危险标签和事件处理器
- 🔏 **内容签名** — HMAC-SHA256 作者签名，章节完整性可验证
- 📋 **举报系统** — 支持举报作品/评论/帖子，管理员处理

### 站点互通
- 🌐 **联邦站点** — 支持远程站点互联与跨站作品同步（模仿 alist）
- 🔗 **公开 API** — `/api/site-info`、`/api/federated/novels`、`/api/federated/sites`

### 🎨 自定义 HTML-UI 美化

NVS 允许作者通过自定义 HTML 模板深度美化作品展示页面和作者主页。模板在独立 iframe 沙盒中运行，安全隔离。

**模板类型**

| 类型 | 显示位置 | 说明 |
|------|----------|------|
| 📖 阅读模板 | 小说阅读页（章节正文上方/下方） | 可调用平台小说 API，支持按钮/表单等交互控件 |
| 👤 作者展现模板 | 作者主页 `/author/:id` | 纯展示或交互，不依赖具体作品 |

**平台 JavaScript API（`window.NVS`）**

| 方法 | 说明 |
|------|------|
| `window.NVS.getNovelData(novelId?)` | 获取小说元数据+完整章节列表，返回 `Promise` |
| `window.NVS.getCurrentChapter()` | 从 iframe URL 解析当前阅读章节号 |
| `window.NVS.sendMessage(type, data)` | 通过 `postMessage` 向父页面发送消息 |
| `window.addEventListener('nvs-message', fn)` | 监听父页面发来的消息 |

**沙盒安全级别**

| 级别 | iframe sandbox 属性 | 适用 |
|------|---------------------|------|
| strict（只读） | `allow-scripts allow-same-origin` | 纯展示 |
| interactive（交互） | + `allow-forms allow-popups` | 含按钮/表单 |

**模板设置入口**：用户头像下拉 → 「🎨 模板设置」→ 分Tab管理阅读模板和作者展现模板

---

### 📦 自定义 HTML 托管（扩展应用）

上传 ZIP 包将完整的 HTML 应用部署到 NVS 平台，在独立沙盒中运行。支持任意前端技术栈（Vue/React/原生 JS），可包含 WASM 模块。

**上传流程**

```
编写应用（HTML+CSS+JS+WASM）→ 打包 ZIP → 上传到扩展应用 → 自动解压+安全扫描 → 沙盒运行
```

**ZIP 包规范**

| 限制项 | 值 |
|--------|-----|
| 压缩包大小 | ≤ 20 MB |
| 解压后总大小 | ≤ 50 MB |
| 压缩比 | ≤ 100:1（防 ZIP 炸弹） |
| 文件数量 | ≤ 500 |
| 允许扩展名 | .html .css .js .wasm .json .png .jpg .svg .woff2 等 |
| 入口文件 | 默认 index.html，可手动选择 |

**运行方式**

| 方式 | 路径 | 说明 |
|------|------|------|
| 完整运行 | `/app/:id` | 全屏沙盒，虚拟路径隐藏真实存储 |
| 弹窗预览 | 卡片「预览」按钮 | 对话框内 iframe 预览 |
| 广场运行 | 广场卡片点击 | 新标签页全屏运行 |

**HTML 应用广场**

作者设置项目为「公开」后，项目出现在广场 Tab 中。上传 ZIP 时自动提取首张图片作为缩略图。作者可设置「允许下载」，访客可下载原始 ZIP 包。

**端口代理**

扩展应用可分配独立端口（49152-65535 高位优先），通过命名路径 `/sandbox/proxy/{作者ID}/{项目名}/` 反向代理访问，端口号不在 URL 中暴露。

**虚拟文件系统 (VFS)**

沙盒应用可通过 REST API 在内存中读写数据，完全隔离于真实文件系统：

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `/api/sandbox-vfs/:id/write` | 写入 `{key, value}`，单值≤1MB，总量≤10MB/100键 |
| GET | `/api/sandbox-vfs/:id/read?key=` | 读取键值 |
| GET | `/api/sandbox-vfs/:id/list` | 列出所有键 |

**多层安全防护**

1. **上传前** — 23条内容安全扫描（提权命令/Shell注入/木马/挖矿/僵尸网络/数据窃取/钓鱼）
2. **解压中** — ZIP 炸弹检测（压缩比+总大小+文件数+路径穿越）
3. **解压后** — 目录锁定（文件→0444只读，目录→0755可写）
4. **运行时** — 写入拦截中间件（允许数据文件，禁止脚本文件）
5. **通信** — CSP + iframe sandbox 双重隔离
6. **路径隐藏** — `/app/:id` 虚拟路径，沙盒内无法探测真实存储位置

**WASM 支持**

- 上传时勾选「允许 WASM」
- 系统自动设置 `Cross-Origin-Embedder-Policy: require-corp`（启用 SharedArrayBuffer）
- 自动设置 `Cross-Origin-Opener-Policy: same-origin`
- CSP 启用 `'wasm-unsafe-eval'`
- `.wasm` 文件返回正确的 `Content-Type: application/wasm`

**展开/收起控件**

Markdown/富文本编辑器支持 `[expand title="标题"]...[/expand]` 语法，渲染为可点击展开的 `<details>` 折叠块。

---

## 🛠 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.22+ · Gin · GORM (SQLite/MySQL) |
| 前端 | Vue 3 · Vite · Element Plus · TypeScript · TipTap · Cherry Markdown 0.11 · ECharts · KaTeX (physics/mhchem) · Mermaid · smiles-drawer (chemfig) |
| 数据库 | SQLite（默认，零配置） / MySQL 8.0 |
| 缓存 | Redis 7（可选，MySQL 模式推荐） |
| 存储 | 文件系统（HTML + JSON） + 内存 VFS（沙盒虚拟文件系统） |
| 扩展 | 自定义 HTML-UI 模板 · ZIP 托管 · WASM · 端口代理 · 虚拟路径 |
| 安全 | 内容扫描（23条规则）· ZIP炸弹检测 · CSP · iframe sandbox · 目录锁定 |
| 构建 | Go `embed` 前端 → 单二进制文件 |

---

## 📦 快速开始

### 方式一：直接运行（推荐）

从 [Releases](https://github.com/HerryABU/nvs-community-web/releases) 下载对应平台的文件：

| 平台 | 文件 | 适用设备 |
|------|------|----------|
| Windows | `nvs-win-x64.exe` | 64位 Windows（最常用） |
| Windows | `nvs-win-arm64.exe` | ARM64 Windows（Surface Pro X 等） |
| Linux | `nvs-linux-x64` | 64位 Linux（x86_64） |
| Linux | `nvs-linux-x32` | 32位 Linux |
| Linux | `nvs-linux-arm64` | ARM64 Linux（树莓派4/5、鲲鹏等） |
| Linux | `nvs-linux-arm32` | ARM32 v7 Linux（树莓派3/4） |
| Linux | `nvs-linux-armv6` | ARM32 v6 Linux（树莓派1/2/Zero） |
| Linux | `nvs-linux-riscv64` | RISC-V 64（VisionFive、LicheePi 等） |
| macOS | `nvs-mac-x64` | Intel Mac |
| macOS | `nvs-mac-arm64` | Apple Silicon Mac (M1/M2/M3) |

> 注：`windows/386`、`windows/arm`、`linux/mips64` 三个目标因 `modernc.org/sqlite` CGo 库暂不支持对应架构而不可用。

下载后运行即可：

```bash
# Linux / macOS
chmod +x nvs-linux-x64
./nvs-linux-x64

# Windows
nvs-win-x64.exe
```

首次启动自动创建 `.env` 文件，浏览器打开日志中列出的任一地址即可。例如：

```
  可访问地址：
    本机:   http://localhost:8080
    本机:   http://127.0.0.1:8080
    局域网: http://192.168.1.3:8080        ← 局域网内其他设备用这个
```

> 无需安装 Go、Node.js、MySQL、Redis、Nginx。SQLite 数据库自动创建，前端已内嵌于 exe 中。

### 方式二：Docker

```bash
git clone https://github.com/HerryABU/nvs-community-web.git
cd nvs
cp .env.example .env
docker-compose up -d
```

### 方式三：从源码构建

```bash
git clone https://github.com/HerryABU/nvs-community-web.git
cd nvs

# 一键构建（Windows）
build.bat

# 或手动：
cd web && npm install && npm run build && cd ..
xcopy /E /I /Y dist server\dist
cd server && go build -ldflags="-s -w" -o nvs-server.exe . && cd ..
```

> **构建说明**：前端 `vite build` 输出到项目根目录 `dist/`，再由 `build.bat` 同步到 `server/dist/` 供 Go `embed` 嵌入。

---

## 🔐 默认管理员

| 字段 | 值 |
|------|-----|
| 邮箱 | `admin@nvs.local` |
| 密码 | `admin123` |

> ⚠️ 登录后请立即修改密码！

---

## 🧑‍💻 源码开发

### 环境要求

| 工具 | 版本 | 用途 |
|------|------|------|
| Go | ≥ 1.22 | 后端编译 |
| Node.js | ≥ 18 | 前端构建 |
| npm | ≥ 9 | 依赖管理 |

### 本地开发

```bash
# 终端 1：启动 Go 后端（监听 :8080）
cd server
go run main.go

# 终端 2：启动 Vue 前端（监听 :5173，API 自动代理到 8080）
cd web
npm run dev
```

浏览器打开 **http://localhost:5173** 进行前端开发（热重载）。Vite 自动将 `/api` 请求代理到 `localhost:8080`。

### 全平台交叉编译

项目提供 `build-all.ps1` 脚本，自动为 **10 个目标平台** 生成预编译二进制文件：

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

---

## ⚙️ 配置参考（.env）

启动时自动生成，也可手动创建或基于 `.env.example` 拷贝：

### 数据库
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DB_DRIVER` | `sqlite` | 数据库驱动（`sqlite` / `mysql`） |
| `DB_HOST` | `127.0.0.1` | MySQL 主机 |
| `DB_PORT` | `3306` | MySQL 端口 |
| `DB_USER` | `nvs_user` | MySQL 用户名 |
| `DB_PASSWORD` | `nvs_pass_2026` | MySQL 密码 |
| `DB_NAME` | `nvs` | MySQL 数据库名 |

### 网络与端口
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SERVER_PORT` | `8080` | 服务端口 |
| `BIND_IPV4` | `0.0.0.0` | IPv4 绑定地址（`0.0.0.0`=所有网卡） |
| `BIND_IPV6` | `::` | IPv6 绑定地址 |
| `ENABLE_IPV6` | `false` | 是否启用 IPv6 双栈 |
| `LOCAL_IPV4` | 自动检测 | 本机 IPv4（联邦互通用） |
| `LOCAL_IPV6` | 自动检测 | 本机 IPv6（联邦互通用） |

### JWT
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `JWT_SECRET` | `change-me-in-production` | JWT 签名密钥（务必修改） |
| `JWT_EXPIRE_HOURS` | `72` | Token 过期时间 |

### SMTP 邮件
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SMTP_HOST` | `smtp.qq.com` | SMTP 服务器 |
| `SMTP_PORT` | `587` | SMTP 端口 |
| `SMTP_USER` | — | 发件邮箱 |
| `SMTP_PASSWORD` | — | SMTP 授权码 |
| `SMTP_FROM` | — | 发件人地址 |

### 存储
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `NOVEL_DATA_DIR` | `./data/novels` | 小说文件存储目录 |
| `UPLOAD_DIR` | `./data/uploads` | 上传文件存储目录 |

### 平台功能
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PLATFORM_FEE_RATE` | `0.10` | 平台抽成比例 |
| `SITE_NAME` | `NVS 类型文学平台` | 站点名称 |
| `EMAIL_VERIFY_ENABLED` | `false` | 邮箱验证开关 |
| `CAPTCHA_ENABLED` | `false` | 滑块验证码开关 |

---

## 🌐 项目结构

```
nvs/
│
├── nvs-server.exe                 # 预编译单文件（下载即用）
├── build.bat                      # 一键构建脚本
├── build-all.ps1                  # 全平台交叉编译
├── .env.example                   # 环境变量模板
├── .env                           # [自动生成] 运行时配置
├── docker-compose.yml             # Docker 容器编排
│
├── server/                        # Go 后端
│   ├── main.go                    # 入口：配置初始化 → 数据库 → 路由 → SPA fallback → 启动
│   ├── embed.go                   # //go:embed all:dist 前端嵌入
│   ├── Dockerfile
│   ├── init.sql                   # MySQL 手动建表脚本
│   ├── go.mod / go.sum
│   │
│   ├── config/
│   │   └── config.go              # 自动创建 .env + 加载 + IPv4/IPv6 检测
│   │
│   ├── models/                    # 数据层（19 个结构体）
│   │   ├── user.go                # User + 管理员 / VIP 检查
│   │   ├── novel.go               # Novel + Chapter + NovelCategory
│   │   ├── comment.go             # Comment
│   │   ├── forum.go               # Forum / Thread / Post
│   │   ├── bookshelf.go           # BookShelf（收藏 + 阅读进度）
│   │   ├── rating.go              # Rating 五维度评分
│   │   └── platform.go            # VipApplication / Report / WithdrawalRequest
│   │                              #   / EarningsRecord / PlatformConfig / FederatedSite
│   │                              #   / FederatedNovel / BlacklistIP / WallConfig
│   │
│   ├── handlers/                  # API 处理器（按功能拆分，20+ 文件）
│   │   ├── auth.go                # 注册 / 登录 / 登出 / 发送验证码 / 验证邮箱
│   │   ├── novel.go               # 作品 CRUD / 分类列表 / 分类统计
│   │   ├── chapter.go             # 章节 CRUD / 内容哈希 / 作者签名 / 验证
│   │   ├── comment.go             # 评论 CRUD
│   │   ├── rating.go              # 评分 upsert / 查询
│   │   ├── forum.go               # 论坛 / 帖子 / 回帖
│   │   ├── bookshelf.go           # 书架管理 + 阅读进度
│   │   ├── author.go              # 作者后台 / 收益 / 提现 / 作者主页
│   │   ├── import_export.go       # 导入预览 / 多格式导入导出
│   │   ├── admin.go               # 管理后台（统计/大屏/用户/VIP/举报/财务/配置/站点）
│   │   └── admin_forum.go         # 论坛管理
│   │
│   ├── middleware/
│   │   └── auth.go                # JWT 鉴权 + 角色检查 + Cookie 管理
│   │
│   ├── security/
│   │   ├── ratelimit.go           # IP 滑动窗口限流（内存）
│   │   ├── sanitize.go            # bluemonday XSS 净化 + 危险标签移除
│   │   └── sign.go                # HMAC-SHA256 内容签名
│   │
│   ├── utils/
│   │   ├── jwt.go                 # JWT 生成/解析
│   │   ├── password.go            # bcrypt 密码哈希
│   │   ├── response.go            # 统一 JSON 响应格式
│   │   └── email.go               # SMTP 验证码发送与校验
│   │
│   └── dist/                      # [自动生成] 前端构建产物（Go embed 使用）
│
├── web/                           # Vue 3 前端
│   ├── vite.config.ts             # outDir → ../dist / dev 代理 /api → 8080
│   ├── package.json               # TipTap · ECharts · KaTeX · Mermaid · Element Plus
│   └── src/
│       ├── router/index.ts        # 17 路由 + JWT 路由守卫
│       ├── stores/                # Pinia
│       │   ├── auth.ts            # 用户认证状态
│       │   └── theme.ts           # 主题切换（亮/暗）
│       ├── api/                   # axios 封装
│       ├── views/                 # 20 个页面组件
│       │   ├── Home.vue / Login.vue / Register.vue
│       │   ├── NovelDetail.vue / Reader.vue
│       │   ├── Editor.vue / ChapterEditor.vue
│       │   ├── AuthorDashboard.vue / AuthorHome.vue / AuthorBlogs.vue
│       │   ├── BlogList.vue / BlogDetail.vue / BlogEditor.vue
│       │   ├── CategoryView.vue / Bookshelf.vue / Follows.vue
│       │   ├── Forums.vue / ForumDetail.vue / ThreadDetail.vue
│       │   ├── AdminDashboard.vue / UserManagement.vue
│       │   └── Login.vue / Register.vue
│       └── components/            # 15 个通用组件
│           ├── NavBar.vue / NovelCard.vue / HomeSearchResults.vue
│           ├── CommentSection.vue / CommentItem.vue
│           ├── StarRating.vue / SensitiveZoneGuard.vue / SlideCaptcha.vue
│           ├── RichTextEditor.vue / AnimatedNumber.vue / DashboardCharts.vue
│           ├── admin/ — AdminSiteSettings.vue / AdminCommunity.vue
│           └── author/ — AuthorCard.vue
│       └── styles/                # 全局样式（含 dark mode）
│
├── dist/                          # [自动生成] 前端构建产物（项目根目录）
├── data/                          # [运行时生成]
│   ├── nvs.db                     # SQLite 数据库
│   ├── novels/                    # 小说 HTML 正文
│   └── uploads/                   # 上传资源
│
└── nginx/
    └── nginx.conf                 # Docker 模式 Nginx 配置
```

---

## 🔌 API 一览

所有 API 均在端口 `8080` 的 `/api` 路径下。

### 认证
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/auth/register` | — | 注册 |
| POST | `/api/auth/login` | — | 登录（限流 5次/60s） |
| POST | `/api/auth/logout` | — | 登出 |
| POST | `/api/auth/send-code` | — | 发送邮箱验证码 |
| POST | `/api/auth/verify-code` | — | 验证邮箱验证码 |
| GET | `/api/auth/me` | JWT | 当前用户信息 |

### 作品
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/novels` | — | 作品列表（分页/搜索/排序） |
| GET | `/api/novels/:id` | — | 作品详情 |
| POST | `/api/novels` | 作者 | 创建作品 |
| PUT | `/api/novels/:id` | 作者 | 更新作品 |
| DELETE | `/api/novels/:id` | 作者 | 删除作品 |
| GET | `/api/categories` | — | 分类列表 |
| GET | `/api/categories/stats` | — | 分类作品统计 |

### 章节
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/novels/:id/chapters` | — | 章节目录 |
| GET | `/api/novels/:id/chapters/:num` | — | 章节内容 |
| GET | `/api/novels/:id/chapters/:num/verify` | — | 章节完整性验证（哈希+签名） |
| POST | `/api/novels/:id/chapters` | 作者 | 创建章节 |
| PUT | `/api/novels/:id/chapters/:num` | 作者 | 更新章节 |
| DELETE | `/api/novels/:id/chapters/:num` | 作者 | 删除章节 |

### 评论
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/comments` | — | 评论列表（支持 `novel_id` / `blog_id`） |
| POST | `/api/comments` | JWT | 创建评论（限流 20次/60s，支持小说和博客） |
| DELETE | `/api/comments/:id` | JWT | 删除评论 |

### 博客
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/blogs` | — | 博客列表 |
| GET | `/api/blogs/:id` | — | 博客详情 |
| GET | `/api/author/:id/blogs` | — | 作者博客列表 |
| POST | `/api/blogs` | JWT | 创建博客 |
| PUT | `/api/blogs/:id` | JWT | 更新博客 |
| DELETE | `/api/blogs/:id` | JWT | 删除博客 |

### 搜索与关注
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/search/authors` | — | 搜索作者 |
| POST | `/api/follow/:id` | JWT | 关注/取关 |
| GET | `/api/follow/stats` | JWT | 关注统计 |

### 隔离墙
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/wall-zone/:zone` | — | 分区隔离详情（三步确认） |

### 评分
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/ratings` | JWT | 提交/更新五维度评分 |
| GET | `/api/ratings` | JWT | 我的评分 |
| GET | `/api/novels/:id/rating` | — | 作品评分统计 |

### 书架
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/bookshelf` | JWT | 书架列表（含作品信息和进度） |
| POST | `/api/bookshelf` | JWT | 添加到书架 |
| DELETE | `/api/bookshelf/:id` | JWT | 从书架移除 |
| GET | `/api/bookshelf/check/:id` | JWT | 检查是否在书架 |
| POST | `/api/bookshelf/progress` | JWT | 更新阅读进度 |

### 论坛
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/forums` | — | 论坛列表 |
| GET | `/api/forums/:id` | — | 论坛详情 |
| GET | `/api/novels/:id/forum` | — | 作品专属论坛 |
| POST | `/api/forums/:id/threads` | JWT | 发帖 |
| GET | `/api/threads/:id` | — | 帖子详情 |
| POST | `/api/threads/:id/posts` | JWT | 回帖 |

### 作者
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/author/novels` | JWT | 我的作品列表 |
| GET | `/api/author/novels/:id/stats` | JWT | 作品统计 |
| GET | `/api/author/dashboard` | JWT | 作者仪表盘 |
| GET | `/api/author/profile/:id` | — | 作者公开主页 |
| GET | `/api/author/forum/:id` | — | 作者专属论坛 |
| GET | `/api/author/earnings` | JWT | 收益记录 |
| POST | `/api/author/withdraw` | JWT | 申请提现 |
| POST | `/api/author/apply-vip` | JWT | 申请 VIP 作者 |

### 导入导出
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/novels/import/preview` | JWT | 导入预览 |
| POST | `/api/novels/import` | JWT | 导入（支持追加模式） |
| POST | `/api/novels/:id/export` | JWT | 导出 ZIP |
| POST | `/api/novels/:id/export/epub` | JWT | 导出 EPUB |
| POST | `/api/novels/:id/export/markdown` | JWT | 导出 Markdown |
| POST | `/api/novels/:id/export/txt` | JWT | 导出 TXT |

### 举报
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/reports` | JWT | 提交举报 |

### 管理后台
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/admin/stats` | 管理员 | 基础统计 |
| GET | `/api/admin/dashboard` | 管理员 | 数据大屏（7天趋势、2×2 图表） |
| GET | `/api/admin/community` | 管理员 | 社区动态（最新用户/作品/评论/帖子） |
| GET | `/api/admin/users` | 管理员 | 用户列表 |
| PUT | `/api/admin/users/:id` | 管理员 | 修改用户 |
| GET | `/api/admin/vip-applications` | 管理员 | VIP 申请列表 |
| POST | `/api/admin/vip-applications/:id/approve` | 管理员 | 批准 VIP |
| GET | `/api/admin/reports` | 管理员 | 举报列表 |
| POST | `/api/admin/reports/:id/handle` | 管理员 | 处理举报 |
| GET | `/api/admin/finance` | 管理员 | 财务总览 |
| GET | `/api/admin/config` | 管理员 | 站长配置 |
| PUT | `/api/admin/config` | 管理员 | 更新站长配置 |
| GET | `/api/admin/wall-config` | 管理员 | 隔离墙配置 |
| PUT | `/api/admin/wall-config` | 管理员 | 更新隔离墙配置 |
| GET | `/api/admin/sites` | 管理员 | 远程站点列表 |
| POST | `/api/admin/sites` | 管理员 | 添加远程站点 |
| PUT | `/api/admin/sites/:id` | 管理员 | 更新远程站点 |
| DELETE | `/api/admin/sites/:id` | 管理员 | 删除远程站点 |
| POST | `/api/admin/sites/:id/sync` | 管理员 | 同步远程站点 |
| GET | `/api/admin/forums` | 管理员 | 论坛管理列表 |
| POST | `/api/admin/forums` | 管理员 | 创建论坛 |
| PUT | `/api/admin/forums/:id` | 管理员 | 编辑论坛 |
| DELETE | `/api/admin/forums/:id` | 管理员 | 删除论坛 |

### 远程站点
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/federated/novels` | — | 联邦作品列表 |
| GET | `/api/federated/sites` | — | 公开站点列表 |
| GET | `/api/site-info` | — | 本站信息 |

### 文件服务
| 路径 | 说明 |
|------|------|
| `/novels/*` | 小说正文静态文件（文件系统） |
| `/uploads/*` | 上传资源静态文件 |
| `/health` | 健康检查（HTML/JSON 双格式） |
| `/*` | SPA fallback → Vue Router |

### 🎨 自定义 UI 模板 (UserFrame)
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/userframes` | JWT | 我的模板列表 |
| POST | `/api/userframes` | JWT | 创建模板（含 HTML 内容） |
| GET | `/api/userframes/:id` | JWT | 模板详情+内容 |
| PUT | `/api/userframes/:id` | JWT | 更新模板 |
| DELETE | `/api/userframes/:id` | JWT | 删除模板 |
| GET | `/api/userframes/public` | — | 浏览公开模板 |
| GET | `/api/userframes/:id/preview` | — | 模板沙盒预览（HTML 页面） |
| GET | `/api/novels/:id/frames` | — | 作品关联的阅读模板 |
| GET | `/api/author/:id/frames` | — | 作者展现模板 |
| GET | `/api/template/novel/:id` | — | 小说数据 API（模板内 JS 调用） |

### 📦 扩展 HTML 托管 (UserHTML)
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/userhtmls` | JWT | 我的扩展列表 |
| POST | `/api/userhtmls/upload` | JWT | 上传 ZIP（multipart/form-data，含安全扫描） |
| GET | `/api/userhtmls/:id` | JWT | 扩展详情+文件列表 |
| PUT | `/api/userhtmls/:id` | JWT | 更新扩展元数据（名称/入口/公开/可下载） |
| DELETE | `/api/userhtmls/:id` | JWT | 删除扩展及全部文件 |
| GET | `/api/userhtmls/:id/preview` | — | 扩展沙盒预览 |
| GET | `/api/userhtmls/:id/download` | — | 下载原始 ZIP（需作者允许） |
| GET | `/api/novels/:id/htmls` | — | 作品关联的扩展 |
| GET | `/api/author/:id/htmls` | — | 作者的公开扩展 |
| GET | `/api/htmls/public` | — | 广场：所有公开扩展（分页） |

### ⚙️ 端口代理 & VFS
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/port/allocate` | JWT | 分配代理端口（需项目名） |
| POST | `/api/port/release` | JWT | 释放端口 |
| GET | `/api/port/list` | JWT | 已分配端口列表 |
| GET | `/api/sandbox/info` | — | 沙盒系统信息 |
| POST | `/api/sandbox-vfs/:id/write` | — | VFS 写入 {key, value} |
| GET | `/api/sandbox-vfs/:id/read?key=` | — | VFS 读取 |
| GET | `/api/sandbox-vfs/:id/list` | — | VFS 键列表 |

### 🔒 虚拟沙盒
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/app/:htmlId` | — | 虚拟沙盒入口（全屏运行，隐藏真实路径） |
| GET | `/app/:htmlId/*` | — | 虚拟沙盒静态资源 |
| ANY | `/sandbox/proxy/:uid/:name/*` | — | 命名端口反向代理 |

---

## 🗺 开发路线图

- [x] **Phase 1 MVP** — 注册登录、作品 CRUD、章节读写（含哈希签名）、分类体系、评论、多维度评分、论坛、导入预览/追加/导出、作者后台、管理后台
- [x] **Phase 1.5 安全与合规** — 敏感区隔离墙（可配置 3~5 步确认）、HMAC-SHA256 内容签名、章节完整性验证、举报系统、IP 黑名单、跨域评论限速
- [x] **Phase 2 生态增强** — 书架收藏与阅读进度、邮箱验证码、滑块验证码、站点联邦互通
- [x] **Phase 2.5 部署体验** — 自动创建 .env、IPv4/IPv6 网络配置、网卡 IP 自动扫描、Windows 防火墙自动配置、代码模块化拆分（后端22文件/前端15组件）、社区动态仪表盘、作者博客分页、收藏/阅读数据图表
- [x] **Phase 2.6 创作增强** — Cherry Markdown 统一渲染引擎、图表切换、搜索增强、隔离墙三级体系、字数趋势、博客 MD 渲染
- [x] **Phase 2.7 个性化 & 扩展** — 自定义 HTML-UI 模板（阅读+作者展现）、ZIP 扩展托管、HTML 应用广场、WASM 沙盒支持、VFS 虚拟文件系统、端口代理、23条内容安全扫描、虚拟路径隐藏、展开/收起控件
- [ ] **Phase 3 高级功能** — 付费阅读闭环、打赏 UI、EPUB 封面增强、反商业爬虫、自动备份
- [ ] **Phase 4 治理与扩展** — 社区仲裁员选举、财务公开面板、API 开放平台、移动端 PWA

---

## 📄 许可证

本项目采用 **[GNU Affero General Public License v3.0 (AGPL-3.0)](https://www.gnu.org/licenses/agpl-3.0.html)**。

这意味着：
- ✅ 你可以自由使用、修改和分发本项目
- ✅ 你可以用于商业用途
- ⚠️ **如果你修改了代码并通过网络提供服务，必须公开你的修改**
- ⚠️ 分发本软件（包括修改版）时必须保留相同的 AGPL-3.0 许可证

> 选择 AGPL v3 是为了确保平台永远对社区开放。即使有人 fork 后作为 SaaS 服务运行，其修改也必须回馈社区。

---

<p align="center">
  <sub>全平台 · 全CPU · 8080 端口 · 永远开源 🌌</sub>
</p>