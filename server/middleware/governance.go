package middleware

import (
	"regexp"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// 警方可用的「内容治理子集」白名单。
//
// 为什么不用中间件整体放开 /api/admin：
//   - /api/admin 里有合规与安全（脱敏配置、实名记录、密钥、蜜罐取证）、
//     支付、AI 配置、用户角色提权——这些警方一律不该看到；
//   - 一放开就是整片暴露，审计也无从逐项追。
//
// 因此这里采用**显式白名单**：只有列出来的「方法 + 路径」对警方放行，
// 其余仍要求 admin。漏写一条的代价是警方看到 403，安全侧失败；
// 反过来（默认放开再排除）漏一条就是数据泄露，方向必须选前者。
//
// 每次命中白名单都会写一条审计（谁、什么角色、哪个方法路径、结果），
// 满足「警方动作可追溯」的要求。
type governanceRule struct {
	method string
	path   *regexp.Regexp
}

func mustRe(p string) *regexp.Regexp { return regexp.MustCompile(p) }

var governanceRules = []governanceRule{
	// ── 内容封禁 / 软删留证处置 ──
	{"GET", mustRe(`^/api/admin/novels$`)},
	{"GET", mustRe(`^/api/admin/novels/\d+/chapters$`)},
	{"POST", mustRe(`^/api/admin/softdel$`)},
	{"POST", mustRe(`^/api/admin/softdel/restore$`)},

	// ── 举报处理 ──
	{"GET", mustRe(`^/api/admin/reports$`)},
	{"POST", mustRe(`^/api/admin/reports/\d+/handle$`)},

	// ── 审核待办（AI 结论 + 人工处置）──
	{"GET", mustRe(`^/api/admin/moderation/records$`)},
	{"GET", mustRe(`^/api/admin/moderation/stats$`)},
	{"POST", mustRe(`^/api/admin/moderation/records/\d+/decide$`)},

	// ── 舆情观测（只读热词统计）──
	{"GET", mustRe(`^/api/admin/hotwords$`)},
}

// policeAllowed 判断警方是否有权访问某个 admin 接口。
func policeAllowed(method, path string) bool {
	// 去掉可能存在的重复斜杠与尾部斜杠，避免用 // 绕过
	p := strings.TrimSuffix(path, "/")
	if p == "" {
		p = path
	}
	for _, r := range governanceRules {
		if r.method == method && r.path.MatchString(p) {
			return true
		}
	}
	return false
}

// AdminOrPoliceGovernance 管理端组级守卫：
//   - admin：全部放行（原有行为不变）
//   - police：仅白名单内的方法与路径放行，其余 403
//   - 其它角色：403
//
// 注意：本中间件替代原来的 AdminRequired，但**不放宽任何非白名单路径**。
func AdminOrPoliceGovernance() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := UserRole(c)
		switch role {
		case "admin":
			c.Next()
			return
		case "police":
			if policeAllowed(c.Request.Method, c.Request.URL.Path) {
				auditPoliceAccess(c, "allow")
				c.Next()
				return
			}
			auditPoliceAccess(c, "deny")
			utils.Forbidden(c, "该操作不在警方的内容治理权限范围内")
			c.Abort()
			return
		default:
			utils.Forbidden(c, "需要管理员权限")
			c.Abort()
			return
		}
	}
}

// auditPoliceAccess 记录警方的每次越权/放行尝试。
//
// 失败不影响主流程：审计少一条总比请求打不出去好；
// 但「deny」也必须记——被拒的尝试本身是重要信号（有人在试探边界）。
func auditPoliceAccess(c *gin.Context, result string) {
	uid := UserID(c)
	if uid == 0 {
		return
	}
	models.RecordUserAudit(&models.UserAuditLog{
		UserID:    uid,
		Action:    "police_governance_" + result,
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
		Detail:    c.Request.Method + " " + c.Request.URL.Path,
		Result:    result,
	})
}
