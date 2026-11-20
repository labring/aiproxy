package model

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"

	"github.com/labring/aiproxy/core/common/conv"
)

// tokenHashKey is a fixed HMAC key. The hash only has to make precomputed
// (rainbow table) lookups impossible; raw keys are 48 random chars, so an
// offline brute force of the hash is infeasible either way.
var tokenHashKey = []byte("aiproxy-token-key-hmac-v1")

// HashTokenKey returns the hex HMAC-SHA256 of a raw key. Every token stores
// it in KeyHash regardless of storage mode; the auth path looks tokens up by
// this value only.
func HashTokenKey(key string) string {
	mac := hmac.New(sha256.New, tokenHashKey)
	_, _ = mac.Write(conv.StringToBytes(key))

	return hex.EncodeToString(mac.Sum(nil))
}

// TokenKeyPreview keeps just enough of a key to recognize it.
func TokenKeyPreview(key string) string {
	if len(key) <= 10 {
		return key
	}

	return key[:6] + "…" + key[len(key)-4:]
}
