# NVS 平台安全审计报告

> 审计日期：2026-08-24　审计范围：server/（Go 后端）与 web/src（Vue 3 前端）全量代码
> 审计方法：自动化模式扫描（SQL 注入 / 命令执行 / SSRF / 路径穿越 / XSS / 越权 / 信息泄露）
> + 高危文件逐行精读（middleware、wasm.go、userhtml.go、upload.go、auth*、db_admin、setup 等）
> + 双代理并行交叉验证。
> 关联法规：公安部令第176号《公安机关网络空间安全监督检查办法》。

## 一、已修复的高危问题（本次审计中直接修复）

### 🔴 S1. JWT 签名密钥为公开默认值 → 任意伪造管理员 Token【已修复】

- **位置**：`config/config.go`（默认 `change-me-in-production`）、`utils/jwt.go:38`
- **风险**：默认密钥公开，攻击者可伪造 `{user_id:1, role:"admin"}` 的 Token，直接接管全部管理员接口。
- **修复**：`config/secure.go` 新增 `GetOrCreateJWTSecret()`，`config.Init()` 检测到默认密钥时自动生成
  随机 64 位 hex 密钥并持久化到 `data/.jwt_secret`（0600 权限）；用户自定义 `JWT_SECRET` 不受影响。

### 🔴 S2. UserHTML entry_file 无路径校验 → 任意文件读取（LFI）【已修复】

- **位置**：`handlers/userhtml.go:256`、`handlers/wasm.go:132/161/186`
- **风险**：`entry_file` 可被改为 `../../.env` 等，`GetHTMLPreview`/`ServeAppHTML` 直接
  `filepath.Join(ExtractDir, EntryFile)` + `os.ReadFile` 回显 → 读取服务器任意文件
  （可进而读取 `data/.secret.key` 解密数据库密码、`log/` 访问日志批量窃取会话）。
- **修复**：
  - 新增 `pathWithin(base, target)` 强校验（`filepath.Rel` 结果不得以 `..` 开头），应用到全部 4 处读取点；
  - `UploadZipHTML`/`UpdateHTML` 保存端校验 `entry_file`：拒绝绝对路径、`..`、`../`。

### 🟠 H1. 沙盒路径前缀匹配穿越（跨用户读取）【已修复】

- **位置**：`handlers/wasm.go:51`、`wasm.go:189`（`strings.HasPrefix(absPath, absBase)` 无路径边界）
- **风险**：用户 1 可访问用户 12/120 的私有沙盒文件（前缀匹配 `/user1` ⊂ `/user12`）。
- **修复**：统一改用 `pathWithin`（`filepath.Rel` 边界校验）。

### 🟡 M3. 通用上传允许 SVG → 存储型 XSS【已修复】

- **位置**：`handlers/upload.go:36/43`
- **风险**：SVG 可内嵌 `<script>`，经 `/uploads` 同源直出 → 盗用受害者会话。
- **修复**：移除 `.svg` 扩展名白名单（图片仅 png/jpg/jpeg/gif/webp/bmp）。

### 🟠 前端沙盒 iframe allow-scripts+allow-same-origin → 沙盒逃逸【已修复】

- **位置**：`web/src/views/frame/HtmlRunner.vue:46`、`SandboxPreview.vue:47`、`Reader.vue:53/140`、
  `HTMLManager.vue:116`、`FrameManager.vue:125/169`、`WasmRunner.vue:25`、`AdminEmergency.vue:93/112`、
  `AuthorTemplateSettings.vue:77`、`DocsPage.vue`；后端 `wasm.go` 的 policy 展示文本与
  `GetSandboxInfo` 返回。
- **风险**：同源组合下，用户上传脚本可 `window.parent` 读写父页面 DOM/localStorage，
  并以受害者登录态调 `/api/*`。
- **修复**：全部 iframe sandbox 移除 `allow-same-origin`（用户内容一律 opaque origin）；
  后端展示文本同步更新。

### 🟠 mermaid securityLevel:'loose' → 全站存储型 XSS【已修复】

- **位置**：`web/src/markdown/renderer.ts:378`
- **风险**：loose 不转义用户 HTML（`%%{init:{"htmlLabels":true}}%%` 配合 `<img onerror>`，
  同 CVE-2026-23630），章节/博客/评论中的 mermaid 代码块可执行任意 JS。
- **修复**：改为 `securityLevel: 'strict'`。

### 🟡 M1. 登录无账号锁定 → 暴力破解【已修复】

