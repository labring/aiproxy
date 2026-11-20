package modelcatalog_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/relay/adaptor/modelcatalog"
	"github.com/stretchr/testify/require"
)

func TestBCEAuthorizationHeaders(t *testing.T) {
	target := "https://qianfan.baidubce.com:8443/v2/model?Action=DescribeSystemModelSets&note=a%20b"
	headers, err := modelcatalog.BCEAuthorizationHeaders(
		http.MethodPost,
		target,
		"access-key",
		"secret-key",
	)
	require.NoError(t, err)
	require.Regexp(
		t,
		`^bce-auth-v1/access-key/[0-9TZ:.-]+/180000/host;x-bce-date/[0-9a-f]{64}$`,
		headers.Get("Authorization"),
	)
	require.Equal(t, headers.Get("X-Bce-Date"), strings.Split(headers.Get("Authorization"), "/")[2])
	_, err = time.Parse("2006-01-02T15:04:05Z", headers.Get("X-Bce-Date"))
	require.NoError(t, err)

	parsed, err := url.Parse(target)
	require.NoError(t, err)
	require.Equal(t, "/v2/model", parsed.Path)
	require.Equal(
		t,
		"qianfan.baidubce.com%3A8443",
		strings.ReplaceAll(url.QueryEscape(strings.ToLower(parsed.Host)), "+", "%20"),
	)
	require.Equal(
		t,
		"2017-02-15T08%3A52%3A48Z",
		strings.ReplaceAll(url.QueryEscape("2017-02-15T08:52:48Z"), "+", "%20"),
	)
	require.Equal(t, "a%20b", strings.ReplaceAll(url.QueryEscape("a b"), "+", "%20"))

	authPrefix := "bce-auth-v1/access-key/" + headers.Get("X-Bce-Date") + "/180000"
	canonicalQuery := strings.ReplaceAll(parsed.Query().Encode(), "+", "%20")
	require.Equal(t, "Action=DescribeSystemModelSets&note=a%20b", canonicalQuery)
	canonicalRequest := strings.Join([]string{
		http.MethodPost,
		parsed.EscapedPath(),
		canonicalQuery,
		"host:" + url.QueryEscape(strings.ToLower(parsed.Host)),
		"x-bce-date:" + url.QueryEscape(headers.Get("X-Bce-Date")),
	}, "\n")
	signingKey := hex.EncodeToString(testHMACSHA256(t, []byte("secret-key"), []byte(authPrefix)))
	signature := hex.EncodeToString(testHMACSHA256(t, []byte(signingKey), []byte(canonicalRequest)))
	require.Equal(t, authPrefix+"/host;x-bce-date/"+signature, headers.Get("Authorization"))
}

func testHMACSHA256(t *testing.T, key, value []byte) []byte {
	t.Helper()

	mac := hmac.New(sha256.New, key)
	_, err := mac.Write(value)
	require.NoError(t, err)

	return mac.Sum(nil)
}
