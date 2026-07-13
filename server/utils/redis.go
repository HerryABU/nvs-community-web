package utils

import (
	"github.com/redis/go-redis/v9"
)

// RedisClient 全局 Redis 客户端
var RedisClient *redis.Client
