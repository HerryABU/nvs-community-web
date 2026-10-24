import { apiPrefix } from '@/api/index';

/**
 * 群组实时消息（SSE）
 *
 * 为什么用 fetch 而不是 EventSource：
 *   - 事件源鉴权依赖 HttpOnly Cookie，EventSource 虽然会带 Cookie，但无法自定义请求头，
 *     也不便于统一处理 401/退避；fetch + ReadableStream 可以带 `X-Requested-With` 并复用 axios 的登录态。
 *   - 断线由上层 composable 控制（指数退避 / 页面隐藏暂停），这里只负责「读一帧、解析一帧」。
 */
export interface GroupStreamEvent {
  name: string;
  data: any;
}

export interface GroupStreamOptions {
  signal: AbortSignal;
  onEvent: (ev: GroupStreamEvent) => void;
  /** 连接建立（收到 hello）时回调 */
  onOpen?: () => void;
}

export async function openGroupStream(groupId: number, opts: GroupStreamOptions): Promise<void> {
  const base = apiPrefix ? `${apiPrefix}/api` : '/api';
  const resp = await fetch(`${base}/groups/${groupId}/stream`, {
    method: 'GET',
    credentials: 'include',
    headers: { Accept: 'text/event-stream', 'X-Requested-With': 'XMLHttpRequest' },
    signal: opts.signal,
  });
  if (!resp.ok || !resp.body) {
    throw new Error(`实时连接失败（HTTP ${resp.status}）`);
  }
  const reader = resp.body.getReader();
  const decoder = new TextDecoder('utf-8');
  let buf = '';

  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });
    // SSE 帧以空行分隔
    let idx = buf.indexOf('\n\n');
    while (idx >= 0) {
      const chunk = buf.slice(0, idx);
      buf = buf.slice(idx + 2);
      idx = buf.indexOf('\n\n');

      let name = 'message';
      let data = '';
      for (const rawLine of chunk.split('\n')) {
        const line = rawLine.replace(/\r$/, '');
        if (!line || line.startsWith(':')) continue; // 注释行（心跳）
        if (line.startsWith('event:')) name = line.slice(6).trim();
        else if (line.startsWith('data:')) data += line.slice(5).trim();
      }
      if (!data) continue;
      if (name === 'hello') {
        opts.onOpen?.();
        continue;
      }
      try {
        opts.onEvent({ name, data: JSON.parse(data) });
      } catch {
        /* 单帧解析失败不影响后续 */
      }
    }
  }
}
