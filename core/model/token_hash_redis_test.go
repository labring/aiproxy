//nolint:testpackage
package model

import (
	"context"
	"testing"

	"github.com/labring/aiproxy/core/common"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func redisKeysContainRaw(ctx context.Context, t *testing.T, client *redis.Client, raws ...string) {
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
			Key:        HashTokenKey(hashedRaw),
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
		plainSlot, err := client.HGetAll(ctx, getTokenCacheKey(plainRaw)).Result()
		require.NoError(t, err)
		require.NotEmpty(t, plainSlot)

		legacyRawSlot, err := client.HGetAll(
			ctx,
			common.RedisKeyf(TokenCacheKey, plainRaw),
		).Result()
		require.NoError(t, err)
		require.Empty(t, legacyRawSlot)

		redisKeysContainRaw(ctx, t, client, hashedRaw, plainRaw)

		// a stale pre-upgrade entry under the raw slot is ignored
		require.NoError(
			t,
			client.HSet(ctx, common.RedisKeyf(TokenCacheKey, plainRaw), "i", 1).Err(),
		)
		modelLocalCache.Flush()
		require.NoError(t, client.Del(ctx, getTokenCacheKey(plainRaw)).Err())

		got, err = GetAndValidateToken(plainRaw)
		require.NoError(t, err)
		require.Equal(t, plain.ID, got.ID)

		// rotation moves the cache to the new slot and drops the old one
		_, newRaw, err := RotateToken(hashed.ID, TokenRotateModeHash, nil)
		require.NoError(t, err)

		_, err = GetAndValidateToken(newRaw)
		require.NoError(t, err)

		oldSlot, err := client.HGetAll(ctx, getTokenCacheKey(HashTokenKey(hashedRaw))).Result()
		require.NoError(t, err)
		require.Empty(t, oldSlot)

		redisKeysContainRaw(ctx, t, client, hashedRaw, newRaw)
	})
}
