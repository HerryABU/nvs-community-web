/**
 * 群二维码内容解析（纯本地，不上传任何图片）
 *
 * 支持的二维码载荷：
 *   1) NVS 分享链接：https://host/xxx/group/12?no=1234567
 *   2) 纯群号：1234567
 *   3) 邀请码：ABCD2345
 *   4) 带前缀的文本：群号:1234567 / NVS-GROUP:1234567
 */
export interface GroupQrPayload {
  no: string;
  kind: 'no' | 'code' | 'unknown';
}

/** 从扫码/粘贴的二维码文本中提取群号或邀请码 */
export function parseGroupQr(text: string): GroupQrPayload {
  const raw = (text || '').trim();
  if (!raw) return { no: '', kind: 'unknown' };

  // 1) URL：优先取 no 参数，其次 /group/:id 后跟的群号
  if (/^https?:\/\//i.test(raw)) {
    try {
      const u = new URL(raw);
      const no = u.searchParams.get('no') || u.searchParams.get('group_no') || '';
      if (no) return { no: normalize(no), kind: kindOf(no) };
      const m = u.pathname.match(/\/group\/(\d+)/i);
      if (m) return { no: m[1], kind: 'no' };
    } catch {
      /* 落到文本解析 */
    }
  }
  // 2) 前缀文本
  const pref = raw.match(/(?:群号|群组号|NVS-GROUP|GROUP)\s*[:：]?\s*([0-9A-Za-z]{5,12})/i);
  if (pref) return { no: normalize(pref[1]), kind: kindOf(pref[1]) };

  // 3) 裸群号 / 邀请码
  const bare = raw.match(/^([0-9A-Za-z]{5,12})$/);
  if (bare) return { no: normalize(bare[1]), kind: kindOf(bare[1]) };

  return { no: '', kind: 'unknown' };
}

function normalize(s: string) {
  return s.trim().toUpperCase();
}

function kindOf(s: string): 'no' | 'code' {
  return /^\d+$/.test(s) ? 'no' : 'code';
}

/** 群分享链接（用于生成二维码；带 no 便于扫码后直接定位） */
export function groupShareUrl(baseUrl: string, groupID: number, groupNo: string): string {
  const base = (baseUrl || '').replace(/\/+$/, '');
  return `${base}/group/${groupID}?no=${encodeURIComponent(groupNo)}`;
}
