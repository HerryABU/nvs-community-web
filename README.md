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
- 🧭 **建站向导** — 首次运行进入可视化安装向导（环境检查 / 数据库连通测试 / 站点名与管理员设置），一次性安装密钥仅控制台显示一次
- 🔐 **数据库配置加密** — 数据库连接信息（含密码）经 AES-256-GCM 加密落盘 `secure-config.json`，不写明文 `.env`
- 🔄 **数据库在线热迁移** — SQLite ↔ MySQL 双向热切换，全程无需重启，逐表复制并上报进度
- 💾 **备份 / 恢复 / 定时备份** — SQLite `VACUUM INTO` 无损快照 / MySQL 全表 JSON 快照，支持按间隔定时调度

### 用户系统
- 🔐 **JWT HttpOnly Cookie 认证** — `SameSite=Lax`，72h 过期
- 🔑 **bcrypt 密码哈希**
- 📧 **邮箱验证码** — 支持 QQ邮箱/Gmail/163 SMTP，6位数字验证码，10分钟有效
- 🧩 **滑块验证码** — `ENABLE_CAPTCHA` 开关控制
- 👥 **用户分级** — 普通用户 / 作者 / VIP作者 / 管理员 / 仲裁员
- 📍 **注册/登录 IP 留存** — 记录 `registration_ip` / `last_login_ip` / `last_login_at`（176号令合规）
- 🕵️ **用户行为审计日志** — 登录成功/失败、登出、改密、注册全记录（含 IP / User-Agent）

### 作品创作
- 📝 **12 大分类** — 硬科幻 / 奇幻 / 推演文学 / 架空历史 / 现实主义 / 悬疑推理 / 实验文学 / 同人区 / 政治区 / 讽刺文学 / 泛二次元区 / 其他
- 🎨 **Cherry Markdown 编辑器（0.11.9 完整版）** — 统一渲染引擎（编辑器 = 阅读器 100% 同源），支持 Markdown / KaTeX 公式（含 physics 物理宏包 + mhchem 化学式）/ Mermaid 图表 / chemfig 化学结构图（SMILES）/ 颜色标记 · 一键切换富文本模式；阅读器 / 博客 / 评论 / 论坛浏览端与编辑器预览共用同一 Cherry 主题样式，章节标题以大字显示（h1 2em 居中，h1-h6 全阶统一）
- 📂 **文件系统存储** — 小说正文以 HTML 存于文件系统，数据库仅存路径索引
- 🔗 **内容可验证** — 每章自动计算 SHA256 哈希 + HMAC-SHA256 作者签名，前端可验证章节完整性
- 📤 **多格式导出** — ZIP / Markdown / EPUB / TXT 一键导出
- 📥 **多格式导入** — 支持 ZIP / Markdown / TXT 导入，自动解析章节结构，支持预览确认
- 📦 **批量导入** — 小说批量导入支持三种模式：**一个文件一章**（标题取文件名，默认）/ 自定义正则拆分（快捷规则：第X章 / Markdown 标题 / 分割线）/ 自动识别标题；可选项「章节标题加第N章前缀」；**编辑模式下导入/批量导入自动追加到当前作品末尾，不会新建书本**（单文件导入同样支持追加）；**博客批量导入**：多文件一键创建多篇博客

### 社区互动
- 💬 **评论系统** — 支持章评和书评，IP 限流保护
- ⭐ **多维度评分** — 类型完成度 / 叙事质量 / 思想深度 / 社区口碑 / 更新稳定性
- 📚 **书架** — 收藏追读，自动记录阅读进度
- 🏠 **论坛双轨制** — 公共论坛广场 + 作品专属子论坛 + 作者专属论坛
- 👑 **作者工作台** — 仪表盘（多指标可切换图表：章节增长/字数增长/评论趋势/收藏/阅读·折线⇄柱状二选一）、作品管理、收益统计、提现申请
- 📝 **作者博客** — 认证作者可发布博客文章，支持置顶、**点赞**、评论，**CSDN 式一/二级分类**（管理员可任意管理），MD 文章右侧**可折叠目录**（#~######）；作者删除的内容进入**回收站**可自行恢复（区分管理员封禁，被封禁仅管理员可恢复、管理员可在「回收站监管」查看全站用户自删内容）
- 🛡️ **管理员面板** — 数据大屏（2×2 ECharts 图表）、社区动态仪表盘、用户管理、分类管理、举报处理、站长配置
- 🏘️ **社区动态** — 管理员可查看最近注册用户、最新作品、最近评论、论坛新帖

