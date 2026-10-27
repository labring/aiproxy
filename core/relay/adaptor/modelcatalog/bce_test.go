package modelcatalog

import (
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBCEAuthorizationHeaders(t *testing.T) {
	target := "https://qianfan.baidubce.com:8443/v2/model?Action=DescribeSystemModelSets&note=a%20b"
	headers, err := BCEAuthorizationHeaders(http.MethodPost, target, "access-key", "secret-key")
	require.NoError(t, err)
	require.Regexp(t, `^bce-auth-v1/access-key/[0-9TZ:.-]+/180000/host;x-bce-date/[0-9a-f]{64}$`, headers.Get("Authorization"))
	require.Equal(t, headers.Get("x-bce-date"), strings.Split(headers.Get("Authorization"), "/")[2])
	_, err = time.Parse("2006-01-02T15:04:05Z", headers.Get("x-bce-date"))
	require.NoError(t, err)

	parsed, err := url.Parse(target)
	require.NoError(t, err)
	require.Equal(t, "/v2/model", parsed.Path)
	require.Equal(t, "qianfan.baidubce.com%3A8443", bceURIEncode(strings.ToLower(parsed.Host)))
	require.Equal(t, "2017-02-15T08%3A52%3A48Z", bceURIEncode("2017-02-15T08:52:48Z"))
	require.Equal(t, "a%20b", bceURIEncode("a b"))

	authPrefix := "bce-auth-v1/access-key/" + headers.Get("x-bce-date") + "/180000"
	canonicalQuery := strings.ReplaceAll(parsed.Query().Encode(), "+", "%20")
	require.Equal(t, "Action=DescribeSystemModelSets&note=a%20b", canonicalQuery)
	canonicalRequest := strings.Join([]string{
		http.MethodPost,
		parsed.EscapedPath(),
		canonicalQuery,
		"host:" + url.QueryEscape(strings.ToLower(parsed.Host)),
		"x-bce-date:" + url.QueryEscape(headers.Get("x-bce-date")),
	}, "\n")
	signingKey := hex.EncodeToString(hmacSHA256([]byte("secret-key"), []byte(authPrefix)))
	signature := hex.EncodeToString(hmacSHA256([]byte(signingKey), []byte(canonicalRequest)))
	require.Equal(t, authPrefix+"/host;x-bce-date/"+signature, headers.Get("Authorization"))
}
