/**
 * 敏感分区检测工具
 * 从后端 API 动态获取敏感分区配置
 */

import { publicApi } from '@/api/admin';

let SENSITIVE_ZONES: string[] = [];
let zonesLoaded = false;

/** 检查文本是否匹配任意敏感分区（支持子串匹配） */
function matchZone(text: string): string | null {
  if (!text) return null;
  if (SENSITIVE_ZONES.includes(text)) return text;
  for (const zone of SENSITIVE_ZONES) {
    if (text.includes(zone)) return zone;
    if (zone.includes(text) && text.length >= 2) return zone;
  }
  return null;
}

const CONFIRMED_KEY = 'nvs-confirmed-zones';
const CONFIRMED_EXPIRY_MS = 24 * 60 * 60 * 1000; // 24 小时后需重新确认

interface ConfirmedEntry {
  zone: string;
  time: number;
}

/** 从后端加载敏感分区列表 */
async function loadZones(): Promise<string[]> {
  if (zonesLoaded) return SENSITIVE_ZONES;
  try {
    const res = await publicApi.getSiteInfo();
    if (res.data.code === 0) {
      const data = res.data.data;
      if (data.wall_enabled && Array.isArray(data.wall_zones)) {
        SENSITIVE_ZONES = data.wall_zones;
      }
    }
  } catch { /* fallback to empty */ }
  zonesLoaded = true;
  return SENSITIVE_ZONES;
}

/** 获取已确认分区（带过期时间） */
function getConfirmedZones(): Set<string> {
  try {
    const raw = localStorage.getItem(CONFIRMED_KEY);
    if (!raw) return new Set();
    const entries: ConfirmedEntry[] = JSON.parse(raw);
    const now = Date.now();
    const valid = entries.filter(e => now - e.time < CONFIRMED_EXPIRY_MS);
    // 清理过期条目
    if (valid.length !== entries.length) {
      localStorage.setItem(CONFIRMED_KEY, JSON.stringify(valid));
    }
    return new Set(valid.map(e => e.zone));
  } catch { return new Set(); }
}

/** 标记分区为已确认 */
export function markZoneConfirmed(zone: string) {
  try {
    const raw = localStorage.getItem(CONFIRMED_KEY);
    const entries: ConfirmedEntry[] = raw ? JSON.parse(raw) : [];
    // 移除旧条目（如果有）
    const filtered = entries.filter(e => e.zone !== zone);
    filtered.push({ zone, time: Date.now() });
    // 清理过期
    const now = Date.now();
    const valid = filtered.filter(e => now - e.time < CONFIRMED_EXPIRY_MS);
    localStorage.setItem(CONFIRMED_KEY, JSON.stringify(valid));
  } catch {}
}

/** 获取上一访问分区 */
export function getLastZone(): string {
  return sessionStorage.getItem('nvs-last-zone') || '';
}

/** 记录当前分区 */
export function setLastZone(zone: string) {
  sessionStorage.setItem('nvs-last-zone', zone);
}

/** 判断是否需要确认弹窗 */
export async function shouldShowGuard(
  zone: string,
  opts?: { authorId?: number; userId?: number; wallEnabled?: boolean },
): Promise<{ needed: boolean; isCrossDomain: boolean; zoneName: string } | null> {
  await loadZones();
  const matchedZone = matchZone(zone);
  if (!matchedZone) return null;
  zone = matchedZone;

  // 作者关闭了隔离墙 → 跳过
  if (opts?.wallEnabled === false) return null;

  // 作者豁免：如果当前用户就是该区作品的作者，不需要确认
  if (opts?.authorId && opts?.userId && opts.authorId === opts.userId) {
    return null;
  }

  // 已确认过（24小时内） → 跳过
  const confirmed = getConfirmedZones();
  if (confirmed.has(zone)) return null;

  const lastZone = getLastZone();
  const isCrossDomain = SENSITIVE_ZONES.includes(lastZone) && lastZone !== zone;

  return {
    needed: true,
    isCrossDomain,
    zoneName: zone,
  };
}