### 安全与合规
- 🧱 **敏感区三级隔离墙** — 系统级（管理员配置敏感分区）+ 论坛级（敏感论坛自动触发）+ 作者级（自定义警告语），3~5步确认弹窗含手动输入，动态配置步骤数/警告语/跨域额外步数
- 🚫 **IP 黑名单** — 管理员可封禁 IP，支持过期时间
- 🚦 **IP 限流** — 滑动窗口算法，内存存储，登录 5次/60s，评论 20次/60s
- 🧹 **XSS 防护** — bluemonday 白名单净化 + 动态正则规则引擎（管理员后台可维护、可测试）+ 正则移除危险标签和事件处理器
- 🚫 **违禁词库** — 管理员后台动态维护，**三种动作**：标记（内容正常发布但进入待审列表，管理员确认违规[可连带软删留证]/放行）/ 删除（直接移除匹配内容）/ 替换（替换为指定文本）；**双匹配模式**：关键词（字面匹配自动转义，填词即用）与正则（Go RE2 全方位：`(?i)` 忽略大小写、`(?m)` 多行、`\p{Han}` Unicode 属性）；**可配置适用范围**（章节 / 评论 / 论坛 / 博客 / 全部）；规则即时生效无需改代码，内置实时测试工具，mark 命中自动写入待审记录（去重）
- 🔏 **内容签名** — HMAC-SHA256 作者签名，章节完整性可验证
- 📋 **举报系统** — 支持举报作品/评论/帖子，管理员处理
- 🗑️ **内容软删除** — 8 张内容表软删除（`deleted_at`），删除不物理移除、正文归档到 `.deleted/`，随时可追溯原始内容；**所有统计均排除软删除数据**（分类分布 / 章节趋势 / 字数趋势 / 评论趋势等管理端与作者端统计全部 `JOIN novels` + `deleted_at IS NULL` 双过滤，已删除作品的章节/字数/评论不再计入）
- 📊 **访问日志关联 user_id** — JSON 一行含 `user_id`/方法/路径/IP/状态/耗时，可追溯到具体用户；**按小时切分**（每天内每小时一个文件），且 **request / database 分文件夹**（`log/requests/<日期>/<小时>.log` 与 `log/database/<日期>/<小时>.log`）
- 🔐 **日志完整性保护** — 每小时文件每日 SHA256 校验 + gzip 归档 + 只读权限（0444）+ 6 个月保留
- 🛡️ **内容安全扫描（23 条规则）** — 提权 / Shell注入 / 木马 / 挖矿 / 僵尸网络 / 数据窃取 / 钓鱼，上传即自动审查
- 🚨 **内容抢救台** — 二级密码 + 短时 token 应急窗口，粘贴 URL 即可预览全部评论/回复并物理硬删除，全程审计留痕

### 站点互通
- 🌐 **联邦站点** — 支持远程站点互联与跨站作品同步（模仿 alist）
- 🔗 **公开 API** — `/api/site-info`、`/api/federated/novels`、`/api/federated/sites`
- 📡 **联邦入站记录** — 记录「谁连接了我」（对方站点名/地址/IP/User-Agent/访问次数），被连方可见连接来源
- 🔁 **联邦同步自报身份** — 同步时向对方自报本站名与地址，便于对方追溯

### 📱 移动端浏览支持
- 🍔 **响应式导航栏** — 窄屏自动收起为汉堡菜单 + 侧滑抽屉（分类 / 论坛 / 博客 / 书架 / 搜索 / 账号全入口）
- 📖 **阅读器移动优化** — 工具栏图标化、正文大字大行距、底部全宽翻页按钮、目录抽屉视口适配
- 📐 **全站响应式** — 首页 / 作品详情 / 分类 / 书架 / 论坛 / 博客 / 消息 / 登录注册等 30+ 页面窄屏布局适配，卡片双列、行式布局纵向堆叠
- 🎛️ **控制面板移动适配** — 管理后台（15 个 admin 页面）+ 作者工作台 / 管理员仪表盘 + 作品 / 章节 / 博客编辑器窄屏可用：工具条自动换行、表单栅格单列、步骤条可横滚、输入框限宽、统计卡双列
- 💬 **弹窗限宽** — 所有 Dialog / Drawer 在窄屏自动限制在视口内，杜绝横向溢出
- 📊 **表格横滚** — 管理后台与列表表格窄屏自动启用横向滚动
- 🧩 **触控友好** — 翻页按钮 / 操作按钮加大、底部安全区适配（`env(safe-area-inset-bottom)`）、表单标签置顶、分页换行居中

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
| 前端 | Vue 3 · Vite · Element Plus · TypeScript · TipTap · Cherry Markdown 0.11.9（完整版，编辑器/阅读器同源引擎） · ECharts · KaTeX (physics/mhchem) · Mermaid · smiles-drawer (chemfig) |
| 数据库 | SQLite（默认，零配置） / MySQL 8.0（支持在线热迁移） |
| 缓存 | Redis 7（可选，MySQL 模式推荐） |
| 存储 | 文件系统（HTML + JSON） + 内存 VFS（沙盒虚拟文件系统） |
| 扩展 | 自定义 HTML-UI 模板 · ZIP 托管 · WASM · 端口代理 · 虚拟路径 |
| 安全 | 内容扫描（23条规则）· 动态 XSS 规则引擎 · 违禁词库（标记/删除/替换 + 关键词/正则）· 软删除（8表）· 审计日志 · 访问日志关联 user_id · 日志完整性保护 · ZIP炸弹检测 · CSP · iframe sandbox · 目录锁定 · 数据库配置 AES-256 加密 |
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

首次启动自动创建 `.env` 文件，并进入**建站向导**。控制台会打印**一次性安装密钥**（仅显示一次），浏览器打开日志中列出的地址完成安装：

```
  首次运行：进入建站向导模式
  向导地址: http://localhost:8080/install
  安装密钥（一次性，仅显示一次，切勿泄露）: xxxxxxxxxxxxxxxx
```

向导中依次完成：环境检查 → 数据库设置（SQLite 零配置 / MySQL）→ 站点名称与管理员账号。安装完成后数据库连接信息会 **AES-256-GCM 加密落盘**（不写明文 `.env`），随后正常启动：

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

> 通过**建站向导**安装时，可在向导中自定义管理员账号与密码（推荐）。

若未自定义（如直接使用已有数据库初始化），系统自动回退创建默认管理员：

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

