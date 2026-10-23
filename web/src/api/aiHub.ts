import api from './index';

// AI 工作台（§5.4）：流式对话 / 绘画 / 视频 / 智能体
// 注意：baseURL 已含前缀（apiPrefix+/api），严禁在此硬编码 /go-nvs。
// 流式接口不走 axios（需读取 SSE 增量），用 fetch + ReadableStream。

export interface AiHubModel {
  id: number;
  name: string;
  model_id: string;
  price_in: number;
  price_out: number;
  price_call: number;
  max_tokens: number;
  /** 上游协议族：openai / mj / gemini */
  api_kind?: string;
  provider_hint?: string;
  note?: string;
}

export interface AiHubStatus {
  ai_enabled: boolean;
  token_enabled: boolean;
  token_name: string;
  balance: number;
  free_quota?: {
    enabled: boolean;
    calls_left: number;
    tokens_left: number;
    used_calls: number;
    used_tokens: number;
    calls_per_day: number;
    max_tokens_per_day: number;
  };
  chat_models: AiHubModel[];
  image_models: AiHubModel[];
  video_models: AiHubModel[];
}

export interface AiAgent {
  id: number;
  name: string;
  emoji: string;
  description: string;
  system_prompt: string;
  greeting: string;
  enabled: boolean;
  sort_order: number;
}

export interface AiChatSession {
  id: number;
  agent_id: number;
  agent_name: string;
  model_id: number;
  title: string;
  share_token?: string | null;
  shared_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface AiChatMessage {
  id: number;
  role: 'user' | 'assistant';
  content: string;
  tokens_in: number;
  tokens_out: number;
  cost: number;
  free?: boolean;
  created_at: string;
}

export const aiHubApi = {
  status() {
    return api.get<{ code: number; data: AiHubStatus }>('/ai/hub/status');
  },
  agents() {
    return api.get<{ code: number; data: { list: AiAgent[] } }>('/ai/agents');
  },
  sessions() {
    return api.get<{ code: number; data: { list: AiChatSession[] } }>('/ai/sessions');
  },
  sessionMessages(id: number) {
    return api.get<{ code: number; data: { session: AiChatSession; messages: AiChatMessage[] } }>(`/ai/sessions/${id}/messages`);
  },
  deleteSession(id: number) {
    return api.delete(`/ai/sessions/${id}`);
  },
  renameSession(id: number, title: string) {
    return api.put(`/ai/sessions/${id}`, { title });
  },
  shareSession(id: number) {
    return api.post<{ code: number; data: { share_token: string; path: string } }>(`/ai/sessions/${id}/share`);
  },
  unshareSession(id: number) {
    return api.delete(`/ai/sessions/${id}/share`);
  },
  /** 公开只读快照（无需登录） */
  sharedSession(token: string) {
    return api.get<{
      code: number;
      data: {
        title: string;
        agent_name: string;
        shared_at: string | null;
        messages: Array<{ role: 'user' | 'assistant'; content: string; created_at: string }>;
      };
    }>(`/ai/shared/${token}`);
  },
  image(data: { model_id: number; prompt: string; size?: string; style?: string; n?: number; byok?: boolean; byok_model?: string }) {
    return api.post<{ code: number; data: { task_id: number; images: string[]; cost: number; free?: boolean; byok?: boolean; balance_after: number } }>('/ai/image', data);
  },
  videoSubmit(data: { model_id: number; prompt: string; duration?: number; size?: string; byok?: boolean; byok_model?: string }) {
    return api.post<{ code: number; data: { task_id: number; status: string; cost: number; free?: boolean; byok?: boolean; balance_after: number } }>('/ai/video', data);
  },
  videoStatus(id: number) {
    return api.get<{ code: number; data: { task_id: number; status: string; video?: string; result?: string; error?: string; cost: number } }>(`/ai/video/${id}`);
  },
  mediaHistory(kind: 'image' | 'video', page = 1) {
    return api.get<{ code: number; data: { list: any[]; total: number } }>('/ai/media', { params: { kind, page } });
  },

  // 管理端智能体
  adminAgents() {
    return api.get<{ code: number; data: { list: any[] } }>('/admin/ai/agents');
  },
  adminCreateAgent(data: any) {
    return api.post('/admin/ai/agents', data);
  },
  adminUpdateAgent(id: number, data: any) {
    return api.put(`/admin/ai/agents/${id}`, data);
  },
  adminDeleteAgent(id: number) {
    return api.delete(`/admin/ai/agents/${id}`);
  },
};

/** 流式对话：fetch 读取 SSE，onChunk 收增量，返回最终结果 */
export async function streamChat(
  payload: { session_id?: number; model_id: number; agent_id?: number; message: string; max_tokens?: number; byok?: boolean; byok_model?: string },
  onChunk: (text: string) => void,
  signal?: AbortSignal,
): Promise<{ content: string; cost: number; free?: boolean; balance_after: number; session_id: number } | null> {
  const base = (api.defaults.baseURL || '') as string;
  const resp = await fetch(`${base}/ai/chat/stream`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(payload),
    signal,
  });
  if (!resp.ok || !resp.body) {
    let msg = `请求失败（${resp.status}）`;
    try {
      const err = await resp.json();
      if (err?.message) msg = err.message;
    } catch { /* ignore */ }
    throw new Error(msg);
  }
  const reader = resp.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let content = '';
  let final: any = null;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    // SSE 事件以空行分隔
    let idx: number;
    while ((idx = buffer.indexOf('\n\n')) >= 0) {
      const rawEvent = buffer.slice(0, idx);
      buffer = buffer.slice(idx + 2);
      for (const line of rawEvent.split('\n')) {
        const t = line.trim();
        if (!t.startsWith('data:')) continue;
        const dataStr = t.slice(5).trim();
        if (!dataStr) continue;
        try {
          const ev = JSON.parse(dataStr);
          if (ev.error) throw new Error(ev.error);
          if (ev.content) {
            content += ev.content;
            onChunk(ev.content);
          }
          if (ev.done) final = ev;
        } catch (e: any) {
          if (e?.message && e.message !== 'Unexpected end of JSON input') throw e;
        }
      }
    }
  }
  if (!final) return null;
  return {
    content,
    cost: final.cost || 0,
    free: !!final.free,
    balance_after: final.balance_after || 0,
    session_id: final.session_id || 0,
  };
}
