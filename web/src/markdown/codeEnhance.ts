/**
 * 代码块增强 —— AI 回复与分享页共用。
 *
 * 三件事：
 *   1. 每个代码块右上角挂工具条：语言标签 + 复制（+ HTML 专属的预览 / 发布）
 *   2. 语法高亮：用 highlight.js（common，36 语言）在渲染后重跑一遍
 *   3. 语言识别：把 `language-ts` / `ts` / `typescript` 统一成 hljs 认得的 id
 *
 * 为什么要在渲染后重跑高亮，而不是直接用 Cherry 的输出：
 *   Cherry 引擎内部打包的是 Prism，且只有 markup/css/clike/javascript 四种语言，
 *   其余语言一律 fallback 成 javascript（python 的 def、go 的 func 全都不认）。
 *   更致命的是它的配色选择器被 `div[data-type=codeBlock]` 限定，
 *   而 AI 渲染走 DOMPurify 白名单（ALLOW_DATA_ATTR: false）会把 data-* 全剔除，
 *   于是 token class 在、颜色却全丢 —— 等于没有高亮。
 *   所以这里不修复 Cherry 的样式作用域，而是自己用 hljs 重打一遍，
 *   语言覆盖和配色都掌握在自己手里，也不影响阅读器/编辑器的既有行为。
 *
 * 安全性：高亮的输入是**已经过 DOMPurify 净化**的纯文本（code.textContent），
 * hljs 输出的 span 只带 class 不带 style，写回时不会引入任何可执行内容。
 */

import '@/styles/ai-code.css';

/** 语言别名 → highlight.js 的语言 id */
const LANG_ALIAS: Record<string, string> = {
  js: 'javascript', jsx: 'javascript', mjs: 'javascript', cjs: 'javascript',
  node: 'javascript', es6: 'javascript',
  ts: 'typescript', tsx: 'typescript', mts: 'typescript',
  py: 'python', py3: 'python', python3: 'python',
  sh: 'bash', shell: 'bash', zsh: 'bash', console: 'bash', terminal: 'bash', shellsession: 'bash',
  yml: 'yaml',
  htm: 'xml', html: 'xml', xhtml: 'xml', xml: 'xml', svg: 'xml', vue: 'xml', svelte: 'xml',
  'c++': 'cpp', cxx: 'cpp', cc: 'cpp', hpp: 'cpp', hxx: 'cpp',
  h: 'c', 'c#': 'csharp', cs: 'csharp', dotnet: 'csharp',
  rs: 'rust', rb: 'ruby', kt: 'kotlin', kts: 'kotlin', golang: 'go',
  md: 'markdown', mdx: 'markdown', mkd: 'markdown',
  jsonc: 'json', json5: 'json', ndjson: 'json',
  ps1: 'powershell', pwsh: 'powershell', ps: 'powershell',
  mysql: 'sql', pgsql: 'sql', postgres: 'sql', postgresql: 'sql', sqlite: 'sql', plsql: 'sql',
  'objective-c': 'objectivec', objc: 'objectivec',
  txt: 'plaintext', text: 'plaintext', plain: 'plaintext', none: 'plaintext', output: 'plaintext',
  dockerfile: 'dockerfile', docker: 'dockerfile',
  conf: 'ini', cfg: 'ini', toml: 'ini', env: 'ini', properties: 'ini',
  patch: 'diff',
  make: 'makefile', mk: 'makefile',
};

/** 认为「这是一份可以直接在浏览器里跑的 HTML」的语言 */
const HTML_LANGS = new Set(['html', 'htm', 'xhtml']);

/** 超过这个长度就不做高亮了 —— hljs 是大致线性的，但几十万字仍会卡住主线程 */
const MAX_HIGHLIGHT_CHARS = 120_000;
/** 没有语言标注时，只有短代码才值得让 hljs 猜 */
const MAX_AUTO_CHARS = 6_000;

export function normalizeLang(raw: string): string {
  const l = (raw || '').trim().toLowerCase();
  if (!l) return '';
  return LANG_ALIAS[l] || l;
}

export interface CodeMeta {
  /** 归一化后的 hljs 语言 id（可能为空 = 未标注） */
  lang: string;
  /** 展示用的语言名（大写，如 HTML / TS） */
  label: string;
  /** 是否可按网页预览 */
  isHtml: boolean;
}

