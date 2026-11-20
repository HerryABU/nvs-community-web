/**
 * 动态详情统一走「真实路由」，不再用 el-dialog 叠一层。
 *
 * 入口只有一个：发现 /discover（图文 / 视频 / 音乐混排）。
 * 点卡片后按内容类型落到各自的独立路由，可深链、可刷新、可分享：
 *   video → /video/:id
 *   music → /music/:id
 *   带图微文 → /image/:id
 *   纯文字微文 → /micro/:id
 *
 * 不要在各页面里手写 `/video/${id}`：微文/图片点进去会被塞进视频路由，
 * 分享链接也会指错地方。
 */
export type RoutePost = { id: number; kind?: string; images?: string[] | unknown };

export function postPath(p: RoutePost): string {
  const kind = p.kind || 'micro';
  if (kind === 'video') return `/video/${p.id}`;
  if (kind === 'music') return `/music/${p.id}`;
  const images = Array.isArray(p.images) ? p.images : [];
  return images.length ? `/image/${p.id}` : `/micro/${p.id}`;
}

export const POST_KIND_LABEL: Record<string, string> = {
  video: '视频',
  music: '音乐',
  micro: '微文',
  image: '图片',
};
