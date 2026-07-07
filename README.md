<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/Vue-3.4-4FC08D?style=flat&logo=vuedotjs" alt="Vue">
  <img src="https://img.shields.io/badge/SQLite-3-003B57?style=flat&logo=sqlite" alt="SQLite">
  <img src="https://img.shields.io/badge/license-AGPL%20v3-blue.svg" alt="License">
</p>

<h1 align="center">📚 NVS — 去中心化网络小说平台</h1>

<p align="center">
  <strong>下载即用 · 默认端口 8080 · 前端内嵌 · 无需数据库</strong><br>
  聚焦高质量类型文学 · 作者自主定价 · 版权归属作者 · 文件系统存储
</p>

---

## ⚡ 为什么选 NVS？

**一个 exe 文件（也有Linux版本），双击运行，浏览器打开 `http://localhost:8080`，你就拥有了一个完整的网络小说平台。** 不需要装 MySQL，不需要配 Nginx，不需要 npm install —— 前端、后端、数据库全部内嵌在一个二进制文件里。

| 对比 | 传统小说站点 | NVS |
|------|------------|-----|
| 部署 | 服务器 + 数据库 + 缓存 + 前端 | 一个 exe，双击即用 |
| 默认端口 | 各组件端口不一 | **统一 8080** |
| 前端 | 需额外部署和配置 | **内嵌在 exe 中** |
| 数据库 | 必须安装 MySQL/PostgreSQL | SQLite 自动创建，零配置 |
| 许可证 | 通常闭源 | **AGPL v3 —— 永远开源** |

---

## ✨ 特性

- 🚀 **直接版本，下载即用** — 仓库提供预编译的 `nvs-server.exe`，下载后双击运行，无需安装任何依赖
- 🔌 **默认端口 8080** — 全平台统一使用 `http://localhost:8080`，前端、API、静态资源均通过同一端口服务
- 📦 **前端内嵌** — Vue 3 前端编译进 Go 二进制文件，不需要额外启动 web server 或 npm
- 🗄️ **SQLite 零配置** — 首次运行时自动创建数据库，不需要安装或配置任何数据库软件
- 📝 **类型文学调性** — 12 大分类（硬科幻 / 推演文学 / 架空历史 / 悬疑推理 / 政治区 / 讽刺文学…），拒绝套路文，支持作品多分类
- 👑 **作者友好** — 自主定价、平台仅抽成 10%、章节内容哈希签名防篡改、支持一键导出迁移（ZIP / Markdown / EPUB / TXT）
- 💬 **社区共治** — 论坛广场 + 作品专属子论坛双轨制、多维度评分、举报与申诉
- 🛡️ **敏感内容隔离** — 同人区 / 政治区进入需 3~5 步确认弹窗，跨域评论限速，可动态配置隔离墙规则
- 🔒 **安全底线** — JWT HttpOnly Cookie、bcrypt 密码哈希、XSS 白名单净化（bluemonday）、IP 限流（滑动窗口）、图片重编码
- 📂 **文件系统存储** — 小说正文以 HTML 存于文件系统，数据库仅存路径索引，读压力降低 ~80%，打包导出天然支持
- 🌐 **站点互通** — 模仿 alist，支持远程站点互联与跨站作品同步
- 🔗 **内容可验证** — 每章自动计算 SHA256 哈希 + HMAC-SHA256 作者签名，章节完整性前端可验
- 🎨 **现代编辑器** — 基于 TipTap 的富文本编辑器，支持 Markdown / LaTeX 公式（KaTeX）/ Mermaid 图表 / 颜色标记

---

## 🛠 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.21+ · Gin · GORM |
| 前端 | Vue 3 · Vite · Element Plus · TypeScript · TipTap · ECharts · KaTeX · Mermaid |
| 数据库 | SQLite（默认，零配置） / MySQL 8.0 |
| 缓存 | Redis 7（可选，MySQL 模式推荐） |
| 存储 | 文件系统（HTML + JSON） |
| 构建 | Go `embed` 前端 → 单二进制文件 |

---

## 📦 快速开始

### 方式一：直接版本（推荐）

