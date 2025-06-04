package reqlimit

import (
	"context"
	"time"

	"github.com/labring/aiproxy/core/common"
	log "github.com/sirupsen/logrus"
)

var (
	memoryGroupModelLimiter = NewInMemoryRecord()
	redisGroupModelLimiter  = newRedisGroupModelRecord()
)

func PushGroupModelRequest(
	ctx context.Context,
	group, model string,
	overed int64,
) (int64, int64, int64) {
	if common.RedisEnabled {
		count, overLimitCount, secondCount, err := redisGroupModelLimiter.PushRequest(
			ctx,
			overed,
			time.Minute,
			1,
			group,
			model,
		)
		if err == nil {
			return count, overLimitCount, secondCount
		}
		log.Error("redis push request error: " + err.Error())
	}
	return memoryGroupModelLimiter.PushRequest(overed, time.Minute, 1, group, model)
}

func GetGroupModelRequest(ctx context.Context, group, model string) (int64, int64) {
	if model == "" {
		model = "*"
	}
	if common.RedisEnabled {
		totalCount, secondCount, err := redisGroupModelLimiter.GetRequest(
			ctx,
			time.Minute,
			group,
			model,
		)
		if err == nil {
			return totalCount, secondCount
		}
		log.Error("redis get request error: " + err.Error())
	}
	return memoryGroupModelLimiter.GetRequest(time.Minute, group, model)
}

var (
	memoryGroupModelTokennameLimiter = NewInMemoryRecord()
	redisGroupModelTokennameLimiter  = newRedisGroupModelTokennameRecord()
)

func PushGroupModelTokennameRequest(
	ctx context.Context,
	group, model, tokenname string,
) (int64, int64, int64) {
	if common.RedisEnabled {
		count, overLimitCount, secondCount, err := redisGroupModelTokennameLimiter.PushRequest(
			ctx,
			0,
			time.Minute,
			1,
			group,
			model,
			tokenname,
		)
		if err == nil {
			return count, overLimitCount, secondCount
		}
		log.Error("redis push request error: " + err.Error())
	}
	return memoryGroupModelTokennameLimiter.PushRequest(0, time.Minute, 1, group, model, tokenname)
}

func GetGroupModelTokennameRequest(
	ctx context.Context,
	group, model, tokenname string,
) (int64, int64) {
	if model == "" {
		model = "*"
	}
	if tokenname == "" {
		tokenname = "*"
	}
	if common.RedisEnabled {
		totalCount, secondCount, err := redisGroupModelTokennameLimiter.GetRequest(
			ctx,
			time.Minute,
			group,
			model,
			tokenname,
		)
		if err == nil {
			return totalCount, secondCount
		}
		log.Error("redis get request error: " + err.Error())
	}
	return memoryGroupModelTokennameLimiter.GetRequest(time.Minute, group, model, tokenname)
}

var (
	memoryChannelModelRecord = NewInMemoryRecord()
	redisChannelModelRecord  = newRedisChannelModelRecord()
)

func PushChannelModelRequest(ctx context.Context, channel, model string) (int64, int64, int64) {
	if common.RedisEnabled {
		count, overLimitCount, secondCount, err := redisChannelModelRecord.PushRequest(
			ctx,
			0,
			time.Minute,
			1,
			channel,
			model,
		)
		if err == nil {
			return count, overLimitCount, secondCount
		}
		log.Error("redis push request error: " + err.Error())
	}
	return memoryChannelModelRecord.PushRequest(0, time.Minute, 1, channel, model)
}

func GetChannelModelRequest(ctx context.Context, channel, model string) (int64, int64) {
	if channel == "" {
		channel = "*"
	}
	if model == "" {
		model = "*"
	}
	if common.RedisEnabled {
		totalCount, secondCount, err := redisChannelModelRecord.GetRequest(
			ctx,
			time.Minute,
			channel,
			model,
		)
		if err == nil {
			return totalCount, secondCount
		}
		log.Error("redis get request error: " + err.Error())
	}
	return memoryChannelModelRecord.GetRequest(time.Minute, channel, model)
}

var (
	memoryGroupModelTokensLimiter = NewInMemoryRecord()
	redisGroupModelTokensLimiter  = newRedisGroupModelTokensRecord()
)

