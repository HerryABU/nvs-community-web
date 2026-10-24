package groups

import "gorm.io/gorm"

// Migrate 群组模块建表。
// 群组是独立功能包（server/groups），models 包不反向引用它，避免包循环；
// 因此在 main 里紧随 models.AutoMigrateAll 之后调用本函数。
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&Group{},
		&GroupMember{},
		&GroupPost{},
		&GroupPostReply{},
		&GroupFile{},
		&UserGroupQuota{},
		&GroupReadState{},
	)
}
