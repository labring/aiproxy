package ipblack

import (
	"context"
	"errors"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/redis/go-redis/v9"
)

const (
	ipBlackKey = "ip_black:%s"
)

func redisSetIPBlack(ctx context.Context, ip string, duration time.Duration) (bool, error) {
	key := common.RedisKeyf(ipBlackKey, ip)

	_, err := common.RDB.SetArgs(ctx, key, duration.Seconds(), redis.SetArgs{Mode: "NX", TTL: duration}).
		Result()
	if errors.Is(err, redis.Nil) {
		// Key already exists, IP is already blocked
		return true, nil
	}

	if err != nil {
		return false, err
	}

	// Key was set successfully, IP was not previously blocked
	return false, nil
}

func redisGetIPIsBlock(ctx context.Context, ip string) (bool, error) {
	key := common.RedisKeyf(ipBlackKey, ip)

	exists, err := common.RDB.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}

	return exists > 0, nil
}
