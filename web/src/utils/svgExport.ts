/**
 * SVG 导出工具（预览 / PNG / 源码下载）
 *
 * 为什么单独一个文件：SVG 是「特殊的 HTML」——它既能当文档预览（沙箱里渲染），
 * 又能当图片导出（画布）。三条链路（AI 产物预览、mermaid 图、chemfig 图）都要用，
 * 各自实现一遍必然出现行为不一致（尺寸、背景、跨域污染处理各不相同）。
 *
 * 安全边界：**行内 <svg> 在用户内容里仍然禁止**（HTML_BLACKLIST + DOMPurify FORBID_TAGS）。
 * SVG 能内嵌 <script>、事件属性、<foreignObject>，放开就是存储型 XSS。
 * 本模块只处理「本来就已经是独立文档 / 已由渲染器生成」的 SVG，不改变净化策略。
 */

/** 判定一段文本是不是 SVG 文档（而不是普通 HTML）。 */
export function isSvgSource(text: string): boolean {
  const s = (text || '').trim();
  if (!s) return false;
  // 以 <svg 开头（允许 <?xml / doctype / 注释在前）
  const head = s.replace(/^(\s|<\?xml[^>]*\?>|<!--[\s\S]*?-->|<!DOCTYPE[^>]*>)*/i, '').trimStart();
  return /^<svg[\s>]/i.test(head);
}

/** 触发浏览器下载一段文本。 */
export function downloadTextFile(text: string, filename: string, mime = 'text/plain;charset=utf-8') {
  const blob = new Blob([text], { type: mime });
  triggerDownload(blob, filename);
}

/** 触发浏览器下载一个 Blob。 */
export function triggerDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.rel = 'noopener';
  document.body.appendChild(a);
  a.click();
  a.remove();
  // 立刻 revoke 会让部分浏览器拿不到内容，延后释放
  setTimeout(() => URL.revokeObjectURL(url), 4000);
}

/** 触发浏览器下载一段文本（复制到剪贴板的降级路径）。 */
export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
}

/**
 * 把 DOM 里的 <svg> 元素序列化为可独立打开的 SVG 文本。
 *
 * 两个必须处理的点：
 *  1. xmlns：DOM 序列化不会自动带上命名空间，直接存盘后用浏览器打开会不渲染；
 *  2. 尺寸：mermaid 生成的 svg 常带 viewBox 而没有宽高，直接导出尺寸会塌成 0。
 */
export function svgNodeToSvgText(svg: SVGElement): string {
  const clone = svg.cloneNode(true) as SVGElement;
  if (!clone.getAttribute('xmlns')) {
    clone.setAttribute('xmlns', 'http://www.w3.org/2000/svg');
  }
  if (!clone.getAttribute('xmlns:xlink')) {
    clone.setAttribute('xmlns:xlink', 'http://www.w3.org/1999/xlink');
  }
  const w = svgWidth(svg);
  const h = svgHeight(svg);
  if (w && !clone.getAttribute('width')) clone.setAttribute('width', String(w));
  if (h && !clone.getAttribute('height')) clone.setAttribute('height', String(h));
  // 内联样式表：mermaid 把主题色放在 <style> 里，clone 会带上，OK。
  const xml = new XMLSerializer().serializeToString(clone);
  return '<?xml version="1.0" encoding="UTF-8"?>\n' + xml;
}

function svgWidth(svg: SVGElement): number {
  const attr = parseFloat(svg.getAttribute('width') || '');
  if (Number.isFinite(attr) && attr > 0) return attr;
  const vb = svg.getAttribute('viewBox');
  if (vb) {
    const parts = vb.split(/[\s,]+/);
    const w = parseFloat(parts[2]);
    if (Number.isFinite(w) && w > 0) return w;
  }
  const rect = svg.getBoundingClientRect();
  return Math.round(rect.width) || 0;
}

function svgHeight(svg: SVGElement): number {
  const attr = parseFloat(svg.getAttribute('height') || '');
  if (Number.isFinite(attr) && attr > 0) return attr;
  const vb = svg.getAttribute('viewBox');
  if (vb) {
    const parts = vb.split(/[\s,]+/);
    const h = parseFloat(parts[3]);
    if (Number.isFinite(h) && h > 0) return h;
  }
  const rect = svg.getBoundingClientRect();
  return Math.round(rect.height) || 0;
}

/**
 * SVG 文本 → PNG Blob。
 *
 * 走 Image + canvas 的常规路子，但有三个必须处理的坑：
 *  1. **跨域污染**：SVG 里若引用了外部图片/字体，canvas 会被污染，
 *     toBlob 直接抛 SecurityError —— 这时要降级成「先下载 SVG 再提示用户手动转」；
 *  2. **尺寸**：没有宽高时按 viewBox 兜底，否则导出的是 0×0；
 *  3. **背景**：SVG 默认透明，PNG 也是透明 —— 保留透明（图表场景更常用），
 *     需要白底时由调用方传 background。
 */
export async function svgTextToPngBlob(
  svgText: string,
  opts: { scale?: number; background?: string } = {},
): Promise<Blob> {
  const scale = opts.scale && opts.scale > 0 ? opts.scale : 2;
  const m = svgText.match(/viewBox\s*=\s*"([^"]+)"/i);
  let w = 0;
  let h = 0;
  if (m) {
    const parts = m[1].split(/[\s,]+/);
    w = parseFloat(parts[2]) || 0;
    h = parseFloat(parts[3]) || 0;
  }
  if (!w || !h) {
    const mw = svgText.match(/\bwidth\s*=\s*"([\d.]+)/i);
    const mh = svgText.match(/\bheight\s*=\s*"([\d.]+)/i);
    w = w || parseFloat(mw?.[1] || '') || 800;
    h = h || parseFloat(mh?.[1] || '') || 600;
  }
  const cw = Math.max(1, Math.round(w * scale));
  const ch = Math.max(1, Math.round(h * scale));

  const url = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(svgText);
  const img = new Image();
  img.decoding = 'sync';
  await new Promise<void>((resolve, reject) => {
    img.onload = () => resolve();
    img.onerror = () => reject(new Error('SVG 无法作为图片加载（可能含外部资源）'));
    img.src = url;
  });

  const canvas = document.createElement('canvas');
  canvas.width = cw;
  canvas.height = ch;
  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('无法创建画布');
  if (opts.background) {
    ctx.fillStyle = opts.background;
    ctx.fillRect(0, 0, cw, ch);
  }
  ctx.drawImage(img, 0, 0, cw, ch);

  return new Promise<Blob>((resolve, reject) => {
    try {
      canvas.toBlob((blob) => {
        if (blob) resolve(blob);
        else reject(new Error('画布导出失败'));
      }, 'image/png');
    } catch (e) {
      reject(new Error('画布被外部资源污染，无法导出 PNG（可先下载 SVG）'));
    }
  });
}

/** DOM 里的 <svg> 元素 → PNG Blob。 */
export async function svgNodeToPngBlob(
  svg: SVGElement,
  opts: { scale?: number; background?: string } = {},
): Promise<Blob> {
  return svgTextToPngBlob(svgNodeToSvgText(svg), opts);
}

/** 统一的文件名清洗（去掉路径分隔符等）。 */
export function safeFileName(name: string, fallback = 'diagram'): string {
  const s = (name || '')
    .replace(/[\\/:*?"<>|\r\n\t]/g, '_')
    .replace(/\s+/g, '-')
    .slice(0, 80);
  return s || fallback;
}
