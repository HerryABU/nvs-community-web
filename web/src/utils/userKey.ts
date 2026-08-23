import { publicApi } from '@/api/admin';

// ─────────────────────────────────────────────────────────────
// 用户标识统一索引方法（数字=主键，用户名兼容访问）
//
// 全站统一入口：任何需要把 URL 参数（可能是数字 id 或用户名）解析成
// 用户信息的页面/组件，都走 resolveUserKey，禁止各自 Number() 转换
// （Number("qbz95") = NaN 是历史 bug 根源）。
// 规则：纯数字 → 直接作为主键 id（不请求后端）；其余 → /api/users/resolve
// 按用户名解析（大小写不敏感）。带内存缓存避免重复请求。
// ─────────────────────────────────────────────────────────────

interface ResolvedUser {
  id: number;
  username: string;
  nickname: string;
  role: string;
}

const cache = new Map<string, ResolvedUser>();

export function clearUserKeyCache() {
  cache.clear();
}

/**
 * 解析用户标识：数字主键 或 用户名。
 * @param key URL 参数（可能为 string | string[]）
 * @param opts.verifyNumeric 为 true 时，纯数字也请求后端验证用户是否存在
 *        （用于需要区分「数字=用户 还是 数字=其他对象 id」的页面，如博客主页/博客管理）
 * @returns 解析结果；失败返回 null
 */
export async function resolveUserKey(
  key: string | string[] | undefined | null,
  opts?: { verifyNumeric?: boolean },
): Promise<ResolvedUser | null> {
  const k = Array.isArray(key) ? key[0] : key;
  if (!k) return null;
  const kStr = String(k).trim();
  if (!kStr) return null;

  // 纯数字：主键优先；默认直接使用（避免额外请求）
  if (/^\d+$/.test(kStr) && !opts?.verifyNumeric) {
    return { id: Number(kStr), username: kStr, nickname: '', role: '' };
  }

  const cacheKey = (opts?.verifyNumeric ? 'v:' : '') + kStr.toLowerCase();
  if (cache.has(cacheKey)) return cache.get(cacheKey)!;

  try {
    const res = await publicApi.resolveUser(kStr);
    if (res.data.code === 0 && res.data.data) {
      const u: ResolvedUser = {
        id: res.data.data.id,
        username: res.data.data.username,
        nickname: res.data.data.nickname,
        role: res.data.data.role,
      };
      cache.set(cacheKey, u);
      return u;
    }
  } catch { /* ignore */ }
  return null;
}
