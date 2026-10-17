import { ref, getCurrentInstance, onUnmounted, watch, toValue, type MaybeRefOrGetter } from 'vue';
import { buildLiveStreamURL } from '@/api/live';

// F19 狂暴模式：评论区实时更新 composable。
// 底层 EventSource（SSE）订阅 realtime Hub 推送的新评论。
//
// 重连策略（断连由前端负责，服务端 15s 心跳探测）：
//   指数退避 1s → 2s → 4s → 8s → … 封顶 30s，每次叠加 ±20% Jitter。
//   - 指数退避：避免服务端未就绪时高频重连打爆接口；
//   - Jitter：大量客户端同时断线（如服务端重启）时打散重连时刻，防「惊群」；
//   - 连接成功（open）后立即重置为 1s，恢复低延迟重连。
// 说明：EventSource 无法携带自定义 header，登录态依赖 HttpOnly cookie
// （同源自动携带，与 api 实例 withCredentials 行为一致）。
const BACKOFF_BASE_MS = 1000;
const BACKOFF_MAX_MS = 30000;

// 频道可为普通字符串或响应式来源（ref / getter）。
// 传 getter 时，频道从空变为有效值（如异步确认狂暴模式开启后）会自动重建连接。
export function useLiveComments(channel: MaybeRefOrGetter<string>, onComment: (comment: any) => void) {
  const connected = ref(false);
  const error = ref('');

  let es: EventSource | null = null;
  let retryTimer: ReturnType<typeof setTimeout> | null = null;
  let attempt = 0; // 连续失败次数（决定退避档位）
  let closed = false; // 主动关闭（组件卸载/服务端明确禁用）后不再重连

  function teardown() {
    if (retryTimer !== null) {
      clearTimeout(retryTimer);
      retryTimer = null;
    }
    es?.close();
    es = null;
  }

  function connect() {
    if (closed) return;
    const ch = toValue(channel);
    if (!ch) return; // 频道未就绪（如狂暴模式未开启），不建连接
    // 关闭旧连接再建，避免叠加
    teardown();

    es = new EventSource(buildLiveStreamURL(ch), { withCredentials: true });

    es.addEventListener('open', () => {
      connected.value = true;
      error.value = '';
      attempt = 0; // 连接成功 → 退避档位归零
    });

    // 服务端批量 flush 后仍按「一条评论一个 message」下发（hub 拆帧保证）
    es.addEventListener('comment', (ev) => {
      try {
        onComment(JSON.parse((ev as MessageEvent).data));
      } catch {
        /* 忽略无法解析的坏包，不断流 */
      }
    });

    es.addEventListener('error', () => {
      connected.value = false;
      error.value = '实时连接断开，重试中';
      // EventSource 内置重连由浏览器按 retry 字段控制，这里改为自管退避，
      // 先关闭再按指数退避重建，行为可预期
      teardown();
      scheduleReconnect();
    });

    // 服务端明确告知「未开启狂暴模式」：永久停止重连（避免对未开启站点空转重试）
    es.addEventListener('disabled', () => {
      closed = true;
      connected.value = false;
      error.value = '实时评论（狂暴模式）未开启';
      teardown();
    });
  }

  function scheduleReconnect() {
    if (closed || retryTimer !== null) return;
    const backoff = Math.min(BACKOFF_BASE_MS * 2 ** attempt, BACKOFF_MAX_MS);
    const jitter = backoff * 0.2 * (Math.random() * 2 - 1); // ±20%
    attempt += 1;
    retryTimer = setTimeout(() => {
      retryTimer = null;
      connect();
    }, backoff + jitter);
  }

  /** 主动断开（组件卸载/离开评论区时调用），幂等 */
  function close() {
    closed = true;
    connected.value = false;
    teardown();
  }

  // 频道响应式变化（如从空变为有效）：软重连（不置 closed，可再次重建）
  if (typeof channel === 'function' || (typeof channel === 'object' && channel !== null && 'value' in channel)) {
    watch(
      () => toValue(channel),
      () => {
        if (closed) return;
        attempt = 0;
        connect();
      },
    );
  }

  connect();

  // 组件卸载自动断开，防泄漏；非 setup 上下文调用时静默跳过
  if (getCurrentInstance()) {
    onUnmounted(close);
  }

  return { connected, error, close };
}
