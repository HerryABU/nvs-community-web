package utils

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

// RedisClient 全局 Redis 客户端
var RedisClient *redis.Client

// ConnectRedis 按参数连接 Redis（host 为空则禁用）。
// 用于 MySQL 模式下切换数据库/建站向导后重连 Redis，验证码与限流落到 Redis。
func ConnectRedis(host, port, password string, db int) {
	if host == "" {
		RedisClient = nil
		return
	}
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     host + ":" + port,
		Password: password,
		DB:       db,
	})
	if _, err := RedisClient.Ping(context.Background()).Result(); err != nil {
		log.Printf("[Redis] 连接失败: %v，验证码将使用内存存储", err)
		RedisClient = nil
	}
}
