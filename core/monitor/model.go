package monitor

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/redis/go-redis/v9"
)

// Redis key prefixes and patterns
const (
	bannedKeySuffix       = ":banned"
	statsKeySuffix        = ":stats"
	modelTotalStatsSuffix = ":total_stats"
	channelKeyPart        = ":channel:"
	groupChannelKeyPrefix = "group_channel:"
)

func modelKeyPrefix() string {
	return common.RedisKey("model:")
}

func groupChannelModelKeyPrefix() string {
	return common.RedisKey("group_channel", "model:")
}

func groupChannelMonitorRedisPrefix() string {
	return common.RedisKey("group_channel")
}

// Redis scripts
var (
	addRequestScript                 = redis.NewScript(addRequestLuaScript)
	getErrorRateScript               = redis.NewScript(getErrorRateLuaScript)
	getStatsSnapshotScript           = redis.NewScript(getStatsSnapshotLuaScript)
	clearChannelModelErrorsScript    = redis.NewScript(clearChannelModelErrorsLuaScript)
	clearChannelAllModelErrorsScript = redis.NewScript(clearChannelAllModelErrorsLuaScript)
	clearAllModelErrorsScript        = redis.NewScript(clearAllModelErrorsLuaScript)
	redisMonitorModel                = newRedisModelMonitor(
		modelKeyPrefix,
		common.RedisKeyPrefix,
		func() *redis.Client {
			return common.RDB
		},
	)
	redisGroupChannelMonitorModel = newRedisModelMonitor(
		groupChannelModelKeyPrefix,
		groupChannelMonitorRedisPrefix,
		func() *redis.Client { return common.RDB },
	)
)

type redisModelMonitor struct {
	keyPrefix   func() string
	redisPrefix func() string
	getRDB      func() *redis.Client
}

func newRedisModelMonitor(
	keyPrefix func() string,
	redisPrefix func() string,
	getRDB func() *redis.Client,
) *redisModelMonitor {
	return &redisModelMonitor{
		keyPrefix:   keyPrefix,
		redisPrefix: redisPrefix,
		getRDB:      getRDB,
	}
}

func (m *redisModelMonitor) modelKeyPrefix() string {
	if m == nil || m.keyPrefix == nil {
		return modelKeyPrefix()
	}

	return m.keyPrefix()
}

func (m *redisModelMonitor) redisKeyPrefix() string {
	if m == nil || m.redisPrefix == nil {
		return common.RedisKeyPrefix()
	}

	return m.redisPrefix()
}

func (m *redisModelMonitor) cacheModel(model string) string {
	if m.modelKeyPrefix() == modelKeyPrefix() {
		return model
	}

	return m.modelKeyPrefix() + model
}

func GetModelsErrorRate(ctx context.Context) (map[string]float64, error) {
	if !common.RedisEnabled {
		return memModelMonitor.GetModelsErrorRate(ctx)
	}

	return redisMonitorModel.GetModelsErrorRate(ctx)
}

func (m *redisModelMonitor) rdb() (*redis.Client, error) {
	if m == nil || m.getRDB == nil {
		return nil, errors.New("redis client getter is nil")
	}

	rdb := m.getRDB()
	if rdb == nil {
		return nil, errors.New("redis client is nil")
	}

	return rdb, nil
}

