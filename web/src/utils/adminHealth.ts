// 后台接口健康状态（极简共享状态，不引 pinia）
//
// 为什么需要它：48 个后台页的错误处理几乎都是 `catch { /* ignore */ }` 或
// `ElMessage.error('加载失败')` —— toast 一闪而过，用户看到的只是"表格空着"，
// 分不清是「本来没数据」还是「接口挂了」。这里由 axios 拦截器统一记录最近一次
// 后台请求的硬失败（网络错误 / 5xx），由 AdminHeader 常驻展示 + 提供重试。
//
// 只记录 5xx 与网络错误：
// 4xx 是业务语义（未授权已由守卫处理、参数错误由页面自己提示），
// 记进来只会变成噪音，反而让真正的故障被忽略。
import { ref, computed } from 'vue';

export interface AdminApiError {
  url: string;
  status: number;
  message: string;
  at: number; // 时间戳（ms）
}

const lastError = ref<AdminApiError | null>(null);
/**
 * 连续失败次数。
 * 阈值取 1：多数后台页进页只发 1~2 个列表请求，阈值设 2 会出现"接口 500 了但什么都不提示"
 * （实测就是这么漏的）。探活/可选接口已在 reportAdminApiError 里排除，不会误报。
 */
const failStreak = ref(0);
const THRESHOLD = 1;
const DISMISS_MS = 60_000;

/** 记录一次后台接口硬失败 */
export function reportAdminApiError(url: string, status: number, message: string) {
  // 探活类接口失败不算故障（页面本来就要处理"未开启"）
  if (/\/admin\/health|\/admin\/ping|\/setup\//.test(url)) return;
  failStreak.value += 1;
  if (failStreak.value < THRESHOLD) return;
  lastError.value = { url, status, message, at: Date.now() };
}

/** 一次成功即可清除（说明后端恢复） */
export function clearAdminApiError() {
  failStreak.value = 0;
  if (lastError.value) lastError.value = null;
}

export function dismissAdminApiError() {
  if (lastError.value) lastError.value = null;
  failStreak.value = 0;
}

export const adminApiError = computed(() => {
  const e = lastError.value;
  if (!e) return null;
  if (Date.now() - e.at > DISMISS_MS) return null;
  return e;
});
