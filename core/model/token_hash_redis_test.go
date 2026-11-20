//nolint:testpackage
package model

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// TestHashedTokenRedisNeverStoresRawKey guards that hashed tokens only ever
// reach redis under their hash slot: neither the key name nor any hash field
// may contain the raw key.
func TestHashedTokenRedisNeverStoresRawKey(t *testing.T) {
	withTestTokenCacheRedisEnv(t, func(ctx context.Context, client *redis.Client) {
		raw := GenerateTokenKey()
		token := &Token{
			Name:       "hashed-redis",
			GroupID:    "hash-redis-group",
			Key:        HashTokenKey(raw),
			KeyPreview: TokenKeyPreview(raw),
		}
		require.NoError(t, DB.Create(token).Error)

		got, err := GetAndValidateToken(raw)
		require.NoError(t, err)
		require.Equal(t, token.ID, got.ID)

		rawSlot, err := client.HGetAll(ctx, getTokenCacheKey(raw)).Result()
		require.NoError(t, err)
		require.Empty(t, rawSlot)

		hashedSlot, err := client.HGetAll(ctx, getTokenCacheKey(HashTokenKey(raw))).Result()
		require.NoError(t, err)
		require.NotEmpty(t, hashedSlot)

		keys, err := client.Keys(ctx, "*").Result()
		require.NoError(t, err)
		require.NotEmpty(t, keys)

		for _, key := range keys {
			require.NotContains(t, key, raw)
		}

		// rotation moves the cache to the new slot and drops the old one
		_, newRaw, err := RotateToken(token.ID, TokenRotateModeHash)
		require.NoError(t, err)

		_, err = GetAndValidateToken(newRaw)
		require.NoError(t, err)

		oldSlot, err := client.HGetAll(ctx, getTokenCacheKey(HashTokenKey(raw))).Result()
		require.NoError(t, err)
		require.Empty(t, oldSlot)

		keys, err = client.Keys(ctx, "*").Result()
		require.NoError(t, err)

		for _, key := range keys {
			require.NotContains(t, key, raw)
			require.NotContains(t, key, newRaw)
		}
	})
}
