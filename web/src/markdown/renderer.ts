/**
 * Markdown 渲染器 — 统一使用 Cherry Markdown Engine
 *
 * 这是整个平台唯一的 Markdown 渲染入口。
 * 编辑器（ChapterEditor / BlogEditor）和阅读器（Reader / BlogDetail / 评论 / 论坛）
 * 全部共享同一个 engineConfig 和 renderMarkdown。
 *
 * 支持的扩展：
 *   - KaTeX 数学公式（含 mhchem 化学式 + physics 物理宏包）
 *   - Mermaid 图表
 *   - Cherry Markdown 标准语法（GFM 表格 / 任务列表 / 删除线 / 代码高亮等）
 *   - 自定义：定义列表 / 缩写 / 脚注 / 提示块
 */

import CherryEngine from 'cherry-markdown/dist/cherry-markdown.engine.core.esm.js';
import katex from 'katex';
import 'katex/dist/contrib/mhchem.mjs';

// ═══ physics 物理宏包 ────────────────────────────────────────────
// KaTeX 不自带 physics 扩展，通过 macros 注入常用命令。
// 参考：https://ctan.org/pkg/physics
// 注意：KaTeX macros 不支持可选参数，所以 \dv[n]{f}{x} 等带阶数的
// 形式请直接用 \frac{\mathrm{d}^n f}{\mathrm{d}x^n} 展开。

const physicsMacros: Record<string, string> = {
  // ── 数量与单位 ──
  '\\qty': '#1\\;\\mathrm{#2}',                        // \qty{1.23}{m/s}
  '\\qtyrange': '#1\\text{--}#2\\;\\mathrm{#3}',      // \qtyrange{1}{5}{m}

  // ── 向量（粗体 / 箭头）──
  '\\vb': '\\mathbf{#1}',                              // 粗体向量
  '\\va': '\\vec{#1}',                                  // 箭头向量
  '\\vu': '\\hat{\\mathbf{#1}}',                       // 单位向量

  // ── 导数（加 \, 细间距改善可读性）──
  '\\dv': '\\frac{\\mathrm{d}\\,#1}{\\mathrm{d}\\,#2}', // 普通导数
  '\\pdv': '\\frac{\\partial\\,#1}{\\partial\\,#2}',     // 偏导数
  '\\odv': '\\frac{\\mathrm{d}\\,#1}{\\mathrm{d}\\,#2}', // 常导数
  '\\fdv': '\\frac{\\delta\\,#1}{\\delta\\,#2}',          // 泛函导数

  // ── 微分符号 ──
  '\\dd': '\\,\\mathrm{d}',                             // 微分 d（前加细间距）
  '\\var': '\\delta',                                    // 变分符号

  // ── 矩阵 ──
  '\\matrix': '\\begin{matrix}#1\\end{matrix}',          // 无括号矩阵
  '\\mqty': '\\begin{pmatrix}#1\\end{pmatrix}',          // 圆括号矩阵

  // ── 括号与绝对值 ──
  '\\abs': '\\left|#1\\right|',                          // 绝对值
  '\\norm': '\\left\\lVert#1\\right\\rVert',            // 范数
  '\\eval': '\\left.#1\\right|_{#2}',                    // 求值

  // ── 狄拉克符号 ──
  '\\bra': '\\langle #1\\rvert',                          // 左态矢 ⟨ψ|
  '\\ket': '\\lvert #1\\rangle',                           // 右态矢 |ψ⟩
  '\\braket': '\\langle #1\\vert #2\\rangle',              // 内积 ⟨ψ|φ⟩
  '\\expval': '\\langle #1\\rangle',                       // 期望值 ⟨A⟩

  // ── 物理常数与算子 ──
  '\\kB': 'k_{\\mathrm{B}}',                             // 玻尔兹曼常数
  '\\hslash': '\\hbar',                                   // 约化普朗克常数
  '\\grad': '\\nabla',                                    // 梯度
  '\\divergence': '\\nabla\\cdot',                        // 散度
  '\\curl': '\\nabla\\times',                             // 旋度
  '\\laplacian': '\\nabla^{2}',                           // 拉普拉斯算子
  '\\order': '\\mathcal{O}',                              // 大 O 记号

  // ── 常用快捷命令 ──
  '\\tr': '\\operatorname{tr}',                           // 迹
  '\\Tr': '\\operatorname{Tr}',
  '\\sgn': '\\operatorname{sgn}',                         // 符号函数
  '\\diag': '\\operatorname{diag}',                       // 对角矩阵
  '\\const': '\\mathrm{const}',                           // 常量
  '\\e': '\\mathrm{e}',                                   // 自然常数
  '\\im': '\\mathrm{i}',                                  // 虚数单位

  // ── 化学式快捷命令（mhchem 已加载 \\ce{}，这些是备选/补充）──
  '\\ch': '\\ce{#1}',                                     // \ch 作为 \ce 的短别名
  '\\ox': '\\overset{#1}{#2}',                            // 氧化数标注
  // 常见离子/分子快捷
  '\\Hplus': '\\ce{H+}',
  '\\OHminus': '\\ce{OH-}',
  '\\HtwoO': '\\ce{H2O}',
  '\\COtwo': '\\ce{CO2}',
};

