import { onBeforeUnmount, ref } from 'vue';
import { openGroupStream, type GroupStreamEvent } from '@/api/group/stream';

/**
 * 群组实时消息生命周期管理
 *
 * - 断线指数退避重连（1s → 2s → 4s … 上限 30s）；
 * - 页面隐藏（切后台/锁屏）时主动断开省资源，回到前台立即重连；
 * - 组件卸载 / 群 ID 变化时自动停止，避免连接泄漏。
 */
export function useGroupStream(
  groupId: () => number,
  onEvent: (name: string, data: any) => void,
) {
  const connected = ref(false);
  const error = ref('');

  let controller: AbortController | null = null;
  let retryTimer: number | null = null;
  let retry = 0;
  let stopped = true;

  function clearRetry() {
    if (retryTimer) {
      window.clearTimeout(retryTimer);
      retryTimer = null;
    }
  }

  function scheduleRetry() {
    if (stopped) return;
    const delay = Math.min(30000, 1000 * Math.pow(2, Math.min(retry, 5)));
    retry += 1;
    clearRetry();
    retryTimer = window.setTimeout(() => {
      retryTimer = null;
      void connect();
    }, delay);
  }

  async function connect() {
    if (stopped) return;
    const id = groupId();
    if (!id || document.hidden) return;

    controller?.abort();
    const c = new AbortController();
    controller = c;
    error.value = '';
    try {
      await openGroupStream(id, {
        signal: c.signal,
        onOpen: () => {
          connected.value = true;
          retry = 0;
        },
        onEvent: (ev: GroupStreamEvent) => onEvent(ev.name, ev.data),
      });
      // 正常结束（服务端关闭/心跳超时）→ 退避重连
      connected.value = false;
      scheduleRetry();
    } catch (e: any) {
      connected.value = false;
      if (c.signal.aborted) return; // 主动断开不重连
      error.value = e?.message || '实时连接中断';
      scheduleRetry();
    }
  }

  function start() {
    if (!stopped) return;
    stopped = false;
    retry = 0;
    void connect();
  }

  function stop() {
    stopped = true;
    connected.value = false;
    clearRetry();
    controller?.abort();
    controller = null;
  }

  function onVisibility() {
    if (document.hidden) {
      controller?.abort();
      controller = null;
      connected.value = false;
      clearRetry();
    } else if (!stopped) {
      retry = 0;
      void connect();
    }
  }

  document.addEventListener('visibilitychange', onVisibility);
  start();

  onBeforeUnmount(() => {
    document.removeEventListener('visibilitychange', onVisibility);
    stop();
  });

  return { connected, error, start, stop };
}
