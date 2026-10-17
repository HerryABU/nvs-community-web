import { renderMarkdown } from '@/markdown/renderer';
import type { MentionRef } from '@/api/novel/types';

// 将评论正文渲染为 HTML，并把 @提及（来自后端解析的 mentions 列表）替换为指向用户主页的链接。
// 内容已在后端经 SanitizeUserContent 净化，此处替换仅作用于文本节点，安全。
export function renderCommentHTML(
  content: string,
  mentions?: MentionRef[],
): string {
  const html = renderMarkdown(content || '');
  if (!mentions || mentions.length === 0) return html;
  let out = html;
  for (const m of mentions) {
    const name = m.name;
    if (!name) continue;
    // 转义正则特殊字符，避免用户名中的符号破坏正则
    const safe = name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const re = new RegExp('@' + safe + '(?![\\w@])', 'g');
    out = out.replace(
      re,
      `<a href="/author/${m.user_id}" class="mention-link" data-uid="${m.user_id}">@${name}</a>`,
    );
  }
  return out;
}
