package reqlimit

import (
	"context"
	"strconv"
	"strings"
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

type ChannelRateScope string

const (
	ChannelRateScopeGlobal ChannelRateScope = "global"
	ChannelRateScopeGroup  ChannelRateScope = "group"
)

func chooseRecordSnapshots(
	redisEnabled bool,
	redisSnapshots []recordSnapshot,
	redisErr error,
	memorySnapshots func() []recordSnapshot,
	logMessage string,
) []recordSnapshot {
	if redisEnabled {
		if redisErr != nil {
			log.Error(logMessage + redisErr.Error())
			return memorySnapshots()
		}

		return redisSnapshots
	}

	return memorySnapshots()
}

func pushRateRecord(
	ctx context.Context,
	redisRecord *redisRateRecord,
	memoryRecord *InMemoryRecord,
	overed int64,
	n int64,
	keys ...string,
) (int64, int64, int64) {
	if common.RedisEnabled {
		count, overLimitCount, secondCount, err := redisRecord.PushRequest(
			ctx,
			overed,
			time.Minute,
			n,
			keys...,
		)
		if err == nil {
			return count, overLimitCount, secondCount
		}

		log.Error("redis push request error: " + err.Error())
	}

	return memoryRecord.PushRequest(overed, time.Minute, n, keys...)
}

func getRateRecord(
	ctx context.Context,
	redisRecord *redisRateRecord,
	memoryRecord *InMemoryRecord,
	keys ...string,
) (int64, int64) {
	if common.RedisEnabled {
		totalCount, secondCount, err := redisRecord.GetRequest(ctx, time.Minute, keys...)
		if err == nil {
			return totalCount, secondCount
		}

		log.Error("redis get request error: " + err.Error())
	}

	return memoryRecord.GetRequest(time.Minute, keys...)
}

//nolint:unparam
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
	}

	rawSnapshots = chooseRecordSnapshots(
		common.RedisEnabled,
		rawSnapshots,
		err,
		func() []recordSnapshot { return memoryRecord.SnapshotByPattern(duration, keys...) },
		"redis snapshot error: ",
	)

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
	return pushRateRecord(
		ctx,
		redisGroupModelLimiter,
		memoryGroupModelLimiter,
		overed,
		1,
		group,
		model,
	)
}

func GetGroupModelRequest(ctx context.Context, group, model string) (int64, int64) {
	if model == "" {
		model = "*"
	}

	return getRateRecord(
		ctx,
		redisGroupModelLimiter,
		memoryGroupModelLimiter,
		group,
		model,
	)
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
	return pushRateRecord(
		ctx,
		redisGroupModelTokennameLimiter,
		memoryGroupModelTokennameLimiter,
		0,
		1,
		group,
		model,
		tokenname,
	)
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

	return getRateRecord(
		ctx,
		redisGroupModelTokennameLimiter,
		memoryGroupModelTokennameLimiter,
		group,
		model,
		tokenname,
	)
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
	memoryGroupChannelModelRecord = NewInMemoryRecord()
	redisGroupChannelModelRecord  = newRedisGroupChannelModelRecord(
		func() *redis.Client { return common.RDB },
	)
)

func PushChannelModelRequest(ctx context.Context, channel, model string) (int64, int64, int64) {
	return pushRateRecord(
		ctx,
		redisChannelModelRecord,
		memoryChannelModelRecord,
		0,
		1,
		channel,
		model,
	)
}

func GetChannelModelRequest(ctx context.Context, channel, model string) (int64, int64) {
	if channel == "" || channel == "*" {
		return getGlobalChannelModelRequest(ctx, model)
	}

	if model == "" {
		model = "*"
	}

	return getChannelModelRequestByPattern(ctx, channel, model)
}

func PushGroupChannelModelRequest(
	ctx context.Context,
	group, channel, model string,
) (int64, int64, int64) {
	return pushRateRecord(
		ctx,
		redisGroupChannelModelRecord,
		memoryGroupChannelModelRecord,
		0,
		1,
		group,
		channel,
		model,
	)
}

func PushScopedChannelModelRequest(
	ctx context.Context,
	scope ChannelRateScope,
	group, channel, model string,
) (int64, int64, int64) {
	if scope == ChannelRateScopeGroup {
		return PushGroupChannelModelRequest(ctx, group, channel, model)
	}

	return PushChannelModelRequest(ctx, channel, model)
}

func GetGroupChannelModelRequest(ctx context.Context, group, channel, model string) (int64, int64) {
	if channel == "" {
		channel = "*"
	}

	if model == "" {
		model = "*"
	}

	return getRateRecord(
		ctx,
		redisGroupChannelModelRecord,
		memoryGroupChannelModelRecord,
		group,
		channel,
		model,
	)
}

func getChannelModelRequestByPattern(ctx context.Context, channel, model string) (int64, int64) {
	if channel == "" {
		channel = "*"
	}

	if model == "" {
		model = "*"
	}

	return getRateRecord(
		ctx,
		redisChannelModelRecord,
		memoryChannelModelRecord,
		channel,
		model,
	)
}

func getGlobalChannelModelRequest(ctx context.Context, model string) (int64, int64) {
	return aggregateChannelModelSnapshots(
		ctx,
		model,
		redisChannelModelRecord,
		memoryChannelModelRecord,
		func(channel string) bool {
			_, err := strconv.ParseInt(channel, 10, 64)
			return err == nil
		},
	)
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
	return pushRateRecord(
		ctx,
		redisGroupModelTokensLimiter,
		memoryGroupModelTokensLimiter,
		maxTokens,
		tokens,
		group,
		model,
	)
}

