// 角色判定统一口径（与后端 models/social 保持一致）。
//
// 管理端角色分层：
//   - super_admin / super  超级管理员（站长），全站唯一
//   - admin                管理员（可有多位）
//   - manager              经理（管理端账号）
//   - police               警方取管台（准管理员）
//
// 兼容性：既有站点首位管理员 role=admin，由后端 SuperAdminID() 回退识别为超级管理员；
// 因此前端「是否超管」优先取后端下发的 is_super 标志，其次看角色。

/** 是否为超级管理员（站长）角色（含 super 别名）。 */
export function isSuperRole(role?: string | null): boolean {
  return role === 'super_admin' || role === 'super';
}

/** 是否是管理端账号（超级管理员 / 管理员 / 经理）。 */
export function isElevatedRole(role?: string | null): boolean {
  return isSuperRole(role) || role === 'admin' || role === 'manager';
}

/** 严格管理员（不含超级管理员/经理）。 */
export function isStrictAdmin(role?: string | null): boolean {
  return role === 'admin';
}
