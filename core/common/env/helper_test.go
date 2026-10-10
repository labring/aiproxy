package env_test

import (
	"testing"

	"github.com/labring/aiproxy/core/common/env"
	"github.com/stretchr/testify/require"
)

func TestPositiveLimits(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  int64
	}{
		{"", 1024},
		{"4096", 4096},
		{"0", 1024},
		{"-1", 1024},
		{"invalid", 1024},
		{"1.5", 1024},
		{"9223372036854775808", 1024},
		{"9223372036854775807", 1024},
	} {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv("TEST_SIZE_LIMIT", tt.value)
			require.Equal(t, tt.want, env.PositiveInt64("TEST_SIZE_LIMIT", 1024))
		})
	}
}
