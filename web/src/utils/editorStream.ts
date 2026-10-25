/**
 * 编辑器流式写入桥接工具
 * ---------------------------------------------------------------
 * 作用：让 AI 流式返回的增量文本「直接落进编辑器」。
 *
 * ⚠️ CodeMirror 版本说明（本项目 cherry-markdown@0.11.9 已升级到 CodeMirror 6）：
 *   - CM6 适配器在 `cherryInstance.editor.editor`（类型 CM6Adapter），
 *     带有 `state`（EditorState）与 `dispatch()`（TransactionSpec）。
 *   - 旧的 CM5 方法（replaceRange / getCursor / setBookmark / indexFromPos …）
 *     在 CM6 上**不存在**，照抄会静默退化（表现为「AI 写了但编辑器没变化」）。
 *   - 插入点用「文档偏移量」数值跟踪，每次 dispatch 后自行前进，
 *     不依赖书签：即使用户中途点走光标，后续增量仍落在正确位置。
 *
 * 目标点 target：
 *   - 'cursor'    在当前光标后追加（续写 / 扩写）
 *   - 'selection' 替换当前选区（改写选中段落）；无选区时退化为 cursor
 *   - 'all'       清空整篇后从开头写（全部重写）
 *
 * 退化策略：拿不到 CM6 适配器时返回 rich=true，由父组件改为「追加到 v-model」。
 */

export type StreamTarget = 'cursor' | 'selection' | 'all';

export interface StreamSession {
  /** CM6 适配器（可能为 null → 回退富文本/纯文本） */
  cm: any;
  /** 流式插入点：文档偏移量（UTF-16 码元，与 CM6 position 语义一致） */
  pos: number;
  /** 是否为富文本模式（无 CM 精确插入） */
  rich: boolean;
  /** 目标点，富文本模式下用于决定末尾追加还是整篇替换 */
  target: StreamTarget;
}

/**
 * 解析出可用的 CM6 适配器。
 * 依次尝试：Cherry 编辑器内部的 adapter → 旧版 cherryInstance.editor.codemirror
 * → 实例上的 codemirror → 实例本身就是 CM 视图。
 */
function resolveCm(cherryInstance: any): any {
  if (!cherryInstance) return null;
  const candidates = [
    cherryInstance.editor?.editor,     // cherry-markdown 0.11+：Editor.editor = CM6Adapter
    cherryInstance.editor?.codemirror, // 旧版 Cherry 挂载点
    cherryInstance.codemirror,         // 实例上直接挂载
    cherryInstance.editor,             // 本身就是 CM 视图
  ];
  for (const c of candidates) {
    if (c && c.state && typeof c.dispatch === 'function') return c;
  }
  return null;
}

/** 文档总长度 */
function docLength(cm: any): number {
  return cm?.state?.doc?.length ?? 0;
}

/** 文档全文 */
function docText(cm: any): string {
  return cm?.state?.doc?.toString?.() ?? '';
}

/** 主选区（无选区时 from===to） */
function mainSelection(cm: any): { from: number; to: number; head: number } {
  const sel = cm?.state?.selection?.main;
  if (!sel) return { from: 0, to: 0, head: 0 };
  return { from: sel.from, to: sel.to, head: sel.head };
}

/** 在 [from, to) 区间替换为 text，并把光标落到插入内容末尾 */
function replaceRange(cm: any, from: number, to: number, text: string): number {
  const at = Math.max(0, Math.min(from, docLength(cm)));
  const end = Math.max(at, Math.min(to, docLength(cm)));
  cm.dispatch({
    changes: { from: at, to: end, insert: text },
    selection: { anchor: at + text.length },
  });
  return at + text.length;
}

/**
 * 开始一次流式写入，按 target 在编辑器里腾好插入点。
 * @param cherryInstance 当前 Cherry 实例（可为 null）
 * @param mode           'md' | 'richtext'
 * @param target         插入目标点
 */
export function beginStream(
  cherryInstance: any,
  mode: 'md' | 'richtext',
  target: StreamTarget,
): StreamSession {
  if (mode !== 'md') {
    // 富文本：无精确光标，交由父组件处理（末尾追加 / 整篇替换）
    return { cm: null, pos: 0, rich: true, target };
  }
  const cm = resolveCm(cherryInstance);
  if (!cm) {
    return { cm: null, pos: 0, rich: true, target };
  }

  if (target === 'all') {
    // 清空整篇，光标落到开头
    const pos = replaceRange(cm, 0, docLength(cm), '');
    return { cm, pos, rich: false, target };
  }

  const sel = mainSelection(cm);
  if (target === 'selection' && sel.from !== sel.to) {
    // 先清掉选区，光标落到选区起点，后续增量在这里顺序写入
    const pos = replaceRange(cm, sel.from, sel.to, '');
    return { cm, pos, rich: false, target };
  }
  // 'cursor'，或 'selection' 但没有选区 → 退化为光标
  return { cm, pos: sel.head, rich: false, target };
}

/**
 * 写入一个增量块。返回 true 表示已写入编辑器（父组件无需再处理富文本回退）。
 */
export function writeStreamChunk(sess: StreamSession, chunk: string): boolean {
  if (sess.rich || !sess.cm || !chunk) return false;
  const at = Math.max(0, Math.min(sess.pos, docLength(sess.cm)));
  sess.cm.dispatch({
    changes: { from: at, insert: chunk },
    selection: { anchor: at + chunk.length },
  });
  sess.pos = at + chunk.length;
  return true;
}

/** 流式结束：清理会话（CM6 无需额外清理书签） */
export function endStream(sess: StreamSession) {
  sess.cm = null;
  sess.pos = 0;
}

/** 读取编辑器当前全文（供父组件保存时兜底） */
export function readEditorText(cherryInstance: any): string {
  const cm = resolveCm(cherryInstance);
  if (cm) return docText(cm);
  return cherryInstance?.getValue?.() ?? '';
}

/** 读取当前选中的文本（无选区返回空串） */
export function readEditorSelection(cherryInstance: any): string {
  const cm = resolveCm(cherryInstance);
  if (cm) {
    const sel = mainSelection(cm);
    if (sel.from === sel.to) return '';
    return cm.state.sliceDoc(sel.from, sel.to);
  }
  return cherryInstance?.editor?.getSelection?.() ?? cherryInstance?.getSelection?.() ?? '';
}

/**
 * 在光标处插入文本（有选区则替换选区）。
 * 优先走 CM6 适配器；拿不到时退回 Cherry 自身的 insert API。
 * @returns 是否插入成功
 */
export function insertAtCursor(cherryInstance: any, text: string): boolean {
  if (!cherryInstance || !text) return false;
  const cm = resolveCm(cherryInstance);
  if (cm) {
    const sel = mainSelection(cm);
    replaceRange(cm, sel.from, sel.to, text);
    return true;
  }
  try {
    cherryInstance.insert?.(text);
    return true;
  } catch {
    return false;
  }
}
