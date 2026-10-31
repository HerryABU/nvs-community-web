/**
 * 剪贴板写入 —— 带降级路径。
 *
 * navigator.clipboard 只在「安全上下文」（https / localhost）可用，
 * 内网 http 部署、iframe 内嵌、旧 Chromium 都会直接 undefined。
 * 所以必须保留 execCommand('copy') 兜底，否则这些环境下复制功能是死的。
 */
export async function copyText(text: string): Promise<boolean> {
  // 首选异步剪贴板 API
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      /* 权限被拒 / 非安全上下文 → 落到兜底 */
    }
  }
  // 兜底：临时 textarea + execCommand（已废弃但兼容性最好）
  try {
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.setAttribute('readonly', '');
    ta.style.cssText = 'position:fixed;top:0;left:-9999px;opacity:0;';
    document.body.appendChild(ta);
    ta.select();
    ta.setSelectionRange(0, ta.value.length);
    const ok = document.execCommand('copy');
    ta.remove();
    return ok;
  } catch {
    return false;
  }
}
