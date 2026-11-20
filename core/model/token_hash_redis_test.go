//nolint:testpackage
package model

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func noRawInRedisKeys(ctx context.Context, t *testing.T, client *redis.Client, raws ...string) {
	t.Helper()

	keys, err := client.Keys(ctx, "*").Result()
	require.NoError(t, err)
	require.NotEmpty(t, keys)

	for _, key := range keys {
		for _, raw := range raws {
			require.NotContains(t, key, raw)
		}
	}
}

// TestTokenRedisNeverStoresRawKey guards that both storage modes only ever
// reach redis under the hash slot: no key name may contain a raw key.
func TestTokenRedisNeverStoresRawKey(t *testing.T) {
	withTestTokenCacheRedisEnv(t, func(ctx context.Context, client *redis.Client) {
		hashedRaw := GenerateTokenKey()
		hashed := &Token{
			Name:       "hashed-redis",
			GroupID:    "hash-redis-group",
			KeyHash:    HashTokenKey(hashedRaw),
			KeyPreview: TokenKeyPreview(hashedRaw),
		}
		require.NoError(t, DB.Create(hashed).Error)

		plainRaw := GenerateTokenKey()
		plain := &Token{
			Name:    "plain-redis",
			GroupID: "hash-redis-group",
			Key:     plainRaw,
		}
		require.NoError(t, DB.Create(plain).Error)

		got, err := GetAndValidateToken(hashedRaw)
		require.NoError(t, err)
		require.Equal(t, hashed.ID, got.ID)

		got, err = GetAndValidateToken(plainRaw)
		require.NoError(t, err)
		require.Equal(t, plain.ID, got.ID)

		hashedSlot, err := client.HGetAll(ctx, getTokenCacheKey(HashTokenKey(hashedRaw))).Result()
		require.NoError(t, err)
		require.NotEmpty(t, hashedSlot)

		// plain tokens cache under their hash too
		plainSlot, err := client.HGetAll(ctx, getTokenCacheKey(HashTokenKey(plainRaw))).Result()
		require.NoError(t, err)
		require.NotEmpty(t, plainSlot)

		noRawInRedisKeys(ctx, t, client, hashedRaw, plainRaw)

		// rotation moves the cache to the new slot and drops the old one
		_, newRaw, err := RotateToken(hashed.ID, TokenRotateModeHash, nil)
		require.NoError(t, err)

		_, err = GetAndValidateToken(newRaw)
		require.NoError(t, err)

		oldSlot, err := client.HGetAll(ctx, getTokenCacheKey(HashTokenKey(hashedRaw))).Result()
		require.NoError(t, err)
		require.Empty(t, oldSlot)

		noRawInRedisKeys(ctx, t, client, hashedRaw, newRaw)
	})
}
