import { ElMessageBox } from 'element-plus';

/**
 * 外链安全：识别站外链接、标记风险链接（短链/IP 直链）、点击前弹窗确认。
 * 与后端 contentguard 的 risky_links 呼应（后端用于审核/折叠，前端用于提示）。
 */

const SHORT_HOSTS = ['bit.ly', 't.cn', 'tinyurl.com', 'goo.gl', 'is.gd', 'v.gd', 'ow.ly', 'cutt.ly', 'suo.im', 'dwz.cn', 'rebrand.ly'];
const SUSPICIOUS_TLDS = ['.top', '.xyz', '.tk', '.ml', '.ga', '.cf', '.gq', '.club', '.loan', '.cam', '.rest', '.icu', '.cyou', '.pw'];

export function hostOf(href: string): string {
  try {
    return new URL(href, location.origin).host.toLowerCase();
  } catch {
    return '';
  }
}

export function isExternalUrl(href: string): boolean {
  if (!href) return false;
  if (href.startsWith('#') || href.startsWith('/') || href.startsWith('./') || href.startsWith('../')) return false;
  if (/^(mailto|tel|javascript|data):/i.test(href)) return false;
  try {
    return new URL(href, location.origin).origin !== location.origin;
  } catch {
    return /^(https?:)?\/\//i.test(href);
  }
}

/** 风险链接：短链 / IP 直链 / 可疑 TLD */
export function isRiskyLink(href: string): boolean {
  const host = hostOf(href);
  if (!host) return false;
  if (SHORT_HOSTS.some((s) => host === s || host.endsWith('.' + s))) return true;
  if (/\/\d{1,3}(\.\d{1,3}){3}(:\d+)?/.test(href)) return true;
  return SUSPICIOUS_TLDS.some((t) => host.endsWith(t));
}

/** 加固链接：target/rel，并标记风险链接 */
export function hardenLink(a: HTMLAnchorElement) {
  const href = a.getAttribute('href') || '';
  if (!href) return;
  a.setAttribute('target', '_blank');
  a.setAttribute('rel', 'noopener noreferrer nofollow');
  if (isExternalUrl(href)) {
    a.classList.add('ext-link');
    if (isRiskyLink(href)) a.classList.add('ext-risk');
  }
}

/**
 * 渲染后的 HTML 中给站外链接加高亮 class（ext-link / ext-risk）。
 * 在 renderMarkdown / renderCommentHTML 返回前调用。
 */
export function decorateLinks(html: string): string {
  if (!html) return html;
  return html.replace(/<a\b([^>]*)href="(https?:\/\/[^"]+)"([^>]*)>/gi, (m, pre, href, post) => {
    const cls = isRiskyLink(href) ? 'ext-link ext-risk' : 'ext-link';
    // 已有 class 则追加，否则新增
    const classRe = /\bclass="([^"]*)"/;
    if (classRe.test(m)) {
      return m.replace(classRe, (_mm, oldCls) => `class="${oldCls} ${cls}"`);
    }
    return `<a${pre}href="${href}"${post} class="${cls}">`;
  });
}

/** 内容区选择器（这些容器内的链接才走外链确认） */
const CONTENT_SELECTOR = '.reader-md, .comment-content, .floor-content, .markdown-body, .blog-content, .novel-content, .thread-content, .post-content';

/**
 * 全局外链守卫：document 委托，点击内容区链接时
 *   ① 加固 target/rel；② 站外链接弹「即将离开本站」确认。
 */
export function bindGlobalExternalGuard(): () => void {
  const handler = async (e: MouseEvent) => {
    const target = e.target as HTMLElement | null;
    const a = target?.closest?.('a[href]') as HTMLAnchorElement | null;
    if (!a) return;
    const container = a.closest(CONTENT_SELECTOR);
    if (!container) return;
    const href = a.getAttribute('href') || '';
    if (!isExternalUrl(href)) return;
    e.preventDefault();
    e.stopPropagation();
    hardenLink(a);
    const risky = isRiskyLink(href);
    try {
      await ElMessageBox.confirm(
        `即将离开本站，跳转到：\n${href}\n\n${risky ? '⚠️ 该链接为短链/IP 直链或可疑域名，风险较高。\n' : ''}请注意核对域名，谨防钓鱼与诈骗。`,
        '外链安全提示',
        { type: 'warning', confirmButtonText: '继续访问', cancelButtonText: '留在本站' },
      );
    } catch {
      return;
    }
    window.open(href, '_blank', 'noopener,noreferrer');
  };
  document.addEventListener('click', handler, true);
  return () => document.removeEventListener('click', handler, true);
}
