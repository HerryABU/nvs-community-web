# 一个 exe 跑起整套小说平台：Go + Vue 3 单文件部署，做到国密双轨与 AI 内容标识

> 65 MB 一个文件，双击就能上线。没有 Nginx、没有 MySQL、没有 npm install。
> 这篇文章讲它怎么做到的，以及在做到「单文件」这条路上必须做对的几件事：
> 前端产物怎么嵌、路由怎么拆、AI 代码块怎么安全地跑起来、国密怎么和国际算法共存、蜜罐怎么不带自曝地骗人。

---

## 一、为什么做这个项目

市面上的网络小说平台几乎被大厂垄断，作者分成低、版权归属模糊、推荐算法推的是"爽文"而非"好文"。作为一个喜欢类型文学（硬科幻、推演文学、架空历史）的读者和技术人，我想做一个**不一样的平台**：

- **作者友好**：作品属于作者，可随时一键导出迁移
- **调性明确**：只做高质量类型文学，不设"爽文""水文"分类
- **技术开放**：小说正文以 HTML 明文存于文件系统，对个人研究友好
- **零运维成本**：SQLite 单文件数据库，编译成一个可执行文件双击即用

于是就有了 **NVS（Novel Verse Station）**。

项目从"能跑"演进到"敢给别人用"，中间踩的坑基本都集中在三处：**单文件打包**、**权限边界**、**AI 接进来之后的合规**。下面按这三条线展开。

---

## 二、技术架构

```
┌──────────────────────────────────────────────────────┐
│                 nvs-server.exe（单文件）               │
│                                                       │
│  ┌─────────────────────────────────────────────────┐  │
│  │  Go 1.22+ / Gin / GORM                          │  │
│  │  routes 路由装配 → handlers 接口 → models 数据层  │  │
│  │  领域子包：ai · aigc · aisafety · honeypot · ...  │  │
│  └─────────────────────────────────────────────────┘  │
│  ┌─────────────────────────────────────────────────┐  │
│  │  Vue 3 前端产物（//go:embed all:dist）            │  │
│  └─────────────────────────────────────────────────┘  │
└──────────────────┬───────────────────────────────────┘
                   │
        ┌──────────▼──────────┐
        │  SQLite（零配置）     │
        │  或 MySQL（可选）     │
        └──────────┬──────────┘
                   │
        ┌──────────▼──────────┐
        │  文件系统             │
        │  章节 HTML / 封面 /   │
        │  上传附件            │
        └─────────────────────┘
```

### 技术选型理由

| 选择 | 理由 |
|------|------|
| **Go + Gin** | 编译成单文件、内存安全、交叉编译方便（x86/ARM/MIPS64/RISC-V 全覆盖） |
| **GORM + glebarez/sqlite** | 纯 Go 的 SQLite 驱动，**不需要 cgo** —— 这是"一个 exe 跨全平台"的前提 |
| **Vue 3 + Element Plus** | 组件库成熟，`<script setup>` + TS 写页面很快 |
| **文件系统存正文** | 数据库只存索引路径，读压力大幅降低，"打包导出"天然支持 |
| **JWT Cookie（HttpOnly）** | 无状态鉴权，服务器不需要存 session |
| **`//go:embed`** | 前端产物编译进二进制，真正做到单文件分发 |

前端重型依赖都是按需分包懒加载的：`cherry-markdown`（Markdown 渲染）、`echarts` + `vue-echarts`（图表）、`katex`（公式）、`mermaid`（流程图）、`three` / `3d-force-graph`（评论关系网）、`highlight.js`（代码高亮）、`@tiptap/*`（富文本编辑器）。

---

## 三、目录结构

重构之后的样子（此前 `main.go` 1852 行、`handlers/` 平铺 155 个文件、`views/` 顶层平铺 52 个页面，找文件基本靠搜索）：

