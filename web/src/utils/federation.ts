import { publicApi } from '@/api/admin';

// 联邦条目混入：按 kind 从服务端取对应类型，绝不在前端跨类型混用。
// 服务端 /api/federated/novels 已做 kind 白名单过滤，这里再按 kind 兜底一次。
export type FedKind = 'novel' | 'blog' | 'forum' | 'micro' | 'subculture';

export interface FedLike {
  id: number;
  kind: FedKind;
  title: string;
  author: string;
  category: string;
  summary: string;
  cover_url: string;
  source_url: string;
  proxy_path: string;
  origin_site: string;
  source_chain: string;
  hop_count: number;
  site_name: string;
  _federated: true;
}

let modes: { embed_enabled?: boolean; independent_enabled?: boolean } | null = null;
export function setFederationModes(v: any) { modes = v; }

export async function loadFederated(kind: FedKind, limit = 8): Promise<FedLike[]> {
  // 未开启嵌入模式时不混入（独立联邦页仍可用）
  if (modes && !modes.embed_enabled && !modes.independent_enabled) return [];
  try {
    const res = await publicApi.getFederatedNovels({ page: 1, page_size: limit, kind });
    const list = res.data?.data?.list || [];
    return list
      .filter((x: any) => !x.kind || x.kind === kind) // 兜底：类型必须一致
      .map((x: any) => ({
        ...x,
        id: x.remote_id ?? x.id,
        kind: x.kind || kind,
        site_name: x.site?.name || x.origin_site || '友站',
        _federated: true as const,
      }));
  } catch {
    return [];
  }
}

// 把联邦条目混入本地列表：用 kind+id 复合键去重，联邦条目放后面（站内优先）。
export function mergeFederated<T extends { id: number | string }>(
  local: T[], fed: FedLike[], make: (f: FedLike) => T,
): T[] {
  const keys = new Set(local.map((x) => String(x.id)));
  const out = [...local];
  for (const f of fed) {
    const key = `${f.kind}:${f.id}`;
    if (keys.has(key)) continue;
    keys.add(key);
    out.push(make(f));
  }
  return out;
}

// 统一的跳转目标：优先本站反代路径，回退原始外链。
export function fedHref(f: FedLike): string {
  return f.proxy_path || f.source_url;
}
