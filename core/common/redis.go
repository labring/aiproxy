package common

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/labring/aiproxy/core/common/config"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

var (
	RDB          *redis.Client
	RedisEnabled = false
)

const defaultRedisKeyPrefix = "aiproxy"

func RedisKeyPrefix() string {
	if config.RedisKeyPrefix == "" {
		return defaultRedisKeyPrefix
	}
	return config.RedisKeyPrefix
}

func RedisKeyf(format string, args ...any) string {
	if len(args) == 0 {
		return RedisKeyPrefix() + ":" + format
	}
	return RedisKeyPrefix() + ":" + fmt.Sprintf(format, args...)
}

func RedisKey(keys ...string) string {
	if len(keys) == 0 {
		panic("redis keys is empty")
	}

	if len(keys) == 1 {
		return RedisKeyPrefix() + ":" + keys[0]
	}

	return RedisKeyPrefix() + ":" + strings.Join(keys, ":")
}

// InitRedisClient This function is called after init()
func InitRedisClient() (err error) {
	redisConn := config.Redis
	if redisConn == "" {
		log.Info("REDIS not set, redis is not enabled")
		return nil
	}

	RedisEnabled = true

	log.Info("redis is enabled")

	opt, err := redis.ParseURL(redisConn)
	if err != nil {
		log.Fatal("failed to parse redis connection string: " + err.Error())
	}

	RDB = redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = RDB.Ping(ctx).Result()
	if err != nil {
		log.Errorf("failed to ping redis: %s", err.Error())
	}

	return nil
}