func PushGroupModelTokensRequest(
	ctx context.Context,
	group, model string,
	maxTokens, tokens int64,
) (int64, int64, int64) {
	if common.RedisEnabled {
		count, overLimitCount, secondCount, err := redisGroupModelTokensLimiter.PushRequest(
			ctx,
			maxTokens,
			time.Minute,
			tokens,
			group,
			model,
		)
		if err == nil {
			return count, overLimitCount, secondCount
		}
		log.Error("redis push request error: " + err.Error())
	}
	return memoryGroupModelTokensLimiter.PushRequest(maxTokens, time.Minute, tokens, group, model)
}

func GetGroupModelTokensRequest(ctx context.Context, group, model string) (int64, int64) {
	if model == "" {
		model = "*"
	}
	if common.RedisEnabled {
		totalCount, secondCount, err := redisGroupModelTokensLimiter.GetRequest(
			ctx,
			time.Minute,
			group,
			model,
		)
		if err == nil {
			return totalCount, secondCount
		}
		log.Error("redis get request error: " + err.Error())
	}
	return memoryGroupModelTokensLimiter.GetRequest(time.Minute, group, model)
}

var (
	memoryGroupModelTokennameTokensLimiter = NewInMemoryRecord()
	redisGroupModelTokennameTokensLimiter  = newRedisGroupModelTokennameTokensRecord()
)

func PushGroupModelTokennameTokensRequest(
	ctx context.Context,
	group, model, tokenname string,
	tokens int64,
) (int64, int64, int64) {
	if common.RedisEnabled {
		count, overLimitCount, secondCount, err := redisGroupModelTokennameTokensLimiter.PushRequest(
			ctx,
			0,
			time.Minute,
			tokens,
			group,
			model,
			tokenname,
		)
		if err == nil {
			return count, overLimitCount, secondCount
		}
		log.Error("redis push request error: " + err.Error())
	}
	return memoryGroupModelTokennameTokensLimiter.PushRequest(
		0,
		time.Minute,
		tokens,
		group,
		model,
		tokenname,
	)
}

func GetGroupModelTokennameTokensRequest(
	ctx context.Context,
	group, model, tokenname string,
) (int64, int64) {
	if model == "" {
		model = "*"
	}
	if tokenname == "" {
		tokenname = "*"
	}
	if common.RedisEnabled {
		totalCount, secondCount, err := redisGroupModelTokennameTokensLimiter.GetRequest(
			ctx,
			time.Minute,
			group,
			model,
			tokenname,
		)
		if err == nil {
			return totalCount, secondCount
		}
		log.Error("redis get request error: " + err.Error())
	}
	return memoryGroupModelTokennameTokensLimiter.GetRequest(time.Minute, group, model, tokenname)
}

var (
	memoryChannelModelTokensRecord = NewInMemoryRecord()
	redisChannelModelTokensRecord  = newRedisChannelModelTokensRecord()
)

func PushChannelModelTokensRequest(
	ctx context.Context,
	channel, model string,
	tokens int64,
) (int64, int64, int64) {
	if common.RedisEnabled {
		count, overLimitCount, secondCount, err := redisChannelModelTokensRecord.PushRequest(
			ctx,
			0,
			time.Minute,
			tokens,
			channel,
			model,
		)
		if err == nil {
			return count, overLimitCount, secondCount
		}
		log.Error("redis push request error: " + err.Error())
	}
	return memoryChannelModelTokensRecord.PushRequest(0, time.Minute, tokens, channel, model)
}

func GetChannelModelTokensRequest(ctx context.Context, channel, model string) (int64, int64) {
	if channel == "" {
		channel = "*"
	}
	if model == "" {
		model = "*"
	}
	if common.RedisEnabled {
		totalCount, secondCount, err := redisChannelModelTokensRecord.GetRequest(
			ctx,
			time.Minute,
			channel,
			model,
		)
		if err == nil {
			return totalCount, secondCount
		}
		log.Error("redis get request error: " + err.Error())
	}
	return memoryChannelModelTokensRecord.GetRequest(time.Minute, channel, model)
}
