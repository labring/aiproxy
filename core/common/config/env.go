package config

import (
	"os"
	"strings"
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
	DynamicRemoteAdminKey string
	WebPath               string
	DisableWeb            bool
	DisableWebRoot        bool
	FfmpegEnabled         bool
	InternalToken         string
	DisableModelConfig    bool
	Redis                 string
	RedisKeyPrefix        string
	ConfigFilePath        string

	// OnCall Lark configuration for urgent alerts
	OnCallLarkAppID     string
	OnCallLarkAppSecret string
	OnCallLarkOpenIDs   []string // comma-separated open IDs

	adminKeyState              atomic.Value
	dynamicRemoteAdminKeyState atomic.Value
	internalTokenState         atomic.Value
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
	adminKeyState.Store(v)
}

func SetDynamicRemoteAdminKey(key string) {
	v := buildTokenVariants(key)
	DynamicRemoteAdminKey = v.raw
	dynamicRemoteAdminKeyState.Store(v)
}

func SetInternalToken(token string) {
	v := buildTokenVariants(token)
	InternalToken = v.raw
	internalTokenState.Store(v)
}

func GetAdminKey() string {
	v, _ := adminKeyState.Load().(tokenVariants)
	return v.raw
}

func GetDynamicRemoteAdminKey() string {
	v, _ := dynamicRemoteAdminKeyState.Load().(tokenVariants)
	return v.raw
}

func GetEffectiveAdminKey() string {
	if key := GetDynamicRemoteAdminKey(); key != "" {
		return key
	}

	return GetAdminKey()
}

func GetInternalToken() string {
	v, _ := internalTokenState.Load().(tokenVariants)
	return v.raw
}

func MatchAdminKey(raw string) bool {
	return matchTokenVariants(raw, adminKeyState)
}

func MatchEffectiveAdminKey(raw string) bool {
	if GetDynamicRemoteAdminKey() != "" {
		return matchTokenVariants(raw, dynamicRemoteAdminKeyState)
	}

	return matchTokenVariants(raw, adminKeyState)
}

func matchTokenVariants(raw string, state atomic.Value) bool {
	v, _ := state.Load().(tokenVariants)
	return raw != "" && (raw == v.raw ||
		raw == v.bearer ||
		raw == v.sk ||
		raw == v.bearerSK)
}

func MatchInternalToken(raw string) bool {
	return matchTokenVariants(raw, internalTokenState)
}

func ReloadEnv() {
	DebugEnabled = env.Bool("DEBUG", false)
	DebugSQLEnabled = env.Bool("DEBUG_SQL", false)
	DisableAutoMigrateDB = env.Bool("DISABLE_AUTO_MIGRATE_DB", false)
	SetAdminKey(os.Getenv("ADMIN_KEY"))
	SetDynamicRemoteAdminKey("")
	WebPath = os.Getenv("WEB_PATH")
	DisableWeb = env.Bool("DISABLE_WEB", false)
	DisableWebRoot = env.Bool("DISABLE_WEB_ROOT", false)
	FfmpegEnabled = env.Bool("FFMPEG_ENABLED", false)
	SetInternalToken(os.Getenv("INTERNAL_TOKEN"))
	DisableModelConfig = env.Bool("DISABLE_MODEL_CONFIG", false)
	Redis = env.String("REDIS", os.Getenv("REDIS_CONN_STRING"))
	RedisKeyPrefix = os.Getenv("REDIS_KEY_PREFIX")
	ConfigFilePath = env.String("CONFIG_FILE_PATH", "./config.yaml")

	// OnCall Lark configuration
	OnCallLarkAppID = os.Getenv("ON_CALL_LARK_APP_ID")
	OnCallLarkAppSecret = os.Getenv("ON_CALL_LARK_APP_SECRET")
	OnCallLarkOpenIDs = parseOpenIDs(os.Getenv("ON_CALL_LARK_OPEN_ID"))
}

// parseOpenIDs parses comma-separated open IDs
func parseOpenIDs(s string) []string {
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")

	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}

	return result
}

func init() {
	ReloadEnv()
}
