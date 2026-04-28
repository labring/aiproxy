//nolint:testpackage
package model

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAsyncUsageClaimPostgresOnlyOneConcurrentClaimer(t *testing.T) {
	withTestPostgresStoreDB(t, func() {
		require.NoError(t, LogDB.AutoMigrate(&AsyncUsageInfo{}))

		now := time.Now()
		info := &AsyncUsageInfo{
			RequestID:  "pg_claim",
			Status:     AsyncUsageStatusPending,
			NextPollAt: now.Add(-time.Second),
		}
		require.NoError(t, LogDB.Create(info).Error)

		const workers = 32

		var claimedCount atomic.Int64

		errs := make(chan error, workers)

		var wg sync.WaitGroup
		wg.Add(workers)

		for i := range workers {
			go func(i int) {
				defer wg.Done()

				claimed, err := TryClaimAsyncUsageInfo(
					&AsyncUsageInfo{ID: info.ID},
					fmt.Sprintf("token-%d", i),
					now.Add(time.Minute),
					now,
				)
				if err != nil {
					errs <- err
					return
				}

				if claimed {
					claimedCount.Add(1)
				}
			}(i)
		}

		wg.Wait()
		close(errs)

		for err := range errs {
			require.NoError(t, err)
		}

		require.Equal(t, int64(1), claimedCount.Load())

		var got AsyncUsageInfo
		require.NoError(t, LogDB.First(&got, info.ID).Error)
		require.NotEmpty(t, got.ProcessingToken)
		require.True(t, got.NextPollAt.After(now))
	})
}

func TestAsyncUsageClaimPostgresLeaseExpiryAndTokenGuard(t *testing.T) {
	withTestPostgresStoreDB(t, func() {
		require.NoError(t, LogDB.AutoMigrate(&AsyncUsageInfo{}))

		now := time.Now()
		firstLeaseUntil := now.Add(time.Minute)
		info := &AsyncUsageInfo{
			RequestID:  "pg_claim_expiry",
			Status:     AsyncUsageStatusPending,
			NextPollAt: now.Add(-time.Second),
		}
		require.NoError(t, LogDB.Create(info).Error)

		claimed, err := TryClaimAsyncUsageInfo(info, "token-old", firstLeaseUntil, now)
		require.NoError(t, err)
		require.True(t, claimed)

		claimed, err = TryClaimAsyncUsageInfo(
			&AsyncUsageInfo{ID: info.ID},
			"token-new",
			now.Add(2*time.Minute),
			now.Add(time.Second),
		)
		require.NoError(t, err)
		require.False(t, claimed)

		claimed, err = TryClaimAsyncUsageInfo(
			&AsyncUsageInfo{ID: info.ID},
			"token-new",
			now.Add(3*time.Minute),
			firstLeaseUntil.Add(time.Second),
		)
		require.NoError(t, err)
		require.True(t, claimed)

		completed, err := CompleteClaimedAsyncUsageInfo(
			&AsyncUsageInfo{ID: info.ID, ProcessingToken: "token-old"},
			Usage{InputTokens: 1, TotalTokens: 1},
			Amount{UsedAmount: 1},
		)
		require.NoError(t, err)
		require.False(t, completed)

		completed, err = CompleteClaimedAsyncUsageInfo(
			&AsyncUsageInfo{ID: info.ID, ProcessingToken: "token-new"},
			Usage{InputTokens: 2, TotalTokens: 2},
			Amount{UsedAmount: 2},
		)
		require.NoError(t, err)
		require.True(t, completed)

		var got AsyncUsageInfo
		require.NoError(t, LogDB.First(&got, info.ID).Error)
		require.Equal(t, AsyncUsageStatusCompleted, got.Status)
		require.Empty(t, got.ProcessingToken)
		require.Equal(t, ZeroNullInt64(2), got.Usage.TotalTokens)
		require.Equal(t, float64(2), got.Amount.UsedAmount)
	})
}
