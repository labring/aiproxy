package config_test

import (
	"testing"

	"github.com/labring/aiproxy/core/common/config"
	"github.com/stretchr/testify/require"
)

func TestReloadEnvLimits(t *testing.T) {
	keys := []string{
		"MAX_REQUEST_BODY_SIZE", "MAX_RESPONSE_BODY_SIZE", "MULTIPART_FORM_MEMORY_LIMIT",
		"MAX_IMAGE_SIZE",
	}
	for _, tt := range []struct {
		name   string
		values []string
		sizes  []int64
	}{
		{name: "defaults", values: []string{"", "", "", ""}, sizes: []int64{50 * 1024 * 1024, 200 * 1024 * 1024, 4 * 1024 * 1024, 10 * 1024 * 1024}},
		{name: "overrides", values: []string{"1024", "2048", "128", "512"}, sizes: []int64{1024, 2048, 128, 512}},
		{name: "invalid values", values: []string{"0", "-1", "invalid", "9223372036854775808"}, sizes: []int64{50 * 1024 * 1024, 200 * 1024 * 1024, 4 * 1024 * 1024, 10 * 1024 * 1024}},
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
		})
	}
}
