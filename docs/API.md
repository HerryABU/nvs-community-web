# API 一览

> 全部对外接口。路径均相对后端根（反向代理前缀由网关处理）。
> 返回 [README](../README.md) · [更新日志](CHANGELOG.md) · [配置](CONFIG.md) · [架构](ARCHITECTURE.md) · [开发](DEVELOPMENT.md) · [特性详解](FEATURES.md)

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
| POST | `/api/comments` | JWT | 创建评论（限流 20次/60s，支持小说和博客）；可带 `images`（附图，仅接受站内 `/uploads/images/` 前缀，最多 9 张，图片走多模态审核） |
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
| POST | `/api/admin/bad-words/search-augment` | 管理员 | 联网检索补充违禁词（`focus` 侧重方向 / `category` 统一归类 / `model_id` 指定模型 / `search_mode`=auto\|endpoint\|native\|prompt；新增规则一律标记动作并按 名称+方式+词 去重） |
| GET | `/api/admin/bad-word-flags` | 管理员 | 违禁词标记待审列表（status 过滤 + 分页） |
| PUT | `/api/admin/bad-word-flags/:id` | 管理员 | 处理标记（确认违规[可选删除内容留证] / 放行） |
| GET | `/api/admin/comment-policy` | 管理员 | 评论区策略：`lock_hides_replies`（楼加密是否连带隐藏楼内回复，默认 false=评论区不加密）/ `locked_floor_can_reply`（是否允许对未解锁的楼回复，默认 false） |
| PUT | `/api/admin/comment-policy` | 管理员 | 保存评论区策略（立即生效） |
| GET | `/api/admin/html-reviews` | 管理员 | HTML 内容审查列表 |
| GET | `/api/admin/html-reviews/:type/:id` | 管理员 | 审查详情 |
| POST | `/api/admin/html-reviews/:type/:id/review` | 管理员 | 复核 HTML 内容 |
| POST | `/api/admin/smtp/test` | 管理员 | SMTP 测试与逐阶段诊断 |
| GET | `/api/admin/softdel` | 管理员 | 内容治理：软删除列表（含留证记录） |
| POST | `/api/admin/softdel` | 管理员 | 删除/封禁任意内容（软删除 + `softdel_` 留证） |
| POST | `/api/admin/softdel/restore` | 管理员 | 恢复被封禁内容 |
| GET | `/api/admin/recycle` | 管理员 | 回收站监管：全站用户自删内容 |
| GET | `/api/admin/novels` | 管理员 | 小说封禁管理列表 |
| GET | `/api/admin/novels/:id/chapters` | 管理员 | 章节封禁管理列表 |
| GET | `/api/admin/blog-categories` | 管理员 | 博客分类管理列表（用户分组 + 待审核徽标） |
| GET | `/api/admin/blog-categories/mode` | 管理员 | 分类审核模式（post_review 事后审核 / pre_public 先公开） |
| PUT | `/api/admin/blog-categories/mode` | 管理员 | 切换分类审核模式 |
| POST | `/api/admin/blog-categories` | 管理员 | 创建分类（管理员直通） |
| PUT | `/api/admin/blog-categories/:id` | 管理员 | 更新分类 |
| DELETE | `/api/admin/blog-categories/:id` | 管理员 | 删除分类 |
| POST | `/api/admin/blog-categories/:id/review` | 管理员 | 审核分类（approve / reject[附原因]） |
| GET | `/api/admin/algorithms` | 管理员 | 算法推荐配置（176号令 算法透明度） |
| PUT | `/api/admin/algorithms` | 管理员 | 更新算法推荐配置 |
| GET | `/api/admin/announcements` | 管理员 | 公告列表 |
| POST | `/api/admin/announcements` | 管理员 | 发布公告 |
| PUT | `/api/admin/announcements/:id` | 管理员 | 更新公告 |
| DELETE | `/api/admin/announcements/:id` | 管理员 | 删除公告 |
| GET | `/api/admin/log-redact` | 管理员 | 日志脱敏配置 |
| PUT | `/api/admin/log-redact` | 管理员 | 更新脱敏配置（热更新） |
| POST | `/api/admin/log-redact/preview` | 管理员 | 脱敏效果预览工具 |
| GET | `/api/admin/security-grade` | 管理员 | 等保/关基识别记录 |
| PUT | `/api/admin/security-grade` | 管理员 | 更新定级记录 |
| GET | `/api/admin/vulnerabilities` | 管理员 | 漏洞整改清单（status/severity 筛选 + 统计） |
| POST | `/api/admin/vulnerabilities` | 管理员 | 新建漏洞记录 |
| PUT | `/api/admin/vulnerabilities/:id` | 管理员 | 更新漏洞（状态/整改时间） |
| DELETE | `/api/admin/vulnerabilities/:id` | 管理员 | 删除漏洞记录 |
| GET | `/api/admin/vulnerabilities/export` | 管理员 | 漏洞清单导出 |
| GET | `/api/admin/compliance/report` | 管理员 | 合规自查报告（一键生成） |
| GET | `/api/admin/compliance/export-logs` | 管理员 | 按时间范围导出访问日志（ZIP） |
| GET | `/api/admin/compliance/export-users` | 管理员 | 导出用户注册信息清单（CSV） |

### 🔧 内容抢救台（应急窗口）
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/admin/emergency/set-password` | 管理员 | 设置/修改二级密码 |
| POST | `/api/admin/emergency/unlock` | 管理员 | 二级密码解锁（限流 10次/60s，失败锁定 15 分钟） |
| POST | `/api/admin/emergency/preview` | 二级密码 token | 拉取内容预览（body 传 token，小说/博客/帖子返回全部评论/回复；token 不落 URL） |
| POST | `/api/admin/emergency/content` | 二级密码 token | 物理硬删除（body 传 token，不可恢复） |
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

### 合规公开接口（176号令）
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/algorithms/description` | — | 算法推荐说明（公开，算法透明度） |
| GET | `/api/announcements` | — | 站点公告列表（仅已发布） |
| GET | `/api/users/me/preferences` | JWT | 用户偏好设置（个性化推荐开关，关闭权） |
| PUT | `/api/users/me/preferences` | JWT | 更新偏好设置 |

### 🎨 AI 绘画
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/ai/image` | JWT | 生成图片（限流 20次/60s）。`mode`=t2i\|i2i\|faceswap；生成前先做提示词初审，命中违规直接拒绝（不调上游、不扣费）；开启出图后审时，判定违规则不返回图片并自动退款 |
| POST | `/api/ai/image/optimize-prompt` | JWT | 优化绘画提示词（把简短描述扩写成信息密度更高的版本，不改变创作意图）。入参 `{ prompt, model_id? }`，返回 `{ prompt, original }` |

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
