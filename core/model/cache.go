package model

import (
	"encoding"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/redis/go-redis/v9"
)

const (
	SyncFrequency    = time.Minute * 3
	GroupModelTPMKey = "group:%s:model_tpm"
	redisTimeout     = 2 * time.Second
)

var (
	_ encoding.BinaryMarshaler = (*redisStringSlice)(nil)
	_ redis.Scanner            = (*redisStringSlice)(nil)
)

type redisStringSlice []string

func (r *redisStringSlice) ScanRedis(value string) error {
	return sonic.Unmarshal(conv.StringToBytes(value), r)
}

func (r redisStringSlice) MarshalBinary() ([]byte, error) {
	return sonic.Marshal(r)
}

type redisTime time.Time

var (
	_ redis.Scanner            = (*redisTime)(nil)
	_ encoding.BinaryMarshaler = (*redisTime)(nil)
)

func (t *redisTime) ScanRedis(value string) error {
	return (*time.Time)(t).UnmarshalBinary(conv.StringToBytes(value))
}

func (t redisTime) MarshalBinary() ([]byte, error) {
	return time.Time(t).MarshalBinary()
}

type redisMap[K comparable, V any] map[K]V

var (
	_ redis.Scanner            = (*redisMap[string, any])(nil)
	_ encoding.BinaryMarshaler = (*redisMap[string, any])(nil)
)

func (r *redisMap[K, V]) ScanRedis(value string) error {
	return sonic.UnmarshalString(value, r)
}

func (r redisMap[K, V]) MarshalBinary() ([]byte, error) {
	return sonic.Marshal(r)
}

type (
	redisGroupModelConfigMap = redisMap[string, GroupModelConfig]
)
