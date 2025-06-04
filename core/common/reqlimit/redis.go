package reqlimit

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/redis/go-redis/v9"
)

type redisRateRecord struct {
	prefix string
}

func newRedisGroupModelRecord() *redisRateRecord {
	return &redisRateRecord{
		prefix: "group-model-record",
	}
}

func newRedisGroupModelTokennameRecord() *redisRateRecord {
	return &redisRateRecord{
		prefix: "group-model-tokenname-record",
	}
}

func newRedisChannelModelRecord() *redisRateRecord {
	return &redisRateRecord{
		prefix: "channel-model-record",
	}
}

func newRedisGroupModelTokensRecord() *redisRateRecord {
	return &redisRateRecord{
		prefix: "group-model-tokens-record",
	}
}

func newRedisGroupModelTokennameTokensRecord() *redisRateRecord {
	return &redisRateRecord{
		prefix: "group-model-tokenname-tokens-record",
	}
}

func newRedisChannelModelTokensRecord() *redisRateRecord {
	return &redisRateRecord{
		prefix: "channel-model-tokens-record",
	}
}

const pushRequestLuaScript = `
local key = KEYS[1]
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

local count = 0
local over_count = 0

local all_fields = redis.call('HGETALL', key)
for i = 1, #all_fields, 2 do
    local field_slice = tonumber(all_fields[i])
    if field_slice < cutoff_slice then
        redis.call('HDEL', key, all_fields[i])
	else
		local c, oc = parse_count(all_fields[i+1])
		count = count + c
		over_count = over_count + oc
	end
end

local current_value = redis.call('HGET', key, tostring(current_time))
local current_c, current_oc = parse_count(current_value)

if max_requests == 0 or count <= max_requests then
	current_c = current_c + n
    count = count + n
else
	current_oc = current_oc + n
	over_count = over_count + n
end
redis.call('HSET', key, current_time, current_c .. ":" .. current_oc)

redis.call('EXPIRE', key, window_seconds)
local current_second_count = current_c + current_oc
return string.format("%d:%d:%d", count, over_count, current_second_count)
`

const getRequestCountLuaScript = `
local pattern = KEYS[1]
local window_seconds = tonumber(ARGV[1])
local current_time = tonumber(ARGV[2])
local cutoff_slice = current_time - window_seconds

local function parse_count(value)
    if not value then return 0, 0 end
    local r, e = value:match("^(%d+):(%d+)$")
    return tonumber(r) or 0, tonumber(e) or 0
end

local total = 0
local current_second_count = 0

local keys = redis.call('KEYS', pattern)
for _, key in ipairs(keys) do
    local count = 0
    local over = 0

    local all_fields = redis.call('HGETALL', key)
    for i=1, #all_fields, 2 do
        local field_slice = tonumber(all_fields[i])
        if field_slice < cutoff_slice then
			redis.call('HDEL', key, all_fields[i])
		else
			local c, oc = parse_count(all_fields[i+1])
			count = count + c
			over = over + oc
            
            if field_slice == current_time then
                current_second_count = current_second_count + c + oc
            end
		end
    end

    total = total + count + over
end

return string.format("%d:%d", total, current_second_count)
`

var (
	pushRequestScript     = redis.NewScript(pushRequestLuaScript)
	getRequestCountScript = redis.NewScript(getRequestCountLuaScript)
)

func (r *redisRateRecord) buildKey(keys ...string) string {
	return r.prefix + ":" + strings.Join(keys, ":")
}

func (r *redisRateRecord) GetRequest(
	ctx context.Context,
	duration time.Duration,
	keys ...string,
) (totalCount, secondCount int64, err error) {
	if !common.RedisEnabled {
		return 0, 0, nil
	}

	pattern := r.buildKey(keys...)

	result, err := getRequestCountScript.Run(
		ctx,
		common.RDB,
		[]string{pattern},
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
	key := r.buildKey(keys...)

	result, err := pushRequestScript.Run(
		ctx,
		common.RDB,
		[]string{key},
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
