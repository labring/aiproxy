package modelcatalog

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func BCEAuthorizationHeaders(method, targetURL, accessKey, secretKey string) (http.Header, error) {
	parsed, err := url.Parse(targetURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") || accessKey == "" || secretKey == "" {
		return nil, ErrInvalidConfiguration
	}

	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	canonicalHeaders := fmt.Sprintf(
		"host:%s\nx-bce-date:%s",
		bceURIEncode(strings.ToLower(parsed.Host)),
		bceURIEncode(timestamp),
	)
	canonicalQuery := strings.ReplaceAll(parsed.Query().Encode(), "+", "%20")
	canonicalRequest := strings.ToUpper(method) + "\n" + parsed.EscapedPath() + "\n" +
		canonicalQuery + "\n" + canonicalHeaders
	authPrefix := fmt.Sprintf("bce-auth-v1/%s/%s/180000", accessKey, timestamp)
	signingKey := hex.EncodeToString(hmacSHA256([]byte(secretKey), []byte(authPrefix)))
	signature := hex.EncodeToString(hmacSHA256([]byte(signingKey), []byte(canonicalRequest)))

	headers := make(http.Header)
	headers.Set("x-bce-date", timestamp)
	headers.Set("Authorization", authPrefix+"/host;x-bce-date/"+signature)

	return headers, nil
}

func bceURIEncode(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

func hmacSHA256(key, value []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(value)
	return mac.Sum(nil)
}
