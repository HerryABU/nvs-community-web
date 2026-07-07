<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/Vue-3.4-4FC08D?style=flat&logo=vuedotjs" alt="Vue">
  <img src="https://img.shields.io/badge/SQLite-3-003B57?style=flat&logo=sqlite" alt="SQLite">
  <img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License">
  <img src="https://img.shields.io/badge/PRs-welcome-brightgreen.svg" alt="PRs">
</p>

<h1 align="center">📚 NVS — 网络小说平台</h1>

<p align="center">
  <strong>一个聚焦高质量类型文学的去中心化网络小说平台</strong><br>
  作者自主定价 · 版权归属作者 · 文件系统存储 · 单文件零配置部署
</p>

---

## ✨ 特性

- 🚀 **零配置部署** — 单 exe 文件双击即用，SQLite 自动创建，无需安装任何依赖
- 📝 **类型文学调性** — 9 大分类（硬科幻/推演文学/架空历史/悬疑推理…），拒绝套路文
- 👑 **作者友好** — 自主定价、平台仅抽成 10%、支持一键导出迁移（ZIP/Markdown/EPUB）
- 💬 **社区共治** — 子论坛+大论坛双轨、多维度评分、作者互评、社区仲裁
- 🔒 **安全底线** — JWT HttpOnly、bcrypt、XSS 白名单净化、IP 限流、图片重编码
- 📂 **文件系统存储** — 小说正文以 HTML 存于文件系统，Nginx 直出，数据库压力降低 80%
- 🌐 **站点互通** — 模仿 alist，支持远程站点互联与跨站作品同步

---

## 🛠 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.21+ + Gin + GORM |
| 前端 | Vue 3 + Vite + Element Plus + TypeScript |
| 数据库 | SQLite（默认） / MySQL 8.0 |
| 缓存 | Redis 7（可选） |
| 存储 | 文件系统（HTML + JSON） |
| 构建 | Go `embed` 前端 → 单二进制文件 |

---

## 📦 快速开始（用户）

### 直接运行（推荐，零配置）

```bash
nvs-server.exe
```

浏览器打开 **http://localhost:8080** 即可使用。SQLite 自动创建，前端已内嵌。

### Docker

```bash
git clone https://github.com/your/nvs.git
cd nvs
cp .env.example .env
docker-compose up -d
```

---

## 🔐 默认管理员

| 字段 | 值 |
|------|-----|
| 邮箱 | `admin@nvs.local` |
| 密码 | `admin123` |

> ⚠️ 登录后请立即修改密码！

---

## 🧑‍💻 源码开发与二次开发

### 环境要求

| 工具 | 版本 | 用途 |
|------|------|------|
| Go | ≥ 1.21 | 后端编译 |
| Node.js | ≥ 18 | 前端构建 |
| npm | ≥ 9 | 依赖管理 |

### 克隆后第一步

```bash
git clone https://github.com/your/nvs.git
cd nvs

# 安装前端依赖
cd web && npm install && cd ..

# 后端依赖会自动下载（首次 go build / go run 时）
```

---

### 项目结构详解

