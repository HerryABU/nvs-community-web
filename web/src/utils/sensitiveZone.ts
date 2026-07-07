/**
 * 敏感分区检测工具
 */

const SENSITIVE_ZONES = ['同人区', '政治文学区', '政治区', '讽刺文学', '泛二次元区'];
const CONFIRMED_KEY = 'nvs-confirmed-zones';
const VISIT_KEY = 'nvs-zone-visits';

/** 获取用户已确认过的分区集合 */
function getConfirmedZones(): Set<string> {
  try {
    const raw = localStorage.getItem(CONFIRMED_KEY);
    return raw ? new Set(JSON.parse(raw)) : new Set();
  } catch { return new Set(); }
}

/** 获取分区访问计数 */
function getZoneVisits(): Record<string, number> {
  try {
    const raw = localStorage.getItem(VISIT_KEY);
    return raw ? JSON.parse(raw) : {};
  } catch { return {}; }
}

/** 记录一次分区访问 */
export function recordZoneVisit(zone: string) {
  if (!SENSITIVE_ZONES.includes(zone)) return;
  const visits = getZoneVisits();
  visits[zone] = (visits[zone] || 0) + 1;
  localStorage.setItem(VISIT_KEY, JSON.stringify(visits));
}

/** 标记分区为已确认 */
export function markZoneConfirmed(zone: string) {
  const zones = getConfirmedZones();
  zones.add(zone);
  localStorage.setItem(CONFIRMED_KEY, JSON.stringify([...zones]));
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
export function shouldShowGuard(
  zone: string,
  opts?: { authorId?: number; userId?: number },
): { needed: boolean; isCrossDomain: boolean; zoneName: string } | null {
  if (!SENSITIVE_ZONES.includes(zone)) return null;

  // 作者豁免：如果当前用户就是该区作品的作者，不需要确认
  if (opts?.authorId && opts?.userId && opts.authorId === opts.userId) {
    return null;
  }

  const confirmed = getConfirmedZones();

  // 已确认过这个分区 → 跳过
  if (confirmed.has(zone)) return null;

  // 读者倾向检测：访问 ≥ 3 次视为常客，跳过
  const visits = getZoneVisits();
  if ((visits[zone] || 0) >= 3) {
    return null;
  }

  const lastZone = getLastZone();
  // 跨域移动（从一个敏感区到另一个敏感区）
  const isCrossDomain = SENSITIVE_ZONES.includes(lastZone) && lastZone !== zone;

  return {
    needed: true,
    isCrossDomain,
    zoneName: zone,
  };
}
