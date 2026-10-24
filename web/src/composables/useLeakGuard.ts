import { ElMessageBox } from 'element-plus';
import api from '@/api';

/**
 * 个人信息泄漏提示（176 号令 + 平台安全）
 * - 格式类（手机/邮箱/身份证/密钥）：高危，默认拦截并要求确认
 * - 语义类（密码/住址/账号）：低危，只提示但允许发送
 * - **检测不留痕**：内容只在本地正则里过一遍，后端只收「命中类型」计数，绝不发原文
 */
export type LeakType = 'phone' | 'email' | 'idcard' | 'apikey' | 'bankcard' | 'semantic';

interface LeakHit {
  type: LeakType;
  label: string;
  high: boolean;
  sample?: string; // 仅本地用于提示，不上传
}

const PATTERNS: Array<{ type: LeakType; label: string; re: RegExp; high: boolean }> = [
  { type: 'phone', label: '手机号', re: /(?<!\d)1[3-9]\d{9}(?!\d)/g, high: true },
  { type: 'email', label: '邮箱地址', re: /[\w.+-]+@[\w-]+\.[\w.-]+/g, high: false },
  { type: 'idcard', label: '身份证号', re: /(?<!\d)\d{17}[\dXx](?!\d)/g, high: true },
  { type: 'bankcard', label: '银行卡号', re: /(?<!\d)\d{16,19}(?!\d)/g, high: true },
  { type: 'apikey', label: '密钥/Token', re: /(sk-[A-Za-z0-9]{16,}|sk-ant-[A-Za-z0-9_-]{16,}|AIza[0-9A-Za-z_-]{20,}|ghp_[A-Za-z0-9]{20,})/g, high: true },
];

const SEMANTIC = ['我的密码', '密码是', '账号是', '口令', '家庭住址', '住址是', '身份证号是', '银行卡号是'];

/** 本地扫描（纯函数，不发起任何请求） */
export function scanLeak(text: string): LeakHit[] {
  const hits: LeakHit[] = [];
  const seen = new Set<LeakType>();
  for (const p of PATTERNS) {
    const m = text.match(p.re);
    if (m && m.length) {
      if (!seen.has(p.type)) {
        seen.add(p.type);
        hits.push({ type: p.type, label: p.label, high: p.high, sample: maskSample(m[0]) });
      }
    }
  }
  if (SEMANTIC.some((k) => text.includes(k))) {
    hits.push({ type: 'semantic', label: '疑似账号/密码/住址等描述', high: false });
  }
  return hits;
}

/** 打码展示（本地用，避免提示框里回显完整敏感值） */
function maskSample(s: string): string {
  if (s.length <= 6) return s[0] + '***';
  return s.slice(0, 3) + '***' + s.slice(-2);
}

/** 上报「命中类型计数」（不含内容），失败静默 */
async function reportTypes(hits: LeakHit[]) {
  try {
    await api.post('/security/leak-hint', { types: hits.map((h) => h.type) });
  } catch {
    /* 统计失败不影响用户 */
  }
}

/**
 * 提交前守卫：高危命中 → 必须确认（默认不放行需二次点击）；低危 → 提示后放行。
 * 返回 true 表示可以继续提交。
 */
export async function guardLeak(text: string, scene = '内容'): Promise<boolean> {
  if (!text || !text.trim()) return true;
  const hits = scanLeak(text);
  if (!hits.length) return true;
  reportTypes(hits);

  const high = hits.filter((h) => h.high);
  const lines = hits.map((h) => `· ${h.label}${h.sample ? `（${h.sample}）` : ''}`).join('\n');

  if (high.length) {
    try {
      await ElMessageBox.confirm(
        `检测到${scene}中可能包含敏感个人信息：\n${lines}\n\n公开发布后任何人都能看到，建议删除后再提交。`,
        '个人信息保护提示',
        { type: 'warning', confirmButtonText: '我已确认，仍要提交', cancelButtonText: '返回修改' },
      );
      return true;
    } catch {
      return false;
    }
  }

  // 低危：提示但不阻断（3 秒自动消失，允许继续）
  try {
    await ElMessageBox.confirm(
      `提示：${scene}中似乎包含账号/密码/住址等信息：\n${lines}\n\n确认这些内容适合公开吗？`,
      '个人信息提醒',
      { type: 'info', confirmButtonText: '继续提交', cancelButtonText: '返回修改' },
    );
    return true;
  } catch {
    return false;
  }
}