```
nvs/
│
├── nvs-server.exe                 # [构建产物] 单文件运行（内嵌前端 + 后端）
├── build.bat                      # [构建] 一键构建脚本（Windows）
├── .env.example                   # [配置] 环境变量模板
├── docker-compose.yml             # [部署] Docker 四容器编排
│
├── server/                        # ── Go 后端 ──
│   ├── main.go                    # ★ 入口：DB 连接 → AutoMigrate → 路由注册 → SPA fallback
│   ├── embed.go                   # ★ 关键：//go:embed all:dist  将前端构建产物嵌入二进制
│   ├── Dockerfile                 # 后端容器镜像
│   ├── init.sql                   # MySQL 手动建表脚本（GORM AutoMigrate 会自动建表）
│   ├── go.mod / go.sum            # Go 依赖
│   │
│   ├── config/
│   │   └── config.go              # 环境变量读取（.env / 系统环境变量）
│   │
│   ├── models/                    # 数据层（GORM 模型 + 查询方法）
│   │   ├── user.go                # User 模型 + CRUD 方法 + DB 全局实例
│   │   ├── novel.go               # Novel + Chapter + 统计方法
│   │   ├── comment.go             # Comment 模型
│   │   ├── forum.go               # Forum / Thread / Post 模型
│   │   ├── rating.go              # Rating 多维度评分模型
│   │   └── platform.go            # VipApplication / Report / WithdrawalRequest / EarningsRecord
│   │                              #   / PlatformConfig / FederatedSite / FederatedNovel / BlacklistIP
│   │
│   ├── handlers/                  # API 处理器（每个文件对应一组路由）
│   │   ├── auth.go                # 注册 / 登录 / 当前用户
│   │   ├── novel.go               # 作品 CRUD / 分类列表 / 列表查询
│   │   ├── chapter.go             # 章节创建 / 读取（文件系统 I/O）/ 更新 / 删除
│   │   ├── comment.go             # 评论创建 / 删除 / 列表
│   │   ├── rating.go              # 评分 upsert / 查询
│   │   ├── forum.go               # 论坛 / 帖子 / 回帖
│   │   ├── author.go              # 作者后台：我的作品 / 数据统计 / 收益 / 提现
│   │   ├── import_export.go       # TXT/Markdown 导入 → ZIP/Markdown/EPUB 导出
│   │   └── admin.go               # 管理后台：统计 / 用户管理 / VIP 审批 / 举报 / 财务 / 站点配置 / 远程互通
│   │
│   ├── middleware/
│   │   ├── auth.go                # JWT 鉴权中间件（从 Cookie 读 token）+ 角色检查
│   │   └── ratelimit.go           # 基于 IP 的内存滑动窗口限流
│   │
│   ├── utils/
│   │   ├── jwt.go                 # JWT 生成 / 解析
│   │   ├── password.go            # bcrypt 哈希 / 校验
│   │   ├── response.go            # 统一 JSON 响应格式
│   │   └── sanitize.go            # HTML 净化辅助
│   │
│   └── dist/                      # [自动生成] 前端构建产物（npm run build 输出，被 embed.go 嵌入）
│
├── web/                           # ── Vue 3 前端 ──
│   ├── vite.config.ts             # ★ Vite 配置：build 输出到 ../server/dist、dev 代理 /api
│   ├── package.json               # 依赖：vue / element-plus / pinia / vue-router / axios / v-md-editor
│   ├── index.html                 # SPA 入口 HTML
│   │
│   └── src/
│       ├── main.ts                # Vue 实例创建 + Element Plus + Router + Pinia
│       ├── App.vue                # 根组件（NavBar + router-view）
│       │
│       ├── router/
│       │   └── index.ts           # 路由表（12 个路由，含登录/注册/作品/阅读/论坛/作者/管理）
│       │
│       ├── stores/                # Pinia 状态管理（用户状态等）
│       ├── api/                   # axios 封装（按模块拆分 API 调用）
│       │
│       ├── views/                 # 页面组件（11 个）
│       │   ├── Home.vue           # 首页：作品列表 + 分类筛选
│       │   ├── Login.vue          # 登录页
│       │   ├── Register.vue       # 注册页
│       │   ├── NovelDetail.vue    # 作品详情：章节目录 + 评分 + 评论
│       │   ├── Reader.vue         # 阅读器
│       │   ├── Editor.vue         # 写作编辑器（富文本/Markdown 双模式）
│       │   ├── AuthorDashboard.vue # 作者后台
│       │   ├── AdminDashboard.vue  # 管理员面板
│       │   ├── Forums.vue         # 论坛广场
│       │   ├── ForumDetail.vue    # 论坛详情 + 帖子列表
│       │   └── ThreadDetail.vue   # 帖子详情 + 回帖
│       │
│       ├── components/            # 通用组件（4 个）
│       │   ├── NavBar.vue         # 顶部导航栏
│       │   ├── NovelCard.vue      # 作品卡片
│       │   ├── CommentSection.vue  # 评论区
│       │   └── StarRating.vue     # 星级评分
│       │
│       └── styles/                # 全局样式
│
├── nginx/
│   └── nginx.conf                 # Docker 模式下的 Nginx 配置（限流 + 反向代理 + 静态直出）
│
├── data/                          # [运行时生成] SQLite 数据库 + 小说文件 + 上传资源
│   ├── nvs.db                     # SQLite 数据库文件
│   ├── novels/                    # 小说 HTML 正文（文件系统存储）
│   └── uploads/                   # 用户上传（头像/封面等）
│
└── 需求.txt                       # 465 行完整需求文档
```