// ═══ 包装 katex — 注入 physics macros ────────────────────────────
// Cherry Markdown 内部通过 externals.katex.renderToString() 渲染公式，
// 这里包装 katex 以自动注入 physics macros，确保编辑器和阅读器一致。

function createPhysicsKatex(originalKatex: typeof katex): typeof katex {
  // 直接包装 renderToString，确保 this 绑定和 macros 合并正确
  const origRender = originalKatex.renderToString.bind(originalKatex);
  return new Proxy(originalKatex, {
    get(target, prop, _receiver) {
      if (prop === 'renderToString') {
        return (formula: string, options?: any) => {
          // 合并 macros：physics > 调用者传入的
          const merged = {
            strict: 'ignore' as const,
            trust: true,
            throwOnError: false,
            ...options,
            macros: {
              ...physicsMacros,
              ...(options?.macros || {}),
            },
          };
          return origRender(formula, merged);
        };
      }
      return Reflect.get(target, prop, target);
    },
  }) as typeof katex;
}

const physicsKatex = createPhysicsKatex(katex);

// ═══ 共享引擎配置（编辑器 + 阅读器统一）═══
// 所有使用 Cherry 的地方都必须引用此配置，确保渲染一致。
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
  // 使用包装后的 katex，自动注入 physics macros
  externals: { katex: physicsKatex },
} as const;

const engine = new CherryEngine(engineConfig as any);

// ═══ 导出 physicsKatex ────────────────────────────────────────────
// 编辑器（ChapterEditor / BlogEditor）创建 Cherry 实例时，
// 也需传入包装后的 katex 以确保预览渲染一致。
export { physicsKatex, physicsMacros };

// ═══ 预处理：展开/收起控件 ═══
function preprocessExpand(md: string): string {
  return md.replace(/\[expand(?:\s+title="([^"]*)")?\]\s*([\s\S]*?)\s*\[\/expand\]/g,
    (_f: string, title: string, body: string) => {
      const label = title || '展开内容';
      const id = 'exp' + Math.random().toString(36).slice(2, 8);
      return `<div class="expand-block" style="border:1px solid var(--border-color,#ddd);border-radius:8px;margin:12px 0;overflow:hidden">
<details id="${id}"><summary style="cursor:pointer;padding:10px 16px;background:var(--card-bg,#f5f5f5);font-weight:600;user-select:none">📋 ${label}</summary>
<div style="padding:12px 16px">${body}</div></details></div>`;
    });
}

