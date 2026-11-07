import api from '@/api/index';
import { appPrefix } from '@/utils/prefix';

// F19 狂暴模式（评论区实时更新）API：
//   - SSE 长连接直连（不走 axios：EventSource 无法带自定义 header，
//     登录态走 HttpOnly cookie 自动携带，与现有 api 实例 withCredentials 一致）
//   - WebSocket 长连接（浏览器友好，复用同一 Hub；管理端按需开启）
//   - MQTT / gRPC 等外部代理由 realtime.Manager 按配置拉起，前端暂不直连
//   - 管理端开关与带宽参数配置

export interface RageConfig {
  enabled: boolean;            // 总开关（默认关闭）
  batch_interval_sec: number;  // 批量 flush 周期（秒，1-60）
  batch_size: number;          // pending 累积立即 flush 阈值（1-100）
  max_conns: number;           // 全局连接数上限（100-10000）
  ws_enabled: boolean;         // WebSocket 传输层（浏览器友好，复用 Hub.Client）
  mqtt_enabled: boolean;       // 内嵌 MQTT Broker（独立端口，默认 1883）
  mqtt_bind: string;           // MQTT 监听地址（默认 0.0.0.0）
  mqtt_port: number;           // MQTT 端口（默认 1883）
  grpc_enabled: boolean;       // 内嵌 gRPC 服务端（HTTP/2 流式，默认 50051）
  grpc_bind: string;           // gRPC 监听地址（默认 0.0.0.0）
  grpc_port: number;           // gRPC 端口（默认 50051）
}

/** 狂暴模式公开探测配置（前端择优传输层：ws > sse） */
export interface LiveConfig {
  enabled: boolean;            // 总开关（关闭则不建任何实时连接）
  ws_enabled: boolean;         // WebSocket 传输层是否开启
  mqtt_enabled: boolean;       // MQTT Broker 是否开启（外部代理，前端不直接连）
  mqtt_port: number;           // MQTT 端口（仅信息展示）
  grpc_enabled: boolean;       // gRPC 服务端是否开启（外部代理，前端不直接连）
  grpc_port: number;           // gRPC 端口（仅信息展示）
}

/**
 * 拼接 SSE 流地址。channel 形如 comment:novel:<小说ID> / comment:blog:<博客ID>。
 * 与 api 实例相同的路径前缀规则（apiPrefix + /api），保证反代前缀场景下可达。
 */
export function buildLiveStreamURL(channel: string): string {
  return `${appPrefix}/api/live/stream?channel=${encodeURIComponent(channel)}`;
}

/**
 * 拼接 WebSocket 流地址（狂暴模式可选传输层）。协议随页面 https 自动升级为 wss，
 * 路径带反代前缀（如 /go-nvs），与 SSE 端点同源可直达。
 */
export function buildLiveWSURL(channel: string): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${proto}//${window.location.host}${appPrefix}/api/live/ws?channel=${encodeURIComponent(channel)}`;
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
  // 公开探测：狂暴模式与各传输层可用状态（前端挂载时先查，关闭则不建连接；ws>sse 择优）
  publicConfig() {
    return api.get<LiveConfig>('/live/config');
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
