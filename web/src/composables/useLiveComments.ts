import { ref, getCurrentInstance, onUnmounted, watch, toValue, type MaybeRefOrGetter } from 'vue';
import { buildLiveStreamURL, buildLiveWSURL } from '@/api/home/live';

// F19 狂暴模式：评论区实时更新 composable。
// 底层订阅 realtime Hub 推送的新评论，传输层支持 SSE 与 WebSocket：
//   - transport: 'sse'（默认）—— EventSource 长连接，登录态走 HttpOnly cookie；
//   - transport: 'ws'        —— WebSocket 长连接（浏览器友好），升级失败/未开启时自动回退 SSE。
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
// 传 getter 时，频道从空变为有效值（如狂暴模式开启后）会自动重建连接。
export function useLiveComments(
  channel: MaybeRefOrGetter<string>,
  onComment: (comment: any) => void,
  options?: { transport?: 'sse' | 'ws' },
) {
  const connected = ref(false);
  const error = ref('');

  const preferredTransport = options?.transport ?? 'sse';
  let usingTransport: 'sse' | 'ws' = preferredTransport; // 回退后锁定为 sse，避免 ws↔sse 抖动

  let es: EventSource | null = null;
  let ws: WebSocket | null = null;
  let retryTimer: ReturnType<typeof setTimeout> | null = null;
  let attempt = 0; // 连续失败次数（决定退避档位）
  let closed = false; // 主动关闭（组件卸载/服务端明确禁用）后不再重连
  let wsOpened = false; // WS 是否曾成功建立（区分「升级失败」与「建立后断开」）
  let fellBack = false; // 是否已从 ws 回退过 sse（仅回退一次）

  function teardown() {
    if (retryTimer !== null) {
      clearTimeout(retryTimer);
      retryTimer = null;
    }
    es?.close();
    es = null;
    ws?.close();
    ws = null;
  }

  // ───────────── SSE 通道 ─────────────
  function connectSSE() {
    if (closed) return;
    const ch = toValue(channel);
    if (!ch) return; // 频道未就绪（如狂暴模式未开启），不建连接
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

  // ───────────── WebSocket 通道 ─────────────
  function connectWS() {
    if (closed) return;
    const ch = toValue(channel);
    if (!ch) return;
    teardown();

    let sock: WebSocket;
    try {
      sock = new WebSocket(buildLiveWSURL(ch));
    } catch {
      // 环境无法构造 WebSocket（异常） → 回退 SSE（仅一次）
      if (!fellBack) {
        fellBack = true;
        usingTransport = 'sse';
        connectSSE();
      }
      return;
    }
    ws = sock;
    wsOpened = false;

    sock.onopen = () => {
      wsOpened = true;
      connected.value = true;
      error.value = '';
      attempt = 0;
    };

    // 服务端按 {"name":"comment","data":{...}} 下发（批量信封已拆成多条消息）
    sock.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data as string);
        if (msg && msg.name === 'comment') onComment(msg.data);
      } catch {
        /* 忽略坏包 */
      }
    };

    sock.onerror = () => {
      /* error 后浏览器必触发 close，统一在 close 里处理 */
    };

    sock.onclose = () => {
      connected.value = false;
      ws = null;
      if (!wsOpened) {
        // 升级失败（未开启 / 反向代理不支持 WebSocket）→ 回退 SSE（仅一次）
        if (!fellBack) {
          fellBack = true;
          usingTransport = 'sse';
          connectSSE();
        }
        return;
      }
      error.value = '实时连接断开，重试中';
      scheduleReconnect();
    };
  }

  // ───────────── 统一入口 ─────────────
  function connect() {
    if (closed) return;
    const ch = toValue(channel);
    if (!ch) return;
    teardown();
    if (usingTransport === 'ws') connectWS();
    else connectSSE();
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
