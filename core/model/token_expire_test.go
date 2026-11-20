//nolint:testpackage
package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTokenExpiredAtAuth(t *testing.T) {
	withTestModelCacheDB(t, func() {
		expired := &Token{
			Name:       "expired",
			GroupID:    "expire-group",
			ExpiredAt:  time.Now().Add(-time.Minute),
		}
		require.NoError(t, InsertToken(expired, true, false))

		_, err := GetAndValidateToken(expired.Key)
		require.ErrorContains(t, err, "is expired")

		valid := &Token{
			Name:      "valid",
			GroupID:   "expire-group",
			ExpiredAt: time.Now().Add(time.Hour),
		}
		require.NoError(t, InsertToken(valid, false, false))
		_, err = GetAndValidateToken(valid.Key)
		require.NoError(t, err)

		never := &Token{Name: "never", GroupID: "expire-group"}
		require.NoError(t, InsertToken(never, false, false))
		_, err = GetAndValidateToken(never.Key)
		require.NoError(t, err)
	})
}

func TestTokenExpiredAtUpdateAndRotate(t *testing.T) {
	withTestModelCacheDB(t, func() {
		token := &Token{Name: "expire-update", GroupID: "expire-group"}
		require.NoError(t, InsertToken(token, true, false))

		// set via update
		update := UpdateTokenRequest{ExpiredAt: ptr(time.Now().Add(-time.Minute).UnixMilli())}
		updated, err := UpdateToken(token.ID, update)
		require.NoError(t, err)
		require.WithinDuration(t, time.Now().Add(-time.Minute), updated.ExpiredAt, time.Second)

		_, err = GetAndValidateToken(token.Key)
		require.ErrorContains(t, err, "is expired")

		// clear via update
		updated, err = UpdateToken(token.ID, UpdateTokenRequest{ExpiredAt: ptr(int64(0))})
		require.NoError(t, err)
		require.True(t, updated.ExpiredAt.IsZero())

		_, err = GetAndValidateToken(token.Key)
		require.NoError(t, err)

		// rotate sets a new expiry together with the new key
		newExpiry := time.Now().Add(-time.Minute)
		rotated, rawKey, err := RotateToken(token.ID, TokenRotateModeKeep, &newExpiry)
		require.NoError(t, err)
		require.WithinDuration(t, newExpiry, rotated.ExpiredAt, time.Second)

		_, err = GetAndValidateToken(rawKey)
		require.ErrorContains(t, err, "is expired")

		// rotate with nil keeps the expiry, zero clears it
		rotated, rawKey, err = RotateToken(token.ID, TokenRotateModeKeep, nil)
		require.NoError(t, err)
		require.WithinDuration(t, newExpiry, rotated.ExpiredAt, time.Second)
		require.ErrorContains(t, authError(t, rawKey), "is expired")

		zero := time.Time{}
		rotated, rawKey, err = RotateToken(token.ID, TokenRotateModeKeep, &zero)
		require.NoError(t, err)
		require.True(t, rotated.ExpiredAt.IsZero())

		_, err = GetAndValidateToken(rawKey)
		require.NoError(t, err)
	})
}

func authError(t *testing.T, key string) error {
	t.Helper()

	_, err := GetAndValidateToken(key)

	return err
}

func ptr[T any](v T) *T {
	return &v
}
