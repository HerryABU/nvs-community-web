# 配置参考（.env）

> 全部环境变量。首次启动会自动生成 .env，绝大多数键无需手改。
> 返回 [README](../README.md) · [更新日志](CHANGELOG.md) · [API](API.md) · [架构](ARCHITECTURE.md) · [开发](DEVELOPMENT.md) · [特性详解](FEATURES.md)

启动时自动生成（模板见 `server/config/config.go` 的 `defaultEnvTemplate`），也可复制仓库根的 **`.env.example`** 改名 `.env` 后填写。

> 密钥类项在示例文件中**留空**：留空时启动会自动生成/派生（JWT 密钥落盘 `data/.jwt_secret`）。生产环境请显式填写。

### 数据库
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DB_DRIVER` | `sqlite` | 数据库驱动（`sqlite` / `mysql`） |
| `DB_PATH` | `data/nvs.db` | SQLite 文件路径（**装 DLL/容器挂载卷时务必设成绝对路径**） |
| `DB_MEMORY` | `false` | SQLite 内存模式（测试用，进程退出即丢） |
| `DB_HOST` | `127.0.0.1` | MySQL 主机 |
| `DB_PORT` | `3306` | MySQL 端口 |
| `DB_USER` | `nvs_user` | MySQL 用户名 |
| `DB_PASSWORD` | 空 | MySQL 密码 |
| `DB_NAME` | `nvs` | MySQL 数据库名 |
| `DB_ROOT_PASSWORD` | 空 | 仅供建站向导创建库/用户时使用，创建后可删 |

> 🔐 **数据库密码加密存储**：通过建站向导或后台「数据库迁移」设置的 MySQL 连接信息（含密码）会经 **AES-256-GCM** 加密写入 `data/secure-config.json`（密钥存 `data/.secret.key`，0600 权限仅本机可读），并自动从 `.env` 移除明文 `DB_*` 行，避免密码明文残留。`config.Init()` 启动时会优先解密加载该加密配置覆盖内存变量。
>
> ⚠️ `config.IsInstalled()` 的判据是 **`data/secure-config.json` 是否存在**（进程内 once 缓存）：把它挪走，哪怕数据库已建站，实例也会退回建站向导。

### Redis（可选，SQLite 模式下不需要）
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `REDIS_HOST` | `127.0.0.1` | Redis 主机 |
| `REDIS_PORT` | `6379` | Redis 端口 |
| `REDIS_PASSWORD` | 空 | Redis 密码 |
| `REDIS_DB` | `0` | 库序号 |

### 网络与端口
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SERVER_PORT` | `8080` | 服务端口 |
| `BIND_IPV4` | `0.0.0.0` | IPv4 绑定地址（`0.0.0.0`=所有网卡） |
| `BIND_IPV6` | `::` | IPv6 绑定地址 |
| `ENABLE_IPV6` | `false` | 是否启用 IPv6 双栈 |
| `LOCAL_IPV4` | 自动检测 | 本机 IPv4（联邦互通用） |
| `LOCAL_IPV6` | 自动检测 | 本机 IPv6（联邦互通用） |

### 站点路径前缀（反向代理挂载）
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `URL_PREFIX` | 空 | 站点路径前缀，如 `/go-nvs`（必须 `/` 开头、不带尾斜杠；留空=挂在域名根）。用于反向代理场景：nginx/网关将 `/go-nvs/` 前缀转发到本站时，访问 `http://网关:8088/go-nvs/login` 即可正常使用（自动剥离前缀 + 前端路由/API/资源/跳转全自适应，前缀名任意） |

> 🔀 **两种代理转发模式均可用**：① `proxy_pass http://后端;`（不带 URI，保留前缀，后端自动剥离）；② `proxy_pass http://后端/;`（带尾部斜杠，去前缀，后端收 `/assets/x.js` 等原始路径）。反向代理仅改写 `<base href>` 而不注入前缀变量时，前端自动从 `document.baseURI` 推导前缀，无需后端配置。

### JWT
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `JWT_SECRET` | 空（自动生成） | JWT 签名密钥；留空启动自动生成随机密钥并持久化，**务必不要用示例值上线** |
| `JWT_EXPIRE_HOURS` | `72` | 访问令牌过期时间（小时） |
| `JWT_REFRESH_EXPIRE_HOURS` | `720` | 刷新令牌过期时间（小时，默认 30 天，用于无感续期） |

### SMTP 邮件
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SMTP_HOST` | `smtp.qq.com` | SMTP 服务器 |
| `SMTP_PORT` | `587` | SMTP 端口 |
| `SMTP_USER` | — | 发件邮箱 |
| `SMTP_PASSWORD` | — | SMTP 授权码 |
| `SMTP_FROM` | — | 发件人地址 |

### 短信网关（手机号登录 / 验证码）
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SMS_GATEWAY` | `log` | `log`=开发网关（验证码只写服务日志，不真发）；`http`=通用 HTTP 网关 |
| `SMS_HTTP_URL` | 空 | `http` 网关地址（POST JSON `{phone,code,sign_name,template_code}`） |
| `SMS_HTTP_TOKEN` | 空 | 网关令牌（请求头 `Authorization: Bearer <token>`） |
| `SMS_SIGN_NAME` | `星海文学` | 短信签名（需在短信平台报备） |
| `SMS_TEMPLATE_CODE` | 空 | 模板 Code（模板内容须含验证码占位符） |
| `SMS_TIMEOUT_SEC` | `5` | 网关超时（秒） |

