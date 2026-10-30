// 设备指纹采集 —— 用于令牌绑定（防 JWT 窃取 + 异地登录）。
//
// ⚠️ 与「蜜罐指纹」是两个独立通道：此处采集的是真实用户指纹，不含任何诱饵 / 陷阱逻辑；
//    蜜罐指纹在 server/honeypot 单独采集并落库，二者不可混用。
//
// 方案：canvas 渲染 + WebGL 渲染器 + 基础环境特征 → 稳定哈希（SHA-256，失败降级 FNV-1a）。
// 采集失败（隐私模式 / 无 canvas）时降级为空串，此时令牌绑定退化为「仅地域」单因子（仍可防异地）。
// 同一会话内结果缓存，保证登录 / 刷新 / 业务请求携带的是同一个指纹。

let cached: string | null = null;
let pending: Promise<string> | null = null;

async function hashRaw(raw: string): Promise<string> {
  try {
    if (typeof crypto !== 'undefined' && crypto.subtle) {
      const buf = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(raw));
      return Array.from(new Uint8Array(buf))
        .map((b) => b.toString(16).padStart(2, '0'))
        .join('');
    }
  } catch {
    /* 降级到 FNV-1a */
  }
  // FNV-1a 32-bit 兜底（无依赖）
  let h = 0x811c9dc5;
  for (let i = 0; i < raw.length; i++) {
    h ^= raw.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return (h >>> 0).toString(16).padStart(8, '0');
}

export async function collectFingerprint(): Promise<string> {
  if (cached !== null) return cached;
  if (pending) return pending;

  pending = (async () => {
    try {
      const parts: string[] = [];

      // 1) Canvas 指纹（ASCII + 中文混排，不同字体 / 系统渲染像素差异显著）
      const canvas = document.createElement('canvas');
      const ctx = canvas.getContext('2d');
      if (ctx) {
        ctx.textBaseline = 'top';
        ctx.font = "16px 'Arial'";
        ctx.fillStyle = '#f60';
        ctx.fillRect(125, 1, 62, 20);
        ctx.fillStyle = '#069';
        ctx.fillText('NVS fp 指纹测试 abc123', 2, 15);
        ctx.fillStyle = 'rgba(102,204,0,0.7)';
        ctx.fillText('NVS fp 指纹测试 abc123', 4, 17);
        parts.push('canvas:' + canvas.toDataURL());
      }

      // 2) WebGL 指纹（显卡 / 驱动层面的强区分维度）
      const gl = document.createElement('canvas').getContext('webgl') as WebGLRenderingContext | null;
      if (gl) {
        const dbg = gl.getExtension('WEBGL_debug_renderer_info');
        const vendor = dbg ? gl.getParameter(dbg.UNMASKED_VENDOR_WEBGL) : gl.getParameter(gl.VENDOR);
        const renderer = dbg
          ? gl.getParameter(dbg.UNMASKED_RENDERER_WEBGL)
          : gl.getParameter(gl.RENDERER);
        parts.push('webgl:' + String(vendor) + '|' + String(renderer));
      }

      // 3) 基础环境特征（稳定维度）
      parts.push('ua:' + (navigator.userAgent || ''));
      parts.push('lang:' + (navigator.language || ''));
      parts.push('tz:' + new Date().getTimezoneOffset());
      parts.push(
        'scr:' +
          (window.screen.width + 'x' + window.screen.height + 'x' + window.screen.colorDepth),
      );
      parts.push('plat:' + (navigator.platform || ''));
      parts.push('touch:' + (('ontouchstart' in window) ? '1' : '0'));

      const raw = parts.join('§');
      const h = await hashRaw(raw);
      // 末 4 位再混入原始长度，进一步降低碰撞概率
      cached = h + (raw.length % 4096).toString(16).padStart(3, '0');
      return cached;
    } catch {
      cached = '';
      return '';
    } finally {
      pending = null;
    }
  })();

  return pending;
}
