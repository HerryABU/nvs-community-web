package models_test

// 触发各域子包 init() 注册，使 AutoMigrateAll 能把拆出的域模型一并迁移。
// 必须用外部测试包（models_test）导入，否则 package models 内部测试直接导入
// 域子包会与「域→models」的引用形成 import cycle。
import (
	_ "nvs-server/models/ai"
	_ "nvs-server/models/aicost"
	_ "nvs-server/models/aihub"
	_ "nvs-server/models/aiops"
	_ "nvs-server/models/apikey"
	_ "nvs-server/models/blog"
	_ "nvs-server/models/platform"
	_ "nvs-server/models/quota"
	_ "nvs-server/models/search"
	_ "nvs-server/models/security"
	_ "nvs-server/models/siteops"
	_ "nvs-server/models/social"
	_ "nvs-server/models/softdel"
	_ "nvs-server/models/theme"
	_ "nvs-server/models/wallet"
)
