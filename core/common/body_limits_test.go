//nolint:testpackage
package common

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/labring/aiproxy/core/common/config"
	"github.com/stretchr/testify/require"
)

func TestBodyLimitsFromEnvironment(t *testing.T) {
	if os.Getenv("AIPROXY_TEST_BODY_LIMITS") != "1" {
		t.Setenv("AIPROXY_TEST_BODY_LIMITS", "1")
		t.Setenv("MAX_REQUEST_BODY_SIZE", "2048")
		t.Setenv("MAX_RESPONSE_BODY_SIZE", "4096")
		t.Setenv("MULTIPART_FORM_MEMORY_LIMIT", "128")
		//nolint:gosec // Re-executes this test binary with a controlled test selector.
		cmd := exec.CommandContext(
			t.Context(),
			os.Args[0],
			"-test.run=^TestBodyLimitsFromEnvironment$",
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		return
	}

	require.EqualValues(t, 2048, config.MaxRequestBodySize)
	require.EqualValues(t, 4096, config.MaxResponseBodySize)
	require.EqualValues(t, 128, config.MultipartFormMemoryLimit)

	for _, knownLength := range []bool{true, false} {
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/",
			strings.NewReader(strings.Repeat("x", 2049)),
		)
		if !knownLength {
			req.ContentLength = -1
		}

		_, err := GetRequestBody(req)
		require.ErrorContains(t, err, "body too large")

		resp := &http.Response{
			Body:          io.NopCloser(strings.NewReader(strings.Repeat("x", 4097))),
			ContentLength: 4097,
		}
		if !knownLength {
			resp.ContentLength = -1
		}

		_, err = GetResponseBody(resp)
		require.ErrorContains(t, err, "body too large")
	}

	var body bytes.Buffer

	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("file", "test.pdf")
	require.NoError(t, err)
	_, err = io.WriteString(file, strings.Repeat("x", 512))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	require.NoError(t, ParseMultipartFormWithLimit(req))
	defer func() { require.NoError(t, req.MultipartForm.RemoveAll()) }()

	uploaded, _, err := req.FormFile("file")
	require.NoError(t, err)

	defer uploaded.Close()

	_, onDisk := uploaded.(*os.File)
	require.True(t, onDisk, "file larger than configured form memory should spill to disk")
}