```
nvs/
├── server/                        # Go 后端
│   ├── main.go                    # 501 行：初始化 → 装配 → 启动
│   ├── embed.go                   # //go:embed all:dist
│   ├── routes/                    # 路由装配（按分组分文件）
│   │   ├── routes.go              #   Register：中间件链 + 依次调用下面五个
│   │   ├── public.go              #   /api 公开组
│   │   ├── protected.go           #   /api 登录组
│   │   ├── admin.go               #   /api/admin 后台组
│   │   ├── police.go              #   /api/police 取管台
│   │   ├── spa.go                 #   静态资源 + NoRoute（SPA 回退）
│   │   └── guards.go              #   建站向导守卫
│   ├── handlers/                  # 接口（根 133 个文件 + 子包）
│   │   ├── export.go              #   子包桥接：导出横切函数给子包用
│   │   ├── context.go             #   鉴权上下文统一封装
│   │   ├── ai/                    #   AI 域（对话流式/绘画/视频/智能体/模型池/安全闸）
│   │   └── group/ honeypot/ oc/ …
│   ├── models/                    # 数据层（133 个文件 / 120+ 结构体）
│   ├── aigc/ aisafety/ sentiment/ # AIGC 标识 / 三层安全闸 / 舆情
│   ├── honeypot/ gate/ risk/      # 蜜罐 / 准入闸门 / 充值风控
│   ├── routes 之外还有：crypto/（国密）· audit/ · ipgeo/ · doctext/ …
│   └── dist/                      # [自动生成] 前端产物，供 embed
│
├── web/                           # Vue 3 前端
│   └── src/
│       ├── views/                 # 页面（顶层 0 文件，全按业务域归入 15 个子目录）
│       │   ├── ai/ auth/ novel/ blog/ forum/ author/ home/ payment/
│       │   ├── me/ site/ comment/ user/ group/ frame/
│       │   └── admin/             #   后台：site / security / content / ai / business / police
│       ├── components/            # NavBar / CommentSection / ai/ · admin/ · editor/ …
│       ├── markdown/              # Cherry 渲染 + DOMPurify 白名单 + 代码块增强
│       └── styles/                # 设计令牌 / 间距档位 / 代码高亮配色
│
├── docs/                          # CHANGELOG · FEATURES · API · CONFIG · ARCHITECTURE · DEVELOPMENT
└── go.gm.mod                      # 国密轨独立依赖清单（默认 go.mod 零国密依赖）
```

---

## 四、核心实现

### 4.1 「一个 exe」是怎么做到的

关键只有一行指令：

```go
//go:embed all:dist
var frontendFS embed.FS
```

但这一行有两个硬约束，直接决定了整个工程的目录形状：

1. **`//go:embed` 的路径相对「源文件所在目录」，且不允许 `..`**。所以 `embed.go` 必须待在 `server/` 根（与 `dist` 同级），也只能属于 `package main`。
2. 前端必须先构建，否则编译直接失败。CI 里顺序反了就嵌进去一个空目录。

于是「路由包拿不到前端 FS」这件事只能靠注入解决：

```go
// main.go
func main() {
	// 前端资源来自 //go:embed，仅 main 包可见，注入给 routes 使用
	routes.FrontendFS = getFrontendFS
	routes.NodistBuild = nodistBuild

	routes.Register(r)
}
```

`server/dist` 被 `.gitignore` 忽略，产物靠每次构建生成 —— 这也是为什么"改了前端但 exe 没变"这种问题，绝大多数情况是忘了重新 `go build`。

**另外留了一条业务轨**：`-tags nodist` 时嵌入一个提示页，不依赖前端产物就能编译和跑测试。好处是改后端逻辑时不用等 1 分多钟的前端构建。

### 4.2 路由装配：从 1852 行到 501 行

改造前的 `main.go` 里近 1300 行都是 `r.POST("/api/xxx", handlers.Xxx)`。改一个接口要在上千行里翻，而且 `main` 的职责被淹没在细节里。

搬出去之后，`main` 只剩装配：

```go
gin.SetMode(gin.ReleaseMode)
r := gin.New()

routes.FrontendFS = getFrontendFS
routes.NodistBuild = nodistBuild
routes.Register(r)
```

`routes.Register` 里保持原来的注册顺序，按分组拆成 7 个文件（最大 492 行）。这里有两个容易踩的点：

**第一，`r.Group` 建子组之后会回到父组继续注册。** 我第一版切分时把 `port` / `bookshelf` 两个 `r.Group` 当作独立段切开，结果后面继续注册在 `protected` 上的路由全跑到了错误的段里 —— 编译能过，是运行期 `gin` 的「同层静态段与通配段冲突」panic 才暴露的。

**第二，`frontendFS` 被 `r.NoRoute` 的闭包捕获。** 静态资源准备和 `NoRoute` 必须在同一个函数里，否则闭包拿不到变量。

顺带说下 SPA 回退，这块我加了白名单而不是"兜底全回退"：

