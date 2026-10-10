package env_test

import (
	"testing"
	"time"

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

func TestPositiveDuration(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  time.Duration
	}{
		{"", time.Minute},
		{"500ms", 500 * time.Millisecond},
		{"10m", 10 * time.Minute},
		{"0", time.Minute},
		{"-1s", time.Minute},
		{"invalid", time.Minute},
		{"999999999999999999h", time.Minute},
	} {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv("TEST_DURATION_LIMIT", tt.value)
			require.Equal(t, tt.want, env.PositiveDuration("TEST_DURATION_LIMIT", time.Minute))
		})
	}
}
