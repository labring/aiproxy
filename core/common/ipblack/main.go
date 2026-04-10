package ipblack

import (
	"context"
	"time"

	"github.com/labring/aiproxy/core/common"
	log "github.com/sirupsen/logrus"
)

const redisTimeout = 2 * time.Second

func SetIPBlackAnyWay(ip string, duration time.Duration) {
	memSetIPBlack(ip, duration)
	cacheSetIPBlackLocal(ip, true)

	if common.RedisEnabled {
		ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
		defer cancel()

		_, err := redisSetIPBlack(ctx, ip, duration)
		if err == nil {
			return
		}

		log.Errorf("failed to set IP %s black: %s", ip, err)
	}
}

func GetIPIsBlockAnyWay(ctx context.Context, ip string) bool {
	if ok, exists := cacheGetIPBlackLocal(ip); exists {
		return ok
	}

	if common.RedisEnabled {
		ok, err := redisGetIPIsBlock(ctx, ip)
		if err == nil {
			cacheSetIPBlackLocal(ip, ok)
			return ok
		}

		log.Errorf("failed to get IP %s is block: %s", ip, err)
	}

	ok := memGetIPIsBlock(ip)
	cacheSetIPBlackLocal(ip, ok)

	return ok
}
