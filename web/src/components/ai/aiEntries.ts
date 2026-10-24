// AI 工作台的功能入口配置。
// 门户宫格与左侧竖向导航共用一份，避免两处定义的名称/图标/顺序漂移。
// 参考 C:/Users/QBZ95/Desktop/testlo（SparkAi 快照）的入口集合：
// 大模型 / 专业绘画 / 灵感广场 / 视频创作 / AI 智能体 / 思维导图 / 会员中心。

export interface AiEntry {
  /** 对应 AiHub 的 activeTab；membership 为出站跳转 */
  key: string;
  title: string;
  desc: string;
  emoji: string;
  gradient: string;
  badge?: string;
  /** 依赖 AI 总开关：未开启时置灰标「未开启」，但入口仍然存在 */
  needsAi?: boolean;
}

export const AI_ENTRIES: AiEntry[] = [
  {
    key: 'chat',
    title: 'AI 对话',
    desc: '与模型实时对话，支持流式输出、上下文与智能体',
    emoji: '💬',
    gradient: 'var(--gradient-primary)',
    badge: '常用',
    needsAi: true,
  },
  {
    key: 'models',
    title: '大模型广场',
    desc: '按类型与厂商浏览可用模型，选一个直接开聊',
    emoji: '🧠',
    gradient: 'var(--gradient-blue)',
    needsAi: true,
  },
  {
    key: 'image',
    title: '专业绘画',
    desc: '文生图与图生图，支持尺寸、风格与批量出图',
    emoji: '🎨',
    gradient: 'var(--gradient-purple)',
    badge: 'HOT',
    needsAi: true,
  },
  {
    key: 'video',
    title: '视频创作',
    desc: '文生视频与图生视频，异步任务与进度回显',
    emoji: '🎬',
    gradient: 'var(--gradient-rose)',
    needsAi: true,
  },
  {
    key: 'gallery',
    title: '灵感广场',
    desc: '浏览全站公开作品，一键复刻同款提示词',
    emoji: '✨',
    gradient: 'var(--gradient-amber)',
  },
  {
    key: 'agents',
    title: 'AI 智能体',
    desc: '写作教练、世界观设定、资料考据等角色化助手',
    emoji: '🤖',
    gradient: 'var(--gradient-teal)',
  },
  {
    key: 'mind',
    title: '思维导图',
    desc: '把大纲或设定整理成结构化脑图',
    emoji: '🕸️',
    gradient: 'var(--gradient-indigo)',
    needsAi: true,
  },
  {
    key: 'membership',
    title: '会员与额度',
    desc: '查看余额与消耗明细，充值或配置自己的上游密钥',
    emoji: '💎',
    gradient: 'var(--gradient-success)',
  },
];
