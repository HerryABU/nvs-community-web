package aicost

import "nvs-server/models"

// Models 返回本域需要迁移的所有模型。原根包 models.AllModels 中的这些类型已下沉到
// 本子包，经 init 注册后由根包统一聚合迁移；根包不 import 本子包，避免导入循环。
func Models() []interface{} {
	return []interface{}{
		&ChannelBalanceSample{},
		&UserAiKey{},
	}
}

func init() {
	models.RegisterDomainModels(Models)
}
