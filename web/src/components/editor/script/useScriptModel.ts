import type { ScriptCharacterView, ScriptRow, ScriptRowKind, ScriptSide } from './types';

// ==================== 话本气泡体：文本 ⇄ 气泡模型 ====================
// 解析规则与服务端 server/scriptmode/script.go 一一对应（两边必须同步改）：
//   1. 空行 → 分隔（script 断并块，mixed 断散文块）
//   2. 整行 ![说明](地址) → 图片
//   3. 整行 【】（）[]() 包裹 → 场景提示
//   4. 角色：台词 → 对话气泡；行首 # / ## 显式指定左右
//   5. 其余 → 旁白（script）/ 散文块（mixed）
//
// 序列化一律给对话行补上显式 # / ## 标记：这样每个角色的左右都是「钉死」的，
// 不会因为某个角色新增了显式标记、导致其他角色按首现顺序重排左右。

const NARRATION_NAMES = new Set(['旁白', '旁述', '旁注', '画外音']);
const STAGE_PAIRS: Array<[string, string]> = [['【', '】'], ['（', '）'], ['[', ']'], ['(', ')']];
const IMAGE_RE = /^!\[([^\]]{0,64})\]\((\S+)\)\s*$/;

let rowSeq = 0;
function nextId(): string {
  rowSeq += 1;
  return `sr_${rowSeq}`;
}

/** 头像/主题色调色板（按名字散列取色，保证同一角色颜色稳定） */
export const SCRIPT_PALETTE = [
  '#5470c6', '#91cc75', '#fac858', '#ee6666', '#73c0de',
  '#3ba272', '#fc8452', '#9a60b4', '#ea7ccc', '#c48853',
];

/** 按名字取一个稳定主题色 */
export function colorForName(name: string): string {
  const s = String(name || '');
  if (!s) return SCRIPT_PALETTE[0];
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0;
  return SCRIPT_PALETTE[h % SCRIPT_PALETTE.length];
}

/** 头像占位字：中文取首字，英文取首字母 */
export function characterInitial(name: string): string {
  const s = String(name || '').trim();
  if (!s) return '?';
  return Array.from(s)[0] || '?';
}

// ---------- 解析原语（与 Go 版逐条对应） ----------

function splitSpeaker(line: string): { name: string; text: string } | null {
  for (let i = 0; i < line.length; i++) {
    const ch = line[i];
    if (ch !== '：' && ch !== ':') continue;
    const name = line.slice(0, i).trim();
    const text = line.slice(i + 1).trim();
    if (!name) return null;
    // 角色名里不允许再出现冒号以外的标点堆砌（如整句 URL「https://」）
    if (/\s/.test(name) && Array.from(name).length > 6) return null;
    if (Array.from(name).length > 16) return null;
    return { name, text };
  }
  return null;
}

function isStage(line: string): string | null {
  for (const [a, b] of STAGE_PAIRS) {
    if (line.startsWith(a) && line.endsWith(b) && line.length > a.length + b.length - 1) {
      const inner = line.slice(a.length, line.length - b.length).trim();
      if (inner) return inner;
    }
  }
  return null;
}

function parseImage(line: string): { alt: string; url: string } | null {
  const m = line.match(IMAGE_RE);
  if (!m) return null;
  return { alt: m[1], url: m[2] };
}

function explicitSide(line: string): { side: ScriptSide; rest: string } | null {
  if (line.startsWith('##')) return { side: 'right', rest: line.slice(2).trim() };
  if (line.startsWith('#')) return { side: 'left', rest: line.slice(1).trim() };
  return null;
}

/** 解析一行对话；side 为 '' 表示未显式指定（按首现顺序自动左右） */
function parseDialogue(line: string): { name: string; text: string; side: ScriptSide | '' } | null {
  const ex = explicitSide(line);
  if (ex) {
    const sp = splitSpeaker(ex.rest);
    if (sp && sp.text && !NARRATION_NAMES.has(sp.name)) {
      return { name: sp.name, text: sp.text, side: ex.side };
    }
  }
  const sp = splitSpeaker(line);
  if (!sp || !sp.text || NARRATION_NAMES.has(sp.name)) return null;
  return { name: sp.name, text: sp.text, side: '' };
}

// ---------- 主解析 ----------

/**
 * 把话本源文本解析成气泡行序列。
 * @param src 源文本
 * @param mode script=纯话本（旁白逐行合并）/ mixed=散文+气泡混排
 */
