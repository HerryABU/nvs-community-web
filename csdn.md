# NVS —— 从零构建一个去中心化网络小说平台（Go + Vue 3 全栈实战）

## 一、为什么要做这个项目

市面上的网络小说平台几乎被大厂垄断，作者分成低、版权归属模糊、推荐算法推的是"爽文"而非"好文"。作为一个喜欢类型文学（硬科幻、推演文学、架空历史）的读者和技术人，我想做一个**不一样的平台**：

- **作者友好**：作品属于作者，可随时一键导出迁移，平台仅抽成 10%
- **调性明确**：只做高质量类型文学，不设"爽文""水文"分类
- **技术开放**：对个人爬虫友好，小说以 HTML 明文存储于文件系统
- **零运维成本**：SQLite 单文件数据库，编译成一个 exe 双击即用

于是就有了 **NVS（Novel Verse Station）**。

---

## 二、技术架构一览

```
┌─────────────────────────────────────────────┐
│                   Nginx                      │
│         静态资源 (HTML/JSON/图片)              │
└──────────────────┬──────────────────────────┘
                   │ 反向代理 /api/*
┌──────────────────▼──────────────────────────┐
│              Go (Gin + GORM)                 │
│    认证 · 评论 · 评分 · 论坛 · 导入导出        │
│    ┌──────────────────────────────────┐      │
│    │  Vue 3 前端 (embed 嵌入二进制)     │      │
│    └──────────────────────────────────┘      │
└──────────┬──────────────┬───────────────────┘
           │              │
    ┌──────▼──────┐  ┌───▼──────────┐
    │  SQLite/MySQL│  │  文件系统     │
    │  用户/评论等  │  │  小说.html正文 │
    └─────────────┘  └──────────────┘
```

### 技术选型理由

| 选择 | 理由 |
|------|------|
| **Go (Gin)** | 编译为单文件、内存安全、并发性能好、交叉编译方便 |
| **GORM + SQLite** | SQLite 零配置，单文件数据库，个人/小团队部署不需要装 MySQL |
| **Vue 3 + Element Plus** | 组件库成熟，适合快速出 MVP；TypeScript 保证类型安全 |
| **文件系统存正文** | 数据库只存路径索引，读压力降低约 80%，"打包导出"天然支持 |
| **JWT Cookie (HttpOnly)** | 无状态鉴权，防 XSS 窃取，服务器不需要存 session |
| **embed 嵌入前端** | Go 1.16+ 的 `//go:embed` 将前端 dist 编译进二进制，真正单文件部署 |

---

## 三、目录结构

```
nvs/
├── nvs-server.exe            # 单文件运行（内嵌前端）
├── build.bat                 # 一键构建脚本
├── server/                   # Go 后端
│   ├── main.go               # 入口：DB、路由、SPA fallback
│   ├── embed.go              # //go:embed all:dist
│   ├── config/               # 环境变量管理
│   ├── models/               # 18 张数据表 + 查询方法
│   ├── handlers/             # API 处理器（9 个模块）
│   ├── middleware/            # JWT 鉴权 + IP 限流
│   └── utils/                # bcrypt、JWT、统一响应
├── web/                      # Vue 3 前端
│   └── src/
│       ├── views/            # 11 个页面组件
│       ├── components/       # NavBar / NovelCard / CommentSection
│       ├── api/              # axios 封装
│       ├── stores/           # Pinia 状态管理
│       └── router/           # Vue Router
└── 需求.txt                  # 465 行完整需求文档
```

---

## 四、核心功能详解

### 4.1 用户系统

- 邮箱注册/登录，密码 bcrypt 哈希
- JWT 存储在 `HttpOnly; SameSite=Lax` Cookie
- 五级角色：**游客 → 读者 → 作者 → VIP 作者 → 管理员**
- 登录失败 5 次锁定 15 分钟（中间件限流）

```go
// 登录接口的限流中间件
public.POST("/auth/login", middleware.RateLimit(5, 60), handlers.Login)
```

### 4.2 作品管理

9 大主分类，拒绝"爽文"标签：

> 硬科幻 | 奇幻 | 推演文学 | 架空历史 | 现实主义 | 悬疑推理 | 实验文学 | 同人区 | 其他

作品表设计：

```go
type Novel struct {
    ID              uint      `gorm:"primaryKey"`
    AuthorID        uint      `gorm:"not null;index"`
    Title           string    `gorm:"size:256;not null"`
    Category        string    `gorm:"size:64;index"`
    Tags            string    `gorm:"type:json"`
    PricePerChapter float64   // 作者自主定价
    Status          string    // draft / published
    TotalWords      int
    TotalChapters   int
    Author          *User     `gorm:"foreignKey:AuthorID"`
}
```

### 4.3 章节存储：文件系统 vs 数据库

这是整个架构中**最关键的决策**。章节正文不存数据库，而是以 HTML 文件存于文件系统：

```
/data/novels/authors/{author_id}/{novel_id}/
├── index.json       # 章节索引 + 元数据
├── 1.html           # 第一章正文
├── 2.html           # 第二章正文
└── cover.jpg        # 封面图
```

好处：
- 数据库只存路径，读压力降低 ~80%
- Nginx 可以直接服务静态 HTML，Go 后端无需参与
- "打包导出"天然支持：把目录打成 ZIP 即可
- 作者可以 FTP/WebDAV 直接管理文件

### 4.4 导入导出