```go
// knownSPARoots 前端 Vue Router 根段白名单（首段路径）。
// 用于严格 SPA 回退：仅这些根段才回退到 index.html，其余（代理前缀、数字前缀、
// 以及任意未知前缀）一律 404，杜绝前缀拼接造成的组件穿越。
var knownSPARoots = map[string]bool{
	"admin": true, "ai": true, "novel": true, "plaza": true, …
}
```

代价是**新增页面必须登记**，否则直接访问/刷新返回 404 —— 而点链接进去是正常的，属于最难自查的一类回归。所以 CI 里有一条 `scripts/check-spa-roots.sh` 做前后端对撞。

### 4.3 handlers 拆子包：两个不用改调用点的技巧

`handlers` 是单一大包，155 个文件平铺。想按业务域拆目录，在 Go 里立刻撞上一个问题：**目录即包，跨包必须导出**，而这些横切函数（`currentUserID`、`ensureAdmin`）被 100+ 处调用。

改一百个调用点风险不小，于是用「导出别名」：

```go
// handlers/export.go —— 子包桥接层
// 原函数名、原调用点一行都不用动，子包通过 handlers.CurrentUserID(...) 调用。
var (
	CurrentUserID   = currentUserID
	CurrentUserRole = currentUserRole
	EnsureAdmin     = ensureAdmin
	AtoiDefault     = atoiDefault
)
```

反过来的依赖（子包里的函数被主包调用）会形成循环 import，用回调变量解耦：

```go
// handlers 侧：默认空实现，语义等于「没有该功能」
var MaybeTriggerAiBot = func(c *gin.Context, comment *models.Comment) {}

// 子包侧：init 时把真实实现注册进来
func init() { handlers.MaybeTriggerAiBot = maybeTriggerAiBot }
```

一行调用点没改，ai 域 36 个文件（含 5 个测试文件）就搬进了 `handlers/ai`。

### 4.4 AI 工作台：模型池 + 渠道路由

AI 部分没有直接把厂商 SDK 缝进来，而是做了一层「协议适配 + 模型池」：

- **多厂商协议**：OpenAI 兼容 / Anthropic / Gemini / Midjourney / Kling / Coze，各自一个 client
- **模型池**：一个逻辑模型可以挂多条渠道（多个 key / 多个中转），失败熔断 + 轮询 + 按场景路由（最便宜 / 最快）
- **BYOK**：用户可以带自己的 key，走自己的额度
- **渠道路由有个真实成本问题**：中转站的计费口径和官方不一样，所以每次请求同时记「虚拟代币」和「真实成本」两个口径，后台能看到真实花了多少钱

拉模型这块踩过一个坑：**很多中转站的站点根 `/models` 返回的是 SPA 的 HTML**，不是 JSON。所以判定必须看内容而不是状态码：

```go
// 候选 /models → 回退 /v1/models；JSON 里没有 data 字段一律视为失败
```

### 4.5 AI 代码块：高亮、复制、沙箱预览、一键发布

这是用户提到最多的一块。四个能力，每个都有一处必须做对的地方。

**高亮**：改用 `highlight.js`（common 构建，36 种语言），独立 chunk 懒加载。为什么不直接用 Markdown 库里自带的高亮 —— 它打包的 Prism 只有 markup/css/clike/javascript 四种，Python 和 Go 全被当 JS 高亮；而且配色选择器被 `div[data-type=codeBlock]` 限定，而 AI 输出走 DOMPurify 白名单（`ALLOW_DATA_ATTR: false`）会把 `data-*` 全剔除，于是 token class 在、颜色全丢 —— 等于没有高亮。

**复制**：代码原文从 `pre > code` 现取，绝不塞进 `data-*` 属性。否则按钮文字（"复制"、"预览"）会被一起复制走。

**预览**：空源 `sandbox` iframe，`allow-scripts` 但**刻意不给** `allow-same-origin`：

```html
<iframe sandbox="allow-scripts allow-popups allow-forms allow-modals" referrerpolicy="no-referrer" />
```

不给 `allow-same-origin` 意味着 iframe 读不到本站 Cookie / localStorage，也不能跳转或改写父页面。代价是 AI 生成的小游戏一碰 `localStorage` 就抛 `SecurityError` 整页白屏 —— 所以往沙箱里注入了一份**内存版 Storage**，让"刷新即丢"代替"直接崩"。

