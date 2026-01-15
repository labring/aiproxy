package config

import (
	"os"
	"strings"

	"github.com/labring/aiproxy/core/common/env"
)

var (
	DebugEnabled         bool
	DebugSQLEnabled      bool
	DisableAutoMigrateDB bool
	AdminKey             string
	WebPath              string
	DisableWeb           bool
	FfmpegEnabled        bool
	InternalToken        string
	DisableModelConfig   bool
	Redis                string
	RedisKeyPrefix       string
	ConfigFilePath       string

	// OnCall Lark configuration for urgent alerts
	OnCallLarkAppID     string
	OnCallLarkAppSecret string
	OnCallLarkOpenIDs   []string // comma-separated open IDs
)

func ReloadEnv() {
	DebugEnabled = env.Bool("DEBUG", false)
	DebugSQLEnabled = env.Bool("DEBUG_SQL", false)
	DisableAutoMigrateDB = env.Bool("DISABLE_AUTO_MIGRATE_DB", false)
	AdminKey = os.Getenv("ADMIN_KEY")
	WebPath = os.Getenv("WEB_PATH")
	DisableWeb = env.Bool("DISABLE_WEB", false)
	FfmpegEnabled = env.Bool("FFMPEG_ENABLED", false)
	InternalToken = os.Getenv("INTERNAL_TOKEN")
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