// ═══ 预处理：缩写 + 定义列表 + physics 快捷替换 ═══
function preprocessMD(md: string): string {
  if (!md) return '';

  // 1) 缩写定义：*[term]: definition
  const abbrMap = new Map<string, string>();
  md = md.replace(/^\*\[(.+?)\]:\s*(.+)$/gm, (_f, term, def) => {
    abbrMap.set(term.trim(), def.trim());
    return '';
  });

  // 2) 定义列表（原始：term\n: definition）
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

  // 3) 缩写映射
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

// ═══ XSS 实体反转义 ──────────────────────────────────────────────
// 后端 bluemonday 会对用户内容做 HTML 实体编码（& → &amp; 等），
// 在输入 CherryEngine 之前需要反转义，否则 < > & 等字符会被
// 当作实体保留，破坏 Markdown/KaTeX/Mermaid 语法解析。
function unescapeHTML(str: string): string {
  return str
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&nbsp;/g, ' ');
}

// ═══ 公共 API ═══

export function renderMarkdown(content: string): string {
  if (!content) return '';
  try {
    // 1) 反转义 HTML 实体（后端 bluemonday 引入的）
    // 2) 预处理 chemfig
    // 3) 预处理缩写/定义列表
    // 4) CherryEngine 渲染（DOMPurify 会做最终 XSS 防护）
    const decoded = unescapeHTML(content);
    const html = engine.makeHtml(preprocessExpand(preprocessMD(preprocessChemfig(decoded)))) as string;
    return html;
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

// ═══ Chemfig 化学结构图渲染 ────────────────────────────────────────
// 使用 smiles-drawer 将 \chemfig{SMILES} 转为 SVG 结构图

// 预处理：\chemfig{CCO} → 占位 div
function preprocessChemfig(md: string): string {
  if (!md) return md;
  // 匹配 \chemfig{SMILES字符串}
  return md.replace(/\\chemfig\{([^}]+)\}/g, (_m, smiles: string) => {
    const s = smiles.trim();
    if (!s) return _m;
    const hash = simpleHash(s).toString(36).slice(0, 8);
    // 使用 img + data URI 占位，在 renderChemfigBlocks 中替换为 canvas
    return `<div class="chemfig-container" data-smiles="${encodeURIComponent(s)}" data-hash="${hash}" style="display:inline-block;margin:4px;vertical-align:middle"><canvas class="chemfig-canvas" width="300" height="300" style="display:none"></canvas></div>`;
  });
}

function simpleHash(s: string): number {
  let h = 0;
  for (let i = 0; i < s.length; i++) {
    h = ((h << 5) - h) + s.charCodeAt(i);
    h |= 0;
  }
  return Math.abs(h);
}

export async function renderChemfigBlocks(container: HTMLElement) {
  const wrappers = container.querySelectorAll<HTMLElement>('.chemfig-container');
  if (!wrappers.length) return;

  try {
    const SmilesDrawer = (await import('smiles-drawer')).default;
    const drawer = new (SmilesDrawer as any).Drawer({
      bondLength: 20,
      bondThickness: 2,
      shortBondLength: 0.8,
      fontSize: 14,
      background: 'transparent',
      highlight: '#818cf8',
    });

    for (const w of wrappers) {
      const smiles = decodeURIComponent(w.getAttribute('data-smiles') || '');
      if (!smiles) continue;
      const canvas = w.querySelector<HTMLCanvasElement>('.chemfig-canvas');
      if (!canvas) continue;
      canvas.style.display = 'block';

      try {
        (SmilesDrawer as any).parse(smiles, (tree: any) => {
          drawer.draw(tree, canvas, 'light', false);
        }, (err: any) => {
          console.warn('[NVS] Chemfig parse error:', err);
          w.innerHTML = `<span style="color:#999;font-size:0.85em">⚠ 无法解析: ${smiles}</span>`;
        });
      } catch {
        w.innerHTML = `<span style="color:#999;font-size:0.85em">⚠ 渲染失败: ${smiles}</span>`;
      }
    }
  } catch {
    // smiles-drawer not available - show text fallback
    for (const w of wrappers) {
      const smiles = decodeURIComponent(w.getAttribute('data-smiles') || '');
      w.innerHTML = `<span class="chemfig-fallback" style="font-family:monospace;background:rgba(102,128,153,0.05);padding:2px 8px;border-radius:4px;font-size:0.9em" title="${smiles}">🧪 \`${smiles}\`</span>`;
    }
  }
}
