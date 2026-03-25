package reqlimit

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/redis/go-redis/v9"
)

type redisRateRecord struct {
	prefix string
	getRDB func() *redis.Client
}

func newRedisRateRecord(prefix string, getRDB func() *redis.Client) *redisRateRecord {
	return &redisRateRecord{
		prefix: prefix,
		getRDB: getRDB,
	}
}

func newRedisGroupModelRecord(getRDB func() *redis.Client) *redisRateRecord {
	return newRedisRateRecord("group-model-record", getRDB)
}

func newRedisGroupModelTokennameRecord(getRDB func() *redis.Client) *redisRateRecord {
	return newRedisRateRecord("group-model-tokenname-record", getRDB)
}

func newRedisChannelModelRecord(getRDB func() *redis.Client) *redisRateRecord {
	return newRedisRateRecord("channel-model-record", getRDB)
}

func newRedisGroupModelTokensRecord(getRDB func() *redis.Client) *redisRateRecord {
	return newRedisRateRecord("group-model-tokens-record", getRDB)
}

func newRedisGroupModelTokennameTokensRecord(getRDB func() *redis.Client) *redisRateRecord {
	return newRedisRateRecord("group-model-tokenname-tokens-record", getRDB)
}

func newRedisChannelModelTokensRecord(getRDB func() *redis.Client) *redisRateRecord {
	return newRedisRateRecord("channel-model-tokens-record", getRDB)
}

const pushRequestLuaScript = `
local bucket_key = KEYS[1]
local meta_key = KEYS[2]
local window_seconds = tonumber(ARGV[1])
local current_time = tonumber(ARGV[2])
local max_requests = tonumber(ARGV[3])
local n = tonumber(ARGV[4])
local cutoff_slice = current_time - window_seconds

local function parse_count(value)
    if not value then return 0, 0 end
    local r, e = value:match("^(%d+):(%d+)$")
    return tonumber(r) or 0, tonumber(e) or 0
end

local function cleanup_expired(last_cleaned, total_count, total_over_count)
    for ts = last_cleaned, cutoff_slice - 1 do
        local expired_value = redis.call('HGET', bucket_key, tostring(ts))
        if expired_value then
            local c, oc = parse_count(expired_value)
            total_count = total_count - c
            total_over_count = total_over_count - oc
            redis.call('HDEL', bucket_key, tostring(ts))
        end
    end

    return total_count, total_over_count
end

local count = tonumber(redis.call('HGET', meta_key, 'total_normal')) or 0
local over_count = tonumber(redis.call('HGET', meta_key, 'total_over')) or 0
local last_cleaned = tonumber(redis.call('HGET', meta_key, 'last_cleaned_second'))

if not last_cleaned then
    last_cleaned = cutoff_slice
    local all_fields = redis.call('HGETALL', bucket_key)
    count = 0
    over_count = 0
    for i = 1, #all_fields, 2 do
        local field_slice = tonumber(all_fields[i])
        if field_slice < cutoff_slice then
            redis.call('HDEL', bucket_key, all_fields[i])
        else
            local c, oc = parse_count(all_fields[i+1])
            count = count + c
            over_count = over_count + oc
        end
    end
else
    count, over_count = cleanup_expired(last_cleaned, count, over_count)
end

local current_value = redis.call('HGET', bucket_key, tostring(current_time))
local current_c, current_oc = parse_count(current_value)

if max_requests == 0 or count <= max_requests then
	current_c = current_c + n
    count = count + n
else
	current_oc = current_oc + n
	over_count = over_count + n
end
redis.call('HSET', bucket_key, current_time, current_c .. ":" .. current_oc)

redis.call(
    'HSET',
    meta_key,
    'total_normal',
    count,
    'total_over',
    over_count,
    'last_cleaned_second',
    cutoff_slice
)

redis.call('EXPIRE', bucket_key, window_seconds)
redis.call('EXPIRE', meta_key, window_seconds)
local current_second_count = current_c + current_oc
return string.format("%d:%d:%d", count, over_count, current_second_count)
`

