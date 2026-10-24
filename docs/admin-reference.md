# 后台功能对照（参考 testlo\管理后台，功能参考 / 设计自研）

> 参考物料：`C:\Users\QBZ95\Desktop\testlo\管理后台\*`（26 个整站保存页）。
> **只参考功能划分，不复用其任何视觉、组件、文案或素材**；NVS 用自有 admin 设计系统实现。

## 一、参考站的后台 IA（12 大类）

| 分类 | 页面 |
|---|---|
| 用户管理 | 用户数据列表 / 用户账户明细 / 用户注册设置 |
| 注册管理 | 邮件注册登录 / 邮箱注册设置 / 微信登录配置 / 短信验证登录 |
| 用户端设置 | 基础配置 / AI功能配置 / 快问预设(废弃) / 动态菜单 / 公告设置 / 画廊轮播图 / 直播入口 / 思维导图 / 访客设置 / 签到奖励 / 顶部功能 / 应用显示 |
| 大模型管理 | 大模型全局配置 / 文件存储配置 / 模型厂家分类 / AI模型配置 / API卡池列表 / 全局头部预设 |
| AI 对话管理 | 对话数据 / 对话组演示 |
| AI 绘画管理 | 绘画全局配置 / 绘画数据管理 / 作品风格分类 / MJ-Niji配置 / GPT-Image-2配置 / Nano Banana配置 |
| CozeAgent | Agent全局配置 / Coze配置管理 / Agent应用管理 / Agent应用会话 |
| AI 视频创作 | 视频全局配置 / 视频数据管理 / 视频模型配置 / 视频风格分类 / 演示数据 / PiKa(旧) |
| 订单管理 | 订单列表 / 套餐设置 / 卡密管理 |
| 应用管理 | 分类列表 / 应用列表 |
| 分销系统 | 基础分销配置 / 佣金账户列表 / 佣金分销明细 / 提现工单 |
| 支付管理 | 微信支付 / 易支付 |

## 二、NVS 现状对照

| 参考页 | NVS 对应实现 | 状态 |
|---|---|---|
| AI 模型配置 | `/admin/ai-pool`（模型池：kind/api_kind/vendor/计价/预设） | ✅ 已有 |
| 模型厂家分类 | `/admin/ai-features` 下半区「模型厂商」字典（CRUD/启停/引用计数/改名级联） | ✅ 本轮新增 |
| AI 功能配置 | `/admin/ai-features` 上半区（BYOK / 混图 / 换脸 / 上下文条数 / 每日限额 / 广场公开） | ✅ 本轮新增 |
| 对话数据 | `/admin/ai-logs` → 对话数据 Tab（按用户筛选 + 会话详情抽屉） | ✅ 本轮新增 |
| Agent 应用会话 | `/admin/ai-logs` 对话 Tab 中「Coze」标记 + 第三方会话 ID | ✅ 本轮新增 |
| 绘画数据管理 / 视频数据管理 | `/admin/ai-logs` → 绘画/视频 Tab（kind/status 筛选） | ✅ 本轮新增 |
| 调用流水（计费） | `/admin/ai-logs` → 调用流水 Tab（tokens/实扣/免费/状态） | ✅ 本轮新增 |
| Agent 应用管理 | `/admin/ai-agents`（含扣子 upstream/bot_id/PAT） | ✅ 已有 |
| Coze 配置管理 | 折进 Agent 表单（bot_id + API 域名 + PAT），未做全局默认值 | 🟡 部分 |
| API 卡池列表（多密钥轮询） | `/admin/ai-pool` 模型详情内维护多把 Key（`ai_model_keys`：Label/Weight/FailCount/Enabled），`AcquireModelKey` 加权轮询 + 失败降权 | ✅ 已有（本文旧版写的「每模型单密钥」已过时） |
| 绘画全局配置 / MJ 配置 / GPT-Image-2 配置 / Nano Banana 配置 | 折进模型池：每模型自带 api_kind + base_url + 单价 | ✅ 等价（更精简） |
| 作品风格分类 / 视频风格分类 | 作品发布时手填 style + 广场按 style 聚合 | 🟡 部分（缺字典） |
| 套餐设置 / 卡密管理 / 订单列表 | `/admin/payment`（档位/订单/补单）+ `/admin/token`（卡密） | ✅ 已有 |
| 应用管理（分类/列表） | NVS 用「扩展应用 /frames」体系（FrameManager） | ✅ 等价 |
| 公告设置 / 动态菜单 / 顶部功能 / 应用显示 | `/admin/announcements`、首页模块配置 | ✅ 已有 |
| 画廊轮播图 / 直播入口 / 签到奖励 / 访客设置 | 无 | ⛔ 待做（非 AI 主线） |
| 基础配置 / 文件存储配置 | `/admin/settings`、`/admin/database` | ✅ 已有 |
| 注册/登录方式 | `/admin/login-methods`、`/admin/realname` | ✅ 已有 |
| 用户数据列表 / 账户明细 | `/admin/users`（role/status 筛选）、钱包流水 | ✅ 已有 |
| 分销系统 | 无（NVS 无分销设计） | ⛔ 不做（商业模式不同） |

## 三、待做项的实现方案（备忘）

### 1. API 卡池（多密钥轮询）
- 表：`ai_model_keys{id, model_id, label, key_enc, weight, enabled, fail_count, last_used_at, created_at}`。
- 取用：`PickModelKey(modelID)` 按 `enabled=1` 且 `fail_count < N` 的池子按权重随机；调用成功 `fail_count=0`，失败 `+1`（超过阈值自动 suspend 并记日志）。
- 管理端：模型行内「密钥池」抽屉，支持批量粘贴导入（每行一把）、启停、删除、测试连通性。
- 兼容：`AiModel.api_key_enc` 保留作为「兜底密钥」，池为空时回退。

### 2. 风格字典
- 表：`ai_styles{id, kind(image/video), name, sort_order, enabled}`；作品发布时改为下拉 + 允许自定义（自定义则落字典）。
- 广场筛选 chips 改为读字典（保持现有 `style` 字段不变，零迁移）。

### 3. Coze 全局默认值
- KV：`ai_coze_default{base_url, timeout_seconds}`，Agent 未填时继承；便于统一切换国内/国际域名。

## 四、本轮新增接口清单（前后端已对齐）

```
GET    /api/admin/ai/audit/stats
GET    /api/admin/ai/audit/sessions?user_id=&page=
GET    /api/admin/ai/audit/sessions/:id/messages
GET    /api/admin/ai/audit/media?kind=&status=&page=
GET    /api/admin/ai/audit/calls?status=&page=
GET    /api/admin/ai/features
PUT    /api/admin/ai/features
GET    /api/admin/ai/vendors
POST   /api/admin/ai/vendors
PUT    /api/admin/ai/vendors/:id
DELETE /api/admin/ai/vendors/:id
```

生效点（不是"存了不用"）：`AiHubChatStream`（每日限额 / BYOK 开关 / 上下文条数）、`AiHubImage`（混图 / 换脸开关）、`AiGalleryList`（广场公开开关）。