func GetGroupModelTokensRequest(ctx context.Context, group, model string) (int64, int64) {
	if model == "" {
		model = "*"
	}

	return getRateRecord(
		ctx,
		redisGroupModelTokensLimiter,
		memoryGroupModelTokensLimiter,
		group,
		model,
	)
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
	return pushRateRecord(
		ctx,
		redisGroupModelTokennameTokensLimiter,
		memoryGroupModelTokennameTokensLimiter,
		0,
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

	return getRateRecord(
		ctx,
		redisGroupModelTokennameTokensLimiter,
		memoryGroupModelTokennameTokensLimiter,
		group,
		model,
		tokenname,
	)
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
	memoryGroupChannelModelTokensRecord = NewInMemoryRecord()
	redisGroupChannelModelTokensRecord  = newRedisGroupChannelModelTokensRecord(
		func() *redis.Client { return common.RDB },
	)
)

func PushChannelModelTokensRequest(
	ctx context.Context,
	channel, model string,
	tokens int64,
) (int64, int64, int64) {
	return pushRateRecord(
		ctx,
		redisChannelModelTokensRecord,
		memoryChannelModelTokensRecord,
		0,
		tokens,
		channel,
		model,
	)
}

func GetChannelModelTokensRequest(ctx context.Context, channel, model string) (int64, int64) {
	if channel == "" || channel == "*" {
		return getGlobalChannelModelTokensRequest(ctx, model)
	}

	if model == "" {
		model = "*"
	}

	return getChannelModelTokensRequestByPattern(ctx, channel, model)
}

func PushGroupChannelModelTokensRequest(
	ctx context.Context,
	group, channel, model string,
	tokens int64,
) (int64, int64, int64) {
	return pushRateRecord(
		ctx,
		redisGroupChannelModelTokensRecord,
		memoryGroupChannelModelTokensRecord,
		0,
		tokens,
		group,
		channel,
		model,
	)
}

func PushScopedChannelModelTokensRequest(
	ctx context.Context,
	scope ChannelRateScope,
	group, channel, model string,
	tokens int64,
) (int64, int64, int64) {
	if scope == ChannelRateScopeGroup {
		return PushGroupChannelModelTokensRequest(ctx, group, channel, model, tokens)
	}

	return PushChannelModelTokensRequest(ctx, channel, model, tokens)
}

func GetGroupChannelModelTokensRequest(
	ctx context.Context,
	group, channel, model string,
) (int64, int64) {
	if channel == "" {
		channel = "*"
	}

	if model == "" {
		model = "*"
	}

	return getRateRecord(
		ctx,
		redisGroupChannelModelTokensRecord,
		memoryGroupChannelModelTokensRecord,
		group,
		channel,
		model,
	)
}

func getChannelModelTokensRequestByPattern(
	ctx context.Context,
	channel, model string,
) (int64, int64) {
	if channel == "" {
		channel = "*"
	}

	if model == "" {
		model = "*"
	}

	return getRateRecord(
		ctx,
		redisChannelModelTokensRecord,
		memoryChannelModelTokensRecord,
		channel,
		model,
	)
}

func getGlobalChannelModelTokensRequest(ctx context.Context, model string) (int64, int64) {
	return aggregateChannelModelSnapshots(
		ctx,
		model,
		redisChannelModelTokensRecord,
		memoryChannelModelTokensRecord,
		func(channel string) bool {
			_, err := strconv.ParseInt(channel, 10, 64)
			return err == nil
		},
	)
}

func aggregateChannelModelSnapshots(
	ctx context.Context,
	model string,
	redisRecord *redisRateRecord,
	memoryRecord *InMemoryRecord,
	matchChannel func(string) bool,
) (int64, int64) {
	model = normalizeAggregateModel(model)
	matchSnapshot := func(snapshot recordSnapshot) bool {
		if len(snapshot.Keys) < 2 {
			return false
		}

		snapshotModel := snapshot.Keys[len(snapshot.Keys)-1]
		if model != "" && snapshotModel != model {
			return false
		}

		channel := strings.Join(snapshot.Keys[:len(snapshot.Keys)-1], ":")

		return matchChannel(channel)
	}

	var (
		snapshots []recordSnapshot
		err       error
	)
	if common.RedisEnabled {
		snapshots, err = redisRecord.Snapshot(ctx, time.Minute)
	}

	snapshots = chooseRecordSnapshots(
		common.RedisEnabled,
		snapshots,
		err,
		func() []recordSnapshot { return memoryRecord.Snapshot(time.Minute) },
		"redis channel model snapshot error: ",
	)

	var totalCount, secondCount int64
	for _, snapshot := range snapshots {
		if !matchSnapshot(snapshot) {
			continue
		}

		totalCount += snapshot.TotalCount
		secondCount += snapshot.SecondCount
	}

	return totalCount, secondCount
}

func normalizeAggregateModel(model string) string {
	if model == "*" {
		return ""
	}

	return model
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
		requestSnapshots = chooseRecordSnapshots(
			true,
			requestSnapshots,
			err,
			func() []recordSnapshot { return memoryChannelModelRecord.Snapshot(time.Minute) },
			"redis snapshot request error: ",
		)

		tokenSnapshots, err = redisChannelModelTokensRecord.Snapshot(ctx, time.Minute)
		tokenSnapshots = chooseRecordSnapshots(
			true,
			tokenSnapshots,
			err,
			func() []recordSnapshot { return memoryChannelModelTokensRecord.Snapshot(time.Minute) },
			"redis snapshot token error: ",
		)
	} else {
		requestSnapshots = memoryChannelModelRecord.Snapshot(time.Minute)
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
