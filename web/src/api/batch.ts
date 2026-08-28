import api from './index';

export interface BatchRequest {
  id?: string;                          // 可选，响应关联 ID
  method?: string;                      // 可选，默认 GET
  path: string;                         // 必填，如 /api/novels
  query?: Record<string, string>;       // 可选，URL query 参数
  body?: unknown;                       // 可选，POST 的 JSON body
}

export interface BatchResult<T = unknown> {
  id: string;
  status: number;
  code: number;
  message: string;
  data: T;
  success: boolean;
  error: string;
}

export interface BatchResponse {
  code: number;
  message: string;
  data: {
    results: BatchResult[];
    total: number;
    succeeded: number;
    failed: number;
  };
}

/**
 * API 聚合中间层：一次 HTTP 请求批量获取多个接口数据，服务器并发执行并一次性返回。
 * 原有各 API 仍可独立直接调用（向后兼容）。
 *
 * @example
 * const res = await batchApi.batch([
 *   { id: 'novels', path: '/api/novels', query: { page_size: '10' } },
 *   { id: 'cats', path: '/api/categories' },
 *   { id: 'site', path: '/api/site-info' },
 * ]);
 */
export const batchApi = {
  batch(requests: BatchRequest[], timeoutMs?: number) {
    return api.post<BatchResponse>('/batch', {
      requests,
      timeout_ms: timeoutMs,
    });
  },
};

/**
 * 便捷封装：一次请求批量拉取多个 GET 接口，返回以 id 为键的结果映射。
 * 服务器并发执行并一次性返回，浏览器只需 1 次 HTTP 往返（减少请求数/连接数/带宽）。
 *
 * @example
 * const map = await batchGet([
 *   { id: 'cats',  path: '/api/categories' },
 *   { id: 'site',  path: '/api/site-info' },
 *   { id: 'news',  path: '/api/novels', query: { page: '1', page_size: '60' } },
 * ]);
 * if (map.cats?.success) categories.value = map.cats.data;
 */
export async function batchGet(
  specs: { id: string; path: string; query?: Record<string, string> }[],
): Promise<Record<string, BatchResult>> {
  const res = await batchApi.batch(
    specs.map((s) => ({ id: s.id, method: 'GET', path: s.path, query: s.query })),
  );
  const map: Record<string, BatchResult> = {};
  for (const r of res.data.data.results) {
    map[r.id] = r;
  }
  return map;
}
