//nolint:testpackage
package monitor

import (
	"testing"

	"github.com/labring/aiproxy/core/common/config"
	relaymeta "github.com/labring/aiproxy/core/relay/meta"
	"github.com/stretchr/testify/require"
)

func TestGetChannelWarnErrorRateUsesChannelValueEvenWhenAutoBalanceDisabled(t *testing.T) {
	meta := &relaymeta.Meta{}
	meta.Channel.WarnErrorRate = 0.42
	meta.Channel.EnabledAutoBalanceCheck = false
	meta.Channel.MaxErrorRate = 0.95

	require.InDelta(t, 0.42, getChannelWarnErrorRate(meta), 0.0001)
	require.InDelta(t, 0.95, getChannelMaxErrorRate(meta), 0.0001)
}

func TestGetChannelWarnErrorRateFallsBackToDefault(t *testing.T) {
	previous := config.GetDefaultWarnNotifyErrorRate()
	config.SetDefaultWarnNotifyErrorRate(previous)
	t.Cleanup(func() {
		config.SetDefaultWarnNotifyErrorRate(previous)
	})

	config.SetDefaultWarnNotifyErrorRate(0.37)

	require.InDelta(t, 0.37, getChannelWarnErrorRate(&relaymeta.Meta{}), 0.0001)
}

func TestGetChannelMaxErrorRateDoesNotDependOnBalanceCheckSwitch(t *testing.T) {
	meta := &relaymeta.Meta{}
	meta.Channel.WarnErrorRate = 0.25
	meta.Channel.MaxErrorRate = 0.88

	require.InDelta(t, 0.88, getChannelMaxErrorRate(meta), 0.0001)

	meta.Channel.EnabledAutoBalanceCheck = true

	require.InDelta(t, 0.88, getChannelMaxErrorRate(meta), 0.0001)
}

func TestShouldTryBanNoPermissionRequiresChannelSwitch(t *testing.T) {
	meta := &relaymeta.Meta{}

	require.False(t, shouldTryBanNoPermission(meta, false))

	meta.Channel.EnabledNoPermissionBan = true

	require.True(t, shouldTryBanNoPermission(meta, false))
	require.False(t, shouldTryBanNoPermission(meta, true))
}
