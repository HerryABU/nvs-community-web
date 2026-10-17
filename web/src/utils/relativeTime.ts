// 相对时间格式化：用户看到「刚刚 / X分钟前 / X小时前 / X天前 / X周前」，超过 30 天显示 YYYY-MM-DD。
// 内部存储为微秒级时间戳（created_micro）或 ISO 字符串（created_at，已含微秒），可精确排序与防重放。

function pad(n: number, len = 2): string {
  return String(n).padStart(len, '0');
}

// 将 created_at(ISO) 或 created_micro(Unix 微秒) 统一转为毫秒时间戳
export function toTimestamp(createdAt?: string, micro?: number): number {
  if (micro && micro > 0) return Math.floor(micro / 1000);
  if (createdAt) {
    const t = new Date(createdAt).getTime();
    if (!isNaN(t)) return t;
  }
  return 0;
}

// 相对时间：<1分钟 刚刚；<1小时 X分钟前；<24小时 X小时前；<30天 X天前；否则 YYYY-MM-DD
export function formatRelativeTime(createdAt?: string, micro?: number): string {
  const t = toTimestamp(createdAt, micro);
  if (!t) return '';
  const diff = Date.now() - t;
  if (diff < 0) return formatDate(t); // 未来时间（时钟偏差）直接绝对显示
  if (diff < 60_000) return '刚刚';
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}分钟前`;
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}小时前`;
  if (diff < 30 * 86_400_000) return `${Math.floor(diff / 86_400_000)}天前`;
  return formatDate(t);
}

// 绝对时间（含微秒）：悬停 Tooltip 用，满足极客查证需求
export function formatAbsoluteMicro(createdAt?: string, micro?: number): string {
  const t = toTimestamp(createdAt, micro);
  if (!t) return '';
  const d = new Date(t);
  let micros = '';
  if (micro && micro > 0) micros = '.' + pad(Math.floor(micro % 1_000_000), 6);
  return (
    `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ` +
    `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}${micros}`
  );
}

function formatDate(t: number): string {
  const d = new Date(t);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}
