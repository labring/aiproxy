package task

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

const (
	adminKeyCachePrefix   = "admin:key"
	adminKeySyncInterval  = 500 * time.Millisecond
	adminKeyCacheInitWait = 5 * time.Second
	adminKeyCacheOpWait   = 2 * time.Second
)

func AdminKeyCacheEnabled() bool {
	return common.RedisEnabled && common.RDB != nil && adminKeyCacheScope() != ""
}

func InitAdminKeyCache(ctx context.Context) error {
	if !AdminKeyCacheEnabled() {
		return nil
	}

	opCtx, cancel := context.WithTimeout(ctx, adminKeyCacheInitWait)
	defer cancel()

	return syncAdminKeyCacheOnce(opCtx)
}

func AdminKeyCacheTask(ctx context.Context) {
	if !AdminKeyCacheEnabled() {
		return
	}

	ticker := time.NewTicker(adminKeySyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			opCtx, cancel := context.WithTimeout(ctx, adminKeyCacheOpWait)
			err := syncAdminKeyCacheOnce(opCtx)
			cancel()
			if err != nil {
				log.Debugf("admin key cache sync failed: %v", err)
			}
		}
	}
}

func syncAdminKeyCacheOnce(ctx context.Context) error {
	cachedAdminKey, err := loadCachedAdminKey(ctx)
	if err != nil {
		return err
	}

	localAdminKey := config.GetAdminKey()
	if cachedAdminKey != "" {
		if cachedAdminKey != localAdminKey {
			config.SetAdminKey(cachedAdminKey)
			log.Info("admin key loaded from redis")
		}

		return nil
	}

	if localAdminKey == "" {
		return nil
	}

	created, err := common.RDB.SetNX(ctx, getAdminKeyCacheKey(), localAdminKey, 0).Result()
	if err != nil {
		return err
	}
	if created {
		log.Info("admin key synced to redis")
		return nil
	}

	cachedAdminKey, err = loadCachedAdminKey(ctx)
	if err != nil {
		return err
	}
	if cachedAdminKey != "" && cachedAdminKey != localAdminKey {
		config.SetAdminKey(cachedAdminKey)
		log.Info("admin key loaded from redis")
	}

	return nil
}

func loadCachedAdminKey(ctx context.Context) (string, error) {
	adminKey, err := common.RDB.Get(ctx, getAdminKeyCacheKey()).Result()
	if err == nil {
		return adminKey, nil
	}
	if err != redis.Nil {
		return "", err
	}

	return "", nil
}

func getAdminKeyCacheKey() string {
	scopeHash := sha256.Sum256([]byte(adminKeyCacheScope()))
	return common.RedisKeyf("%s:%s", adminKeyCachePrefix, hex.EncodeToString(scopeHash[:]))
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