> 🔐 **数据库密码加密存储**：通过建站向导或后台「数据库迁移」设置的 MySQL 连接信息（含密码）会经 **AES-256-GCM** 加密写入 `data/secure-config.json`（密钥存 `data/.secret.key`，仅本机可读），并自动从 `.env` 移除明文 `DB_*` 行，避免密码明文残留。`config.Init()` 启动时会优先解密加载该加密配置覆盖内存变量。

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

## 📝 版本更新记录（文件级改动说明）

### v2.1.3 统一用户索引 + 博客分类导入（2026-08-23）

> 修复 `/author/{用户名}/blogs/{id}` 无法访问的 NaN bug；博客管理支持把已有博客导入/归入分类（单篇 + 批量）。

| 功能 | 涉及文件 |
|---|---|
| **统一用户索引方法** `resolveUserKey`（数字=主键直接使用；用户名→`/api/users/resolve` 大小写不敏感；带缓存；`verifyNumeric` 供区分"数字=用户还是博客id"） | `web/src/utils/userKey.ts`（新） |
| 博客详情页修复：`Number("qbz95")=NaN` 导致 `/author/{用户名}/blogs/{id}` 404 → 改统一索引 | `web/src/views/BlogDetail.vue` |
| 博客主页/管理页改用统一索引（替代各自直调） | `web/src/views/BlogHome.vue`、`web/src/views/BlogManager.vue` |
| **博客分类导入**：每篇博客「设分类」下拉（归入一/二/三级分类或未分类）+「批量分类」勾选多篇一键移动 | `web/src/views/BlogManager.vue` |
| 冒烟测试扩展至 44 项（作者博客列表/详情用户名兼容） | `server/smoke_test.sh` |

### v2.1.2 全面审查补遗（2026-08-23）

> 全站核查"功能遗漏/模块干扰/前端接口路径"：无跨模块干扰；修复 4 处遗漏。

| 功能 | 涉及文件 |
|---|---|
| `ListAuthorBlogs`（`/api/author/:id/blogs`）兼容用户名 | `server/handlers/social_blog.go` |
| BlogHome 公开主页分类栏升级三级（语义修正：不再用全站计数过滤） | `web/src/views/BlogHome.vue` |
| BlogEditor 编辑分类路径恢复竞态修复 | `web/src/views/BlogEditor.vue` |
| BlogList 移动端三级分类样式适配 | `web/src/views/BlogList.vue` |

### v2.1.1 分类管理下放给普通作者（2026-08-23）

> 作者在「我的博客管理」的 **分类管理** 标签页即可：新增分类（一级/二级/三级）、编辑名称、**设置挂靠父分类**、**拖拽挂靠**（拖到另一分类上成为其子分类）、删除分类；管理员后台同样保留入口。

| 功能 | 涉及文件 |
|---|---|
| 分类 CRUD 鉴权从管理员放宽为**登录用户**（作者/管理员均可；层级/同名/防成环校验不变） | `server/handlers/blog_category.go`（`Create/Update/DeleteBlogCategory` + `requireLogin`）、`server/main.go` |
| 普通路由 `POST/PUT/DELETE /api/blog-categories`（admin 组路由保留，指向同一实现） | `server/main.go` |
| **作者端分类管理 UI**：Tab 标签页 + 新增/编辑/删除 + 拖拽挂靠 + 挂靠父分类下拉 | `web/src/views/BlogManager.vue` |
| 前端 API 封装 | `web/src/api/social/index.ts`（`createCategory/updateCategory/deleteCategory`） |
| 单元测试适配新函数名 | `server/handlers/recycle_softdel_test.go` |
| 冒烟测试扩展至 41 项（普通作者新增/挂靠/删除分类） | `server/smoke_test.sh` |

### v2.1.0 URL 规范化 + 博客管理/主页 + 三级分类（2026-08-23）

| 功能 | 涉及文件 |
|---|---|
| **作者博客管理页** `/author/blog/{用户名}`（仅本人，数字 id 兼容；他人访问自动转公开主页） | `web/src/views/BlogManager.vue`（新）、`web/src/router/index.ts`、`web/src/views/AuthorDashboard.vue` |
| **公开博客主页** `/blog/{用户名}`（数字 id 兼容；数字非用户时兼容旧单篇链接自动重定向） | `web/src/views/BlogHome.vue`（新）、`web/src/router/index.ts` |
| **用户标识解析**：数字=主键 id，用户名兼容（大小写不敏感）；作者主页/博客列表/博客详情/阅读页全部支持 | `server/handlers/author_profile.go`（`ResolveUserKey`）、`server/handlers/social_blog.go`、`server/main.go`、`web/src/api/admin.ts`、`web/src/views/AuthorHome.vue`、`AuthorBlogs.vue` |
| **/novel/:id 规范重定向** → `/author/{authorId}/novel/:id`（数字主键）；阅读链接规范化 | `web/src/views/NovelDetail.vue` |
| **分类支持三级**（一级→二级→三级；四级拒绝；删除/成环校验；树递归组装） | `server/models/blog_category.go`、`server/handlers/blog_category.go`、`server/models/blog.go`、`web/src/views/BlogList.vue`、`BlogEditor.vue`（级联选择器）、`admin/AdminBlogCategories.vue` |
| 单元测试：用户解析 + 三级分类层级 | `server/handlers/recycle_softdel_test.go` |
| 冒烟测试扩展至 36 项（用户名解析/三级分类/一级过滤含三级） | `server/smoke_test.sh` |