- **位置**：`handlers/auth_login.go`（`models/user.go:29-30` 字段早已定义未启用）
- **修复**：实现账号级失败锁定——连续 5 次密码错误锁定 15 分钟，登录成功清零。

### 🟡 M2. 注册无限流 → 批量灌号【已修复】

- **位置**：`server/main.go:187`
- **修复**：注册路由挂 `security.RateLimit(10, 60)`。

### 🟢 L3. JWT Cookie 无 Secure 标志【已修复（可配置）】

- **位置**：`middleware/auth.go:77/91`
- **修复**：新增 `COOKIE_SECURE` 环境变量（默认 false 兼容本地 HTTP，生产 HTTPS 下设为 true）。

## 二、第二轮修复（2026-08-24 补充：原「待下一迭代」项全部完成）

| 编号 | 级别 | 问题 | 修复方式 |
|------|------|------|---------|
| M4 | 中危 | 沙盒 VFS 无鉴权（仅凭 htmlId 读写） | `sandbox_vfs.go`：VFSWrite/VFSDelete 增加登录 + `requireOwnedHTML`（htmlId 归属校验），未登录 401 / 非归属 403；read/list 保持公开（沙盒应用匿名渲染必需） |
| M5 | 中危 | 联邦同步 SSRF | `admin_site.go`：新增 `isSafeFederatedURL`（仅 http/https + 拒绝回环/内网/链路本地/CGNAT，DNS 解析校验），`http.Client{Timeout:10s}` |
| L1 | 低危 | /api/health 信息泄露 | `health.go`：公开响应收敛——移除 DB 驱动、原始错误详情、用户/作品/论坛计数、bundle 名 |
| L2 | 低危 | DB 审计日志记录完整 SQL 绑定值 | `gorm_logger.go`：`parameterizeSQL` 把字符串绑定值替换为 `?`（密码哈希/邮箱/Token/正文不落盘） |
| L4 | 低危 | 端口分配 IDOR | `port.go`：AllocatePort/ReleasePort 更新前校验 html 归属（`id=? AND user_id=?`） |
| L5 | 低危 | 隐藏论坛元数据泄露 | `forum.go`：未解锁时仅返回最小信息（id/type/locked），不再返回名称/描述；支持 X-Forum-Room/X-Forum-Password 头 |
| — | 中危 | 无 CSRF token | 新增 `middleware/csrf.go` CSRFGuard：登录态写操作强制 X-Requested-With 头；前端 axios 拦截器统一附加（`web/src/api/index.ts`）+ 单元测试 |
| — | 中危 | 明文 HTTP / 缺安全头 | `nginx/nginx.conf`：新增 CSP/Referrer-Policy 安全头 + HTTPS 443 配置示例（HSTS 注释待启用） |
| — | 低危 | 论坛密码/管理 token 进 URL | 论坛：密码/房间号改存 sessionStorage + `X-Forum-Password`/`X-Forum-Room` 头（Forums.vue/ForumDetail.vue/forums.ts/forum.go）；应急：preview/delete 改 POST body（admin.ts/AdminEmergency.vue/emergency.go/main.go） |

## 三、扫描结论（未发现问题的领域）

- **SQL 注入**：`models/` 全部 `Where/Order/Raw` 为参数化或白名单，未发现可注入点。
- **命令执行**：`exec.Command` 仅用于 Windows 防火墙配置，无用户输入注入。
- **ZIP 解压**：`userhtml.go` 已有 zip-slip 防护、压缩比/总大小限制、解压后目录只读锁定。
- **图片重编码**：作者头像/图片上传走重编码防马流程。
- **管理员路由鉴权**：抽查全部 admin handler 均调用 `ensureAdmin`。
- **XSS 规则库/违禁词库**：动态规则 + 内容消毒机制完善。

## 四、修复优先级总结

1. ~~JWT 密钥随机化~~（S1，已修复）
2. ~~entry_file 路径穿越 LFI~~（S2，已修复）
3. ~~沙盒路径边界校验~~（H1，已修复）
4. ~~沙盒 iframe 同源逃逸 + mermaid loose XSS~~（已修复）
5. ~~登录/注册反滥用~~（M1/M2，已修复）
6. ~~VFS 鉴权 / 联邦 SSRF / CSRF / health 收敛 / SQL 日志参数化 / IDOR / 元数据泄露 / 敏感信息移出 URL~~（M4/M5/L1/L2/L4/L5 + CSRF + nginx + URL 治理，第二轮全部修复）

> 截至 2026-08-24，审计报告全部发现项均已修复或提供运维层配置方案（HTTPS/HSTS 需部署时启用）。