---

### 本地开发流程

```bash
# 终端 1：启动 Go 后端（监听 :8080）
cd server
go run main.go

# 终端 2：启动 Vue 前端（监听 :5173，API 自动代理到 8080）
cd web
npm run dev
```

浏览器打开 **http://localhost:5173** 进行前端开发（热重载）。

> **原理**：`vite.config.ts` 中配置了 `/api` 代理到 `localhost:8080`，因此前端开发时无需手动处理跨域。

---

### 编译构建

#### Windows（一键）

```bash
build.bat
```

脚本自动完成：`npm build`（前端） → `go build`（后端含前端嵌入，`-ldflags="-s -w"` 去除调试信息） → 输出 `nvs-server.exe`。

#### 手动分步（跨平台）

```bash
# 第一步：构建前端，产物输出到 server/dist/
cd web
npm run build

# 第二步：编译 Go，embed 自动嵌入 server/dist/
cd ../server
go build -ldflags="-s -w" -o nvs-server.exe .

# 产物：server/nvs-server.exe
```

#### 交叉编译

```bash
# 编译 Linux 版本
cd server
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o nvs-server .

# 编译 macOS 版本
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o nvs-server .
```

> **关于 `embed.go`**：Go 1.16+ 的 `//go:embed all:dist` 指令在编译时将 `server/dist/` 目录的全部内容嵌入到二进制文件中。因此运行时无需外部前端文件，真正单文件部署。**前端必须先构建（`npm run build`），否则编译失败。**

---

### 数据库切换：SQLite → MySQL

默认使用 SQLite，无需任何配置。如需切换到 MySQL：

1. 编辑 `.env` 文件（从 `.env.example` 复制）：

```bash
DB_DRIVER=mysql
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=nvs_user
DB_PASSWORD=your_password
DB_NAME=nvs
```

2. 启动 MySQL 并创建数据库：

```sql
CREATE DATABASE nvs CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

3. 重启服务 — GORM AutoMigrate 会自动建表。

> SQLite 模式不需要 Redis；MySQL 模式建议搭配 Redis（`REDIS_HOST` / `REDIS_PORT` 环境变量）。

---

### 环境变量一览

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
| `JWT_EXPIRE_HOURS` | `72` | Token 过期时间（小时） |
| `SERVER_PORT` | `8080` | 服务端口 |
| `NOVEL_DATA_DIR` | `./data/novels` | 小说文件存储目录 |
| `UPLOAD_DIR` | `./data/uploads` | 上传文件存储目录 |

> 环境变量通过 `os.LookupEnv` 读取，支持 `.env` 文件（Docker Compose）或系统环境变量。

---

### 架构关键设计决策

| 决策 | 说明 | 影响 |
|------|------|------|
| 章节正文存文件系统 | Novel.Chapter 表只存 `content_path` 路径，正文是 `data/novels/authors/{aid}/{nid}/{num}.html` | 数据库读压力降低 ~80%；导出即打包目录；Nginx 可直出静态文件 |
| `//go:embed` 嵌入前端 | `embed.go` 把 `dist/` 编译进二进制 | 真正单文件部署，无需 nginx 或 CDN |
| JWT HttpOnly Cookie | token 存在 Cookie 而非 localStorage | 防 XSS 窃取；每次请求自动携带 |
| GORM AutoMigrate | 启动时自动建表/加列 | 零 DBA 运维；切换环境无需手动执行 SQL |
| 9 大分类硬编码 | `Categories` 数组定义在 `handlers/novel.go` | 加分类需改代码（见下文"常见二开场景"） |
| 白名单 XSS（bluemonday） | 作者正文允许安全 HTML 标签；用户输入全部 `html.EscapeString` | 作者可排版，但无法注入恶意脚本 |

---

### 常见二开场景

#### 1. 添加新分类

编辑 `server/handlers/novel.go` 中的 `Categories` 切片：

```go
var Categories = []string{
    "硬科幻", "奇幻", "推演文学", "架空历史",
    "现实主义", "悬疑推理", "实验文学", "同人区",
    "新分类",   // ← 添加这行
    "其他",
}
```

前端筛选下拉框会自动读取 `/api/categories`，无需改前端。

#### 2. 添加新的 API 路由

在 `server/main.go` 中注册路由，在 `server/handlers/` 中新增处理器文件：

