package honeypot

import (
	"nvs-server/models"

	"gorm.io/gorm"
)

// modelsDB 取主库连接（保持 models 依赖集中在这一个文件，便于独立进程替换）
func modelsDB() *gorm.DB {
	return models.DB
}
