package config

import (
	"github.com/labring/aiproxy/core/common/env"
)

// Limits are loaded at startup by ReloadEnv. Size limits are measured in bytes.
var (
	MaxRequestBodySize       int64
	MaxResponseBodySize      int64
	MultipartFormMemoryLimit int64
	MaxImageSize             int64
)

func reloadLimits() {
	MaxRequestBodySize = env.PositiveInt64("MAX_REQUEST_BODY_SIZE", 50*1024*1024)
	MaxResponseBodySize = env.PositiveInt64("MAX_RESPONSE_BODY_SIZE", 200*1024*1024)
	MultipartFormMemoryLimit = env.PositiveInt64("MULTIPART_FORM_MEMORY_LIMIT", 4*1024*1024)
	MaxImageSize = env.PositiveInt64("MAX_IMAGE_SIZE", 10*1024*1024)
}
