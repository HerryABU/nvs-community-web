import {
  Setting, View, DataAnalysis, Medal, Warning, Bell, DataLine, Monitor,
  Collection, Lock, Histogram, Stamp, Key, ChatDotRound, Cpu, WarningFilled,
  Box, Notebook, TrendCharts, Connection, Postcard, Lightning, Brush, Grid, Message,
  Timer, Coin, MagicStick, ChatLineRound, Present, DataAnalysis, Operation, CreditCard,
} from '@element-plus/icons-vue';

// 管理员控制台「管理功能」分类数据（F8：分类折叠 + ?category=/?item= 深链定位）。
// 从 AdminDashboard.vue 抽出，避免仪表盘文件膨胀（无上帝类）；新增功能只需在此追加条目。

export interface AdminMenuItem {
  title: string;
  desc: string;
  path: string;
  icon: any;
  color: string;
}

export interface AdminMenuCategory {
  key: string;
  title: string;
  icon: any;
  color: string;
  items: AdminMenuItem[];
}

export const adminMenuCategories: AdminMenuCategory[] = [
  {
    key: 'site',
    title: '站点配置',
    icon: Setting,
    color: 'var(--gradient-blue)',
    items: [
      { title: '站点设置', desc: '站点名称、首页主模块、平台指南、备案、robots.txt', path: '/admin/settings', icon: Setting, color: 'var(--gradient-blue)' },
      { title: '登录方式管理', desc: '邮箱/手机号/微信扫码入口 · 注册验证 · SMTP 通道 · 域名限制', path: '/admin/login-methods', icon: Message, color: 'var(--gradient-indigo)' },
      { title: '自定义主页', desc: '编排首页区块：头图/公告/模块/博客/版块/最新帖/自定义HTML/链接', path: '/admin/home-blocks', icon: Grid, color: 'var(--gradient-teal)' },
      { title: '自定义主题', desc: 'alist 式全局主题：CSS 消毒后全站生效', path: '/admin/theme', icon: Brush, color: 'var(--gradient-pink)' },
      { title: '狂暴模式', desc: '评论区实时推送（SSE）开关与带宽参数', path: '/admin/rage', icon: Lightning, color: 'var(--gradient-purple)' },
      { title: '数据库管理', desc: 'SQLite↔MySQL 迁移 / 多库备份', path: '/admin/database', icon: DataLine, color: 'var(--gradient-teal)' },
      { title: '健康检查', desc: '服务器 / 数据库 / 网络 / 前端 API', path: '/admin/health', icon: Monitor, color: 'var(--gradient-success)' },
      { title: '远程站点互通', desc: '远程 NVS 站点互联与作品同步', path: '/admin/sites', icon: Connection, color: 'var(--gradient-teal)' },
    ],
  },
  {
    key: 'security',
    title: '合规与安全',
    icon: Lock,
    color: 'var(--gradient-danger)',
    items: [
      { title: '合规中心', desc: '等保指引 · 检查工具箱 · 自查报告', path: '/admin/compliance', icon: Medal, color: 'var(--gradient-success)' },
      { title: '漏洞管理', desc: '漏洞整改闭环留痕 · 报告导出', path: '/admin/vulnerabilities', icon: Warning, color: 'var(--gradient-danger)' },
      { title: '日志脱敏', desc: '生产环境日志敏感信息脱敏 · 176号令合规', path: '/admin/log-redact', icon: View, color: 'var(--gradient-danger)' },
      { title: '隔离墙配置', desc: '敏感分区列表与确认流程', path: '/admin/wall', icon: Lock, color: 'var(--gradient-accent)' },
      { title: '隔离墙访问记录', desc: '谁/何时进入敏感分区 · 176号令合规', path: '/admin/wall-access', icon: Histogram, color: 'var(--gradient-blue)' },
      { title: '网络诊断与攻击流量', desc: '用户分布 · IP/UA 分类 · 防爬攻击识别', path: '/admin/net-diag', icon: Connection, color: 'var(--gradient-accent)' },
      { title: '限流与 API Key', desc: '四层身份配额 · 爬虫识别 · 访问审计', path: '/admin/ratelimit', icon: Key, color: 'var(--gradient-accent)' },
      { title: 'XSS 规则库', desc: '维护 XSS 防御正则规则（动态更新）', path: '/admin/security', icon: Key, color: 'var(--gradient-accent)' },
    ],
  },
  {
    key: 'content',
    title: '内容治理',
    icon: WarningFilled,
    color: 'var(--gradient-accent)',
    items: [
      { title: '举报处理', desc: '审核举报、删除违规内容', path: '/admin/reports', icon: Warning, color: 'var(--gradient-accent)' },
      { title: '违禁词库', desc: '标记/删除/替换 · 关键词+正则双模式', path: '/admin/badwords', icon: Warning, color: 'var(--gradient-danger)' },
      { title: '内容封禁', desc: '小说/章节封禁 · 记录176原因 · 可解封', path: '/admin/ban', icon: Lock, color: 'var(--gradient-danger)' },
      { title: '内容留证', desc: 'softdel 回收站 · 删除/封禁可追溯', path: '/admin/softdel', icon: Box, color: 'var(--gradient-purple)' },
      { title: '内容抢救台', desc: '二级密码应急预览/软删除留证', path: '/admin/emergency', icon: WarningFilled, color: 'var(--gradient-accent)' },
      { title: 'HTML/WASM 审核', desc: '可疑内容人工判定 · 封存件处置 · 审核记录', path: '/admin/html-review', icon: Cpu, color: 'var(--gradient-purple)' },
      { title: '作者主页审核', desc: '自定义公开主页消毒预览 · 原文送站长复核 · 通过/驳回', path: '/admin/home-customize-review', icon: View, color: 'var(--gradient-pink)' },
      { title: '热词统计', desc: '评论/回帖高频词聚合 · 舆情观测', path: '/admin/hotwords', icon: Histogram, color: 'var(--gradient-blue)' },
      { title: '实名与未成年保护', desc: '核验提供商配置 · 实名策略 · 未成年模式 · 实名记录', path: '/admin/realname', icon: Postcard, color: 'var(--gradient-success)' },
    ],
  },
  {
    key: 'community',
    title: '社区运营',
    icon: ChatDotRound,
    color: 'var(--gradient-teal)',
    items: [
      { title: '论坛板块管理', desc: '板块增删、审查、排序', path: '/admin/forums', icon: ChatDotRound, color: 'var(--gradient-blue)' },
      { title: '博客分类与回收站', desc: '博客分类管理 · 全站用户自删内容监管', path: '/admin/blog-categories', icon: Notebook, color: 'var(--gradient-teal)' },
      { title: '社区动态', desc: '最近用户、作品、评论、帖子', path: '/admin/community', icon: TrendCharts, color: 'var(--gradient-teal)' },
      { title: 'web 项目额度与审核', desc: '广场发布额度配置 + 超额发布审核队列', path: '/admin/web-quota', icon: Box, color: 'var(--gradient-purple)' },
      { title: '每日发文限额', desc: '按用户等级配置每日发布数量/字数 · 按人覆盖 · AI 计入', path: '/admin/daily-publish', icon: Timer, color: 'var(--gradient-indigo)' },
      { title: '代币与卡密', desc: '平台内消费积分开关/命名 · 卡密生成作废 · 授信调整', path: '/admin/token', icon: Coin, color: 'var(--gradient-amber)' },
      { title: '在线支付通道', desc: '微信/支付宝聚合充值 · 档位面额 · 幂等到账 · 手动补单', path: '/admin/payment', icon: CreditCard, color: 'var(--gradient-success)' },
      { title: '公告栏', desc: '站点公告发布 · 合规通知', path: '/admin/announcements', icon: Bell, color: 'var(--gradient-blue)' },
      { title: '书目分类管理', desc: '平台书目分类的增删改', path: '/admin/categories', icon: Collection, color: 'var(--gradient-purple)' },
    ],
  },
  {
    key: 'algo',
    title: '算法与推荐',
    icon: DataAnalysis,
    color: 'var(--gradient-purple)',
    items: [
      { title: '算法管理', desc: '推荐排序规则 · 算法说明 · 用户选择权', path: '/admin/algorithms', icon: DataAnalysis, color: 'var(--gradient-purple)' },
    ],
  },
  {
    key: 'ai',
    title: 'AI 能力',
    icon: MagicStick,
    color: 'var(--gradient-purple)',
    items: [
      { title: 'AI 模型池', desc: 'OpenAI 兼容上游配置 · 密钥加密存储 · 按量计费（对话/绘画/视频）', path: '/admin/ai-pool', icon: MagicStick, color: 'var(--gradient-purple)' },
      { title: 'AI 智能体', desc: '对话人设预设 · 系统提示词 · 工作台卡片', path: '/admin/ai-agents', icon: ChatLineRound, color: 'var(--gradient-indigo)' },
      { title: '免费 AI 额度', desc: '每日免费调用额度（次数/tokens）· 用尽才扣代币 · 按人覆盖', path: '/admin/ai-free-quota', icon: Present, color: 'var(--gradient-teal)' },
      { title: 'AI 数据审计', desc: '会话/作品/调用流水只读查询 · 排查与对账', path: '/admin/ai-logs', icon: DataAnalysis, color: 'var(--gradient-blue)' },
      { title: 'AI 功能配置', desc: '能力开关（BYOK/混图/换脸）· 上下文条数 · 每日限额 · 厂商字典', path: '/admin/ai-features', icon: Operation, color: 'var(--gradient-teal)' },
      { title: 'AI 评论机器人', desc: '评论区 @ai 召唤回复 · 每日限流 · 发起人付费', path: '/admin/ai-bot', icon: ChatLineRound, color: 'var(--gradient-pink)' },
    ],
  },
  {
    key: 'police',
    title: '警方取证',
    icon: Stamp,
    color: 'var(--gradient-danger)',
    items: [
      { title: '警方取管台', desc: '取证留证 · 软删除查看 · 凭证下载', path: '/police-console', icon: Stamp, color: 'var(--gradient-danger)' },
    ],
  },
];