还有个细节：**代码块没写语言标注时也要能识别 HTML**。Markdown 引擎会把没写语言的块强行标成 `language-javascript`，但它同时在包裹层留了原始的 `data-lang`（空串 = 作者确实没写）。判据必须是「`data-lang` 为空才允许按内容猜 HTML」—— 我第一版写成「属性不存在才猜」，结果裸 HTML 块永远拿不到预览入口。

**发布**：把这段 HTML 打成 STORE 模式的单文件 ZIP，调平台既有的上传接口：

```ts
// web/src/utils/miniZip.ts —— 约 80 行，零依赖
// 表驱动 CRC32 + local file header + central directory + EOCD
const blob = createZip([{ name: 'index.html', data: html }]);
```

为什么是 STORE（不压缩）：压缩率检查、解压炸弹防护这些预检逻辑本来就在后端；前端只要给出结构合法的 ZIP，预检、封存、额度、审核整条链路一行后端代码都不用改。**实测 Go 的 `archive/zip` 能正确解析产物**（UTF-8 flag、中文文件名、`&<>` 特殊字符全无损）。

### 4.6 AIGC 内容标识（GB 45438—2025）

生成式内容要能被识别，标准里要求两部分：**显式标识**（人能看见）和**隐式标识**（机器能读）。

显式标识是烧进图片像素的角标，字号按短边比例算（国标要求不少于短边的 5%）：

```go
shortSide := math.Min(float64(w), float64(h))
pct := cfg.FontSizePct
if pct < 0.05 {
	pct = 0.05 // 国标下限，配置层已兜一次，这里再兜一次
}
fontSize := math.Max(14, math.Round(shortSide*pct))
```

隐式标识写进图片元数据（XMP），七个字段：生成标识、内容生产者、内容制作编号、内容传播平台、生成服务提供者、生成模型、生成时间。这里没有引第三方元数据库（依赖要求 Go 1.27+，当时环境不满足），改成自己按 JPEG APP1 / PNG iTXt / WebP XMP chunk 注入 —— 反而更可控。

导出管道是**参数进、二进制出**：裁剪 → 缩放 → 烧角标 → 编码 → 写 XMP，全程内存流，不落临时文件。

### 4.7 安全：设备指纹绑定与令牌加密密码

**访问令牌与「设备指纹 + 登录地」强绑定**。前端每个请求带 `X-Device-Fingerprint`，后端校验令牌里的绑定签名是否匹配：

```go
const DeviceFingerprintHeader = "X-Device-Fingerprint"

// 当前设备指纹 + 登录地与令牌签发时一致 → 放行；
// 登录地解析失败（geo 抖动）时，放宽到「仅比对设备指纹」以降低误伤。
if utils.ComputeBinding(fp, region) == claims.Binding { … }
```

这个设计带来的**最隐蔽的一个 bug**：AI 对话为了读 SSE 用了原生 `fetch`，绕过了 axios 拦截器，因此没带这个头 —— 表现是所有普通接口都正常，只有 AI 对话 401「登录环境已变更」，而且只有真开了指纹绑定才暴露。凡是绕过 axios 的请求，都要手动补齐三个头：`X-Requested-With`、`X-Device-Fingerprint`、`credentials: 'include'`。

**上游 API Key 落库加密的密钥由站长自定义密码派生**（PBKDF2-HMAC-SHA256，随机盐、20 万轮），存 `data/token-vault.key`（0600），与 JWT 密钥、蜜罐密钥三者互不复用。换密码时全部 Key 按"全有或全无"迁移：先全部解密成功再统一写回，绝不半新半旧把模型搞挂。

### 4.8 蜜罐：诱饵最大的失败是自曝

这一节是踩坑踩出来的。蜜罐会伪造一批"泄露"文件骗扫描器，比如假的 `.env`。第一版长这样：

```ini
# jwt（泄漏示例 / 已轮换）      ← 直接告诉他这是假的
JWT_SECRET=f854e80e...
CANARY=NVS-CANARY-F3F4D64300C1   ← 变量名和值都在自曝
```

攻击者扫到 `.env` 第一眼就识破，后面所有取证白做（他会换 IP 重来）。改后：

```ini
APP_NAME="NVS"
APP_ENV=production
APP_URL=https://app.nvs-prod.cn

APP_KEY=base64:e634a1fb1184e66d9add7f777fcd8be5c46e479a474cf20f
JWT_SECRET=9c2f...
DB_HOST=10.0.12.31
DB_PASSWORD=7a3e...
# 运维备注：这台的密钥 3 月轮换过一次，脚本在 /opt/ops/rotate.sh
```

