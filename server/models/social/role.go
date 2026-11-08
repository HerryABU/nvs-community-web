package social

// 角色常量与权限判定（权限模型）
//
// 权限层级（由高到低）：
//
//	admin   管理员（全权限）
//	police  警方取管台（准管理员：仅可取证 / 查看软删除，无普通管理权；不可自注册）
//	vip_author / author / arbitrator / reader
const (
	// 管理端角色分层（用户明确要求）：
	//   - super_admin 超级管理员（站长）：全站唯一一位，可增删改查（含删除规则）。
	//   - admin       管理员：可有多位，仅新增/编辑，删除需站长批准。
	//   - manager     经理：管理端账号，权限同 admin。
	//   - police      警方取管台：准管理员，仅取证/查看软删除，不可自注册。
	RoleSuperAdmin = "super_admin"
	RoleSuper      = "super" // 别名：与 super_admin 等价，兼容「super」称呼
	RoleAdmin      = "admin"
	RoleManager    = "manager"
	RolePolice     = "police" // 警方取管台：准管理员，仅可取证/查看软删除；不可自注册，只能由管理员提权
	RoleVipAuthor  = "vip_author"
	RoleAuthor     = "author"
	RoleArbitrator = "arbitrator"
	RoleReader     = "reader"
)

// IsSuperRole 是否为超级管理员（站长）角色（含 super 别名）。
func IsSuperRole(role string) bool { return role == RoleSuperAdmin || role == RoleSuper }

// IsElevated 是否是管理端账号（超级管理员 / 管理员 / 经理）。
// 警方走自己的台，不算 elevated。
func IsElevated(role string) bool {
	return IsSuperRole(role) || role == RoleAdmin || role == RoleManager
}

// ValidPromotableRoles 管理员可在「用户管理」通用编辑里直接设置的角色。
// 注意：police 不在此列 —— 必须通过专用提权接口设置，以保留审计痕迹，
// 防止普通角色编辑通道被误用/滥用导致越权。
func ValidPromotableRoles() map[string]bool {
	return map[string]bool{
		RoleReader:     true,
		RoleAuthor:     true,
		RoleVipAuthor:  true,
		RoleArbitrator: true,
		RoleAdmin:      true,
		RoleManager:    true,
		// 注意：RoleSuperAdmin / RoleSuper 不在此列 —— 超级管理员全站唯一，
		// 由建站首位账号或既有 role=admin 最小 ID 回退产生，不允许通用编辑指派。
	}
}

func IsPolice(role string) bool        { return role == RolePolice }
func IsAdmin(role string) bool         { return role == RoleAdmin }
func IsAdminOrPolice(role string) bool { return IsElevated(role) || role == RolePolice }
