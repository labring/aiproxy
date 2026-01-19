package ipblack

import (
	"context"
	"time"

	"github.com/labring/aiproxy/core/common"
	log "github.com/sirupsen/logrus"
)

const redisTimeout = 2 * time.Second

func SetIPBlackAnyWay(ip string, duration time.Duration) {
	if common.RedisEnabled {
		ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
		defer cancel()

		_, err := redisSetIPBlack(ctx, ip, duration)
		if err == nil {
			return
		}

		log.Errorf("failed to set IP %s black: %s", ip, err)
	}

	memSetIPBlack(ip, duration)
}

func GetIPIsBlockAnyWay(ctx context.Context, ip string) bool {
	if common.RedisEnabled {
		ok, err := redisGetIPIsBlock(ctx, ip)
		if err == nil {
			return ok
		}

		log.Errorf("failed to get IP %s is block: %s", ip, err)
	}

	return memGetIPIsBlock(ip)
}
