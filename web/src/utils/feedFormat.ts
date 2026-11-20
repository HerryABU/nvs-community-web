// 内容流展示用的格式化小工具（播放量/时长）。放在一处，避免各页面各写一遍。

/** 1.2万 / 3.4亿 / 987 —— 中文习惯的紧凑数字 */
export function formatCount(n?: number): string {
  const v = Number(n || 0);
  if (!isFinite(v) || v <= 0) return '0';
  if (v < 10000) return String(v);
  if (v < 100000000) {
    const w = v / 10000;
    return (w >= 100 ? Math.round(w) : Math.round(w * 10) / 10) + '万';
  }
  const y = v / 100000000;
  return (y >= 100 ? Math.round(y) : Math.round(y * 10) / 10) + '亿';
}

/** 秒 → 03:25 / 1:02:03 */
export function formatDuration(sec?: number): string {
  const s = Math.max(0, Math.floor(Number(sec) || 0));
  if (!s) return '';
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const r = s % 60;
  const pad = (x: number) => String(x).padStart(2, '0');
  return h > 0 ? `${h}:${pad(m)}:${pad(r)}` : `${pad(m)}:${pad(r)}`;
}

/** ISO → 3天前 / 2026-11-19 */
export function formatWhen(iso?: string): string {
  if (!iso) return '';
  const t = new Date(iso).getTime();
  if (!isFinite(t)) return '';
  const diff = Date.now() - t;
  const min = Math.floor(diff / 60000);
  if (min < 1) return '刚刚';
  if (min < 60) return `${min} 分钟前`;
  const h = Math.floor(min / 60);
  if (h < 24) return `${h} 小时前`;
  const d = Math.floor(h / 24);
  if (d < 30) return `${d} 天前`;
  return iso.slice(0, 10);
}