金丝雀（溯源值）伪装成一份普通密钥，变量名与注释都不提它：

```go
func newCanary(style string) string {
	if strings.EqualFold(style, "labeled") {   // 便于全文搜索自查，代价是更易识破
		return "NVS-CANARY-" + strings.ToUpper(randHex(6))
	}
	return "base64:" + randHex(24)             // 默认：伪装
}
```

三个必须记住的原则：

1. **不能出现任何自曝标记** —— `CANARY` / `HONEY` / `DECOY` / `蜜罐` / `示例` / `已轮换` 全禁。测试里有反向断言守着，命中任一个就失败。
2. **金丝雀必须持久化**。原来每次进程启动重新随机，重启后值就变，站长拿它去外部比对永远对不上。现在存库，重启不变。
3. **同一次运行内内容必须稳定**。刷新两次内容就变 = 自己招供。生成逻辑放在注册路由时执行一次、闭包里捕获，绝不能放进 handler。

诱饵内容全部可在后台配（站点名、假域名、内网地址、运维备注、金丝雀挂载点与风格），保存走**独立接口**：主配置保存是全量覆盖语义，只提交诱饵字段会把「启用」开关顺手清成 false —— 站长会以为"改个假域名，蜜罐怎么自己关了"。

### 4.9 国密双轨：不是二选一，是同时支持

很多项目把国密做成「换一套编译」，结果是开了国密就只能国密。NVS 是三档：

| 轨道 | 含义 | 适用 |
|---|---|---|
| `intl`（默认） | 标准 TLS | 普通站点，**零国密依赖**，`go version -m` 可验 |
| `gm` | 国密单轨（SM2 双证书 + SM3 + SM4） | 需要合规的纯内网环境 |
| `dual` | **同一端口按 ClientHello 自动分流** | 既要国密客户端，也要普通浏览器能进 |

`dual` 才是"双轨"的实际含义：国密客户端走 SM2 握手，普通浏览器走标准 TLS，同一端口两边都能进。

实现上默认构建完全不引任何国密包，靠 build tag 隔离：

```
crypto/gm_disabled.go   //go:build !gm    —— 空实现，说明怎么开启
crypto/gm_purego.go     //go:build gm     —— 纯 Go 实现，不需要 cgo
crypto/gm_tongsuo.go    //go:build gm && tongsuo —— 走铜锁
```

纯 Go 那档很关键：**不需要 cgo、不需要装铜锁，交叉编译照样能出单文件**，这让"国密版"和"普通版"是同一种交付形态。

国密轨有独立的依赖清单 `go.gm.mod`（默认 `go.mod` 保持零国密依赖）。这里踩过一个必须记的坑：`go.gm.mod` 会随主干依赖变动而落后，某次新功能引入了三个新包，国密轨直接编译失败 —— 而默认轨一切正常，`make test` 也全绿，**只有真的去构建国密版才发现它已经废了**。CI 里必须有一条专门构建 gm 轨的任务。

---

## 五、部署方式

### 方式一：直接运行（推荐）

```bash
# 下载 nvs-server.exe（或对应平台二进制），双击或命令行运行
./nvs-server
# 浏览器打开 http://localhost:8080，按建站向导走完
```

不需要 MySQL、Redis、Nginx —— SQLite 自动创建，前端已内嵌。首次启动自动生成 `.env`，按需改完重启即可。

### 方式二：Docker

```bash
git clone https://github.com/HerryABU/nvs-community-web.git
cd nvs-community-web
docker compose up -d
```

### 方式三：本地开发

```bash
# 后端（默认 8080，首次启动自动生成 .env）
cd server && go run .

# 前端（另一个终端，vite 已代理 /api）
cd web && npm install && npm run dev
```

### 构建

```bash
make dist                                  # 前端 → dist/
cd server && go build -trimpath -buildvcs=false -ldflags "-s -w" -o nvs-server .
```

体积这块有几个实测数据（Windows amd64，含内嵌前端）：

| 项 | 前 | 后 | 做法 |
|---|---|---|---|
| 默认轨 exe | 73.6 MB | **65.1 MB** | 不再嵌 `.gz`/`.br` 副本 |
| 国密轨 exe | 89.9 MB | **72.0 MB** | 补上 `-s -w`（此前完全没加） |