/**
 * 取「作者原始语言标注」，而不是 Cherry 改写后的值。
 *
 * Cherry 渲染代码块时会做两件让人误判的事：
 *   1. 未标注语言 → 把 lang 强制 fallback 成 `javascript`，写进 pre 的 class；
 *   2. 同时在包裹层 div 上留 `data-lang` = **原始标注**（未标注就是空串）。
 * 所以判断「到底标没标语言」只能看 data-lang：
 *   属性不存在 → 引擎不是 Cherry（或老版本），退回读 class；
 *   属性存在但为空 → 确认未标注，不要被 class 的 javascript 骗了。
 */
function readAuthoredLang(pre: Element, code: Element | null): { lang: string; authored: boolean } {
  const wrap = pre.closest('[data-lang]');
  if (wrap) {
    return { lang: (wrap.getAttribute('data-lang') || '').trim().toLowerCase(), authored: true };
  }
  const fromClass = readLangAttr(pre) || readLangAttr(code);
  return { lang: fromClass, authored: !!fromClass };
}

function readLangAttr(el: Element | null): string {
  const cls = el?.getAttribute('class') || '';
  const m = cls.match(/(?:^|\s)language-([\w+#.-]+)/i);
  return m ? m[1].toLowerCase() : '';
}

/** 从代码内容做保守的 HTML 猜测：只认 unmistakable 的开头 */
function looksLikeHtml(src: string): boolean {
  const head = src.replace(/^\uFEFF/, '').trimStart().slice(0, 400).toLowerCase();
  return (
    head.startsWith('<!doctype html') ||
    head.startsWith('<html') ||
    (head.startsWith('<body') && head.includes('</'))
  );
}

export function detectCodeMeta(pre: Element, code: Element | null): CodeMeta {
  const { lang: rawLang } = readAuthoredLang(pre, code);
  const src = code?.textContent || '';
  // 判断依据是「作者到底写没写语言」，不是「data-lang 属性在不在」：
  //   未标注（rawLang 为空）→ 允许按内容猜 HTML（AI 常不写 ```html）
  //   明确标了 js/python → 就是那个语言，绝不猜（否则把 JS 认成网页）
  const explicitlyLabelled = !!rawLang;
  const isHtml = HTML_LANGS.has(rawLang) || (!explicitlyLabelled && looksLikeHtml(src));
  const lang = isHtml ? 'xml' : normalizeLang(rawLang);
  return {
    lang,
    label: (rawLang || (isHtml ? 'html' : '')).toUpperCase(),
    isHtml,
  };
}

const BUTTON_DEFS: Array<{ act: string; text: string; title: string; htmlOnly?: boolean }> = [
  { act: 'preview', text: '预览', title: '在沙箱里运行这段 HTML', htmlOnly: true },
  { act: 'publish', text: '发布', title: '发布到 HTML 应用广场', htmlOnly: true },
  { act: 'copy', text: '复制', title: '复制代码', htmlOnly: false },
];

/**
 * 给每个 `<pre>` 挂工具条。
 *
 * 用 DOMParser 后处理而不是改渲染引擎：cherry 的 HTML 结构随版本会变，
 * 而 v-html 出来的 `pre/code` 是稳定契约，按 DOM 处理最不容易被上游升级打穿。
 * 代码原文**不写进 data-* 属性**（会让 innerHTML 体积翻倍、还要处理转义），
 * 事件回调时统一从 `pre > code` 的 textContent 现取。
 */
export function enhanceCodeBlocks(html: string): string {
  if (!html || !html.includes('<pre')) return html;
  try {
    const doc = new DOMParser().parseFromString(html, 'text/html');
    doc.querySelectorAll('pre').forEach((pre) => {
      if (pre.querySelector('.code-bar')) return;
      // 兜底：没有 <code> 的裸 pre（Cherry 的缩进代码块）自己包一层，
      // 这样取代码时不会把工具条的按钮文字一起复制走。
      let code = pre.querySelector('code');
      if (!code) {
        code = doc.createElement('code');
        code.textContent = pre.textContent || '';
        pre.textContent = '';
        pre.appendChild(code);
      }
      if (!(code.textContent || '').trim()) return;

      const meta = detectCodeMeta(pre, code);
      // 高亮阶段要用，挂在自有属性上（不参与 DOMPurify 之前的安全边界）
      if (meta.lang) code.setAttribute('data-lang', meta.lang);
      if (meta.isHtml) pre.setAttribute('data-html', '1');

      const bar = doc.createElement('div');
      bar.className = 'code-bar';
      if (meta.label) {
        const tag = doc.createElement('span');
        tag.className = 'code-lang';
        tag.textContent = meta.label;
        bar.appendChild(tag);
      }
      for (const def of BUTTON_DEFS) {
        if (def.htmlOnly && !meta.isHtml) continue;
        const btn = doc.createElement('button');
        btn.type = 'button';
        btn.className = 'code-act';
        btn.dataset.act = def.act;
        btn.title = def.title;
        btn.textContent = def.text;
        bar.appendChild(btn);
      }
      pre.appendChild(bar);
    });
    return doc.body.innerHTML;
  } catch {
    return html;
  }
}

// ── 语法高亮 ────────────────────────────────────────────────────────
let hljsPromise: Promise<any> | null = null;

function loadHljs(): Promise<any> {
  // 懒加载：highlight.js 只在真正出现代码块时才拉一次独立 chunk，
  // 不拖首屏。模块级缓存保证整站只下载/初始化一次。
  if (!hljsPromise) {
    hljsPromise = import('highlight.js/lib/common')
      .then((m: any) => m.default || m)
      .catch((e) => {
        console.warn('[NVS] highlight.js 加载失败，代码块将不带配色:', e);
        throw e;
      });
  }
  return hljsPromise;
}

/**
 * 对容器内所有尚未高亮的 `pre > code` 做着色。
 * 可重复调用：已处理的元素带 data-nvs-hl 标记，直接跳过。
 */
export async function highlightCodeBlocks(root: ParentNode | null | undefined): Promise<void> {
  if (!root) return;
  const codes = Array.from(root.querySelectorAll<HTMLElement>('pre > code')).filter(
    (c) => c.dataset.nvsHl !== '1',
  );
  if (!codes.length) return;

  let hljs: any;
  try {
    hljs = await loadHljs();
  } catch {
    return;
  }
  // await 期间 DOM 可能已被整体替换（流式重渲染 / 切换会话），
  // 被摘掉的节点继续写 innerHTML 只是白做功，直接跳过。
  for (const code of codes) {
    if (!code.isConnected) continue;
    const raw = code.textContent || '';
    if (!raw.trim()) {
      code.dataset.nvsHl = '1';
      continue;
    }
    if (raw.length > MAX_HIGHLIGHT_CHARS) continue; // 太长：保持纯文本，下次也不重试

    const lang = code.dataset.lang || '';
    let out = '';
    try {
      if (lang && lang !== 'plaintext' && hljs.getLanguage(lang)) {
        out = hljs.highlight(raw, { language: lang, ignoreIllegals: true }).value;
      } else if (!lang && raw.length <= MAX_AUTO_CHARS) {
        // 没标语言：让 hljs 猜一次（短代码猜得准，长文本猜得慢且容易乱标）
        out = hljs.highlightAuto(raw).value;
      }
    } catch {
      out = '';
    }
    if (out) code.innerHTML = out;
    code.dataset.nvsHl = '1';
  }
}

// ── 事件回调辅助 ────────────────────────────────────────────────────
/** 按钮所在代码块的源码 */
export function readCodeOf(btn: Element): string {
  const pre = btn.closest('pre');
  if (!pre) return '';
  return pre.querySelector('code')?.textContent || '';
}

/** 按钮所在代码块的语言（归一化后） */
export function readLangOf(btn: Element): string {
  const pre = btn.closest('pre');
  if (!pre) return '';
  const code = pre.querySelector('code');
  return code?.getAttribute('data-lang') || detectCodeMeta(pre, code).lang;
}

/** 按钮所在代码块是否按 HTML 处理 */
export function isHtmlBlock(btn: Element): boolean {
  const pre = btn.closest('pre');
  if (!pre) return false;
  if (pre.getAttribute('data-html') === '1') return true;
  return detectCodeMeta(pre, pre.querySelector('code')).isHtml;
}
