//nolint:testpackage
package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResetDailyTokenPeriodAdvancesOnceAtUTCDayBoundary(t *testing.T) {
	withTestModelCacheDB(t, func() {
		todayStart := time.Now().UTC().Truncate(24 * time.Hour)
		token := &Token{
			Name:                   "daily-quota",
			PeriodQuota:            20,
			PeriodType:             PeriodTypeDaily,
			PeriodLastUpdateTime:   todayStart.Add(-time.Nanosecond),
			PeriodLastUpdateAmount: 10,
			UsedAmount:             100,
			Status:                 TokenStatusEnabled,
		}
		require.NoError(t, DB.Create(token).Error)

		require.NoError(t, ResetTokenPeriodUsage(token.ID))

		afterFirstReset, err := GetTokenByID(token.ID)
		require.NoError(t, err)
		require.Equal(t, todayStart, afterFirstReset.PeriodLastUpdateTime.UTC())
		require.Equal(t, 100.0, afterFirstReset.PeriodLastUpdateAmount)

		require.NoError(
			t,
			DB.Model(&Token{}).Where("id = ?", token.ID).Update("used_amount", 105).Error,
		)
		require.NoError(t, ResetTokenPeriodUsage(token.ID))

		afterSecondReset, err := GetTokenByID(token.ID)
		require.NoError(t, err)
		require.Equal(t, todayStart, afterSecondReset.PeriodLastUpdateTime.UTC())
		require.Equal(t, 100.0, afterSecondReset.PeriodLastUpdateAmount)
	})
}
