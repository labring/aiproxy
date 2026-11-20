package model

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"

	"github.com/labring/aiproxy/core/common/conv"
)

// hashedKeyPrefix marks a Key column that stores a hash instead of the raw
// key. Raw keys are 48 chars from keyChars, which never contains ':'.
const hashedKeyPrefix = "h:"

// tokenHashKey is a fixed HMAC key. The hash only has to make precomputed
// (rainbow table) lookups impossible; raw keys are 48 random chars, so an
// offline brute force of the hash is infeasible either way.
var tokenHashKey = []byte("aiproxy-token-key-hmac-v1")

func HashTokenKey(key string) string {
	mac := hmac.New(sha256.New, tokenHashKey)
	_, _ = mac.Write(conv.StringToBytes(key))

	return hashedKeyPrefix + hex.EncodeToString(mac.Sum(nil))
}

func IsHashedTokenKey(storedKey string) bool {
	return len(storedKey) > len(hashedKeyPrefix) &&
		storedKey[:len(hashedKeyPrefix)] == hashedKeyPrefix
}

// TokenKeyPreview keeps just enough of a key to recognize it.
func TokenKeyPreview(key string) string {
	if len(key) <= 10 {
		return key
	}

	return key[:6] + "…" + key[len(key)-4:]
}
