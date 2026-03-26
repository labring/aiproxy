//go:build enterprise

// Package session provides JWT-based session management for the enterprise module.
// It is intentionally placed in a separate sub-package to avoid circular imports
// between the enterprise and feishu packages.
package session

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/labring/aiproxy/core/common/config"
)

const (
	// JWTExpiry is the default expiry for enterprise session JWTs.
	JWTExpiry = 7 * 24 * time.Hour
	// JWTRefreshThreshold — if remaining lifetime < this, issue a new token.
	JWTRefreshThreshold = 24 * time.Hour
)

// Claims holds the JWT payload for enterprise web sessions.
// This is completely independent of the tokens table (API Keys).
type Claims struct {
	Role    string `json:"role"`
	GroupID string `json:"group_id"`
	jwt.RegisteredClaims
}

// GenerateJWT creates a signed JWT for an enterprise (Feishu) user.
// The subject is the Feishu open_id.
func GenerateJWT(feishuOpenID, role, groupID string) (string, error) {
	if config.AdminKey == "" {
		return "", errors.New("ADMIN_KEY not configured, cannot sign JWT")
	}

	now := time.Now()
	claims := Claims{
		Role:    role,
		GroupID: groupID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   feishuOpenID,
			ExpiresAt: jwt.NewNumericDate(now.Add(JWTExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.AdminKey))
}

// ParseJWT validates and parses a session JWT string.
// Returns the claims if valid, or an error if expired / tampered / malformed.
func ParseJWT(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(config.AdminKey), nil
		},
	)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

// ShouldRefresh returns true if the token expires within JWTRefreshThreshold.
func ShouldRefresh(claims *Claims) bool {
	if claims.ExpiresAt == nil {
		return false
	}
	return time.Until(claims.ExpiresAt.Time) < JWTRefreshThreshold
}