### v2.0.0 功能更新（2026-08-23）

> 本次更新：2 个严重 bug 修复 + 博客体系升级 + 软删除治理完善 + 测试体系。

#### 1️⃣ 严重 bug 修复

| 改动 | 涉及文件 |
|---|---|
| **访问日志中间件 64KB 截断请求体**（长章节/长博客 JSON 被截断 → 保存报「没有标题/参数错误」，根因） | `server/middleware/access_log.go` |
| **批量导入永远新建小说**（不追加到当前作品） | `server/handlers/import_export.go`、`web/src/api/novel.ts`、`web/src/views/Editor.vue` |
| 创建作品标题 trim 必填校验（空标题友好提示） | `server/handlers/novel.go` |
| 博客创建绑定错误信息更明确（「请填写博客标题和内容」） | `server/handlers/social_blog.go` |
| 章节保存后重复建章（id 标记为 -1 而非真实 id） | `web/src/views/ChapterEditor.vue` |

#### 2️⃣ 博客体系升级（分类 / 点赞 / 评论 / 目录）

| 功能 | 涉及文件 |
|---|---|
| **CSDN 式一/二级分类**（新表 `blog_categories` + `author_blogs.category_id`） | `server/models/blog_category.go`（新）、`server/models/blog.go`、`server/models/db.go`、`server/handlers/blog_category.go`（新）、`server/handlers/social_blog.go`、`server/main.go` |
| 分类管理（管理员任意增删改 + 同名/层级约束） | `web/src/views/admin/AdminBlogCategories.vue`（新）、`web/src/api/admin.ts`、`web/src/views/AdminDashboard.vue`、`web/src/router/index.ts` |
| 博客列表左侧分类栏 + 一级过滤含二级 | `web/src/views/BlogList.vue` |
| 博客编辑器分类下拉 | `web/src/views/BlogEditor.vue` |
| **点赞**（新表 `blog_likes` + `like_count` 冗余计数，toggle） | `server/models/blog_category.go`、`server/handlers/blog_category.go`、`server/main.go`、`web/src/views/BlogDetail.vue`、`web/src/api/social/index.ts` |
| 评论（复用 Comment.blog_id 链路 + 总数展示） | `web/src/components/CommentSection.vue`、`web/src/views/BlogDetail.vue` |
| **MD 可折叠目录**（#~###### 目录树、节点折叠、滚动高亮） | `web/src/views/BlogDetail.vue` |
| **目录面板折叠 + 全屏阅读** | `web/src/views/BlogDetail.vue` |

#### 3️⃣ 软删除治理（用户自删 vs 管理员封禁）

| 功能 | 涉及文件 |
|---|---|
| 新增 `deleted_by` 字段（user=自删 / admin=封禁） | `server/models/novel.go`、`server/models/comment.go`、`server/models/blog.go` |
| 用户删除标记 user；管理员封禁标记 admin（含级联章节） | `server/handlers/comment.go`、`server/handlers/admin_softdel.go` |
| 解封/恢复清除 deleted_by | `server/handlers/admin_ban.go` |
| **作者回收站**（自删内容可恢复，封禁内容拒绝） | `server/handlers/recycle.go`（新）、`server/main.go`、`web/src/api/novel.ts`、`web/src/views/AuthorDashboard.vue` |
| **管理员回收站监管**（全站用户自删列表 + 恢复） | `server/handlers/recycle.go`、`web/src/views/admin/AdminBlogCategories.vue` |

#### 4️⃣ 测试体系

