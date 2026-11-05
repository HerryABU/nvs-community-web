package softdel

import "nvs-server/models"

// Models 返回本域需要迁移的所有模型。原根包 models.AllModels 中的这些类型已下沉到
// 本子包，经 init 注册后由根包统一聚合迁移；根包不 import 本子包，避免导入循环。
func Models() []interface{} {
	return []interface{}{
		&SoftDelBlog{},
		&SoftDelChapter{},
		&SoftDelComment{},
		&SoftDelForum{},
		&SoftDelNovel{},
		&SoftDelPost{},
		&SoftDelThread{},
		&SoftDelUser{},
		&UserFrame{},
		&UserHTML{},
	}
}

func init() {
	models.RegisterDomainModels(Models)
	// 迁移辅助 / 留证实现随本子包下沉，覆盖根包函数变量（根包不回引本子包）。
	models.DropPostDeletedAtHook = DropPostDeletedAt
	models.DropLegacyIndexesHook = DropLegacyIndexes
	models.BackfillUserHTMLExtractedHook = BackfillUserHTMLExtracted
	models.ArchiveSoftDeleteHook = ArchiveSoftDelete
}