const getRequestCountLuaScript = `
local exact_meta_key = KEYS[1]
local pattern = KEYS[2]
local window_seconds = tonumber(ARGV[1])
local current_time = tonumber(ARGV[2])
local cutoff_slice = current_time - window_seconds

local function parse_count(value)
    if not value then return 0, 0 end
    local r, e = value:match("^(%d+):(%d+)$")
    return tonumber(r) or 0, tonumber(e) or 0
end

local function cleanup_meta(bucket_key, meta_key)
    local count = tonumber(redis.call('HGET', meta_key, 'total_normal')) or 0
    local over = tonumber(redis.call('HGET', meta_key, 'total_over')) or 0
    local last_cleaned = tonumber(redis.call('HGET', meta_key, 'last_cleaned_second'))

    if not last_cleaned then
        last_cleaned = cutoff_slice
        local all_fields = redis.call('HGETALL', bucket_key)
        count = 0
        over = 0
        for i=1, #all_fields, 2 do
            local field_slice = tonumber(all_fields[i])
            if field_slice < cutoff_slice then
                redis.call('HDEL', bucket_key, all_fields[i])
            else
                local c, oc = parse_count(all_fields[i+1])
                count = count + c
                over = over + oc
            end
        end
    else
        for ts = last_cleaned, cutoff_slice - 1 do
            local expired_value = redis.call('HGET', bucket_key, tostring(ts))
            if expired_value then
                local c, oc = parse_count(expired_value)
                count = count - c
                over = over - oc
                redis.call('HDEL', bucket_key, tostring(ts))
            end
        end
    end

    redis.call(
        'HSET',
        meta_key,
        'total_normal',
        count,
        'total_over',
        over,
        'last_cleaned_second',
        cutoff_slice
    )
    redis.call('EXPIRE', bucket_key, window_seconds)
    redis.call('EXPIRE', meta_key, window_seconds)

    return count, over
end

if exact_meta_key ~= '' then
    local exact_bucket_key = string.gsub(exact_meta_key, ':meta$', ':buckets')
    local count, over = cleanup_meta(exact_bucket_key, exact_meta_key)
    local current_value = redis.call('HGET', exact_bucket_key, tostring(current_time))
    local current_c, current_oc = parse_count(current_value)
    return string.format("%d:%d", count + over, current_c + current_oc)
end

local total = 0
local current_second_count = 0

local keys = redis.call('KEYS', pattern)
for _, meta_key in ipairs(keys) do
    local bucket_key = string.gsub(meta_key, ':meta$', ':buckets')
    local count, over = cleanup_meta(bucket_key, meta_key)
    total = total + count + over

    local current_value = redis.call('HGET', bucket_key, tostring(current_time))
    local current_c, current_oc = parse_count(current_value)
    current_second_count = current_second_count + current_c + current_oc
end

return string.format("%d:%d", total, current_second_count)
`

var (
	pushRequestScript     = redis.NewScript(pushRequestLuaScript)
	getRequestCountScript = redis.NewScript(getRequestCountLuaScript)
)

func (r *redisRateRecord) buildKey(keys ...string) string {
	return common.RedisKey(r.prefix + ":" + strings.Join(keys, ":"))
}

func (r *redisRateRecord) buildBucketKey(keys ...string) string {
	return r.buildKey(keys...) + ":buckets"
}

func (r *redisRateRecord) buildMetaKey(keys ...string) string {
	return r.buildKey(keys...) + ":meta"
}

func (r *redisRateRecord) GetRequest(
	ctx context.Context,
	duration time.Duration,
	keys ...string,
) (totalCount, secondCount int64, err error) {
	rdb := r.getRDB()
	if rdb == nil {
		return 0, 0, errors.New("redis client is nil")
	}

	exactMetaKey := ""

	pattern := r.buildMetaKey(keys...)
	if !hasWildcard(keys) {
		exactMetaKey = pattern
		pattern = ""
	}

	result, err := getRequestCountScript.Run(
		ctx,
		rdb,
		[]string{exactMetaKey, pattern},
		duration.Seconds(),
		time.Now().Unix(),
	).Text()
	if err != nil {
		return 0, 0, err
	}

	parts := strings.Split(result, ":")
	if len(parts) != 2 {
		return 0, 0, errors.New("invalid result format")
	}

	totalCountInt, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, err
	}

	secondCountInt, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, err
	}

	return totalCountInt, secondCountInt, nil
}

func (r *redisRateRecord) PushRequest(
	ctx context.Context,
	overed int64,
	duration time.Duration,
	n int64,
	keys ...string,
) (normalCount, overCount, secondCount int64, err error) {
	bucketKey := r.buildBucketKey(keys...)
	metaKey := r.buildMetaKey(keys...)

	rdb := r.getRDB()
	if rdb == nil {
		return 0, 0, 0, errors.New("redis client is nil")
	}

	result, err := pushRequestScript.Run(
		ctx,
		rdb,
		[]string{bucketKey, metaKey},
		duration.Seconds(),
		time.Now().Unix(),
		overed,
		n,
	).Text()
	if err != nil {
		return 0, 0, 0, err
	}

	parts := strings.Split(result, ":")
	if len(parts) != 3 {
		return 0, 0, 0, errors.New("invalid result")
	}

	countInt, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, 0, err
	}

	overLimitCountInt, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, 0, err
	}

	secondCountInt, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return 0, 0, 0, err
	}

	return countInt, overLimitCountInt, secondCountInt, nil
}

func (r *redisRateRecord) Snapshot(
	ctx context.Context,
	duration time.Duration,
) ([]recordSnapshot, error) {
	rdb := r.getRDB()
	if rdb == nil {
		return nil, errors.New("redis client is nil")
	}

	pattern := common.RedisKey(r.prefix+":*") + ":meta"
	iter := rdb.Scan(ctx, 0, pattern, 0).Iterator()
	nowUnix := time.Now().Unix()
	windowSeconds := duration.Seconds()
	snapshots := make([]recordSnapshot, 0)

	for iter.Next(ctx) {
		metaKey := iter.Val()

		result, err := getRequestCountScript.Run(
			ctx,
			rdb,
			[]string{metaKey, ""},
			windowSeconds,
			nowUnix,
		).Text()
		if err != nil {
			return nil, err
		}

		parts := strings.Split(result, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid result format: %s", result)
		}

		totalCount, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return nil, err
		}

		secondCount, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, err
		}

		baseKey := strings.TrimSuffix(
			strings.TrimPrefix(metaKey, common.RedisKey(r.prefix+":")),
			":meta",
		)
		snapshots = append(snapshots, recordSnapshot{
			Keys:        parseKeys(baseKey),
			TotalCount:  totalCount,
			SecondCount: secondCount,
		})
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return snapshots, nil
}