| 类型 | 涉及文件 |
|---|---|
| 单元测试：访问日志 body 还原/脱敏/ multipart | `server/middleware/access_log_test.go`（新） |
| 单元测试：导入解析/标题清理 | `server/handlers/import_export_test.go`（新） |
| 单元测试：博客分类/点赞模型 | `server/models/blog_category_test.go`（新） |
| 集成测试：deleted_by 区分/回收站权限/分类管理 API | `server/handlers/recycle_softdel_test.go`（新） |
| 端到端冒烟测试（30 项） | `server/smoke_test.sh`（新） |

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
│   │   ├── config.go              # 自动创建 .env + 加载 + IPv4/IPv6 检测
│   │   ├── secure.go              # 数据库配置 AES-256-GCM 加密存储（.secret.key + secure-config.json）
│   │   └── setup.go               # 建站向导状态 / 一次性安装密钥
│   │
│   ├── models/                    # 数据层（40 个结构体）
│   │   ├── user.go                # User + 管理员 / VIP 检查（含注册/登录 IP）
│   │   ├── novel.go               # Novel + Chapter + NovelCategory（软删除）
│   │   ├── comment.go             # Comment（软删除）
│   │   ├── forum.go               # Forum / Thread / Post / ForumMute（软删除）
│   │   ├── blog.go                # AuthorBlog 作者博客（软删除）
│   │   ├── bookshelf.go           # BookShelf（收藏 + 阅读进度）
│   │   ├── rating.go              # Rating 五维度评分
│   │   ├── follow.go              # Follow 关注关系
│   │   ├── userframe.go           # UserFrame / UserHTML 自定义模板与扩展（软删除）
│   │   ├── platform.go            # VipApplication / Report / WithdrawalRequest
│   │   │                          #   / EarningsRecord / BlacklistIP
│   │   ├── platform_config.go     # PlatformConfig 站长配置键值
│   │   ├── platform_federated.go  # FederatedSite / FederatedNovel / FederatedInbound
│   │   ├── platform_wall.go       # WallConfig 隔离墙配置
│   │   ├── user_audit.go          # UserAuditLog 用户行为审计日志
│   │   ├── emergency_audit.go     # EmergencyAuditLog 管理员审计日志
│   │   ├── log_archive.go         # 访问/数据库日志归档调度（SHA256 + gzip + 6个月保留，适配小时文件）
│   │   ├── xss_rule.go            # XssRule 动态 XSS 正则规则
│   │   ├── badword.go             # BadWord 违禁词规则 + BadWordFlag 标记待审记录
│   │   ├── backup.go              # 备份 / 恢复 / 定时备份调度器
│   │   └── db.go                  # AllModels 迁移 + 跨库复制 + 热切换
│   │
│   ├── handlers/                  # API 处理器（按功能拆分，60+ 文件）
│   │   ├── auth_register.go       # 注册（记录注册 IP + 审计）
│   │   ├── auth_login.go          # 登录 / 登出 / 当前用户（登录 IP + 审计）
│   │   ├── auth_password.go       # 邮箱验证码 / 忘记重置密码 / 改密码（审计）
│   │   ├── novel.go               # 作品 CRUD / 分类列表 / 分类统计（软删除）
│   │   ├── novel_public.go        # 公开作品列表 / 搜索 / 旧路径 301
│   │   ├── chapter_read.go        # 章节读取 / 完整性校验
│   │   ├── chapter_write.go       # 章节 CRUD / SHA256 + HMAC 签名（软删除归档）
│   │   ├── comment.go             # 评论 CRUD（软删除）
│   │   ├── rating.go              # 五维评分 upsert / 查询
│   │   ├── forum.go               # 论坛 / 帖子 / 回帖
│   │   ├── forum_custom.go        # 用户自建论坛 / 版主删除（审计）
│   │   ├── bookshelf.go           # 书架管理 + 阅读进度
│   │   ├── social_follow.go       # 关注 / 取关 / 统计
│   │   ├── social_blog.go         # 作者博客 CRUD（软删除）
│   │   ├── author_profile.go      # 作者主页 / 头像 / 资料
│   │   ├── author_dashboard.go    # 作者数据大屏
│   │   ├── import_export.go       # 导入预览 / 多格式导入导出
│   │   ├── export_format.go       # EPUB / Markdown / TXT / DOCX 导出
│   │   ├── setup.go               # 建站向导（环境检查 / DB 测试 / 安装）
│   │   ├── db_admin.go            # 数据库健康 / 在线热迁移 / 备份恢复
│   │   ├── admin.go               # 管理后台（统计/大屏/社区动态）
│   │   ├── admin_user.go          # 用户管理 / 禁言 / 封禁（审计）+ 用户审计查询
│   │   ├── admin_report.go        # 举报处理（审计）
│   │   ├── admin_forum.go         # 论坛管理 / 审查
│   │   ├── admin_site.go          # 站长配置 / 隔离墙 / 联邦站点 / 入站记录
│   │   ├── emergency.go           # 内容抢救台（二级密码 + 硬删除 + 审计）
│   │   ├── userframe.go           # UserFrame 模板 CRUD
│   │   ├── userhtml.go            # UserHTML 扩展 ZIP 上传 / CRUD
│   │   ├── html_review.go         # HTML 内容人工复核
│   │   ├── sandbox_vfs.go         # 沙盒虚拟文件系统 VFS
│   │   ├── port.go                # 端口分配 / 命名代理
│   │   ├── xss_rule.go            # XSS 规则库 CRUD / 测试
│   │   ├── badword.go             # 违禁词库 CRUD / 测试 + 标记待审处理（确认/删除/放行）
│   │   ├── batch.go               # API 聚合（一次请求批量获取）
│   │   └── context.go             # 鉴权上下文统一封装（currentUserID 等）
│   │
│   ├── middleware/
│   │   ├── auth.go                # JWT 鉴权 + 角色检查 + Cookie 管理
│   │   ├── context.go             # 鉴权上下文 key 常量 + getter（单一权威来源）
│   │   ├── access_log.go          # 访问日志（JSON + user_id + 按小时滚动，log/requests/ 文件夹）
│   │   ├── header_logger.go       # 请求头日志（敏感头脱敏）
│   │   ├── recovery.go            # panic 恢复（不泄露堆栈）
│   │   ├── sandbox.go             # 沙盒安全头（CSP / COOP / COEP）
│   │   ├── sandbox_fs.go          # 沙盒写入保护 / 脚本扩展名拦截
│   │   └── portproxy.go           # 命名端口反向代理
│   │
│   ├── security/
│   │   ├── ratelimit.go           # IP 滑动窗口限流（内存）
│   │   ├── sanitize.go            # bluemonday XSS 净化 + 动态规则 + 危险标签移除
│   │   ├── sign.go                # HMAC-SHA256 内容签名 + 作者签名密钥
│   │   ├── content_scan.go        # 内容安全扫描（23 条规则）
│   │   ├── badwords.go            # 违禁词引擎（关键词/正则编译缓存 + 扫描/净化 + 范围过滤）
│   │   └── sandbox_fs.go          # 沙盒目录锁定 / 路径穿越防护
│   │
│   ├── utils/
│   │   ├── jwt.go                 # JWT 生成/解析
│   │   ├── password.go            # bcrypt 密码哈希
│   │   ├── response.go            # 统一 JSON 响应格式
│   │   ├── email.go               # SMTP 验证码发送与校验
│   │   └── zipbomb.go             # ZIP 炸弹检测（压缩比/大小/数量/穿越）
│   │
│   └── dist/                      # [自动生成] 前端构建产物（Go embed 使用）
│
├── web/                           # Vue 3 前端
│   ├── vite.config.ts             # outDir → ../dist / dev 代理 /api → 8080
│   ├── package.json               # TipTap · ECharts · KaTeX · Mermaid · Element Plus
│   └── src/
│       ├── router/index.ts        # 18 路由 + JWT 路由守卫
│       ├── stores/                # Pinia
│       │   ├── auth.ts            # 用户认证状态
│       │   └── theme.ts           # 主题切换（亮/暗）
│       ├── api/                   # axios 封装
│       ├── views/                 # 30+ 页面组件
│       │   ├── Home.vue / Login.vue / Register.vue
│       │   ├── NovelDetail.vue / Reader.vue
│       │   ├── Editor.vue / ChapterEditor.vue
│       │   ├── AuthorDashboard.vue / AuthorHome.vue / AuthorBlogs.vue
│       │   ├── BlogList.vue / BlogDetail.vue / BlogEditor.vue
│       │   ├── CategoryView.vue / Bookshelf.vue / Follows.vue
│       │   ├── Forums.vue / ForumDetail.vue / ThreadDetail.vue
│       │   ├── AdminDashboard.vue / UserManagement.vue
│       │   ├── admin/ — AdminSecurity(XSS规则库) / AdminBadWords(违禁词库) / AdminReports / AdminSoftDel / AdminBan 等
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

