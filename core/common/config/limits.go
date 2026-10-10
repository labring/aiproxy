package config

import (
	"time"

	"github.com/labring/aiproxy/core/common/env"
)

// Limits are loaded at startup by ReloadEnv. Size limits are measured in bytes.
var (
	MaxRequestBodySize       int64
	MaxResponseBodySize      int64
	MultipartFormMemoryLimit int64
	MaxImageSize             int64
	Doc2XStatusPollInterval  time.Duration
	Doc2XStatusTimeout       time.Duration
)

func reloadLimits() {
	MaxRequestBodySize = env.PositiveInt64("MAX_REQUEST_BODY_SIZE", 50*1024*1024)
	MaxResponseBodySize = env.PositiveInt64("MAX_RESPONSE_BODY_SIZE", 200*1024*1024)
	MultipartFormMemoryLimit = env.PositiveInt64("MULTIPART_FORM_MEMORY_LIMIT", 4*1024*1024)
	MaxImageSize = env.PositiveInt64("MAX_IMAGE_SIZE", 10*1024*1024)
	Doc2XStatusPollInterval = env.PositiveDuration("DOC2X_STATUS_POLL_INTERVAL", time.Second)
	Doc2XStatusTimeout = env.PositiveDuration("DOC2X_STATUS_TIMEOUT", 10*time.Minute)
}