最大的浪费点很反直觉：`vite-plugin-compression2` 会给每个 js/css 再生成 gzip 和 brotli 副本，共 642 个文件 / 8.5 MB。但本项目是单 exe 部署，压缩由 `gin-contrib/gzip` 在响应时实时完成 —— 这些副本**永远不会被读到**，而 `go:embed` 照单全收。只有「Nginx / CDN 直接服务 dist 目录」的部署才需要它们，所以做成可选开关而不是默认。

没上 UPX：能再压一半，但拖慢冷启动，而且极易触发 SmartScreen 与杀软误报 —— 单文件分发的体验损失大于收益。

---

## 六、API 概览

| 模块 | 方法 | 端点 | 认证 |
|------|------|------|------|
| 认证 | POST | `/api/auth/register` · `/api/auth/login` | 无 |
| | GET | `/api/auth/me` | JWT |
| 作品 | GET | `/api/novels` · `/api/novels/:id` | 无 |
| | POST | `/api/novels` | 作者 |
| 章节 | GET | `/api/novels/:id/chapters/:num` | 无 |
| | POST | `/api/novels/:id/chapters` | 作者 |
| 评论 | GET/POST | `/api/comments` | POST 需 JWT |
| 论坛 | GET | `/api/forums` · `/api/threads/:id` | 无 |
| AI | POST | `/api/ai/chat/stream`（SSE） | JWT |
| | POST | `/api/ai/image` · `/api/ai/video` | JWT |
| | POST | `/api/ai/aigc/export`（参数进、图片出） | JWT |
| 上传 | POST | `/api/userhtmls/upload`（单文件 HTML 应用） | JWT |
| 后台 | GET | `/api/admin/crypto/status` · `/api/admin/honeypot/config` | 管理员 |
| 健康 | GET | `/api/health` | 无 |

完整接口表（含请求体与权限矩阵）见仓库 `docs/API.md`。

---

## 七、踩过的坑（要点总结）

1. **`//go:embed` 不允许 `..`** —— 它只能待在 `package main` 且与目标目录同级。任何"把 embed 逻辑挪进子包"的想法都会撞墙，正确做法是注入。
2. **移动源文件会打断相对路径** —— 目录重构时 `./police/xxx.vue` 变成 `./PoliceFoo.vue`，构建前必须跑一次全量相对引用扫描。
3. **Go 里「拆目录」= 「拆包」** —— 跨文件调用的未导出函数必须导出化。用「导出别名 + 回调变量」可以做到零调用点改动。
4. **绕过 axios 的 fetch 必须手动补齐拦截器那三个头** —— 否则指纹绑定一开，流式接口静默 401。
5. **全量覆盖语义的配置接口是陷阱** —— 只提交部分字段时，请求体里没带的 bool 会被 bind 成 false。要么改部分更新语义，要么拆独立接口。
6. **GORM `Create` 会跳过零值** —— `labeled` 这种本该是 `false` 的字段会被列默认值 `true` 覆盖，等于伪造审计数据。写库后要用 map 回填再校验。
7. **前端容器不可见时（`display:none`）图表库不会建 canvas** —— ECharts 在隐藏 tab 里挂载拿到的是 0×0，切回来还是空白。
8. **Markdown 白名单与代码高亮天然冲突** —— 高亮的配色选择器依赖 `data-*` 或特定 class，白名单一剔就全丢。要么放行必要属性，要么渲染后自己重打一遍。

---

## 八、写在最后

这个项目是我对"理想网络文学平台"的一次探索。技术上追求**极简部署**（一个 exe 搞定）和**开放存储**（文件系统即数据库），理念上坚持**作者主权**和**类型文学调性**。

安全与合规部分不是"加个开关"就完事 —— 访问令牌绑定、令牌加密密码、AI 三层安全闸、AIGC 内容标识、国密双轨、蜜罐诱饵，每一项都要想清楚"被绕过时会发生什么"。这部分代码占了后端相当比例，也是我认为最值得读的部分。

如果你也是类型文学爱好者，或者对 Go + Vue 全栈 + 单文件部署感兴趣，欢迎 Star & PR。

> 项目地址：<https://github.com/HerryABU/nvs-community-web>
> 许可证：**AGPL-3.0**（强 copyleft，衍生作品需同样开源）
> 文档：`docs/` 下有 FEATURES（功能细节）、API、CONFIG、ARCHITECTURE、DEVELOPMENT（构建与发版约定）、CHANGELOG