## 🗄️ 数据字典与数据库改进

### 外键约束
- SQLite 连接通过 DSN `_pragma=foreign_keys(1)` **每连接自动启用外键约束**（启动时校验 `PRAGMA foreign_keys = 1` 否则拒绝启动）；MySQL 原生启用。
- 热迁移（SQLite ↔ MySQL）复制期间临时关闭外键检查（`PRAGMA foreign_keys=OFF` / `SET FOREIGN_KEY_CHECKS=0`），完成后恢复，避免表复制顺序触发外键失败。
- 启用后删除被引用记录会被拒绝（如删除仍有回帖/评论的用户），由业务层先级联处理或归档。

### posts 表删除语义（方案A：统一走 softdel_posts）
- **posts 表不再有 `deleted_at` 列**：回帖删除 = 先 `ArchiveSoftDelete` 归档完整快照到 `softdel_posts` 留证表，再物理删除原始记录。
- 与 novels/chapters/comments/threads 等「软删除 + softdel_* 留证」双轨逻辑语义一致，消除 `deleted_at` 与 `softdel_posts` 两套删除逻辑并存的混乱。
- 升级迁移：启动时 `DropPostDeletedAt` 自动检测并重建 posts 表（SQLite 建新表→拷贝→重命名，保留全部数据与自增序列；MySQL 直接 `DROP COLUMN`），幂等可重复执行。
- `softdel_posts` 及其余 `softdel_*` 留证表保持不动。
- ⚠️ 回帖删除后不可解封恢复（物理删除），管理员「解封/恢复」对 post 类型仅清除封禁原因标记。

### 核心索引
启动迁移自动创建缺失索引并清理被替代的旧索引：

| 表 | 索引 | 用途 |
|----|------|------|
| novels | `idx_novels_author_status` | 按作者查作品并筛选状态 |
| posts | `idx_posts_thread_created` / `idx_posts_user_id` | 帖内回帖列表 / 按用户查回帖 |
| threads | `idx_threads_forum_created` | 按版块查主题帖 |
| comments | `idx_comments_novel_created` / `idx_comments_parent` | 作品评论列表 / 子评论 |
| notifications | `idx_notifications_user_read_created` | 未读通知倒序 |
| reports | `idx_reports_target` | 按目标查举报 |
| user_audit_logs | `idx_user_audit_user_time` | 按用户查审计日志 |
| zone_access_logs | `idx_zone_access_user_zone` | 按用户+分区查隔离墙记录 |
| forum_mutes | `idx_forum_mutes_forum_user` | 论坛内禁言查询 |

（chapters 由唯一复合索引 `uk_novel_chapter(novel_id, chapter_number)` 覆盖，无需重复索引；follows / bookshelf / ratings / federated_novels 已有等价索引。）

### 预留表 / 字段（功能未开发）
以下表与字段当前仅建表占位，**无业务代码**，请勿误用：

| 表 | 字段/表 | 状态 | 说明 |
|----|---------|------|------|
| users | `real_name` / `id_card` / `bank_card` | 预留 | 企业版实名认证与支付绑定，暂未启用 |
| vip_applications | 整表 | 预留 | VIP 会员申请体系，暂未开发 |
| earnings_records | 整表 | 预留 | 作者收益记录，暂未开发 |
| withdrawal_requests | 整表 | 预留 | 提现申请流程，暂未开发 |

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