```go
// main.go — 公开路由
public.GET("/api/new-feature", handlers.NewFeature)

// main.go — 需认证路由
protected.POST("/api/new-feature", handlers.CreateNewFeature)

// handlers/new_feature.go
func NewFeature(c *gin.Context) { ... }
func CreateNewFeature(c *gin.Context) { ... }
```

#### 3. 添加新的数据表

在 `server/models/` 中新增模型文件：

```go
// models/new_feature.go
type NewFeature struct {
    ID   uint   `gorm:"primaryKey"`
    Name string `gorm:"size:128"`
}

func (NewFeature) TableName() string { return "new_features" }
```

然后在 `main.go` 的 `AutoMigrate` 调用中添加 `&models.NewFeature{}`：

```go
models.DB.AutoMigrate(
    // ... 现有模型 ...
    &models.NewFeature{},
)
```

#### 4. 添加新的前端页面

- 在 `web/src/views/` 中创建新 Vue 组件
- 在 `web/src/router/index.ts` 中注册路由
- 如需 API 调用，在 `web/src/api/` 中新增 axios 封装

#### 5. 定制前端样式

- 全局样式在 `web/src/styles/`
- Element Plus 主题变量可通过覆盖 CSS 变量修改
- 组件级样式使用 `<style scoped>` 避免污染

#### 6. 修改认证逻辑

JWT 相关代码集中在两个文件：
- `server/utils/jwt.go` — Token 生成 / 解析，Claims 结构体（包含 `UserID` / `Email` / `Role`）
- `server/middleware/auth.go` — Cookie 读写 / 鉴权中间件 / 角色检查

如需改用 Header Bearer Token（而非 Cookie），修改这两个文件即可。

---

### 代码规范

- **Go**：遵循标准 `gofmt` 格式，handler → model → utils 分层调用
- **Vue**：Composition API（`<script setup>`），TypeScript 类型标注
- **命名**：Go 用驼峰（`GetUserByEmail`），Vue 组件用 PascalCase（`NovelCard.vue`），API 用蛇形（`price_per_chapter`）
- **错误处理**：handler 层统一用 `utils.BadRequest` / `utils.InternalError` / `utils.Success` 等返回，不直接 `c.JSON`
- **SQL**：所有查询用 GORM 参数化，禁止字符串拼接 SQL

---

## 🔌 API 一览

| 模块 | 方法 | 端点 | 认证 |
|------|------|------|------|
| 认证 | POST | `/api/auth/register` | — |
| | POST | `/api/auth/login` | — |
| | GET | `/api/auth/me` | JWT |
| 作品 | GET | `/api/novels` | — |
| | POST | `/api/novels` | 作者 |
| | GET | `/api/novels/:id` | — |
| 章节 | GET | `/api/novels/:id/chapters` | — |
| | GET | `/api/novels/:id/chapters/:num` | — |
| | POST | `/api/novels/:id/chapters` | 作者 |
| 评论 | GET | `/api/comments` | — |
| | POST | `/api/comments` | JWT |
| 评分 | POST | `/api/ratings` | JWT |
| | GET | `/api/novels/:id/rating` | — |
| 论坛 | GET | `/api/forums` | — |
| | POST | `/api/forums/:id/threads` | JWT |
| 导入导出 | POST | `/api/novels/import` | 作者 |
| | POST | `/api/novels/:id/export/epub` | 作者 |
| | POST | `/api/novels/:id/export/markdown` | 作者 |
| 管理 | GET | `/api/admin/stats` | 管理员 |
| | GET | `/api/admin/users` | 管理员 |
| 远程 | GET | `/api/federated/novels` | — |

---

## 🗺 开发路线图

- [x] **Phase 1 MVP** — 注册登录、作品 CRUD、章节读写、分类、评论、评分、论坛、导入导出、管理后台
- [ ] **Phase 2 社区** — 内容确认弹窗、段评增强、作者互评、新书保护期
- [ ] **Phase 3 高级** — VIP 付费闭环、打赏、EPUB 增强、反爬虫、零宽水印
- [ ] **Phase 4 治理** — 仲裁员选举、财务公开、API 开放平台、移动端 PWA

---

## 📄 许可证

[MIT](LICENSE) © 2026

---

<p align="center">
  <sub>如果你也厌倦了套路文，欢迎加入 🌌</sub>
</p>
