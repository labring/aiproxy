//nolint:testpackage
package model

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestHashTokenKeyStableAndMarked(t *testing.T) {
	raw := GenerateTokenKey()

	require.Len(t, raw, 48)
	require.NotContains(t, raw, ":")

	for _, r := range raw {
		require.Contains(t, keyChars, string(r))
	}

	hashed := HashTokenKey(raw)
	require.Len(t, hashed, len(hashedKeyPrefix)+64)
	require.Equal(t, hashed, HashTokenKey(raw))
	require.NotEqual(t, raw, hashed)
	require.True(t, IsHashedTokenKey(hashed))
	require.False(t, IsHashedTokenKey(raw))
	require.False(t, IsHashedTokenKey("h:"))
	require.False(t, IsHashedTokenKey(""))
}

func TestHashTokenKeyDiffersPerKey(t *testing.T) {
	require.NotEqual(t, HashTokenKey(GenerateTokenKey()), HashTokenKey(GenerateTokenKey()))
}

func TestTokenKeyPreview(t *testing.T) {
	require.Equal(t, "abcdef…wxyz", TokenKeyPreview("abcdefghijklmnopqrstuvwxyz"))
	require.Equal(t, "short", TokenKeyPreview("short"))
}

func TestTokenBeforeCreateKeepsPresetHash(t *testing.T) {
	withTestModelCacheDB(t, func() {
		raw := GenerateTokenKey()

		hashed := &Token{
			Name:       "preset-hash",
			GroupID:    "hash-group",
			Key:        HashTokenKey(raw),
			KeyPreview: TokenKeyPreview(raw),
		}
		require.NoError(t, InsertToken(hashed, true, false))
		require.Equal(t, HashTokenKey(raw), hashed.Key)

		plain := &Token{Name: "preset-plain", GroupID: "hash-group"}
		require.NoError(t, InsertToken(plain, false, false))
		require.Len(t, plain.Key, 48)
		require.Empty(t, plain.KeyPreview)
	})
}

func TestTokenHashAuthAndRotate(t *testing.T) {
	withTestModelCacheDB(t, func() {
		raw := GenerateTokenKey()
		token := &Token{
			Name:       "hashed",
			GroupID:    "hash-group",
			Key:        HashTokenKey(raw),
			KeyPreview: TokenKeyPreview(raw),
		}
		require.NoError(t, InsertToken(token, true, false))

		got, err := GetAndValidateToken(raw)
		require.NoError(t, err)
		require.Equal(t, token.ID, got.ID)

		plain := &Token{Name: "plain", GroupID: "hash-group"}
		require.NoError(t, InsertToken(plain, false, false))
		gotPlain, err := GetAndValidateToken(plain.Key)
		require.NoError(t, err)
		require.Equal(t, plain.ID, gotPlain.ID)

		_, err = GetAndValidateToken(GenerateTokenKey())
		require.Error(t, err)

		// keep mode
		rotated, newRaw, err := RotateToken(token.ID, TokenRotateModeKeep, nil)
		require.NoError(t, err)
		require.Len(t, newRaw, 48)
		require.True(t, IsHashedTokenKey(rotated.Key))
		require.Equal(t, TokenKeyPreview(newRaw), rotated.KeyPreview)
		require.Equal(t, token.Name, rotated.Name)

		_, err = GetAndValidateToken(raw)
		require.Error(t, err)
		got, err = GetAndValidateToken(newRaw)
		require.NoError(t, err)
		require.Equal(t, token.ID, got.ID)

		// hash -> plain
		rotated, newRaw, err = RotateToken(token.ID, TokenRotateModePlain, nil)
		require.NoError(t, err)
		require.Equal(t, newRaw, rotated.Key)
		require.Empty(t, rotated.KeyPreview)

		_, err = GetAndValidateToken(newRaw)
		require.NoError(t, err)

		// plain -> hash
		rotated, newRaw, err = RotateToken(token.ID, TokenRotateModeHash, nil)
		require.NoError(t, err)
		require.Equal(t, HashTokenKey(newRaw), rotated.Key)
		require.Equal(t, TokenKeyPreview(newRaw), rotated.KeyPreview)
		_, err = GetAndValidateToken(newRaw)
		require.NoError(t, err)

		_, _, err = RotateToken(token.ID, "invalid", nil)
		require.Error(t, err)

		_, _, err = RotateGroupToken("missing-group", token.ID, "", nil)
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
		_, _, err = RotateToken(99999, "", nil)
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})
}

func TestSearchTokensHashAware(t *testing.T) {
	withTestModelCacheDB(t, func() {
		raw := GenerateTokenKey()
		hashed := &Token{
			Name:       "search-hashed",
			GroupID:    "hash-group",
			Key:        HashTokenKey(raw),
			KeyPreview: TokenKeyPreview(raw),
		}
		require.NoError(t, InsertToken(hashed, true, false))

		plain := &Token{Name: "search-plain", GroupID: "hash-group"}
		require.NoError(t, InsertToken(plain, false, false))

		tokens, total, err := SearchTokens("hash-group", "", 1, 20, "", 0, "", raw)
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
		require.Len(t, tokens, 1)
		require.Equal(t, hashed.ID, tokens[0].ID)

		tokens, total, err = SearchTokens("hash-group", "", 1, 20, "", 0, "", plain.Key)
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
		require.Equal(t, plain.ID, tokens[0].ID)

		// the visible fragment of a hashed key is its preview
		previewPrefix := TokenKeyPreview(raw)[:6]
		tokens, total, err = SearchTokens("hash-group", previewPrefix, 1, 20, "", 0, "", "")
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
		require.Equal(t, hashed.ID, tokens[0].ID)

		tokens, total, err = SearchGroupTokens("hash-group", previewPrefix, 1, 20, "", 0, "", "")
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
		require.Equal(t, hashed.ID, tokens[0].ID)

		tokens, total, err = SearchGroupTokens("hash-group", "", 1, 20, "", 0, "", raw)
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
		require.Equal(t, hashed.ID, tokens[0].ID)
	})
}
