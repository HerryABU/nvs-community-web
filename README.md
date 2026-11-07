<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/Vue-3.4-4FC08D?style=flat&logo=vuedotjs" alt="Vue">
  <img src="https://img.shields.io/badge/SQLite-3-003B57?style=flat&logo=sqlite" alt="SQLite">
  <img src="https://img.shields.io/badge/version-4.8.4-4F46E5?style=flat" alt="Version">
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

---

## ✨ 特性

> 下面按模块列出**全部能力名** —— 每条的具体参数、机制与边界条件见 [docs/FEATURES.md](docs/FEATURES.md)。

### 部署与运维
- 🌍 **全平台全CPU支持**
- 🚀 **下载即用**
- ⚙️ **自动创建 .env**
- 🔌 **配置化绑定**
- 🌐 **启动时列出所有访问地址**
- 🛡️ **Windows 防火墙自动配置**
- 📦 **前端内嵌**
- 🗄️ **SQLite 零配置**
- 🧭 **建站向导**
- 🔐 **数据库配置加密**
- 🔄 **数据库在线热迁移**
- 💾 **备份 / 恢复 / 定时备份**
- 🔀 **站点路径前缀挂载（反向代理）**
- ⚡ **静态资源强缓存**
- 🏷️ **页脚开源署名**

### 用户系统
- 🔐 **JWT HttpOnly Cookie 认证**
- 🔑 **bcrypt 密码哈希**
- 📧 **邮箱验证码**
- 🧩 **滑块验证码**
- 👥 **用户分级**
- 🪪 **个人中心统一入口**
- 📍 **注册/登录 IP 留存**
- 🕵️ **用户行为审计日志**

### 作品创作
- 📝 **12 大分类**
- 📖 **四种叙事形态（散文 / 话本气泡 / 漫画体 / 图片模式）** — 散文（Markdown）/ 话本气泡（角色对白）/ 漫画体（逐页翻看）/ 图片模式（流览有序图集，每章存 `[{url,caption}]`）；形态按章节生效，图片形态正文可空、以图集为准
- 📊 **每篇内容统计（阅读 / 点赞 / 收藏）** — 作者仪表盘逐篇明细，小说收藏复用书架、博客独立收藏表
- 🎨 **Cherry Markdown 编辑器（0.11.9 完整版）**
- 📂 **文件系统存储**
- 🔗 **内容可验证**
- 📤 **多格式导出**
- 📥 **多格式导入**
- 📦 **批量导入**

### 🤖 AI 工作台（对话 / 绘画 / 视频 / 智能体）- 🧭 **竖列式入口**
- 💬 **标准对话页**
- 🧑‍💻 **代码块：高亮 · 复制 · HTML/SVG 预览 · 一键发布**
- 🖼️ **SVG 可预览 · 可导出 PNG · 可下载源码**（SVG 是特殊的 HTML）
- 🧠 **思考型模型**
- 🎨 **专业绘画**
- ✨ **提示词优化**（把简短描述扩写成信息密度更高的提示词，不改创作意图）
- 🎬 **视频创作**
- 🧩 **智能体 / 思维导图 / 灵感广场 / 提示词库**
- 🎛 **模型池与路由**
- 🗂 **模型池两级导航**
- 🔌 **填渠道即可拉模型 + 联通测试**
- 💰 **渠道余额与真实成本**
- 🧮 **虚拟代币 ≠ 真金白银**
- 📈 **模型统计三个维度**
- 🔑 **BYOK**
- 🧾 **单次请求消耗双口径**
- 🏷️ **AIGC 内容标识（GB 45438—2025 / 标识办法）**
- 🧑‍💼 **智能体投稿 + 站长审核**
- 🧩 **左栏自定义菜单（站长配置）**

### 📖 阅读体验（精读 · 朗读 · 评论）
- 🔤 **阅读器**
- ✍️ **划线批注**
- 🔊 **语音朗读**
- 💬 **楼层式评论区**
- 🖼 **评论附图（多模态审核留证）**
- 🔃 **评论排序**
- 🎚 **评论密度**
- 🕸 **评论网（社交图谱）**
- 🔒 **加密楼**（评论区是否连带加密由站长决定）
- 🖼️ **图表导出（SVG / PNG / 源码）**