**导入**：支持 TXT / Markdown 文件，正则解析章节边界：

```go
// 支持多种章节格式
var chapterPatterns = []*regexp.Regexp{
    regexp.MustCompile(`(?m)^第[零一二三四五六七八九十百千\d]+章\s*[^\n]*`),
    regexp.MustCompile(`(?m)^Chapter\s+\d+`),
    regexp.MustCompile(`(?m)^#{1,3}\s+.*`),
    regexp.MustCompile(`(?m)^-{3,}\s*$`),  // 分割线
}
```

**导出**：支持 ZIP（HTML + JSON）、Markdown、EPUB 三种格式，一键下载。

### 4.5 安全设计

| 层面 | 方案 |
|------|------|
| SQL 注入 | GORM 参数化查询 + 最小权限数据库账号 |
| XSS | 用户输入 `html.EscapeString`，作者正文 `bluemonday` 白名单净化 |
| 密码 | bcrypt (cost=10) |
| 图片上传 | `image.Decode` 解码后 `webp.Encode` 重编码，丢弃元数据 |
| 限流 | 基于 IP 的滑动窗口：登录 5次/60s、评论 20次/60s、列表 100次/60s |
| 鉴权 | JWT HttpOnly Cookie，无状态设计 |

### 4.6 管理后台

管理员面板包含：
- 📊 站点统计（用户数、作品数、评论数）
- 👥 用户管理（封禁/解封、角色变更）
- ✅ VIP 作者申请审批
- 🚨 举报处理（作品/评论/帖子举报）
- 💰 财务总览（平台抽成、作者收益）
- ⚙️ 站长配置（站点名称、VIP 开关等）
- 🌐 远程站点互通（类似 alist 的站点互联）

### 4.7 远程站点互通

模仿 alist 的"挂载其他存储"理念，NVS 支持站点互联：

```go
type FederatedSite struct {
    Name        string    // 远程站点名称
    URL         string    // 站点地址
    APIURL      string    // API 端点
    Status      string    // active / inactive
    LastSyncAt  *time.Time
    NovelCount  int
}
```

管理员添加远程站点后，系统自动同步对方平台的公开作品列表，读者可以在本站浏览和检索跨站作品。

---

## 五、部署方式

### 方式一：直接运行 exe（零配置）

```bash
# 下载 nvs-server.exe，双击或命令行运行
nvs-server.exe
# 浏览器打开 http://localhost:8080
```

无需安装 MySQL、Redis、Nginx——SQLite 自动创建，前端已内嵌。

### 方式二：Docker 一键启动

```bash
git clone https://github.com/your/nvs.git
cd nvs
cp .env.example .env
docker-compose up -d
```

启动 4 个容器：MySQL 8.0 + Redis 7 + Go 后端 + Nginx。

### 方式三：本地开发

```bash
# 后端
cd server && go run main.go

# 前端（另一个终端）
cd web && npm install && npm run dev
```

前端开发模式自动代理 API 到 `localhost:8080`，支持热重载。

### 构建

```bash
build.bat
# 自动执行：npm build → go build -ldflags="-s -w" → 输出 nvs-server.exe
```

---

## 六、API 一览

| 模块 | 方法 | 端点 | 认证 |
|------|------|------|------|
| 认证 | POST | `/api/auth/register` | 无 |
| | POST | `/api/auth/login` | 无 |
| | GET | `/api/auth/me` | JWT |
| 作品 | GET | `/api/novels` | 无 |
| | POST | `/api/novels` | 作者 |
| | GET | `/api/novels/:id` | 无 |
| 章节 | GET | `/api/novels/:id/chapters` | 无 |
| | GET | `/api/novels/:id/chapters/:num` | 无 |
| | POST | `/api/novels/:id/chapters` | 作者 |
| 评论 | GET | `/api/comments` | 无 |
| | POST | `/api/comments` | JWT |
| 评分 | POST | `/api/ratings` | JWT |
| | GET | `/api/novels/:id/rating` | 无 |
| 论坛 | GET | `/api/forums` | 无 |
| | POST | `/api/forums/:id/threads` | JWT |
| 导入导出 | POST | `/api/novels/import` | 作者 |
| | POST | `/api/novels/:id/export/epub` | 作者 |
| 管理 | GET | `/api/admin/stats` | 管理员 |

---

## 七、开发路线图

- [x] **Phase 1 MVP**：注册登录、作品发布、阅读、分类、评论、评分、作者后台、管理后台、导入导出
- [ ] **Phase 2 社区**：内容确认机制、段评/章评增强、论坛完善、作者互评、新书保护期
- [ ] **Phase 3 高级**：VIP 付费系统、打赏闭环、多格式导出（EPUB 增强）、反商业爬虫、不可见水印
- [ ] **Phase 4 治理**：仲裁员选举、财务公开、API 开放平台、移动端适配

---

## 八、写在最后

这个项目是我对"理想网络文学平台"的一次探索。技术上追求**极简部署**（一个 exe 搞定）和**开放存储**（文件系统即数据库），理念上坚持**作者主权**和**类型文学调性**。

如果你也是类型文学爱好者，或者对 Go + Vue 全栈项目感兴趣，欢迎 Star & PR。

> 默认管理员：`admin@nvs.local` / `admin123`（请登录后立即修改）

---

*项目地址：[GitHub](https://github.com/your/nvs) | 许可证：MIT*
