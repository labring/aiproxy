package config

import (
	"os"

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
}

func init() {
	ReloadEnv()
}
