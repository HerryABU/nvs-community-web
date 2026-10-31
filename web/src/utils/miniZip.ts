/**
 * miniZip — 极简单文件 ZIP 编码器（STORE 模式，不压缩）
 *
 * 用途：AI 对话里生成的单个 HTML 想发布到「HTML 应用广场」，
 * 而广场的上传接口（POST /api/userhtmls/upload）只收 .zip。
 * 与其在后端再开一条并行的发布链路（那会把预检 / 封存 / 额度 / 审核
 * 整套安全逻辑复制一遍，两份实现迟早走偏），不如在前端把字符串打成
 * 一个合法的 ZIP —— 后端完全复用现有管线，零改动。
 *
 * 为什么是 STORE 而不是 DEFLATE：
 *   1. 浏览器原生没有同步 deflate，引入 pako/jszip 只为压一个 HTML 不值得；
 *   2. 后端有「压缩比 ≤ 100:1」的炸弹检测，DEFLATE 反而更容易踩线；
 *   3. 单文件 HTML 体积本就有限（≤ 2MB），压不压对传输没影响。
 *
 * 格式依据：PKWARE APPNOTE 6.3.x —— Local file header / Central directory /
 * End of central directory 三段结构，全部小端序。
 */

// ── CRC-32（IEEE 802.3，多项式 0xEDB88320）────────────────────────
// 表驱动实现，首次调用时构建（256 项，开销可忽略）。
let crcTable: Uint32Array | null = null;

function getCrcTable(): Uint32Array {
  if (crcTable) return crcTable;
  const table = new Uint32Array(256);
  for (let i = 0; i < 256; i++) {
    let c = i;
    for (let k = 0; k < 8; k++) {
      c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    }
    table[i] = c >>> 0;
  }
  crcTable = table;
  return table;
}

export function crc32(data: Uint8Array): number {
  const table = getCrcTable();
  let crc = 0xffffffff;
  for (let i = 0; i < data.length; i++) {
    crc = table[(crc ^ data[i]) & 0xff] ^ (crc >>> 8);
  }
  return (crc ^ 0xffffffff) >>> 0;
}

// ── DOS 时间戳（ZIP 用 1980 纪元的 16 位日期 / 时间）──────────────
function toDosDateTime(d: Date): { date: number; time: number } {
  const year = Math.max(1980, d.getFullYear());
  const date = ((year - 1980) << 9) | ((d.getMonth() + 1) << 5) | d.getDate();
  const time = (d.getHours() << 11) | (d.getMinutes() << 5) | Math.floor(d.getSeconds() / 2);
  return { date, time };
}

const utf8 = (s: string): Uint8Array => new TextEncoder().encode(s);

export interface ZipEntry {
  /** ZIP 内路径，正斜杠分隔，如 `index.html` */
  name: string;
  /** 文本或二进制内容 */
  data: string | Uint8Array;
}

/**
 * 把若干条目打成一个 STORE 模式的 ZIP Blob。
 * 文件名按 UTF-8 编码并在通用标志位标记 bit 11（0x0800），
 * 这样中文名在任何解压器里都不会乱码。
 */
export function createZip(entries: ZipEntry[], when: Date = new Date()): Blob {
  if (!entries.length) throw new Error('ZIP 至少需要一个文件');
  const { date, time } = toDosDateTime(when);
  const FLAG_UTF8 = 0x0800;

  const locals: Uint8Array[] = [];
  const centrals: Uint8Array[] = [];
  let offset = 0;

  for (const entry of entries) {
    const nameBytes = utf8(entry.name.replace(/\\/g, '/'));
    const data = typeof entry.data === 'string' ? utf8(entry.data) : entry.data;
    const sum = crc32(data);
    const size = data.length;

    // ── Local file header (30 + nameLen) ──
    const local = new Uint8Array(30 + nameBytes.length);
    const lv = new DataView(local.buffer);
    lv.setUint32(0, 0x04034b50, true); // signature
    lv.setUint16(4, 20, true); // version needed (2.0)
    lv.setUint16(6, FLAG_UTF8, true); // general purpose flags
    lv.setUint16(8, 0, true); // compression method: 0 = store
    lv.setUint16(10, time, true);
    lv.setUint16(12, date, true);
    lv.setUint32(14, sum, true); // crc32
    lv.setUint32(18, size, true); // compressed size
    lv.setUint32(22, size, true); // uncompressed size
    lv.setUint16(26, nameBytes.length, true);
    lv.setUint16(28, 0, true); // extra field length
    local.set(nameBytes, 30);
    locals.push(local);
    locals.push(data);

    // ── Central directory header (46 + nameLen) ──
    const central = new Uint8Array(46 + nameBytes.length);
    const cv = new DataView(central.buffer);
    cv.setUint32(0, 0x02014b50, true); // signature
    cv.setUint16(4, 20, true); // version made by
    cv.setUint16(6, 20, true); // version needed
    cv.setUint16(8, FLAG_UTF8, true);
    cv.setUint16(10, 0, true); // method: store
    cv.setUint16(12, time, true);
    cv.setUint16(14, date, true);
    cv.setUint32(16, sum, true);
    cv.setUint32(20, size, true);
    cv.setUint32(24, size, true);
    cv.setUint16(28, nameBytes.length, true);
    cv.setUint16(30, 0, true); // extra
    cv.setUint16(32, 0, true); // comment
    cv.setUint16(34, 0, true); // disk number start
    cv.setUint16(36, 0, true); // internal attributes
    cv.setUint32(38, 0, true); // external attributes
    cv.setUint32(42, offset, true); // local header offset
    central.set(nameBytes, 46);
    centrals.push(central);

    offset += local.length + size;
  }

  const centralSize = centrals.reduce((n, c) => n + c.length, 0);

  // ── End of central directory (22) ──
  const eocd = new Uint8Array(22);
  const ev = new DataView(eocd.buffer);
  ev.setUint32(0, 0x06054b50, true);
  ev.setUint16(4, 0, true); // this disk
  ev.setUint16(6, 0, true); // disk with cd
  ev.setUint16(8, entries.length, true);
  ev.setUint16(10, entries.length, true);
  ev.setUint32(12, centralSize, true);
  ev.setUint32(16, offset, true); // cd offset
  ev.setUint16(20, 0, true); // comment length

  return new Blob([...locals, ...centrals, eocd] as BlobPart[], { type: 'application/zip' });
}
