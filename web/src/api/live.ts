import api from './index';
import { appPrefix } from '@/utils/prefix';

// F19 狂暴模式（评论区实时更新）API：
//   - SSE 长连接直连（不走 axios：EventSource 无法带自定义 header，
//     登录态走 HttpOnly cookie 自动携带，与现有 api 实例 withCredentials 一致）
//   - 管理端开关与带宽参数配置

export interface RageConfig {
  enabled: boolean;            // 总开关（默认关闭）
  batch_interval_sec: number;  // 批量 flush 周期（秒，1-60）
  batch_size: number;          // pending 累积立即 flush 阈值（1-100）
  max_conns: number;           // 全局 SSE 连接数上限（100-10000）
}

/**
 * 拼接 SSE 流地址。channel 形如 comment:novel:<小说ID> / comment:blog:<博客ID>。
 * 与 api 实例相同的路径前缀规则（apiPrefix + /api），保证反代前缀场景下可达。
 */
export function buildLiveStreamURL(channel: string): string {
  return `${appPrefix}/api/live/stream?channel=${encodeURIComponent(channel)}`;
}

/** 小说/章节评论频道 key（chapter_number 在事件负载里，前端自行按章节过滤） */
export function novelCommentChannel(novelId: number): string {
  return `comment:novel:${novelId}`;
}

/** 作者博客评论频道 key */
export function blogCommentChannel(blogId: number): string {
  return `comment:blog:${blogId}`;
}

export const liveApi = {
  // 公开探测：狂暴模式是否开启（前端挂载时先查，关闭则不建 SSE 连接）
  publicConfig() {
    return api.get<{ enabled: boolean }>('/live/config');
  },
  // 读取狂暴模式配置（管理端）
  getRageConfig() {
    return api.get<RageConfig>('/admin/rage-config');
  },
  // 保存狂暴模式配置（管理端，立即生效）
  updateRageConfig(cfg: RageConfig) {
    return api.put('/admin/rage-config', cfg);
  },
};
