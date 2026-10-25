/**
 * LaTeX 定界符归一化
 * ---------------------------------------------------------------
 * 背景：Cherry 的数学语法只认 `$$…$$`（块）与 `$…$`（行内）。
 * 但模型（尤其从 LaTeX 语料学来的）经常输出 `\[ … \]` / `\( … \)`，
 * 这两种写法在渲染器里会被当成普通文本，用户看到的是转义括号而不是公式。
 *
 * 这里在 markdown 解析**之前**把两种写法归一成 Cherry 认的写法：
 *   \[ … \]  →  $$ … $$（内容跨行时补成独占行的块式）
 *   \( … \)  →  $ … $
 *
 * 只处理正文：围栏代码块（``` / ~~~）与行内代码（`…`）里的内容一律原样保留，
 * 否则代码示例里的 LaTeX 会被改写。
 *
 * 转义判定：`\\[`（偶数个反斜杠 + 括号）是「字面反斜杠 + 方括号」，不改；
 * `\[`（奇数个）才是数学定界符。
 */

/** 归一化 LaTeX 定界符 */
export function normalizeLatexDelimiters(src: string): string {
  if (!src || (src.indexOf('\\[') < 0 && src.indexOf('\\(') < 0)) return src;

  const n = src.length;
  let out = '';
  let i = 0;

  while (i < n) {
    // ── 围栏代码块：整段原样复制 ──
    if (src.startsWith('```', i) || src.startsWith('~~~', i)) {
      const marker = src.slice(i, i + 3);
      const end = src.indexOf(marker, i + 3);
      const stop = end === -1 ? n : end + 3;
      out += src.slice(i, stop);
      i = stop;
      continue;
    }

    // ── 行内代码：原样复制 ──
    if (src[i] === '`') {
      const end = src.indexOf('`', i + 1);
      const stop = end === -1 ? n : end + 1;
      out += src.slice(i, stop);
      i = stop;
      continue;
    }

    // ── 反斜杠串：判断是字面反斜杠还是数学定界符 ──
    if (src[i] === '\\') {
      let j = i;
      while (j < n && src[j] === '\\') j++;
      const run = j - i;
      const next = j < n ? src[j] : '';
      const isOpen = (next === '[' || next === '(') && run % 2 === 1;

      if (!isOpen) {
        out += src.slice(i, j);
        i = j;
        continue;
      }

      // 奇数个反斜杠：前面 run-1 个是字面反斜杠，最后一个是定界符
      const closeCh = next === '[' ? ']' : ')';
      let k = j + 1;
      let found = -1;
      while (k < n) {
        // 定界符没闭合就撞上代码围栏 → 视为不成立，原样输出
        if (src.startsWith('```', k) || src.startsWith('~~~', k)) break;
        if (src[k] === '\\' && src[k + 1] === closeCh) {
          found = k;
          break;
        }
        k++;
      }

      out += '\\'.repeat(run - 1); // 字面反斜杠保留
      if (found < 0) {
        // 没有闭合：把 `[` / `(` 原样写回，避免吞字符
        out += next;
        i = j + 1;
        continue;
      }

      const body = src.slice(j + 1, found);
      if (next === '[') {
        // 块公式：跨行时补成独占行的 $$ 段，单行直接 $$…$$
        out += body.includes('\n') ? `$$\n${body.trim()}\n$$` : `$$${body}$$`;
      } else {
        out += `$${body}$`;
      }
      i = found + 2;
      continue;
    }

    out += src[i];
    i++;
  }

  return out;
}
