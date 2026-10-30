/**
 * 警方（police）可见的「内容治理子集」路径白名单。
 *
 * 为什么要有这个文件：后端对 /api/admin/* 用的是 AdminOrPoliceGovernance
 * （警方仅放行治理白名单、其余逐项 403），前端必须按同一口径收敛，
 * 否则会出现「菜单点得进去、页面白屏 403」或「模板泄漏给非授权角色」。
 *
 * 路由守卫与后台侧栏共用本常量，避免两处各写一份而漂移。
 */
export const POLICE_ALLOWED_PATHS: ReadonlySet<string> = new Set<string>([
  // 警方台本体
  '/police',
  '/police/moderation',
  '/police/reports',
  '/police/ban',
  // 复用后台视图的治理页（警方可进；合规与安全、AI 配置、用户管理不在其中）
  '/admin/moderation',
  '/admin/reports',
  '/admin/ban',
  '/admin/ai-moderation',
  '/admin/badwords',
  '/admin/softdel',
  '/admin/sentiment',
  '/admin/hotwords',
]);

/** 该角色能否访问这个路径（仅用于 admin / police 前缀的收敛判断）。 */
export function canAccessPath(role: string | undefined, path: string): boolean {
  const isAdminArea = path === '/admin' || path.startsWith('/admin/');
  const isPoliceArea = path === '/police' || path.startsWith('/police/');

  if (role === 'admin') return true;
  if (role === 'police') {
    if (isPoliceArea) return true;
    return isAdminArea && POLICE_ALLOWED_PATHS.has(path);
  }
  // 其他角色 / 未登录：两个区域都不许进
  return !isAdminArea && !isPoliceArea;
}