func (m *redisModelMonitor) GetModelsErrorRate(ctx context.Context) (map[string]float64, error) {
	rdb, err := m.rdb()
	if err != nil {
		return nil, err
	}

	result := make(map[string]float64)
	keyPrefix := m.modelKeyPrefix()
	pattern := keyPrefix + "*" + modelTotalStatsSuffix

	now := time.Now().UnixMilli()

	iter := rdb.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		model := strings.TrimPrefix(key, keyPrefix)
		model = strings.TrimSuffix(model, modelTotalStatsSuffix)

		rate, err := getErrorRateScript.Run(
			ctx,
			rdb,
			[]string{key},
			now,
		).Float64()
		if err != nil {
			return nil, err
		}

		result[model] = rate
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// AddRequest adds a request record, returns the current error rate and checks
// whether channel-model should be temporarily banned.
func AddRequest(
	ctx context.Context,
	model string,
	channelID int64,
	isError, tryBan bool,
	maxErrorRate float64,
) (errorRate float64, banExecution bool, err error) {
	if !common.RedisEnabled {
		errorRate, banExecution = memModelMonitor.AddRequest(
			model,
			channelID,
			isError,
			tryBan,
			maxErrorRate,
		)

		return errorRate, banExecution, nil
	}

	return redisMonitorModel.AddRequest(
		ctx,
		model,
		channelID,
		isError,
		tryBan,
		maxErrorRate,
	)
}

func AddRequestByChannelKey(
	ctx context.Context,
	modelName string,
	channelKey string,
	isError, tryBan bool,
	maxErrorRate float64,
) (errorRate float64, banExecution bool, err error) {
	if strings.HasPrefix(channelKey, groupChannelKeyPrefix) {
		return AddGroupChannelRequestByChannelKey(
			ctx,
			modelName,
			channelKey,
			isError,
			tryBan,
			maxErrorRate,
		)
	}

	if channelKey == "" {
		channelKey = "0"
	}

	if !common.RedisEnabled {
		errorRate, banExecution = memModelMonitor.AddRequestByChannelKey(
			modelName,
			channelKey,
			isError,
			tryBan,
			maxErrorRate,
		)

		return errorRate, banExecution, nil
	}

	return redisMonitorModel.AddRequestByChannelKey(
		ctx,
		modelName,
		channelKey,
		isError,
		tryBan,
		maxErrorRate,
	)
}

func AddGroupChannelRequestByChannelKey(
	ctx context.Context,
	modelName string,
	channelKey string,
	isError, tryBan bool,
	maxErrorRate float64,
) (errorRate float64, banExecution bool, err error) {
	if channelKey == "" {
		channelKey = "0"
	}

	if !common.RedisEnabled {
		errorRate, banExecution = memGroupChannelModelMonitor.AddRequestByChannelKey(
			modelName,
			channelKey,
			isError,
			tryBan,
			maxErrorRate,
		)

		return errorRate, banExecution, nil
	}

	return redisGroupChannelMonitorModel.AddRequestByChannelKey(
		ctx,
		modelName,
		channelKey,
		isError,
		tryBan,
		maxErrorRate,
	)
}

func (m *redisModelMonitor) AddRequest(
	ctx context.Context,
	model string,
	channelID int64,
	isError, tryBan bool,
	maxErrorRate float64,
) (errorRate float64, banExecution bool, err error) {
	rdb, err := m.rdb()
	if err != nil {
		return 0, false, err
	}

	errorFlag := 0
	if isError {
		errorFlag = 1
	} else {
		tryBan = false
	}

	now := time.Now().UnixMilli()

	val, err := addRequestScript.Run(
		ctx,
		rdb,
		[]string{m.redisKeyPrefix(), model},
		channelID,
		errorFlag,
		now,
		maxErrorRate,
		tryBan,
		getBanDuration().Milliseconds(),
	).Slice()
	if err != nil {
		return 0, false, err
	}

	banExecution, errorRate, err = parseAddRequestResult(val)
	if err != nil {
		return 0, false, err
	}

	return errorRate, banExecution, nil
}

func (m *redisModelMonitor) AddRequestByChannelKey(
	ctx context.Context,
	modelName string,
	channelKey string,
	isError, tryBan bool,
	maxErrorRate float64,
) (errorRate float64, banExecution bool, err error) {
	rdb, err := m.rdb()
	if err != nil {
		return 0, false, err
	}

	errorFlag := 0
	if isError {
		errorFlag = 1
	} else {
		tryBan = false
	}

	val, err := addRequestScript.Run(
		ctx,
		rdb,
		[]string{m.redisKeyPrefix(), modelName},
		channelKey,
		errorFlag,
		time.Now().UnixMilli(),
		maxErrorRate,
		tryBan,
		getBanDuration().Milliseconds(),
	).Slice()
	if err != nil {
		return 0, false, err
	}

	banExecution, errorRate, err = parseAddRequestResult(val)

	return errorRate, banExecution, err
}

func parseAddRequestResult(result []any) (banExecution bool, errorRate float64, err error) {
	if len(result) != 2 {
		return false, 0, fmt.Errorf("unexpected add request result length: %d", len(result))
	}

	banExecution, err = parseLuaBoolNumber(result[0])
	if err != nil {
		return false, 0, fmt.Errorf("parse ban execution: %w", err)
	}

	errorRate, err = parseLuaFloat(result[1])
	if err != nil {
		return false, 0, fmt.Errorf("parse error rate: %w", err)
	}

	return banExecution, errorRate, nil
}

func parseLuaBoolNumber(value any) (bool, error) {
	number, err := parseLuaFloat(value)
	if err != nil {
		return false, err
	}

	return math.Abs(number) > 0, nil
}

func parseLuaFloat(value any) (float64, error) {
	switch v := value.(type) {
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	case []byte:
		return strconv.ParseFloat(string(v), 64)
	case nil:
		return 0, nil
	default:
		return 0, fmt.Errorf("unsupported lua result type %T", value)
	}
}

func buildStatsKey(keyPrefix, model, channelID string) string {
	return fmt.Sprintf(
		"%s%s%s%v%s",
		keyPrefix,
		model,
		channelKeyPart,
		channelID,
		statsKeySuffix,
	)
}

func getModelChannelID(keyPrefix, key string) (string, int64, bool) {
	content := strings.TrimPrefix(key, keyPrefix)
	content = strings.TrimSuffix(content, statsKeySuffix)

	model, channelIDStr, ok := strings.Cut(content, channelKeyPart)
	if !ok {
		return "", 0, false
	}

	channelID, err := strconv.ParseInt(channelIDStr, 10, 64)
	if err != nil {
		return "", 0, false
	}

	return model, channelID, true
}

func getModelChannelKey(keyPrefix, key string) (string, string, bool) {
	content := strings.TrimPrefix(key, keyPrefix)
	content = strings.TrimSuffix(content, statsKeySuffix)

	modelName, channelKey, ok := strings.Cut(content, channelKeyPart)
	if !ok {
		return "", "", false
	}

	return modelName, channelKey, true
}

// GetChannelModelErrorRates gets error rates for a specific channel
func GetChannelModelErrorRates(ctx context.Context, channelID int64) (map[string]float64, error) {
	if !common.RedisEnabled {
		return memModelMonitor.GetChannelModelErrorRates(ctx, channelID)
	}

	return redisMonitorModel.GetChannelModelErrorRates(ctx, channelID)
}

func GetChannelModelErrorRate(
	ctx context.Context,
	model string,
	channelID int64,
) (float64, error) {
	if !common.RedisEnabled {
		return memModelMonitor.GetChannelModelErrorRate(ctx, model, channelID)
	}

	return redisMonitorModel.GetChannelModelErrorRate(ctx, model, channelID)
}

func (m *redisModelMonitor) GetChannelModelErrorRates(
	ctx context.Context,
	channelID int64,
) (map[string]float64, error) {
	rdb, err := m.rdb()
	if err != nil {
		return nil, err
	}

	result := make(map[string]float64)
	keyPrefix := m.modelKeyPrefix()
	pattern := buildStatsKey(keyPrefix, "*", strconv.FormatInt(channelID, 10))
	now := time.Now().UnixMilli()

	iter := rdb.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()

		model, _, ok := getModelChannelID(keyPrefix, key)
		if !ok {
			continue
		}

		rate, err := getErrorRateScript.Run(
			ctx,
			rdb,
			[]string{key},
			now,
		).Float64()
		if err != nil {
			return nil, err
		}

		result[model] = rate
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func GetModelChannelErrorRate(ctx context.Context, model string) (map[int64]float64, error) {
	if !common.RedisEnabled {
		return memModelMonitor.GetModelChannelErrorRate(ctx, model)
	}

	return redisMonitorModel.GetModelChannelErrorRate(ctx, model)
}

func GetModelChannelErrorRateByKey(ctx context.Context, model string) (map[string]float64, error) {
	if !common.RedisEnabled {
		return memModelMonitor.GetModelChannelErrorRateByKey(ctx, model)
	}
	return redisMonitorModel.GetModelChannelErrorRateByKey(ctx, model)
}

func GetGroupChannelModelErrorRateByKey(
	ctx context.Context,
	model string,
) (map[string]float64, error) {
	if !common.RedisEnabled {
		return memGroupChannelModelMonitor.GetModelChannelErrorRateByKey(ctx, model)
	}
	return redisGroupChannelMonitorModel.GetModelChannelErrorRateByKey(ctx, model)
}

func (m *redisModelMonitor) GetModelChannelErrorRateByKey(
	ctx context.Context,
	model string,
) (map[string]float64, error) {
	cacheModel := m.cacheModel(model)
	if result, ok := getModelChannelStringErrorRateLocal(cacheModel); ok {
		return result, nil
	}

	return loadWithLocalKeyLock(
		monitorLocalLoadLocker,
		modelChannelErrorRateLocalCacheKey(cacheModel)+":string",
		func() (map[string]float64, bool) {
			return getModelChannelStringErrorRateLocal(cacheModel)
		},
		func() (map[string]float64, error) {
			rdb, err := m.rdb()
			if err != nil {
				return nil, err
			}

			result := make(map[string]float64)
			keyPrefix := m.modelKeyPrefix()
			pattern := buildStatsKey(keyPrefix, model, "*")
			now := time.Now().UnixMilli()

			iter := rdb.Scan(ctx, 0, pattern, 0).Iterator()
			for iter.Next(ctx) {
				key := iter.Val()

				_, channelKey, ok := getModelChannelKey(keyPrefix, key)
				if !ok {
					continue
				}

				rate, err := getErrorRateScript.Run(ctx, rdb, []string{key}, now).Float64()
				if err != nil {
					return nil, err
				}

				result[channelKey] = rate
			}

			if err := iter.Err(); err != nil {
				return nil, err
			}

			setModelChannelStringErrorRateLocalUnlocked(cacheModel, result)

			return result, nil
		},
	)
}

func (m *redisModelMonitor) GetModelChannelErrorRate(
	ctx context.Context,
	model string,
) (map[int64]float64, error) {
	cacheModel := m.cacheModel(model)
	if result, ok := getModelChannelErrorRateLocal(cacheModel); ok {
		return result, nil
	}

	return loadWithLocalKeyLock(
		monitorLocalLoadLocker,
		modelChannelErrorRateLocalCacheKey(cacheModel),
		func() (map[int64]float64, bool) {
			return getModelChannelErrorRateLocal(cacheModel)
		},
		func() (map[int64]float64, error) {
			rdb, err := m.rdb()
			if err != nil {
				return nil, err
			}

			result := make(map[int64]float64)
			keyPrefix := m.modelKeyPrefix()
			pattern := buildStatsKey(keyPrefix, model, "*")
			now := time.Now().UnixMilli()

			iter := rdb.Scan(ctx, 0, pattern, 0).Iterator()
			for iter.Next(ctx) {
				key := iter.Val()

				_, channelID, ok := getModelChannelID(keyPrefix, key)
				if !ok {
					continue
				}

				rate, err := getErrorRateScript.Run(
					ctx,
					rdb,
					[]string{key},
					now,
				).Float64()
				if err != nil {
					return nil, err
				}

				result[channelID] = rate
			}

			if err := iter.Err(); err != nil {
				return nil, err
			}

			setModelChannelErrorRateLocalUnlocked(cacheModel, result)

			return result, nil
		},
	)
}

func (m *redisModelMonitor) GetChannelModelErrorRate(
	ctx context.Context,
	model string,
	channelID int64,
) (float64, error) {
	cacheModel := m.cacheModel(model)
	if rate, ok := getChannelModelErrorRateLocal(cacheModel, channelID); ok {
		return rate, nil
	}

	if rates, ok := getModelChannelErrorRateLocal(cacheModel); ok {
		rate := rates[channelID]
		setChannelModelErrorRateLocalUnlocked(cacheModel, channelID, rate)
		return rate, nil
	}

	return loadWithLocalKeyLock(
		monitorLocalLoadLocker,
		channelModelErrorRateLocalCacheKey(cacheModel, channelID),
		func() (float64, bool) {
			return getChannelModelErrorRateLocal(cacheModel, channelID)
		},
		func() (float64, error) {
			rdb, err := m.rdb()
			if err != nil {
				return 0, err
			}

			rate, err := getErrorRateScript.Run(
				ctx,
				rdb,
				[]string{
					buildStatsKey(m.modelKeyPrefix(), model, strconv.FormatInt(channelID, 10)),
				},
				time.Now().UnixMilli(),
			).Float64()
			if err != nil {
				return 0, err
			}

			setChannelModelErrorRateLocalUnlocked(cacheModel, channelID, rate)

			return rate, nil
		},
	)
}

func GetChannelModelErrorRateByKey(
	ctx context.Context,
	modelName, channelKey string,
) (float64, error) {
	if strings.HasPrefix(channelKey, groupChannelKeyPrefix) {
		return GetGroupChannelChannelModelErrorRateByKey(ctx, modelName, channelKey)
	}

	if !common.RedisEnabled {
		return memModelMonitor.GetChannelModelErrorRateByKey(ctx, modelName, channelKey)
	}

	return redisMonitorModel.GetChannelModelErrorRateByKey(ctx, modelName, channelKey)
}

func GetGroupChannelChannelModelErrorRateByKey(
	ctx context.Context,
	modelName, channelKey string,
) (float64, error) {
	if !common.RedisEnabled {
		return memGroupChannelModelMonitor.GetChannelModelErrorRateByKey(ctx, modelName, channelKey)
	}
	return redisGroupChannelMonitorModel.GetChannelModelErrorRateByKey(ctx, modelName, channelKey)
}

func (m *redisModelMonitor) GetChannelModelErrorRateByKey(
	ctx context.Context,
	modelName string,
	channelKey string,
) (float64, error) {
	cacheModel := m.cacheModel(modelName)
	if rate, ok := getChannelModelStringErrorRateLocal(cacheModel, channelKey); ok {
		return rate, nil
	}

	if rates, ok := getModelChannelStringErrorRateLocal(cacheModel); ok {
		rate := rates[channelKey]
		setChannelModelStringErrorRateLocalUnlocked(cacheModel, channelKey, rate)
		return rate, nil
	}

	return loadWithLocalKeyLock(
		monitorLocalLoadLocker,
		channelModelErrorRateStringLocalCacheKey(cacheModel, channelKey),
		func() (float64, bool) {
			return getChannelModelStringErrorRateLocal(cacheModel, channelKey)
		},
		func() (float64, error) {
			rdb, err := m.rdb()
			if err != nil {
				return 0, err
			}

			rate, err := getErrorRateScript.Run(
				ctx,
				rdb,
				[]string{buildStatsKey(m.modelKeyPrefix(), modelName, channelKey)},
				time.Now().UnixMilli(),
			).Float64()
			if err != nil {
				return 0, err
			}

			setChannelModelStringErrorRateLocalUnlocked(cacheModel, channelKey, rate)

			return rate, nil
		},
	)
}

// GetBannedChannelsWithModel gets banned channels for a specific model
func GetBannedChannelsWithModel(ctx context.Context, model string) ([]int64, error) {
	if !common.RedisEnabled {
		return memModelMonitor.GetBannedChannelsWithModel(ctx, model)
	}

	return redisMonitorModel.GetBannedChannelsWithModel(ctx, model)
}

func (m *redisModelMonitor) GetBannedChannelsWithModel(
	ctx context.Context,
	model string,
) ([]int64, error) {
	rdb, err := m.rdb()
	if err != nil {
		return nil, err
	}

	result := []int64{}
	prefix := m.modelKeyPrefix() + model + channelKeyPart
	pattern := prefix + "*" + bannedKeySuffix
	iter := rdb.Scan(ctx, 0, pattern, 0).Iterator()

	for iter.Next(ctx) {
		key := iter.Val()
		channelIDStr := strings.TrimSuffix(strings.TrimPrefix(key, prefix), bannedKeySuffix)

		channelID, err := strconv.ParseInt(channelIDStr, 10, 64)
		if err != nil {
			continue
		}

		result = append(result, channelID)
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// GetBannedChannelsMapWithModel gets banned channels for a specific model as a map for efficient lookups
func GetBannedChannelsMapWithModel(ctx context.Context, model string) (map[int64]struct{}, error) {
	if !common.RedisEnabled {
		return memModelMonitor.GetBannedChannelsMapWithModel(ctx, model)
	}

	return redisMonitorModel.GetBannedChannelsMapWithModel(ctx, model)
}

func GetBannedChannelKeysMapWithModel(
	ctx context.Context,
	model string,
) (map[string]struct{}, error) {
	if !common.RedisEnabled {
		return memModelMonitor.GetBannedChannelsMapWithModelByKey(ctx, model)
	}

	return redisMonitorModel.GetBannedChannelKeysMapWithModel(ctx, model)
}

func GetGroupChannelBannedChannelKeysMapWithModel(
	ctx context.Context,
	model string,
) (map[string]struct{}, error) {
	if !common.RedisEnabled {
		return memGroupChannelModelMonitor.GetBannedChannelsMapWithModelByKey(ctx, model)
	}

	return redisGroupChannelMonitorModel.GetBannedChannelKeysMapWithModel(ctx, model)
}

func (m *redisModelMonitor) GetBannedChannelsMapWithModel(
	ctx context.Context,
	model string,
) (map[int64]struct{}, error) {
	cacheModel := m.cacheModel(model)
	if result, ok := getBannedChannelsLocal(cacheModel); ok {
		return result, nil
	}

	return loadWithLocalKeyLock(
		monitorLocalLoadLocker,
		bannedChannelsLocalCacheKey(cacheModel),
		func() (map[int64]struct{}, bool) {
			return getBannedChannelsLocal(cacheModel)
		},
		func() (map[int64]struct{}, error) {
			rdb, err := m.rdb()
			if err != nil {
				return nil, err
			}

			result := make(map[int64]struct{})
			prefix := m.modelKeyPrefix() + model + channelKeyPart
			pattern := prefix + "*" + bannedKeySuffix
			iter := rdb.Scan(ctx, 0, pattern, 0).Iterator()

			for iter.Next(ctx) {
				key := iter.Val()
				channelIDStr := strings.TrimSuffix(strings.TrimPrefix(key, prefix), bannedKeySuffix)

				channelID, err := strconv.ParseInt(channelIDStr, 10, 64)
				if err != nil {
					continue
				}

				result[channelID] = struct{}{}
			}

			if err := iter.Err(); err != nil {
				return nil, err
			}

			setBannedChannelsLocalUnlocked(cacheModel, result)

			return result, nil
		},
	)
}

func (m *redisModelMonitor) GetBannedChannelKeysMapWithModel(
	ctx context.Context,
	model string,
) (map[string]struct{}, error) {
	cacheModel := m.cacheModel(model)
	if result, ok := getBannedChannelKeysLocal(cacheModel); ok {
		return result, nil
	}

	return loadWithLocalKeyLock(
		monitorLocalLoadLocker,
		bannedChannelsStringLocalCacheKey(cacheModel),
		func() (map[string]struct{}, bool) {
			return getBannedChannelKeysLocal(cacheModel)
		},
		func() (map[string]struct{}, error) {
			rdb, err := m.rdb()
			if err != nil {
				return nil, err
			}

			result := make(map[string]struct{})
			prefix := m.modelKeyPrefix() + model + channelKeyPart
			pattern := prefix + "*" + bannedKeySuffix
			iter := rdb.Scan(ctx, 0, pattern, 0).Iterator()

			for iter.Next(ctx) {
				key := iter.Val()
				channelKey := strings.TrimSuffix(strings.TrimPrefix(key, prefix), bannedKeySuffix)
				result[channelKey] = struct{}{}
			}

			if err := iter.Err(); err != nil {
				return nil, err
			}

			setBannedChannelKeysLocalUnlocked(cacheModel, result)

			return result, nil
		},
	)
}

// ClearChannelModelErrors clears errors for a specific channel and model
func ClearChannelModelErrors(ctx context.Context, model string, channelID int) error {
	if !common.RedisEnabled {
		return memModelMonitor.ClearChannelModelErrors(ctx, model, channelID)
	}

	return redisMonitorModel.ClearChannelModelErrors(ctx, model, channelID)
}

func (m *redisModelMonitor) ClearChannelModelErrors(
	ctx context.Context,
	model string,
	channelID int,
) error {
	return m.ClearChannelModelErrorsByKey(ctx, model, strconv.Itoa(channelID))
}

func ClearChannelModelErrorsByKey(ctx context.Context, model, channelKey string) error {
	if strings.HasPrefix(channelKey, groupChannelKeyPrefix) {
		return ClearGroupChannelModelErrorsByKey(ctx, model, channelKey)
	}

	if !common.RedisEnabled {
		return memModelMonitor.ClearChannelModelErrorsByKey(ctx, model, channelKey)
	}

	return redisMonitorModel.ClearChannelModelErrorsByKey(ctx, model, channelKey)
}

func ClearGroupChannelModelErrorsByKey(
	ctx context.Context,
	model string,
	channelKey string,
) error {
	if !common.RedisEnabled {
		return memGroupChannelModelMonitor.ClearChannelModelErrorsByKey(ctx, model, channelKey)
	}

	return redisGroupChannelMonitorModel.ClearChannelModelErrorsByKey(ctx, model, channelKey)
}

func (m *redisModelMonitor) ClearChannelModelErrorsByKey(
	ctx context.Context,
	model string,
	channelKey string,
) error {
	rdb, err := m.rdb()
	if err != nil {
		return err
	}

	err = clearChannelModelErrorsScript.Run(
		ctx,
		rdb,
		[]string{m.redisKeyPrefix(), model},
		channelKey,
	).Err()
	if err == nil {
		cacheModel := m.cacheModel(model)
		deleteModelChannelErrorRateLocal(cacheModel)
		deleteChannelModelStringErrorRateLocal(cacheModel, channelKey)
		deleteBannedChannelsLocal(cacheModel)
	}

	return err
}

// ClearChannelAllModelErrors clears all errors for a specific channel
func ClearChannelAllModelErrors(ctx context.Context, channelID int) error {
	if !common.RedisEnabled {
		return memModelMonitor.ClearChannelAllModelErrors(ctx, channelID)
	}

	return redisMonitorModel.ClearChannelAllModelErrors(ctx, channelID)
}

func (m *redisModelMonitor) ClearChannelAllModelErrors(ctx context.Context, channelID int) error {
	return m.ClearChannelAllModelErrorsByKey(ctx, strconv.Itoa(channelID))
}

func ClearChannelAllModelErrorsByKey(ctx context.Context, channelKey string) error {
	if strings.HasPrefix(channelKey, groupChannelKeyPrefix) {
		return ClearGroupChannelAllModelErrorsByKey(ctx, channelKey)
	}

	if !common.RedisEnabled {
		return memModelMonitor.ClearChannelAllModelErrorsByKey(ctx, channelKey)
	}

	return redisMonitorModel.ClearChannelAllModelErrorsByKey(ctx, channelKey)
}

func ClearGroupChannelAllModelErrorsByKey(ctx context.Context, channelKey string) error {
	if !common.RedisEnabled {
		return memGroupChannelModelMonitor.ClearChannelAllModelErrorsByKey(ctx, channelKey)
	}

	return redisGroupChannelMonitorModel.ClearChannelAllModelErrorsByKey(ctx, channelKey)
}

func (m *redisModelMonitor) ClearChannelAllModelErrorsByKey(
	ctx context.Context,
	channelKey string,
) error {
	rdb, err := m.rdb()
	if err != nil {
		return err
	}

	err = clearChannelAllModelErrorsScript.Run(
		ctx,
		rdb,
		[]string{m.redisKeyPrefix()},
		channelKey,
	).Err()
	if err == nil {
		flushMonitorLocalCache()
	}

	return err
}

// ClearAllModelErrors clears all error records
func ClearAllModelErrors(ctx context.Context) error {
	if !common.RedisEnabled {
		return memModelMonitor.ClearAllModelErrors(ctx)
	}

	return redisMonitorModel.ClearAllModelErrors(ctx)
}

func (m *redisModelMonitor) ClearAllModelErrors(ctx context.Context) error {
	rdb, err := m.rdb()
	if err != nil {
		return err
	}

	err = clearAllModelErrorsScript.Run(ctx, rdb, []string{m.redisKeyPrefix()}).Err()
	if err == nil {
		flushMonitorLocalCache()
	}

	return err
}

// GetAllBannedModelChannels gets all banned channels for all models
func GetAllBannedModelChannels(ctx context.Context) (map[string][]int64, error) {
	if !common.RedisEnabled {
		return memModelMonitor.GetAllBannedModelChannels(ctx)
	}

	return redisMonitorModel.GetAllBannedModelChannels(ctx)
}

func (m *redisModelMonitor) GetAllBannedModelChannels(
	ctx context.Context,
) (map[string][]int64, error) {
	rdb, err := m.rdb()
	if err != nil {
		return nil, err
	}

	result := make(map[string][]int64)
	keyPrefix := m.modelKeyPrefix()
	pattern := keyPrefix + "*" + channelKeyPart + "*" + bannedKeySuffix
	iter := rdb.Scan(ctx, 0, pattern, 0).Iterator()

	for iter.Next(ctx) {
		key := iter.Val()
		parts := strings.TrimPrefix(key, keyPrefix)
		parts = strings.TrimSuffix(parts, bannedKeySuffix)

		model, channelIDStr, ok := strings.Cut(parts, channelKeyPart)
		if !ok {
			continue
		}

		channelID, err := strconv.ParseInt(channelIDStr, 10, 64)
		if err != nil {
			continue
		}

		if _, exists := result[model]; !exists {
			result[model] = []int64{}
		}

		result[model] = append(result[model], channelID)
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// GetAllChannelModelErrorRates gets error rates for all channels and models
func GetAllChannelModelErrorRates(ctx context.Context) (map[int64]map[string]float64, error) {
	if !common.RedisEnabled {
		return memModelMonitor.GetAllChannelModelErrorRates(ctx)
	}

	return redisMonitorModel.GetAllChannelModelErrorRates(ctx)
}

func GetAllModelChannelStats(
	ctx context.Context,
) (map[string]map[int64]ModelChannelStatsSnapshot, error) {
	if !common.RedisEnabled {
		return memModelMonitor.GetAllModelChannelStats(ctx)
	}

	return redisMonitorModel.GetAllModelChannelStats(ctx)
}

func (m *redisModelMonitor) GetAllChannelModelErrorRates(
	ctx context.Context,
) (map[int64]map[string]float64, error) {
	rdb, err := m.rdb()
	if err != nil {
		return nil, err
	}

	result := make(map[int64]map[string]float64)
	keyPrefix := m.modelKeyPrefix()
	pattern := buildStatsKey(keyPrefix, "*", "*")
	now := time.Now().UnixMilli()

	iter := rdb.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()

		model, channelID, ok := getModelChannelID(keyPrefix, key)
		if !ok {
			continue
		}

		rate, err := getErrorRateScript.Run(
			ctx,
			rdb,
			[]string{key},
			now,
		).Float64()
		if err != nil {
			return nil, err
		}

		if _, exists := result[channelID]; !exists {
			result[channelID] = make(map[string]float64)
		}

		result[channelID][model] = rate
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (m *redisModelMonitor) GetAllModelChannelStats(
	ctx context.Context,
) (map[string]map[int64]ModelChannelStatsSnapshot, error) {
	rdb, err := m.rdb()
	if err != nil {
		return nil, err
	}

	result := make(map[string]map[int64]ModelChannelStatsSnapshot)
	keyPrefix := m.modelKeyPrefix()
	pattern := buildStatsKey(keyPrefix, "*", "*")
	now := time.Now().UnixMilli()

	iter := rdb.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()

		model, channelID, ok := getModelChannelID(keyPrefix, key)
		if !ok {
			continue
		}

		stats, err := getStatsSnapshotScript.Run(ctx, rdb, []string{key}, now).Int64Slice()
		if err != nil {
			return nil, err
		}

		if _, exists := result[model]; !exists {
			result[model] = make(map[int64]ModelChannelStatsSnapshot)
		}

		bannedKey := fmt.Sprintf(
			"%s%s%s%d%s",
			keyPrefix,
			model,
			channelKeyPart,
			channelID,
			bannedKeySuffix,
		)

		banned, err := rdb.Exists(ctx, bannedKey).Result()
		if err != nil {
			return nil, err
		}

		reqCount := int64(0)
		errCount := int64(0)

		if len(stats) > 0 {
			reqCount = stats[0]
		}

		if len(stats) > 1 {
			errCount = stats[1]
		}

		result[model][channelID] = ModelChannelStatsSnapshot{
			Requests: reqCount,
			Errors:   errCount,
			Banned:   banned > 0,
		}
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// Lua scripts
const (
	addRequestLuaScript = `
local prefix = KEYS[1]
local model = KEYS[2]
local channel_id = ARGV[1]
local is_error = tonumber(ARGV[2])
local now_ts = tonumber(ARGV[3])
local max_error_rate = tonumber(ARGV[4])
local try_ban = tonumber(ARGV[5])
local banExpiry = tonumber(ARGV[6])

local banned_key = prefix .. ":model:" .. model .. ":channel:" .. channel_id .. ":banned"
local stats_key = prefix .. ":model:" .. model .. ":channel:" .. channel_id .. ":stats"
local model_stats_key = prefix .. ":model:" .. model .. ":total_stats"
local maxSliceCount = 12
local statsExpiry = maxSliceCount * 10 * 1000
local current_slice = math.floor(now_ts / 10 / 1000)
local total_req_field = "__meta_total_req"
local total_err_field = "__meta_total_err"
local last_cleaned_field = "__meta_last_cleaned_slice"

local function parse_req_err(value)
    if not value then return 0, 0 end
    local r, e = value:match("^(%d+):(%d+)$")
    return tonumber(r) or 0, tonumber(e) or 0
end

local function get_clean_req_err(key)
	local total_req = tonumber(redis.call("HGET", key, total_req_field)) or 0
	local total_err = tonumber(redis.call("HGET", key, total_err_field)) or 0
	local last_cleaned = tonumber(redis.call("HGET", key, last_cleaned_field))
	local min_valid_slice = current_slice - maxSliceCount

    if not last_cleaned then
        total_req = 0
        total_err = 0
        local all_slices = redis.call("HGETALL", key)
        for i = 1, #all_slices, 2 do
            local slice = tonumber(all_slices[i])
            if slice then
                if slice < min_valid_slice then
                    redis.call("HDEL", key, all_slices[i])
                else
                    local req, err = parse_req_err(all_slices[i+1])
                    total_req = total_req + req
                    total_err = total_err + err
                end
            end
        end
    else
        for slice = last_cleaned, min_valid_slice - 1 do
            local value = redis.call("HGET", key, tostring(slice))
            if value then
                local req, err = parse_req_err(value)
                total_req = total_req - req
                total_err = total_err - err
                redis.call("HDEL", key, tostring(slice))
            end
        end
    end

    redis.call(
        "HSET",
        key,
        total_req_field,
        total_req,
        total_err_field,
        total_err,
        last_cleaned_field,
        min_valid_slice
    )
    redis.call("PEXPIRE", key, statsExpiry)

	return total_req, total_err
end

local function update_stats(key)
    local total_req, total_err = get_clean_req_err(key)
    local req, err = parse_req_err(redis.call("HGET", key, current_slice))
    req = req + 1
    err = err + (is_error == 1 and 1 or 0)
    total_req = total_req + 1
    total_err = total_err + (is_error == 1 and 1 or 0)
    redis.call(
        "HSET",
        key,
        current_slice,
        req .. ":" .. err,
        total_req_field,
        total_req,
        total_err_field,
        total_err,
        last_cleaned_field,
        current_slice - maxSliceCount
    )
    redis.call("PEXPIRE", key, statsExpiry)
    return req, err
end

update_stats(stats_key)
update_stats(model_stats_key)

local function check_channel_error()
    local already_banned = redis.call("EXISTS", banned_key) == 1
    local total_req, total_err = get_clean_req_err(stats_key)
    local error_rate = 0
    if total_req >= 10 then
        error_rate = total_err / total_req
    end
    local error_rate_str = tostring(error_rate)

	if try_ban == 1 then
		if already_banned then
			return {0, error_rate_str}
		end
		redis.call("SET", banned_key, 1)
		redis.call("PEXPIRE", banned_key, banExpiry)
		return {1, error_rate_str}
	end

	if total_req < 10 then
		return {0, 0}
	end

	-- Check if we should ban (only if max_error_rate is set and exceeded)
	if max_error_rate > 0 and error_rate >= max_error_rate then
		if already_banned then
			return {0, error_rate_str}
		end
		redis.call("SET", banned_key, 1)
		redis.call("PEXPIRE", banned_key, banExpiry)
		return {1, error_rate_str}
	end

	return {0, error_rate_str}
end

return check_channel_error()
`

	getErrorRateLuaScript = `
local stats_key = KEYS[1]
local now_ts = tonumber(ARGV[1])
local maxSliceCount = 12
local current_slice = math.floor(now_ts / 10 / 1000)
local total_req_field = "__meta_total_req"
local total_err_field = "__meta_total_err"
local last_cleaned_field = "__meta_last_cleaned_slice"
local statsExpiry = maxSliceCount * 10 * 1000

local function parse_req_err(value)
    if not value then return 0, 0 end
    local r, e = value:match("^(%d+):(%d+)$")
    return tonumber(r) or 0, tonumber(e) or 0
end

local function get_clean_req_err(key)
	local total_req = tonumber(redis.call("HGET", key, total_req_field)) or 0
	local total_err = tonumber(redis.call("HGET", key, total_err_field)) or 0
	local last_cleaned = tonumber(redis.call("HGET", key, last_cleaned_field))
	local min_valid_slice = current_slice - maxSliceCount

    if not last_cleaned then
        total_req = 0
        total_err = 0
        local all_slices = redis.call("HGETALL", key)
        for i = 1, #all_slices, 2 do
            local slice = tonumber(all_slices[i])
            if slice then
                if slice < min_valid_slice then
                    redis.call("HDEL", key, all_slices[i])
                else
                    local req, err = parse_req_err(all_slices[i+1])
                    total_req = total_req + req
                    total_err = total_err + err
                end
            end
        end
    else
        for slice = last_cleaned, min_valid_slice - 1 do
            local value = redis.call("HGET", key, tostring(slice))
            if value then
                local req, err = parse_req_err(value)
                total_req = total_req - req
                total_err = total_err - err
                redis.call("HDEL", key, tostring(slice))
            end
        end
    end

    redis.call(
        "HSET",
        key,
        total_req_field,
        total_req,
        total_err_field,
        total_err,
        last_cleaned_field,
        min_valid_slice
    )
    redis.call("PEXPIRE", key, statsExpiry)
	return total_req, total_err
end

local total_req, total_err = get_clean_req_err(stats_key)
if total_req < 10 then return 0 end
return string.format("%.2f", total_err / total_req)
`

	getStatsSnapshotLuaScript = `
local stats_key = KEYS[1]
local now_ts = tonumber(ARGV[1])
local maxSliceCount = 12
local current_slice = math.floor(now_ts / 10 / 1000)
local total_req_field = "__meta_total_req"
local total_err_field = "__meta_total_err"
local last_cleaned_field = "__meta_last_cleaned_slice"
local statsExpiry = maxSliceCount * 10 * 1000

local function parse_req_err(value)
    if not value then return 0, 0 end
    local r, e = value:match("^(%d+):(%d+)$")
    return tonumber(r) or 0, tonumber(e) or 0
end

local total_req = tonumber(redis.call("HGET", stats_key, total_req_field)) or 0
local total_err = tonumber(redis.call("HGET", stats_key, total_err_field)) or 0
local last_cleaned = tonumber(redis.call("HGET", stats_key, last_cleaned_field))
local min_valid_slice = current_slice - maxSliceCount

if not last_cleaned then
    total_req = 0
    total_err = 0
    local all_slices = redis.call("HGETALL", stats_key)
    for i = 1, #all_slices, 2 do
        local slice = tonumber(all_slices[i])
        if slice then
            if slice < min_valid_slice then
                redis.call("HDEL", stats_key, all_slices[i])
            else
                local req, err = parse_req_err(all_slices[i+1])
                total_req = total_req + req
                total_err = total_err + err
            end
        end
    end
else
    for slice = last_cleaned, min_valid_slice - 1 do
        local value = redis.call("HGET", stats_key, tostring(slice))
        if value then
            local req, err = parse_req_err(value)
            total_req = total_req - req
            total_err = total_err - err
            redis.call("HDEL", stats_key, tostring(slice))
        end
    end
end

redis.call(
    "HSET",
    stats_key,
    total_req_field,
    total_req,
    total_err_field,
    total_err,
    last_cleaned_field,
    min_valid_slice
)
redis.call("PEXPIRE", stats_key, statsExpiry)

return { total_req, total_err }
`

	clearChannelModelErrorsLuaScript = `
local prefix = KEYS[1]
local model = KEYS[2]
local channel_id = ARGV[1]
local stats_key = prefix .. ":model:" .. model .. ":channel:" .. channel_id .. ":stats"
local banned_key = prefix .. ":model:" .. model .. ":channel:" .. channel_id .. ":banned"

redis.call("DEL", stats_key)
redis.call("DEL", banned_key)
return redis.status_reply("ok")
`

	clearChannelAllModelErrorsLuaScript = `
local prefix = KEYS[1]
local function del_keys(pattern)
    local keys = redis.call("KEYS", pattern)
    if #keys > 0 then redis.call("DEL", unpack(keys)) end
end

local channel_id = ARGV[1]
local stats_pattern = prefix .. ":model:*:channel:" .. channel_id .. ":stats"
local banned_pattern = prefix .. ":model:*:channel:" .. channel_id .. ":banned"

del_keys(stats_pattern)
del_keys(banned_pattern)

return redis.status_reply("ok")
`

	clearAllModelErrorsLuaScript = `
local prefix = KEYS[1]
local function del_keys(pattern)
    local keys = redis.call("KEYS", pattern)
    if #keys > 0 then redis.call("DEL", unpack(keys)) end
end

del_keys(prefix .. ":model:*:channel:*:stats")
del_keys(prefix .. ":model:*:channel:*:banned")

return redis.status_reply("ok")
`
)