### 微信扫码登录（开放平台网站应用）
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `WECHAT_APP_ID` | 空 | 开放平台 AppID |
| `WECHAT_APP_SECRET` | 空 | 开放平台 AppSecret |
| `WECHAT_CALLBACK` | 空 | 完整回调地址（须与平台登记域名一致，含路径前缀），如 `https://example.com/go-nvs/api/auth/wechat/callback` |

### 存储与用户自定义内容
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `NOVEL_DATA_DIR` | `./data/novels` | 小说文件存储目录 |
| `UPLOAD_DIR` | `./data/uploads` | 上传文件存储目录 |
| `USER_FRAME_DIR` | `./data/userframe` | 自定义 HTML-UI 模板目录 |
| `USER_HTML_DIR` | `./data/userhtml` | 扩展应用（HTML 托管）目录 |

### 平台功能
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PLATFORM_FEE_RATE` | `0.10` | 平台抽成比例 |
| `SITE_NAME` | `NVS 类型文学平台` | 站点名称 |
| `EMAIL_VERIFY_ENABLED` | `false` | 邮箱验证开关 |
| `CAPTCHA_ENABLED` | `false` | 滑块验证码开关（**开站后建议开启**，登录/注册/发码/找回密码统一走一次性 pass token） |
| `TIANDITU_TK` | 空 | 天地图「浏览器端」tk（标准地图底图）。留空=只用内嵌 ip2region 出排行、不画边界；填了即启用中国/全球标准地图。也可在后台「运营 → 地图服务」填，后台值优先 |
| `MAP_REVIEW_NO` | 空 | 审图号（如 `GS(2024)0650号`），对外展示标准地图时标注；同样可在后台填写 |

### 安全 / 反向代理
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `TRUSTED_PROXIES` | 空 | 信任的反向代理地址（逗号分隔 IP/CIDR）。留空=不信任任何代理头，客户端 IP 取真实 TCP 对端（防 `X-Forwarded-For` 伪造越权） |
| `COOKIE_SECURE` | `false` | JWT Cookie 强制 Secure（仅 HTTPS 传输；生产启用 HTTPS 后务必 `true`） |
| `SETUP_LOCAL_ONLY` | `true` | 建站向导仅允许本机 localhost 访问（防公网/反向代理越权进入安装向导） |
| `REAL_NAME_ENC_KEY` | 空 | 实名信息（姓名/证件）AES-256-GCM 密钥（hex 或 base64 的 32 字节）。留空则从 `JWT_SECRET` 派生 —— **轮换 JWT_SECRET 会导致历史实名数据不可解密**，生产请独立配置 |

### 国密 / TLS 双轨（可选模块）
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `NVS_CRYPTO_TRACK` | `intl` | `intl`=标准 RSA/ECC+AES（零国密依赖）/ `gm`=纯国密 SM2/SM3/SM4+TLCP / `dual`=双轨自适应协商 |
| `NVS_CRYPTO_INTL_CERT` / `NVS_CRYPTO_INTL_KEY` | 空 | 国际轨证书；都不配 = 纯 HTTP（适合反代终止 TLS） |
| `NVS_CRYPTO_GM_PROVIDER` | 空 | 国密实现：`tongsuo`（CGO，官方绑定）/ `purego`（纯 Go，无 CGO） |
| `NVS_CRYPTO_SM_SIGN_CERT` / `_SIGN_KEY` / `_ENC_CERT` / `_ENC_KEY` | 空 | 国密双证书四件套（TLCP 需签名+加密两套） |
| `NVS_CRYPTO_SESSION_TICKETS_DISABLED` | `false` | 禁用 TLS 会话复用 |
| `NVS_TLS_HANDSHAKE_LOG` | 空 | 握手记录 JSONL 路径（记录协议/套件/SNI，不含业务数据） |

> 责任边界：框架不代做等保/密评；是否启用及合规责任由站长自行承担（见 `docs/gm-mapping.md`）。

### 日志敏感信息脱敏（176号令合规）
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `LOG_REDACT_SENSITIVE` | `false` | 日志脱敏开关：`true`=生产模式（自动脱敏 Cookie/Authorization 令牌、body/query 敏感字段）；`false`=开发模式（保留原始数据，方便调试） |
| `LOG_REDACT_MASK` | `***REDACTED***` | 脱敏掩码（密码类固定 `******`，其余字段可用此掩码） |
| `LOG_SENSITIVE_FIELDS` | `password,token,authorization,cookie,secret,apikey` | 需脱敏的字段名列表（逗号分隔，可扩展） |

> 🧾 脱敏配置支持管理后台可视化维护（`/admin/log-redact`），保存后运行时热更新，无需重启；`.env` 仅为种子默认值。