### 👥 群组与实时消息
- 🏘 **群组**
- 📡 **SSE 实时推送**
- 🔔 **未读计数**
- 💌 **私信**
- 📣 **消息中心**

### 🎭 话本 · 协作 · 版本控制 · OC 模式
- 💬 **话本气泡体**
- 👥 **多人协作**
- 🗂 **章节版本控制**
- 🎭 **OC 模式群组**
- 🧾 **提示词模板引擎**

### 💎 会员与额度
- 💳 **代币经济**
- 👑 **会员套餐**
- 🎁 **每日免费 AI 额度**
- 🧾 **会员页**
- 🔁 **支付通道**

### 社区互动
- 💬 **评论系统**
- ⭐ **多维度评分**
- 📚 **书架**
- 🏠 **论坛双轨制**
- 👑 **作者工作台**
- 📝 **作者博客**
- 🛡️ **管理员面板**
- 📊 **数据概览（三幕式）**
- 🏘️ **社区动态**

### 安全与合规
- 🧱 **敏感区三级隔离墙**
- 🚫 **IP 黑名单**
- 🚦 **IP 限流**
- 🔑 **四层身份 API Key 体系**
- 🤝 **联邦 Key 双向可见**
- 🧹 **XSS 防护**
- 🚫 **违禁词库**
- 🔏 **内容签名**
- 📋 **举报系统**
- 🗑️ **内容软删除**
- 📊 **访问日志关联 user_id**
- 🔐 **日志完整性保护**
- 🛡️ **内容安全扫描（23 条规则）**
- 🚨 **内容抢救台**
- 🧾 **日志敏感信息脱敏**
- 🛡️ **CSRF 防护**
- 🔑 **登录账号级锁定**
- 🔐 **JWT 随机密钥**
- 🔐 **密码双轨（国际算法 / 国密 SM2·SM3·SM4，可按 ClientHello 同一端口分流）**
- 🗝 **令牌加密密码（独立于 JWT）**
- 📢 **公告栏**
- 🧭 **176号令合规工具箱**
- 🕵️ **蜜罐体系（四档可调）**
- 🎭 **蜜罐诱饵内容可配置（假 .env / 假库 / 持久化金丝雀）**
- 🧱 **隔离墙访问仪表盘**
- 📡 **网络诊断与攻击流量面板**
- 🗺️ **访问来源地域（纯本地解析 + 标准地图）**
- 🚨 **警方取管台（police 角色）**
- 🧯 **AI 内容可追溯**
- 🛡️ **AI 安全闸三层**（规则引擎 + 蓝队判定模型 + 用户风险画像）
- 🧬 **跨轮拼装合成链检测**（拦 ADRO 式长周期越狱：零件递进到管制物质即断链）
- 🧪 **拒绝后泄漏检测**（含反应方程式硬信号：给出方程式即视为已给步骤）
- 🖼️ **评论图片多模态审核**（违规图不落库，留证三件套：软删 + 审核日志 + 审计日志）
- 🎨 **AI 绘画审核两层**（提示词初审：生成前拦，不扣费不调上游 / 出图后审：可选项，违规则撤回并退款）
- 🔎 **违禁词联网补充**（/search 检索 + 模型原生联网双模式，可手工录入兜底）
- 🎛️ **作者工作台独立视图**（顶部切换栏 + `?tab=` 参数：收视率图 / 作品 / 博客 / HTML / API Key / 论坛与回收站，一屏只装一个视图）
- ⏱️ **自定义主题 JS 延迟加载**（alist 语义：立即 / 空闲时 / 延时 N 秒三种加载方式，不与首屏抢主线程）
- 🕊️ **绥靖模式（以疏代堵）**（AI 安全闸第四种终止模式：用户完全无感，危险内容改写为引导内容并静默入档）
- 📛 **名单子系统（可疑 → 恶意两级）**（判定分越线自动升级；人工改档 / 清除 / 加注全程留痕）
- 🧾 **一键取证包导出**（名单记录 + 会话 + 消息 + 判定日志 + SHA256 完整性清单）
- 🤝 **警方取证审批流**（警方申请 → 站长批准 → 方可导出指定用户；越权导出拒绝并记审计）

