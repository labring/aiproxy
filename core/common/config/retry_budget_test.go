package config_test

import (
	"testing"

	"github.com/labring/aiproxy/core/common/config"
	"github.com/stretchr/testify/require"
)

func TestRetryBudgetEnvironmentOverrideAndCap(t *testing.T) {
	t.Setenv("RETRY_BUDGET", "")

	oldBudget := config.GetRetryBudget()
	t.Cleanup(func() { config.SetRetryBudget(oldBudget) })

	for _, tt := range []struct {
		env   string
		value int64
		want  int64
	}{
		{value: 30, want: 30},
		{env: "60", value: 30, want: 60},
		{env: "0", value: 30},
		{env: "181", want: 180},
		{env: "-1"},
		{value: 999, want: 180},
	} {
		t.Run(tt.env, func(t *testing.T) {
			t.Setenv("RETRY_BUDGET", tt.env)
			config.SetRetryBudget(tt.value)
			require.Equal(t, tt.want, config.GetRetryBudget())
		})
	}
}
