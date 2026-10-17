// editorShortcuts.ts — 编辑器快捷键（Ctrl/Cmd + B/I/U/S）统一工具。
//
// 设计：评论区等「纯文本 textarea」没有富文本编辑器的快捷键，这里提供一套
// 通用方案，对所有 el-input(textarea) / 原生 textarea 生效：
//   - Ctrl/Cmd + B → 粗体（包裹 **...** 或 <b>...</b>）
//   - Ctrl/Cmd + I → 斜体（包裹 *...*  或 <i>...</i>）
//   - Ctrl/Cmd + U → 下划线（包裹 <u>...</u>）
//   - Ctrl/Cmd + S → 保存，且 preventDefault 拦截浏览器「网页另存为」对话框
//
// 用法：把 editorKeydownHandler 直接绑在 el-input 的 @keydown 上即可（事件会
// 从内部 textarea 冒泡到组件根，e.target 即 textarea）。富文本（Tiptap）的
// B/I/U 由编辑器自身默认绑定，这里只给它补一个 Ctrl+S → emit('save')。

export type WrapPair = [string, string];

export interface EditorShortcutTokens {
  bold: WrapPair;
  italic: WrapPair;
  underline: WrapPair;
}

// Markdown 语境（评论区）：用 Markdown 语法包裹选区
export const MARKDOWN_TOKENS: EditorShortcutTokens = {
  bold: ['**', '**'],
  italic: ['*', '*'],
  underline: ['<u>', '</u>'],
};

// HTML 语境（自定义主页代码框）：直接插入 HTML 标签
export const HTML_TOKENS: EditorShortcutTokens = {
  bold: ['<b>', '</b>'],
  italic: ['<i>', '</i>'],
  underline: ['<u>', '</u>'],
};

// 在 textarea 当前选区两侧包裹 before/after，并还原光标到包裹后的原选区。
// 通过派发原生 input 事件让 el-input 的 v-model 同步（不依赖内部 ref 名）。
export function applyTextareaWrap(
  el: HTMLTextAreaElement,
  before: string,
  after: string,
): void {
  const start = el.selectionStart;
  const end = el.selectionEnd;
  const val = el.value;
  const selected = val.slice(start, end);
  const next = val.slice(0, start) + before + selected + after + val.slice(end);
  el.value = next;
  const selStart = start + before.length;
  const selEnd = selStart + selected.length;
  el.focus();
  el.setSelectionRange(selStart, selEnd);
  // 触发 el-input 内部监听的原生 input，从而更新 v-model
  el.dispatchEvent(new Event('input', { bubbles: true }));
}

export interface EditorShortcutOptions {
  tokens: EditorShortcutTokens;
  // Ctrl/Cmd + S 时回调（保存）；未提供则不拦截 S
  onSave?: () => void;
}

// 生成 keydown 处理函数。直接绑定到 el-input @keydown 即可。
// 命中 Ctrl/Cmd + B/I/U 包裹选区；命中 Ctrl/Cmd + S 且焦点在 textarea 内时
// 拦截浏览器保存并执行 onSave。焦点不在 textarea 时不拦截（避免误触）。
export function editorKeydownHandler(
  e: KeyboardEvent,
  opts: EditorShortcutOptions,
): void {
  if (!(e.ctrlKey || e.metaKey)) return;
  const key = e.key.toLowerCase();

  if (key === 's') {
    const t = e.target as HTMLElement | null;
    if (!t || t.tagName !== 'TEXTAREA') return; // 仅在评论框内拦截浏览器保存
    e.preventDefault();
    opts.onSave?.();
    return;
  }

  if (key !== 'b' && key !== 'i' && key !== 'u') return;
  const ta = e.target as HTMLTextAreaElement | null;
  if (!ta || ta.tagName !== 'TEXTAREA') return;

  let pair: WrapPair;
  if (key === 'b') pair = opts.tokens.bold;
  else if (key === 'i') pair = opts.tokens.italic;
  else pair = opts.tokens.underline;

  e.preventDefault();
  applyTextareaWrap(ta, pair[0], pair[1]);
}
