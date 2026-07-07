package config

import (
	"os"
)

var (
	// Database
	DBDriver   string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// Redis
	RedisHost string
	RedisPort string

	// JWT
	JWTSecret     string
	JWTExpireHours string

	// File storage
	NovelDataDir string
	UploadDir    string

	// Server
	ServerPort string
)

func Init() {
	DBDriver = getEnv("DB_DRIVER", "sqlite")
	DBHost = getEnv("DB_HOST", "127.0.0.1")
	DBPort = getEnv("DB_PORT", "3306")
	DBUser = getEnv("DB_USER", "nvs_user")
	DBPassword = getEnv("DB_PASSWORD", "nvs_pass_2026")
	DBName = getEnv("DB_NAME", "nvs")

	RedisHost = getEnv("REDIS_HOST", "127.0.0.1")
	RedisPort = getEnv("REDIS_PORT", "6379")

	JWTSecret = getEnv("JWT_SECRET", "change-me-in-production")
	JWTExpireHours = getEnv("JWT_EXPIRE_HOURS", "72")

	NovelDataDir = getEnv("NOVEL_DATA_DIR", "./data/novels")
	UploadDir = getEnv("UPLOAD_DIR", "./data/uploads")

	ServerPort = getEnv("SERVER_PORT", "8080")
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}
