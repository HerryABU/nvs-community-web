package models

// 角色常量与权限判定（权限模型）
//
// 权限层级（由高到低）：
//
//	admin   管理员（全权限）
//	police  警方取管台（准管理员：仅可取证 / 查看软删除，无普通管理权；不可自注册）
//	vip_author / author / arbitrator / reader
const (
	RoleAdmin      = "admin"
	RolePolice     = "police" // 警方取管台：准管理员，仅可取证/查看软删除；不可自注册，只能由管理员提权
	RoleVipAuthor  = "vip_author"
	RoleAuthor     = "author"
	RoleArbitrator = "arbitrator"
	RoleReader     = "reader"
)

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
	}
}

func IsPolice(role string) bool        { return role == RolePolice }
func IsAdmin(role string) bool         { return role == RoleAdmin }
func IsAdminOrPolice(role string) bool { return role == RoleAdmin || role == RolePolice }