从 [Releases](https://github.com/your/nvs/releases) 页面下载 `nvs-server.exe`，双击运行，或命令行：

```bash
nvs-server.exe
```

浏览器打开 **http://localhost:8080**，开始使用。

> 无需安装 Go、Node.js、MySQL、Redis、Nginx。SQLite 数据库自动创建，前端已内嵌于 exe 中，所有服务通过端口 8080 统一提供。

### 方式二：Docker

```bash
git clone https://github.com/your/nvs.git
cd nvs
cp .env.example .env
docker-compose up -d
```

### 方式三：从源码构建

```bash
git clone https://github.com/your/nvs.git
cd nvs

# 安装前端依赖
cd web && npm install && cd ..

# 一键构建（Windows）
build.bat

# 或手动：npm run build → go build
cd web && npm run build && cd ../server && go build -ldflags="-s -w" -o nvs-server.exe .
```

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
| Go | ≥ 1.21 | 后端编译 |
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

### 交叉编译

```bash
# Linux
cd server
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o nvs-server .

# macOS
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o nvs-server .
```

> **关于 `embed.go`**：Go 1.16+ 的 `//go:embed all:dist` 将 `server/dist/` 全部嵌入二进制。**前端必须先 `npm run build`，否则编译失败。**

---

## 🌐 项目结构

```
nvs/
│
├── nvs-server.exe                 # ★ 预编译直接版本（下载即用）
├── build.bat                      # 一键构建脚本
├── .env.example                   # 环境变量模板
├── docker-compose.yml             # Docker 容器编排
├── csdn.md                        # CSDN 发布版项目介绍
├── 需求.txt                       # 465 行完整需求文档
│
├── server/                        # Go 后端
│   ├── main.go                    # ★ 入口：DB 连接 → AutoMigrate → 路由注册 → SPA fallback
│   ├── embed.go                   # ★ //go:embed all:dist —— 前端嵌入二进制
│   ├── Dockerfile                 # 后端容器镜像
│   ├── init.sql                   # MySQL 手动建表脚本
│   ├── go.mod / go.sum            # Go 依赖
│   │
│   ├── config/
│   │   └── config.go              # 环境变量读取
│   │
│   ├── models/                    # 数据层（18 张数据表）
│   │   ├── user.go                # User（含签名密钥、实名认证）
│   │   ├── novel.go               # Novel + Chapter + NovelCategory 多分类
│   │   ├── comment.go             # Comment
│   │   ├── forum.go               # Forum / Thread / Post
│   │   ├── rating.go              # Rating 多维度评分
│   │   └── platform.go            # VipApplication / Report / WithdrawalRequest / EarningsRecord
│   │                              #   / PlatformConfig / FederatedSite / FederatedNovel / BlacklistIP
│   │                              #   / WallConfig 隔离墙 · GetCategories 动态分类
│   │
│   ├── handlers/                  # API 处理器（10 个模块）
│   │   ├── auth.go                # 注册 / 登录 / 登出 / 当前用户
│   │   ├── novel.go               # 作品 CRUD / 分类列表与统计 / 排序
│   │   ├── chapter.go             # 章节 CRUD + 内容哈希 + 作者签名 + 章节验证
│   │   ├── comment.go             # 评论 CRUD
│   │   ├── rating.go              # 评分 upsert / 查询
│   │   ├── forum.go               # 论坛 / 帖子 / 回帖 / 作品论坛 / 作者论坛
│   │   ├── author.go              # 作者后台：统计 / 仪表盘 / 收益 / 提现 / 作者主页
│   │   ├── import_export.go       # 导入预览/追加 + ZIP/MD/EPUB/TXT 导出
│   │   ├── admin.go               # 管理后台：统计/仪表盘/用户/VIP/举报/财务/配置/站点
│   │   └── admin_forum.go         # 论坛管理
│   │
│   ├── middleware/
│   │   ├── auth.go                # JWT 鉴权 + 角色检查
│   │   └── ratelimit.go           # IP 滑动窗口限流
│   │
│   ├── utils/
│   │   ├── jwt.go                 # JWT 生成/解析
│   │   ├── password.go            # bcrypt
│   │   ├── response.go            # 统一响应格式
│   │   ├── sanitize.go            # HTML 净化
│   │   └── crypto.go              # SHA256 哈希 + HMAC-SHA256 签名
│   │
│   └── dist/                      # [自动生成] 前端构建产物
│
├── web/                           # Vue 3 前端
│   ├── vite.config.ts             # ★ build → ../server/dist，dev 代理 /api → 8080
│   ├── package.json               # TipTap · ECharts · KaTeX · Mermaid · Element Plus
│   └── src/
│       ├── router/index.ts        # 14 个路由 + 路由守卫
│       ├── stores/                # Pinia 状态管理
│       ├── api/                   # axios 封装
│       ├── views/                 # 14 个页面
│       │   ├── Home.vue           # 首页
│       │   ├── Login.vue          # 登录
│       │   ├── Register.vue       # 注册
│       │   ├── NovelDetail.vue    # 作品详情
│       │   ├── Reader.vue         # 阅读器
│       │   ├── Editor.vue         # TipTap 富文本/Markdown 编辑器
│       │   ├── ChapterEditor.vue   # 章节编辑
│       │   ├── AuthorDashboard.vue # 作者后台
│       │   ├── AuthorHome.vue     # 作者主页
│       │   ├── CategoryView.vue   # 分类浏览
│       │   ├── AdminDashboard.vue  # 管理员面板（含 ECharts 大屏）
│       │   ├── Forums.vue         # 论坛广场
│       │   ├── ForumDetail.vue    # 论坛详情
│       │   └── ThreadDetail.vue   # 帖子详情
│       ├── components/            # 9 个通用组件
│       │   ├── NavBar.vue         # 顶部导航
│       │   ├── NovelCard.vue      # 作品卡片
│       │   ├── CommentSection.vue  # 评论区
│       │   ├── CommentItem.vue    # 单条评论
│       │   ├── StarRating.vue     # 星级评分
│       │   ├── SensitiveZoneGuard.vue # ★ 敏感区隔离墙（3~5 步确认）
│       │   ├── RichTextEditor.vue # TipTap 编辑器
│       │   ├── AnimatedNumber.vue # 数字动画
│       │   └── DashboardCharts.vue # ECharts 图表
│       └── styles/                # 全局样式（含 dark mode）
│
├── nginx/
│   └── nginx.conf                 # Docker 模式 Nginx 配置
│
└── data/                          # [运行时生成]
    ├── nvs.db                     # SQLite 数据库
    ├── novels/                    # 小说 HTML 正文
    └── uploads/                   # 上传资源
```

---

## ⚙️ 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DB_DRIVER` | `sqlite` | 数据库驱动（`sqlite` / `mysql`） |
| `DB_HOST` | `127.0.0.1` | MySQL 主机 |
| `DB_PORT` | `3306` | MySQL 端口 |
| `DB_USER` | `nvs_user` | MySQL 用户名 |
| `DB_PASSWORD` | `nvs_pass_2026` | MySQL 密码 |
| `DB_NAME` | `nvs` | MySQL 数据库名 |
| `REDIS_HOST` | `127.0.0.1` | Redis 主机 |
| `REDIS_PORT` | `6379` | Redis 端口 |
| `JWT_SECRET` | `change-me-in-production` | JWT 签名密钥（务必修改） |
| `JWT_EXPIRE_HOURS` | `72` | Token 过期时间 |
| `SERVER_PORT` | `8080` | **服务端口（默认 8080）** |
| `NOVEL_DATA_DIR` | `./data/novels` | 小说文件存储目录 |
| `UPLOAD_DIR` | `./data/uploads` | 上传文件存储目录 |
| `PLATFORM_FEE_RATE` | `0.10` | 平台抽成比例 |
| `SITE_NAME` | `NVS 类型文学平台` | 站点名称 |

---

## 🔌 API 一览

所有 API 均在端口 **8080** 的 `/api` 路径下。

### 认证
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/auth/register` | — | 注册 |
| POST | `/api/auth/login` | — | 登录（限流 5次/60s） |
| POST | `/api/auth/logout` | — | 登出 |
| GET | `/api/auth/me` | JWT | 当前用户 |

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
| GET | `/api/comments` | — | 评论列表 |
| POST | `/api/comments` | JWT | 创建评论（限流 20次/60s） |
| DELETE | `/api/comments/:id` | JWT | 删除评论 |

### 评分
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/ratings` | JWT | 提交/更新评分 |
| GET | `/api/ratings` | JWT | 我的评分 |
| GET | `/api/novels/:id/rating` | — | 作品评分统计 |

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
| GET | `/api/author/novels` | JWT | 我的作品 |
| GET | `/api/author/novels/:id/stats` | JWT | 作品统计 |
| GET | `/api/author/dashboard` | JWT | 作者仪表盘 |
| GET | `/api/author/profile/:id` | — | 作者公开主页 |
| GET | `/api/author/forum/:id` | — | 作者专属论坛 |
| GET | `/api/author/earnings` | JWT | 收益记录 |
| POST | `/api/author/withdraw` | JWT | 申请提现 |
| POST | `/api/author/apply-vip` | JWT | 申请 VIP |

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
| GET | `/api/admin/dashboard` | 管理员 | 数据大屏（7天趋势） |
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

---

## 🗺 开发路线图

- [x] **Phase 1 MVP** — 注册登录、作品 CRUD、多分类、章节读写（含哈希签名）、分类浏览、评论、多维度评分、论坛、导入预览/追加/导出、作者后台、管理后台
- [x] **Phase 1.5 安全与合规** — 敏感区隔离墙（3~5 步确认）、内容哈希 + 作者签名、章节完整性验证、举报系统、IP 黑名单、跨域评论限速框架
- [ ] **Phase 2 社区增强** — 段评/章评增强、作者互评系统、新书保护期、社区仲裁员选举
- [ ] **Phase 3 高级功能** — 付费阅读闭环、打赏 UI、EPUB 封面增强、反商业爬虫、自动备份
- [ ] **Phase 4 治理与扩展** — 财务公开面板、API 开放平台、移动端 PWA、公共差投票

---

## 📄 许可证

本项目采用 **[GNU Affero General Public License v3.0 (AGPL-3.0)](https://www.gnu.org/licenses/agpl-3.0.html)**。

这意味着：
- ✅ 你可以自由使用、修改和分发本项目
- ✅ 你可以用于商业用途
- ⚠️ **如果你修改了代码并通过网络提供服务，必须公开你的修改**（这是 AGPL 与 GPL 的关键区别）
- ⚠️ 分发本软件（包括修改版）时必须保留相同的 AGPL-3.0 许可证

> 选择 AGPL v3 是为了确保平台永远对社区开放。即使有人 fork 后作为 SaaS 服务运行，其修改也必须回馈社区。

---

<p align="center">
  <sub>一个 exe，8080 端口，永远开源 🌌</sub>
</p>
