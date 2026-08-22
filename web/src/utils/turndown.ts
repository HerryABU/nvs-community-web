// 共享的 HTML → Markdown 转换工具（按需懒加载 turndown，避免在每个编辑器重复实例化）。
// 仅在实际需要时（富文本转 Markdown 保存）才加载 turndown 依赖。
let service: any = null;

export async function htmlToMarkdown(html: string): Promise<string> {
  if (!html) return '';
  if (!service) {
    const TurndownModule = await import('turndown');
    service = new TurndownModule.default({
      headingStyle: 'atx',
      hr: '---',
      bulletListMarker: '-',
      codeBlockStyle: 'fenced',
    });
  }
  return service.turndown(html);
}
