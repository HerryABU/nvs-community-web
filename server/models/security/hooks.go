package security

import "nvs-server/models"

// Models 返回本域（安全/合规治理）需要迁移的所有模型。
// 原根包 models.AllModels 中的 XssRule / BadWord / BadWordFlag / Vulnerability /
// Blacklist / CrisisEvent 已下沉到本子包，经 init 注册后由根包统一聚合迁移，
// 根包不再直接 import 本子包，避免「根↔域」导入循环。
func Models() []interface{} {
	return []interface{}{
		&XssRule{},
		&BadWord{},
		&BadWordFlag{},
		&Vulnerability{},
		&Blacklist{},
		&CrisisEvent{},
	}
}

func init() {
	models.RegisterDomainModels(Models)
}