### API 聚合
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/batch` | — | 一次请求批量并发获取多个接口数据（限流 30次/60s） |

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
| GET | `/api/blogs` | — | 博客列表（`category_id` 过滤，一级自动含三级子孙） |
| GET | `/api/blogs/:id` | — | 博客详情 |
| GET | `/api/blogs/:id/likes` | — | 点赞数 + 当前用户是否已赞 |
| POST | `/api/blogs/:id/like` | JWT | 点赞 / 取消点赞（toggle） |
| GET | `/api/blog-categories` | — | 博客分类树（一/二/三级 + 博客数） |
| GET | `/api/author/:id/blogs` | — | 作者博客列表 |
| POST | `/api/blogs` | JWT | 创建博客（可带 `category_id`） |
| PUT | `/api/blogs/:id` | JWT | 更新博客（可改分类） |
| DELETE | `/api/blogs/:id` | JWT | 删除博客（用户自删，进回收站） |

### 博客分类管理（管理员）
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/admin/blog-categories` | JWT(admin) | 分类树 |
| POST | `/api/admin/blog-categories` | JWT(admin) | 新建分类（`parent_id` 指定二级/三级） |
| PUT | `/api/admin/blog-categories/:id` | JWT(admin) | 修改分类（最多三级，防成环） |
| DELETE | `/api/admin/blog-categories/:id` | JWT(admin) | 删除分类（其下博客回未分类） |

### 回收站（软删除区分用户自删 / 管理员封禁）
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/author/recycle` | JWT | 作者回收站（自己删除的小说/博客） |
| POST | `/api/author/recycle/restore` | JWT | 恢复自己删除的内容（仅 `deleted_by=user`） |
| GET | `/api/admin/recycle` | JWT(admin) | 全站用户自删内容监管（可配合 `/admin/softdel/restore` 恢复） |

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
| GET | `/api/author/profile/:id` | — | 作者公开主页（`:id` 兼容用户名，数字是主键） |
| GET | `/api/author/forum/:id` | — | 作者专属论坛（兼容用户名） |
| GET | `/api/author/earnings` | JWT | 收益记录 |
| POST | `/api/author/withdraw` | JWT | 申请提现 |
| POST | `/api/author/apply-vip` | JWT | 申请 VIP 作者 |

### 用户标识解析（数字=主键 id，用户名兼容访问，大小写不敏感）
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/users/resolve?key=xxx` | — | 轻量解析用户标识 → `{id, username, nickname, role}` |

### 导入导出
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/novels/import/preview` | JWT | 导入预览 |
| POST | `/api/novels/import` | JWT | 导入（支持追加模式） |
| POST | `/api/novels/import/batch` | JWT | 批量导入（一个文件一章 / 自定义正则 / 自动识别） |
| POST | `/api/blogs/import/batch` | JWT | 博客批量导入（每个文件一篇，标题取文件名） |
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
| GET | `/api/admin/users/audit` | 管理员 | 用户行为审计日志（可按 user_id/action 筛选） |
| PUT | `/api/admin/users/:id` | 管理员 | 修改用户 |
| DELETE | `/api/admin/users/:id` | 管理员 | 删除/封禁用户 |
| POST | `/api/admin/users/:id/mute` | 管理员 | 禁言用户 |
| POST | `/api/admin/users/:id/unmute` | 管理员 | 解除禁言 |
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
| GET | `/api/admin/sites/inbound` | 管理员 | 入站连接记录（谁连了我） |
| POST | `/api/admin/sites` | 管理员 | 添加远程站点 |
| PUT | `/api/admin/sites/:id` | 管理员 | 更新远程站点 |
| DELETE | `/api/admin/sites/:id` | 管理员 | 删除远程站点 |
| POST | `/api/admin/sites/:id/sync` | 管理员 | 同步远程站点 |
| GET | `/api/admin/forums` | 管理员 | 论坛管理列表 |
| POST | `/api/admin/forums` | 管理员 | 创建论坛 |
| PUT | `/api/admin/forums/:id` | 管理员 | 编辑论坛 |
| DELETE | `/api/admin/forums/:id` | 管理员 | 删除论坛 |
| POST | `/api/admin/forums/:id/review` | 管理员 | 自建论坛审查 |
| GET | `/api/admin/xss-rules` | 管理员 | XSS 动态规则库 |
| POST | `/api/admin/xss-rules` | 管理员 | 新增 XSS 规则 |
| PUT | `/api/admin/xss-rules/:id` | 管理员 | 更新 XSS 规则 |
| DELETE | `/api/admin/xss-rules/:id` | 管理员 | 删除 XSS 规则 |
| POST | `/api/admin/xss-rules/test` | 管理员 | 测试规则正则 |
| GET | `/api/admin/bad-words` | 管理员 | 违禁词库规则列表 |
| POST | `/api/admin/bad-words` | 管理员 | 新增违禁词规则 |
| PUT | `/api/admin/bad-words/:id` | 管理员 | 更新违禁词规则 |
| DELETE | `/api/admin/bad-words/:id` | 管理员 | 删除违禁词规则 |
| POST | `/api/admin/bad-words/test` | 管理员 | 测试违禁词规则（关键词/正则 × 标记/删除/替换） |
| GET | `/api/admin/bad-word-flags` | 管理员 | 违禁词标记待审列表（status 过滤 + 分页） |
| PUT | `/api/admin/bad-word-flags/:id` | 管理员 | 处理标记（确认违规[可选删除内容留证] / 放行） |
| GET | `/api/admin/html-reviews` | 管理员 | HTML 内容审查列表 |
| GET | `/api/admin/html-reviews/:type/:id` | 管理员 | 审查详情 |
| POST | `/api/admin/html-reviews/:type/:id/review` | 管理员 | 复核 HTML 内容 |

### 🔧 内容抢救台（应急窗口）
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/admin/emergency/set-password` | 管理员 | 设置/修改二级密码 |
| POST | `/api/admin/emergency/unlock` | 管理员 | 二级密码解锁（限流 10次/60s，失败锁定 15 分钟） |
| GET | `/api/admin/emergency/preview` | 二级密码 token | 拉取内容预览（粘贴 URL，小说/博客/帖子返回全部评论/回复） |
| DELETE | `/api/admin/emergency/content` | 二级密码 token | 物理硬删除（不可恢复） |
| GET | `/api/admin/emergency/audit` | 管理员 | 抢救台审计日志 |