export function parseScriptRows(src: string, mode: 'script' | 'mixed' = 'script'): ScriptRow[] {
  const lines = String(src || '').replace(/\r\n/g, '\n').replace(/\r/g, '\n').split('\n');
  const rows: ScriptRow[] = [];

  const charSide = new Map<string, ScriptSide>();
  let autoCount = 0;
  const autoSide = (name: string): ScriptSide => {
    const known = charSide.get(name);
    if (known) return known;
    const s: ScriptSide = autoCount % 2 === 1 ? 'right' : 'left';
    autoCount += 1;
    charSide.set(name, s);
    return s;
  };

  let lastIdx = -1;
  let pendingBlank = false;
  const proseBuf: string[] = [];
  const flushProse = () => {
    if (!proseBuf.length) return;
    const text = proseBuf.join('\n');
    rows.push({ id: nextId(), kind: 'prose', speaker: '', text, side: 'left', raw: text, blankBefore: pendingBlank });
    pendingBlank = false;
    proseBuf.length = 0;
  };

  for (const raw of lines) {
    const line = raw.trim();
    if (line === '') {
      if (mode === 'mixed' && proseBuf.length) proseBuf.push('');
      lastIdx = -1;
      pendingBlank = true;
      continue;
    }
    const img = parseImage(line);
    if (img) {
      if (mode === 'mixed') flushProse();
      rows.push({
        id: nextId(), kind: 'image', speaker: '', text: '', side: 'left',
        alt: img.alt, url: img.url, raw: line, blankBefore: pendingBlank,
      });
      pendingBlank = false;
      lastIdx = -1;
      continue;
    }
    const stage = isStage(line);
    if (stage !== null) {
      if (mode === 'mixed') flushProse();
      rows.push({
        id: nextId(), kind: 'stage', speaker: '', text: stage, side: 'left',
        raw: line, blankBefore: pendingBlank,
      });
      pendingBlank = false;
      lastIdx = -1;
      continue;
    }
    const dlg = parseDialogue(line);
    if (dlg) {
      if (mode === 'mixed') flushProse();
      let side = dlg.side;
      if (side === '') side = autoSide(dlg.name);
      else charSide.set(dlg.name, side);
      const prev = lastIdx >= 0 ? rows[lastIdx] : null;
      if (prev && prev.kind === 'dialogue' && prev.speaker === dlg.name && prev.side === side) {
        prev.text += '\n' + dlg.text;
        continue;
      }
      rows.push({
        id: nextId(), kind: 'dialogue', speaker: dlg.name, text: dlg.text, side,
        raw: line, blankBefore: pendingBlank,
      });
      pendingBlank = false;
      lastIdx = rows.length - 1;
      continue;
    }
    // 非对话行
    if (mode === 'mixed') {
      lastIdx = -1;
      proseBuf.push(raw);
      pendingBlank = false;
      continue;
    }
    const prev = lastIdx >= 0 ? rows[lastIdx] : null;
    if (prev && prev.kind === 'narration') {
      prev.text += '\n' + line;
      continue;
    }
    rows.push({
      id: nextId(), kind: 'narration', speaker: '', text: line, side: 'left',
      raw: line, blankBefore: pendingBlank,
    });
    pendingBlank = false;
    lastIdx = rows.length - 1;
  }
  flushProse();
  return rows;
}

// ---------- 序列化 ----------

/** 一行气泡 → 源文本行 */
function rowToLines(r: ScriptRow): string[] {
  switch (r.kind) {
    case 'dialogue': {
      const mark = r.side === 'right' ? '##' : '#';
      // 合并过的多段台词要逐段重复角色名，否则第二段会被解析成旁白
      return String(r.text || '')
        .split('\n')
        .filter((s) => s.trim() !== '')
        .map((s) => `${mark}${r.speaker}：${s}`);
    }
    case 'stage':
      return [`【${r.text}】`];
    case 'image':
      return [`![${r.alt || ''}](${r.url || ''})`];
    default:
      return String(r.text || '').split('\n');
  }
}

/** 气泡行序列 → 话本源文本 */
export function serializeScriptRows(rows: ScriptRow[]): string {
  const out: string[] = [];
  rows.forEach((r, i) => {
    if (i > 0 && r.blankBefore) out.push('');
    out.push(...rowToLines(r));
  });
  return out.join('\n');
}

// ---------- 角色 ----------

/** 按首现顺序取出正文里出现的角色名（与服务端 Speakers 一致） */
export function collectSpeakerNames(rows: ScriptRow[]): string[] {
  const out: string[] = [];
  const seen = new Set<string>();
  for (const r of rows) {
    if (r.kind !== 'dialogue') continue;
    if (seen.has(r.speaker)) continue;
    seen.add(r.speaker);
    out.push(r.speaker);
  }
  return out;
}

/**
 * 合并「作品角色表」与「正文里出现但未登记的角色」，得到编辑器左侧角色栏。
 * 未登记角色也会给出稳定的占位色，方便作者一眼分辨。
 */
export function mergeCharacterViews(
  registered: Array<{ name: string; avatar: string; color: string; side: string; intro: string }>,
  rows: ScriptRow[],
): ScriptCharacterView[] {
  const out: ScriptCharacterView[] = [];
  const seen = new Set<string>();
  for (const c of registered) {
    if (!c.name || seen.has(c.name)) continue;
    seen.add(c.name);
    out.push({
      name: c.name,
      avatar: c.avatar || '',
      color: c.color || colorForName(c.name),
      side: c.side === 'right' ? 'right' : 'left',
      registered: true,
      intro: c.intro || '',
    });
  }
  for (const name of collectSpeakerNames(rows)) {
    if (seen.has(name)) continue;
    seen.add(name);
    out.push({
      name,
      avatar: '',
      color: colorForName(name),
      side: 'left',
      registered: false,
      intro: '',
    });
  }
  return out;
}

/** 新建一行（供编辑器插入用） */
export function makeRow(kind: ScriptRowKind, patch: Partial<ScriptRow> = {}): ScriptRow {
  return {
    id: nextId(),
    kind,
    speaker: '',
    text: '',
    side: 'left',
    raw: '',
    blankBefore: false,
    ...patch,
  };
}
