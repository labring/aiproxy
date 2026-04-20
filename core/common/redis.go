package common

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
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
const adminKeyCachePrefix = "admin:key"
const adminKeySyncInterval = 500 * time.Millisecond

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

	if config.GetAdminKey() != "" {
		if err := syncAdminKeyCache(ctx, config.GetAdminKey()); err != nil {
			log.Errorf("failed to sync admin key cache: %s", err.Error())
		}
	} else {
		if err := loadAdminKeyCache(ctx); err != nil {
			log.Errorf("failed to load admin key cache: %s", err.Error())
		}
	}

	if hasAdminKeyCacheScope() {
		go watchAdminKeyCache()
	}

	return nil
}

func loadAdminKeyCache(ctx context.Context) error {
	if config.GetAdminKey() != "" {
		return nil
	}

	adminKey, err := loadCachedAdminKey(ctx)
	if err != nil {
		return err
	}
	if adminKey == "" {
		return nil
	}

	config.SetAdminKey(adminKey)
	log.Info("admin key loaded from redis")

	return nil
}

func syncAdminKeyCache(ctx context.Context, adminKey string) error {
	if !hasAdminKeyCacheScope() {
		return nil
	}

	return RDB.Set(ctx, getAdminKeyCacheKey(), adminKey, 0).Err()
}

func loadCachedAdminKey(ctx context.Context) (string, error) {
	if !hasAdminKeyCacheScope() {
		return "", nil
	}

	adminKey, err := RDB.Get(ctx, getAdminKeyCacheKey()).Result()
	if err == nil {
		return adminKey, nil
	}
	if err != redis.Nil {
		return "", err
	}

	return "", nil
}

func watchAdminKeyCache() {
	ticker := time.NewTicker(adminKeySyncInterval)
	defer ticker.Stop()

	lastSeen := config.GetAdminKey()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		adminKey, err := loadCachedAdminKey(ctx)
		cancel()
		if err != nil || adminKey == "" || adminKey == lastSeen {
			continue
		}

		config.SetAdminKey(adminKey)
		lastSeen = adminKey
		log.Warn("admin key updated from redis")
	}
}

func getAdminKeyCacheKey() string {
	scopeHash := sha256.Sum256([]byte(adminKeyCacheScope()))
	return RedisKeyf("%s:%s", adminKeyCachePrefix, hex.EncodeToString(scopeHash[:]))
}

func hasAdminKeyCacheScope() bool {
	return adminKeyCacheScope() != ""
}

func adminKeyCacheScope() string {
	switch {
	case config.GetInternalToken() != "":
		return "internal-token:" + config.GetInternalToken()
	case os.Getenv("SEALOS_JWT_KEY") != "":
		return "sealos-jwt:" + os.Getenv("SEALOS_JWT_KEY")
	case config.RedisKeyPrefix != "":
		return "redis-prefix:" + config.RedisKeyPrefix
	default:
		return ""
	}
}
