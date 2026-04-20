package config

import (
	"os"
	"sync/atomic"

	"github.com/labring/aiproxy/core/common/env"
)

type tokenVariants struct {
	raw      string
	bearer   string
	sk       string
	bearerSK string
}

var (
	DebugEnabled          bool
	DebugSQLEnabled       bool
	DisableAutoMigrateDB  bool
	AdminKey              string
	adminKeyBearer        string
	adminKeySK            string
	adminKeyBearerSK      string
	WebPath               string
	DisableWeb            bool
	FfmpegEnabled         bool
	InternalToken         string
	internalTokenBearer   string
	internalTokenSK       string
	internalTokenBearerSK string
	DisableModelConfig    bool
	Redis                 string
	RedisKeyPrefix        string

	adminKeyState      atomic.Value
	internalTokenState atomic.Value
)

func buildTokenVariants(token string) tokenVariants {
	if token == "" {
		return tokenVariants{}
	}

	sk := "sk-" + token
	return tokenVariants{
		raw:      token,
		bearer:   "Bearer " + token,
		sk:       sk,
		bearerSK: "Bearer " + sk,
	}
}

func SetAdminKey(key string) {
	v := buildTokenVariants(key)
	AdminKey = v.raw
	adminKeyBearer = v.bearer
	adminKeySK = v.sk
	adminKeyBearerSK = v.bearerSK
	adminKeyState.Store(v)
}

func SetInternalToken(token string) {
	v := buildTokenVariants(token)
	InternalToken = v.raw
	internalTokenBearer = v.bearer
	internalTokenSK = v.sk
	internalTokenBearerSK = v.bearerSK
	internalTokenState.Store(v)
}

func GetAdminKey() string {
	v, _ := adminKeyState.Load().(tokenVariants)
	return v.raw
}

func GetInternalToken() string {
	v, _ := internalTokenState.Load().(tokenVariants)
	return v.raw
}

func MatchAdminKey(raw string) bool {
	v, _ := adminKeyState.Load().(tokenVariants)
	return raw != "" && (raw == v.raw ||
		raw == v.bearer ||
		raw == v.sk ||
		raw == v.bearerSK)
}

func MatchInternalToken(raw string) bool {
	v, _ := internalTokenState.Load().(tokenVariants)
	return raw != "" && (raw == v.raw ||
		raw == v.bearer ||
		raw == v.sk ||
		raw == v.bearerSK)
}

func ReloadEnv() {
	DebugEnabled = env.Bool("DEBUG", false)
	DebugSQLEnabled = env.Bool("DEBUG_SQL", false)
	DisableAutoMigrateDB = env.Bool("DISABLE_AUTO_MIGRATE_DB", false)
	SetAdminKey(os.Getenv("ADMIN_KEY"))
	WebPath = os.Getenv("WEB_PATH")
	DisableWeb = env.Bool("DISABLE_WEB", false)
	FfmpegEnabled = env.Bool("FFMPEG_ENABLED", false)
	SetInternalToken(os.Getenv("INTERNAL_TOKEN"))
	DisableModelConfig = env.Bool("DISABLE_MODEL_CONFIG", false)
	Redis = env.String("REDIS", os.Getenv("REDIS_CONN_STRING"))
	RedisKeyPrefix = os.Getenv("REDIS_KEY_PREFIX")
}

func init() {
	ReloadEnv()
}
