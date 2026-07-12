/**
 * Markdown 渲染器 — 统一使用 Cherry Markdown Engine
 */

import CherryEngine from 'cherry-markdown/dist/cherry-markdown.engine.core.esm.js';
import katex from 'katex';
import 'katex/dist/contrib/mhchem.mjs';

// ═══ 共享引擎配置（编辑器 + 阅读器统一）═══
export const engineConfig = {
  engine: {
    global: { classicBr: false },
    syntax: {
      mathBlock: { engine: 'katex' as const },
      inlineMath: { engine: 'katex' as const },
      codeBlock: { lineNumber: true, copyCode: true, mermaid: { svg2img: false } },
      header: { anchorStyle: 'none' as const },
      table: { enableChart: false },
      fontEmphasis: { allowWhitespace: false },
    },
  },
  externals: { katex },
} as const;

const engine = new CherryEngine(engineConfig as any);

// ═══ 预处理：缩写 + 定义列表 ═══
function preprocessMD(md: string): string {
  if (!md) return '';

  const abbrMap = new Map<string, string>();
  md = md.replace(/^\*\[(.+?)\]:\s*(.+)$/gm, (_f, term, def) => {
    abbrMap.set(term.trim(), def.trim());
    return '';
  });

  const lines = md.split('\n');
  const out: string[] = [];
  let i = 0;
  let inDL = false;
  let dlLines: string[] = [];

  function flushDL() {
    if (dlLines.length === 0) return;
    out.push(dlLines.join('\n'));
    dlLines = [];
    inDL = false;
  }

  while (i < lines.length) {
    const raw = lines[i];
    const t = raw.trim();
    const nextT = i + 1 < lines.length ? lines[i + 1].trim() : '';

    const isTermStart =
      t !== '' && !t.startsWith('#') && !t.startsWith('```') && !t.startsWith('|') &&
      !t.startsWith('>') && !t.startsWith('- ') && !t.startsWith('* ') &&
      !t.startsWith('1.') && nextT.startsWith(': ');

    if (isTermStart && !inDL) {
      out.push(raw);
      inDL = true;
      dlLines = [`<dl><dt>${t}</dt>`];
      i++; continue;
    }
    if (inDL && t !== '' && !t.startsWith(': ') && nextT.startsWith(': ')) {
      dlLines.push(`<dt>${t}</dt>`);
      i++; continue;
    }
    if (inDL && t.startsWith(': ')) {
      dlLines.push(`<dd>${t.slice(2)}</dd>`);
      i++; continue;
    }
    if (inDL && t === '') { flushDL(); out.push(''); i++; continue; }
    if (inDL) { flushDL(); out.push(raw); i++; continue; }
    out.push(raw);
    i++;
  }
  if (inDL) flushDL();
  md = out.join('\n');

  if (abbrMap.size > 0) {
    for (const [term, def] of abbrMap) {
      const esc = term.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
      md = md.replace(
        new RegExp(`\\b${esc}\\b`, 'g'),
        `<abbr title="${def.replace(/"/g, '&quot;')}">${term}</abbr>`,
      );
    }
  }
  return md;
}

// ═══ 公共 API ═══

export function renderMarkdown(content: string): string {
  if (!content) return '';
  try {
    return engine.makeHtml(preprocessMD(content)) as string;
  } catch (e) {
    console.error('[NVS] renderMarkdown error:', e);
    return `<pre>${content.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;')}</pre>`;
  }
}

export async function renderMermaidBlocks(container: HTMLElement) {
  // CherryEngine 生成 data-lang="mermaid" 属性，转为 mermaid.run() 能识别的 class
  const wrappers = container.querySelectorAll<HTMLElement>('[data-lang="mermaid"]');
  if (!wrappers.length) return;

  // 将每个 wrapper 转为 <div class="mermaid">原始源码</div>，让 mermaid.run() 统一处理
  // 这样和 Cherry 编辑器内部使用相同的渲染路径
  for (const w of wrappers) {
    const code = w.querySelector('code');
    const src = code?.textContent?.trim();
    if (!src) continue;
    const div = document.createElement('div');
    div.className = 'mermaid';
    div.textContent = src;
    w.replaceWith(div);
  }

  // 调用 mermaid.run() 自动渲染所有 .mermaid 元素
  try {
    const m = await import('mermaid');
    const mm = (m as any).default || m;
    mm.initialize({
      startOnLoad: false,
      securityLevel: 'loose',
      theme: 'default',
      themeVariables: { fontFamily: 'sans-serif', fontSize: '14px' },
    });
    await mm.run();
    // run() 后 .mermaid div 被替换为 SVG，包一层 mermaid-container
    for (const el of container.querySelectorAll<HTMLElement>('svg')) {
      const parent = el.parentElement;
      if (parent?.classList.contains('mermaid-container')) continue;
      if (!parent?.id?.startsWith('mermaid-') && !parent?.classList.contains('mermaid')) continue;
      const wrap = document.createElement('div');
      wrap.className = 'mermaid-container';
      parent.insertBefore(wrap, el);
      wrap.appendChild(el);
      el.style.maxWidth = '100%';
      el.style.height = 'auto';
    }
  } catch { /* mermaid not available */ }
}