### 🗄️ 数据库管理
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/admin/db/health` | 管理员 | 数据库健康检查 |
| GET | `/api/admin/db/current` | 管理员 | 当前连接摘要（密码脱敏） |
| POST | `/api/admin/db/migrate` | 管理员 | SQLite ↔ MySQL 在线热迁移（无需重启） |
| GET | `/api/admin/db/job/:id` | 管理员 | 迁移任务进度 |
| POST | `/api/admin/db/backup` | 管理员 | 立即备份 |
| GET | `/api/admin/db/backups` | 管理员 | 备份列表 |
| DELETE | `/api/admin/db/backups/:name` | 管理员 | 删除备份 |
| GET | `/api/admin/db/backups/:name/download` | 管理员 | 下载备份 |
| POST | `/api/admin/db/restore` | 管理员 | 恢复备份（仅 SQLite） |
| GET | `/api/admin/db/backup-config` | 管理员 | 定时备份配置 |
| PUT | `/api/admin/db/backup-config` | 管理员 | 更新定时备份配置 |

### 🧭 建站向导
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/setup/status` | — | 安装状态 |
| GET | `/api/setup/env-check` | 安装密钥 | 环境检查 |
| POST | `/api/setup/db-test` | 安装密钥 | 测试数据库连通 |
| POST | `/api/setup/install` | 安装密钥 | 执行安装（装后路由注销） |

### 远程站点
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/federated/novels` | — | 联邦作品列表 |
| GET | `/api/federated/sites` | — | 公开站点列表 |
| GET | `/api/site-info` | — | 本站信息 |

> 📡 上述联邦公开接口会记录「谁连接了我」（入站连接：对方站点名/地址/IP/User-Agent/访问次数），被连方可在后台「远程站点互通 → 谁连了我」查看。联邦同步时本站会向对方自报身份（`source_name`/`source_url`）。

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
- [x] **Phase 2.5 部署体验** — 自动创建 .env、IPv4/IPv6 网络配置、网卡 IP 自动扫描、Windows 防火墙自动配置、代码模块化拆分、社区动态仪表盘、作者博客分页、收藏/阅读数据图表
- [x] **Phase 2.6 创作增强** — Cherry Markdown 统一渲染引擎、图表切换、搜索增强、隔离墙三级体系、字数趋势、博客 MD 渲染
- [x] **Phase 2.7 个性化 & 扩展** — 自定义 HTML-UI 模板（阅读+作者展现）、ZIP 扩展托管、HTML 应用广场、WASM 沙盒支持、VFS 虚拟文件系统、端口代理、23条内容安全扫描、虚拟路径隐藏、展开/收起控件
- [x] **Phase 2.8 部署与合规（176号令）** — 建站向导（一次性安装密钥 + 数据库加密配置）、SQLite ↔ MySQL 在线热迁移、自动备份/恢复、注册/登录 IP 留存、内容软删除（8 表）、用户行为审计日志、访问日志关联 user_id、日志完整性保护（SHA256 归档 + 6 个月保留）、管理员审核日志规范、联邦入站记录
- [ ] **Phase 3 高级功能** — 付费阅读闭环、打赏 UI、EPUB 封面增强、反商业爬虫
- [x] **Phase 3.5 移动端浏览** — 全站响应式布局、汉堡菜单导航、阅读器移动优化、弹窗限宽、表格横滚、触控友好
- [x] **Phase 3.6 渲染统一与统计准确性** — Cherry 0.11.9 完整版统一引擎（阅读器/博客/评论/论坛与编辑器预览同源同主题）、章节标题大字（h1-h6 !important 全阶统一）、浏览端 Cherry 主题副作用修复（.cherry-previewer 分屏 50% 宽度回归）、全量统计排除软删除数据（分类分布 / 章节 / 字数 / 评论趋势）、删除章节编号自动补齐（renumberChapters）
- [x] **Phase 3.7 阅读器完整版渲染 + 标题大字 + 日志小时切分** — 渲染器/阅读器切换为 Cherry 完整版构建产物（cherry-markdown.esm.js headless 实例，与编辑器 100% 同源同产物）、阅读器正文自动注入「第N章 标题」一级标题大字（参照上传 md 已有标题则不重复；存量数据库章节渲染期自动生效，不改文件不破坏签名）、日志按小时切分且 request/database 分文件夹（log/requests/<日期>/<小时>.log）、归档与 SHA256 适配小时文件、社区最近帖子统计排除软删除
- [x] **Phase 3.8 内容治理：违禁词库** — 动态违禁词规则引擎（关键词+正则双匹配、标记/删除/替换三动作、章节/评论/论坛/博客可配置范围）、章节/评论/帖子/博客 7 处写入点自动净化 + mark 命中进待审队列（去重）、管理员后台「违禁词库」页面（规则 CRUD + 实时测试 + 待审处理[确认违规可连带软删留证/放行] + 分页）、引擎单测覆盖
- [ ] **Phase 4 治理与扩展** — 社区仲裁员选举、财务公开面板、API 开放平台、移动端 PWA（离线缓存 / 安装到主屏）

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