### 🗳 舆论阵营分析（阵营图谱）
- 🕸 **阵营图谱** — 节点=阵营、边=共现，权重仅表规模、不裁决对错；SVG 无新依赖，只聚合已审核标记
- 🔍 **三维度范围** — 全局总览 / 按评论区 / 按议题（议题经内容桥接），范围标题实时回显
- 🔒 **按评论区开关** — 超级管理员可设平台默认 + 单评论区开/关阵营分析；关闭的评论区不进图谱、AI 分析被拒
- 🧠 **AI 思考过程可见可折叠** — 回传原生思考链 + 自述分析依据，前端可折叠面板
- 🔑 **模型锁定** — 阵营分析所选模型写入 localStorage，刷新/重开保持，不再每次回退第一个
- 🗣 **语言格式全自由** — 阵营语言格式改为自由文本标签（饭圈/政治/普通人/混用/未识别仅作参考），hash 着色
- ✅ **审核流** — 普通管理员新增标记进 pending，唯一站长审核通过才进图谱；修改/删除走申请由站长批准
- 🛡 **强中立提示词常驻** — 只标记阵营与语言格式，严禁判对错/倒戈多数派；严格模式安全提示词绝不复用至绥靖模式

### 站点互通
- 🌐 **联邦站点**
- 🔗 **公开 API**
- 📡 **联邦入站记录**
- 🔁 **联邦同步自报身份**

### 📱 移动端浏览支持
- 🍔 **响应式导航栏**
- 📖 **阅读器移动优化**
- 📐 **全站响应式**
- 🎛️ **控制面板移动适配**
- 💬 **弹窗限宽**
- 📊 **表格横滚**
- 🧩 **触控友好**
- 👆 **触控目标统一 ≥36px**（按钮 / 图标 / 链接 / 表格行内操作）
- 🔍 **iOS 输入不缩放**（输入类元素字号 ≥16px，避免 Safari 聚焦时放大整页）
- 📱 **刘海屏安全区**（导航栏、正文、阅读器工具条三处偏移同步）
- 🖥️ **后台窄屏可用**（侧栏自动折叠、顶栏精简、图表下拉命中区放大）

### 🎨 自定义 HTML-UI 美化

### 📦 自定义 HTML 托管（扩展应用）
- 上传时勾选「允许 WASM」
- 系统自动设置 `Cross-Origin-Embedder-Policy: require-corp`（启用 SharedArrayBuffer）
- 自动设置 `Cross-Origin-Opener-Policy: same-origin`
- CSP 启用 `'wasm-unsafe-eval'`
- `.wasm` 文件返回正确的 `Content-Type: application/wasm`


---

## 🛠 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.25 · Gin · GORM (SQLite/MySQL) |
| 前端 | Vue 3.4 · Vite 5 · Element Plus 2.7（按需导入） · TypeScript 5.5 · TipTap · Cherry Markdown 0.11.9（完整版，编辑器/阅读器同源引擎） · ECharts 6 · KaTeX (physics/mhchem) · Mermaid · smiles-drawer (chemfig) · three.js / 3d-force-graph（评论网图谱） |
| AI | 多厂商协议（OpenAI 兼容 / Anthropic / Gemini / MJ / Kling / Coze）· SSE 流式 · 密钥池与熔断 · 路由策略与自动降级 · 思考型模型 reasoning 透传 · 语音合成 |
| 数据库 | SQLite（默认，零配置） / MySQL 8.0（支持在线热迁移） |
| 缓存 | Redis 7（可选，MySQL 模式推荐） |
| 实时 | SSE（每业务域独立 Hub；单向广播、过反代简单、Cookie 鉴权无需 token 进 URL） |
| 存储 | 文件系统（HTML + JSON） + 内存 VFS（沙盒虚拟文件系统） |
| 扩展 | 自定义 HTML-UI 模板 · ZIP 托管 · WASM · 端口代理 · 虚拟路径 |
| 安全 | 内容扫描（23条规则）· 动态 XSS 规则引擎 · 违禁词库（标记/删除/替换 + 关键词/正则）· 软删除（8表）· 审计日志 · 访问日志关联 user_id · 日志完整性保护 · 日志脱敏 · 滑块验证码 · CSRF · ZIP炸弹检测 · CSP · iframe sandbox · 目录锁定 · 数据库配置 AES-256 加密 |
| 质量 | `make check`（SPA 白名单 + 组件解析审计 + go build/vet/test + 前端 build）· Playwright-core 四档宽度与功能级真浏览器验收 · 仓库内 git hooks · GitHub Actions（CI + 多平台 Release） |
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

