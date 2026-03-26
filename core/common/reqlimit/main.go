package reqlimit

import (
	"context"
	"strconv"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

type ChannelModelRate struct {
	RPM int64 `json:"rpm"`
	TPM int64 `json:"tpm"`
	RPS int64 `json:"rps"`
	TPS int64 `json:"tps"`
}

type RateSnapshot struct {
	Keys        []string `json:"keys"`
	TotalCount  int64    `json:"total_count"`
	SecondCount int64    `json:"second_count"`
}

func snapshotRateRecord(
	ctx context.Context,
	duration time.Duration,
	redisRecord *redisRateRecord,
	memoryRecord *InMemoryRecord,
	keys ...string,
) ([]RateSnapshot, error) {
	var (
		rawSnapshots []recordSnapshot
		err          error
	)

	if common.RedisEnabled {
		rawSnapshots, err = redisRecord.SnapshotByPattern(ctx, duration, keys...)
		if err != nil {
			log.Error("redis snapshot error: " + err.Error())
		}
	}

	if len(rawSnapshots) == 0 {
		rawSnapshots = memoryRecord.SnapshotByPattern(duration, keys...)
	}

	snapshots := make([]RateSnapshot, 0, len(rawSnapshots))
	for _, snapshot := range rawSnapshots {
		snapshots = append(snapshots, RateSnapshot{
			Keys:        append([]string(nil), snapshot.Keys...),
			TotalCount:  snapshot.TotalCount,
			SecondCount: snapshot.SecondCount,
		})
	}

	return snapshots, nil
}

var (
	memoryGroupModelLimiter = NewInMemoryRecord()
	redisGroupModelLimiter  = newRedisGroupModelRecord(func() *redis.Client { return common.RDB })
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

func GetGroupModelRequestSnapshots(
	ctx context.Context,
	group, model string,
) ([]RateSnapshot, error) {
	if group == "" {
		group = "*"
	}
	if model == "" {
		model = "*"
	}

	return snapshotRateRecord(
		ctx,
		time.Minute,
		redisGroupModelLimiter,
		memoryGroupModelLimiter,
		group,
		model,
	)
}

var (
	memoryGroupModelTokennameLimiter = NewInMemoryRecord()
	redisGroupModelTokennameLimiter  = newRedisGroupModelTokennameRecord(
		func() *redis.Client { return common.RDB },
	)
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

func GetGroupModelTokennameRequestSnapshots(
	ctx context.Context,
	group, model, tokenname string,
) ([]RateSnapshot, error) {
	if group == "" {
		group = "*"
	}
	if model == "" {
		model = "*"
	}
	if tokenname == "" {
		tokenname = "*"
	}

	return snapshotRateRecord(
		ctx,
		time.Minute,
		redisGroupModelTokennameLimiter,
		memoryGroupModelTokennameLimiter,
		group,
		model,
		tokenname,
	)
}

var (
	memoryChannelModelRecord = NewInMemoryRecord()
	redisChannelModelRecord  = newRedisChannelModelRecord(
		func() *redis.Client { return common.RDB },
	)
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
	redisGroupModelTokensLimiter  = newRedisGroupModelTokensRecord(
		func() *redis.Client { return common.RDB },
	)
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

func GetGroupModelTokensRequestSnapshots(
	ctx context.Context,
	group, model string,
) ([]RateSnapshot, error) {
	if group == "" {
		group = "*"
	}
	if model == "" {
		model = "*"
	}

	return snapshotRateRecord(
		ctx,
		time.Minute,
		redisGroupModelTokensLimiter,
		memoryGroupModelTokensLimiter,
		group,
		model,
	)
}

var (
	memoryGroupModelTokennameTokensLimiter = NewInMemoryRecord()
	redisGroupModelTokennameTokensLimiter  = newRedisGroupModelTokennameTokensRecord(
		func() *redis.Client { return common.RDB },
	)
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

func GetGroupModelTokennameTokensRequestSnapshots(
	ctx context.Context,
	group, model, tokenname string,
) ([]RateSnapshot, error) {
	if group == "" {
		group = "*"
	}
	if model == "" {
		model = "*"
	}
	if tokenname == "" {
		tokenname = "*"
	}

	return snapshotRateRecord(
		ctx,
		time.Minute,
		redisGroupModelTokennameTokensLimiter,
		memoryGroupModelTokennameTokensLimiter,
		group,
		model,
		tokenname,
	)
}

var (
	memoryChannelModelTokensRecord = NewInMemoryRecord()
	redisChannelModelTokensRecord  = newRedisChannelModelTokensRecord(
		func() *redis.Client { return common.RDB },
	)
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

func GetAllChannelModelRates(ctx context.Context) (map[int64]map[string]ChannelModelRate, error) {
	requests := make(map[int64]map[string]ChannelModelRate)
	appendSnapshot := func(snapshot recordSnapshot, assign func(rate *ChannelModelRate)) {
		if len(snapshot.Keys) != 2 {
			return
		}

		channelID, err := strconv.ParseInt(snapshot.Keys[0], 10, 64)
		if err != nil {
			return
		}

		model := snapshot.Keys[1]

		if _, ok := requests[channelID]; !ok {
			requests[channelID] = make(map[string]ChannelModelRate)
		}

		rate := requests[channelID][model]
		assign(&rate)
		requests[channelID][model] = rate
	}

	var (
		requestSnapshots []recordSnapshot
		tokenSnapshots   []recordSnapshot
		err              error
	)

	if common.RedisEnabled {
		requestSnapshots, err = redisChannelModelRecord.Snapshot(ctx, time.Minute)
		if err != nil {
			log.Error("redis snapshot request error: " + err.Error())
		}

		tokenSnapshots, err = redisChannelModelTokensRecord.Snapshot(ctx, time.Minute)
		if err != nil {
			log.Error("redis snapshot token error: " + err.Error())
		}
	}

	if len(requestSnapshots) == 0 {
		requestSnapshots = memoryChannelModelRecord.Snapshot(time.Minute)
	}

	if len(tokenSnapshots) == 0 {
		tokenSnapshots = memoryChannelModelTokensRecord.Snapshot(time.Minute)
	}

	for _, snapshot := range requestSnapshots {
		appendSnapshot(snapshot, func(rate *ChannelModelRate) {
			rate.RPM = snapshot.TotalCount
			rate.RPS = snapshot.SecondCount
		})
	}

	for _, snapshot := range tokenSnapshots {
		appendSnapshot(snapshot, func(rate *ChannelModelRate) {
			rate.TPM = snapshot.TotalCount
			rate.TPS = snapshot.SecondCount
		})
	}

	return requests, nil
}
