/**
 * 编辑器流式写入桥接工具
 * ---------------------------------------------------------------
 * 作用：让 AI 流式返回的增量文本「直接落进编辑器」。
 *  - Markdown 模式（Cherry / CodeMirror）：用 bookmark 锚定插入点，
 *    每个 chunk 在锚点处 replaceRange，锚点自动随插入右移，
 *    保证「续写/改写/扩写」顺序正确、且实时可见。
 *  - 富文本模式：无精确光标概念，统一回退为「末尾追加」到 v-model。
 *
 * 目标点 target：
 *  - 'cursor'   在当前光标后追加（续写 / 扩写）
 *  - 'selection'替换当前选区（改写选中段落）；无选区时退化为 cursor
 *  - 'all'      清空整篇后从开头写（全部重写）
 *
 * 该模块只认 CodeMirror 实例的通用方法（setBookmark / replaceRange /
 * getCursor / listSelections），不耦合 Cherry 具体版本。
 */

export type StreamTarget = 'cursor' | 'selection' | 'all';

export interface StreamSession {
  /** CodeMirror 实例（可能为 null → 回退富文本/纯文本） */
  cm: any;
  /** 插入锚点书签 */
  bookmark: any;
  /** 是否为富文本模式（无 CM 精确插入） */
  rich: boolean;
  /** 目标点，富文本模式下用于决定末尾追加还是整篇替换 */
  target: StreamTarget;
}

/** 取 Cherry 内部的 CodeMirror 实例（不同版本挂在 editor.codemirror 或自身 .codemirror） */
function getCM(cherryInstance: any): any {
  if (!cherryInstance) return null;
  return cherryInstance.editor?.codemirror || cherryInstance.codemirror || null;
}

/**
 * 开始一次流式写入，按 target 在编辑器里腾好锚点。
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
    return { cm: null, bookmark: null, rich: true, target };
  }
  const cm = getCM(cherryInstance);
  if (!cm || typeof cm.setBookmark !== 'function') {
    return { cm: null, bookmark: null, rich: true, target };
  }

  let pos: any;
  if (target === 'all') {
    const end = cm.posFromIndex(cm.getValue().length);
    cm.replaceRange('', { line: 0, ch: 0 }, end);
    pos = { line: 0, ch: 0 };
  } else if (target === 'selection') {
    const sels = cm.listSelections ? cm.listSelections() : [];
    let range: [any, any] | null = null;
    if (sels && sels.length) {
      const f = sels[0].anchor;
      const t = sels[0].head;
      const fi = cm.indexFromPos(f);
      const ti = cm.indexFromPos(t);
      if (fi !== ti) range = fi < ti ? [f, t] : [t, f];
    }
    if (range) {
      cm.replaceRange('', range[0], range[1]);
      pos = range[0];
    } else {
      pos = cm.getCursor(); // 无选区退化为光标
    }
  } else {
    pos = cm.getCursor();
  }

  const bookmark = cm.setBookmark(pos, { insertLeft: true });
  return { cm, bookmark, rich: false, target };
}

/**
 * 写入一个增量块。返回 true 表示已写入 CM（父组件无需再处理富文本回退）。
 */
export function writeStreamChunk(sess: StreamSession, chunk: string): boolean {
  if (sess.rich || !sess.cm || !sess.bookmark) return false;
  const p = sess.bookmark.find();
  if (!p) return false;
  sess.cm.replaceRange(chunk, p);
  // insertLeft:true → 书签随插入右移，下一 chunk 自动接在后面
  return true;
}

/** 流式结束：清理书签 */
export function endStream(sess: StreamSession) {
  try {
    sess.bookmark?.clear?.();
  } catch {
    /* ignore */
  }
  sess.bookmark = null;
}