> 注：`windows/386`、`windows/arm`、`linux/mips64` 三个目标不可用 —— 前两者受 Go 工具链与 `modernc.org/sqlite` 架构支持限制，后者因 `modernc.org/libc` 无 mips64 实现。改 cgo 也解决不了（该 SQLite 是纯 Go 转译实现，不经过 cgo）。

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


## 📚 文档导航

| 想知道 | 去哪看 |
|---|---|
| 每个版本改了什么 | [docs/CHANGELOG.md](docs/CHANGELOG.md) |
| 功能细节、参数、边界条件 | [docs/FEATURES.md](docs/FEATURES.md) |
| 所有接口 | [docs/API.md](docs/API.md) |
| 所有环境变量 | [docs/CONFIG.md](docs/CONFIG.md) |
| 目录结构 / 数据库表 | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| 本地开发 / 构建 / 测试 / 发版 | [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) |

---

## 🧑‍💻 本地开发

```bash
# 1. 后端（默认 8080，首次启动自动生成 .env）
cd server && go run .

# 2. 前端（另开终端，vite dev server 已代理 /api）
cd web && npm install && npm run dev

# 3. 打成单文件 exe（前端产物 embed 进 Go 二进制）
make dist && cd server && go build -trimpath -ldflags "-s -w" -o nvs-server.exe .
```

构建矩阵、测试轨道（`nodist` / `embed` / `gm`）、质量门禁脚本、发版与版本号约定见 [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)。

---

## 🌐 项目结构（顶层）

```
nvs/
├── server/        # Go 后端：main.go 入口 · routes 路由装配 · models 数据层 · handlers 接口 · 各领域子包
├── web/           # Vue 3 前端：views 页面 · components 组件 · api 封装 · markdown 渲染
├── dist/          # [构建产物] 前端打包输出，供 server embed
├── scripts/       # 质量门禁与真浏览器验收
├── docs/          # 文档（本目录）
└── go.gm.mod      # 国密轨独立依赖清单（默认 go.mod 零国密依赖）
```

完整目录树与数据库表见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)。

---

## 🗺 开发路线图

- [x] **Phase 1 MVP** — 注册登录、作品 CRUD、章节读写（含哈希签名）、分类体系、评论、多维度评分、论坛、导入预览/追加/导出、作者后台、管理后台
- [x] **Phase 1.5 安全与合规** — 敏感区隔离墙（可配置 3~5 步确认）、HMAC-SHA256 内容签名、章节完整性验证、举报系统、IP 黑名单、跨域评论限速
- [x] **Phase 2 生态增强** — 书架收藏与阅读进度、邮箱验证码、滑块验证码、站点联邦互通
- [x] **Phase 2.5 部署体验** — 自动创建 .env、IPv4/IPv6 网络配置、网卡 IP 自动扫描、Windows 防火墙自动配置、代码模块化拆分、社区动态仪表盘、作者博客分页、收藏/阅读数据图表
- [x] **Phase 2.6 创作增强** — Cherry Markdown 统一渲染引擎、图表切换、搜索增强、隔离墙三级体系、字数趋势、博客 MD 渲染
- [x] **Phase 2.7 个性化 & 扩展** — 自定义 HTML-UI 模板（阅读+作者展现）、ZIP 扩展托管、HTML 应用广场、WASM 沙盒支持、VFS 虚拟文件系统、端口代理、23条内容安全扫描、虚拟路径隐藏、展开/收起控件
- [x] **Phase 2.8 部署与合规（176号令）** — 建站向导（一次性安装密钥 + 数据库加密配置）、SQLite ↔ MySQL 在线热迁移、自动备份/恢复、注册/登录 IP 留存、内容软删除（8 表）、用户行为审计日志、访问日志关联 user_id、日志完整性保护（SHA256 归档 + 6 个月保留）、管理员审核日志规范、联邦入站记录
- [x] **Phase 2.9 流量治理** — 四层身份 API Key 体系、爬虫识别（无头/伪装/高频降级）、**已知名头爬虫直接 403 阻断（名单可配）**、联邦 Key 双向可见、按身份访问日志审计
- [ ] **Phase 3 高级功能** — 付费阅读闭环、打赏 UI、EPUB 封面增强
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
