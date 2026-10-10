package config_test

import (
	"testing"
	"time"

	"github.com/labring/aiproxy/core/common/config"
	"github.com/stretchr/testify/require"
)

func TestReloadEnvLimits(t *testing.T) {
	keys := []string{
		"MAX_REQUEST_BODY_SIZE", "MAX_RESPONSE_BODY_SIZE", "MULTIPART_FORM_MEMORY_LIMIT",
		"MAX_IMAGE_SIZE", "DOC2X_STATUS_POLL_INTERVAL", "DOC2X_STATUS_TIMEOUT",
	}
	for _, tt := range []struct {
		name     string
		values   []string
		sizes    []int64
		interval time.Duration
		timeout  time.Duration
	}{
		{name: "defaults", values: []string{"", "", "", "", "", ""}, sizes: []int64{50 * 1024 * 1024, 200 * 1024 * 1024, 4 * 1024 * 1024, 10 * 1024 * 1024}, interval: time.Second, timeout: 10 * time.Minute},
		{name: "overrides", values: []string{"1024", "2048", "128", "512", "500ms", "2m"}, sizes: []int64{1024, 2048, 128, 512}, interval: 500 * time.Millisecond, timeout: 2 * time.Minute},
		{name: "invalid values", values: []string{"0", "-1", "invalid", "9223372036854775808", "0s", "-1s"}, sizes: []int64{50 * 1024 * 1024, 200 * 1024 * 1024, 4 * 1024 * 1024, 10 * 1024 * 1024}, interval: time.Second, timeout: 10 * time.Minute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(config.ReloadEnv)

			for i, key := range keys {
				t.Setenv(key, tt.values[i])
			}

			config.ReloadEnv()
			require.Equal(
				t,
				tt.sizes,
				[]int64{
					config.MaxRequestBodySize,
					config.MaxResponseBodySize,
					config.MultipartFormMemoryLimit,
					config.MaxImageSize,
				},
			)
			require.Equal(t, tt.interval, config.Doc2XStatusPollInterval)
			require.Equal(t, tt.timeout, config.Doc2XStatusTimeout)
		})
	}
}
