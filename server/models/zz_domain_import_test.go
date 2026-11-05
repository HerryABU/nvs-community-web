package models_test

// 触发各域子包 init() 注册，使 AutoMigrateAll 能把拆出的域模型（如博客）一并迁移。
// 必须用外部测试包（models_test）导入，否则 package models 内部测试直接导入
// models/blog 会与 blog→models 的引用形成 import cycle。
import _ "nvs-server/models/blog"
