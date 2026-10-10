//nolint:testpackage
package controller

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/model"
	"github.com/stretchr/testify/require"
)

func TestValidateImagesEditsRequestRejectsNonMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequestWithContext(
		context.Background(),
		"POST",
		"/v1/images/edits",
		strings.NewReader("model=gpt-image-1&prompt=test"),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	err := ValidateImagesEditsRequest(c, model.ModelConfig{MaxImageGenerationCount: 1})
	require.Error(t, err)
	require.Equal(t, "images edits requests must use multipart/form-data", err.Error())

	var requestParamErr *RequestParamError
	require.ErrorAs(t, err, &requestParamErr)
	require.Equal(t, 400, requestParamErr.StatusCode)
}

func TestValidateImagesEditsRequestRejectsTooLargeN(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("model", "gpt-image-1"))
	require.NoError(t, writer.WriteField("prompt", "test"))
	require.NoError(t, writer.WriteField("n", "2"))
	require.NoError(t, writer.Close())

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/images/edits", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	err := ValidateImagesEditsRequest(c, model.ModelConfig{MaxImageGenerationCount: 1})
	require.Error(t, err)
	require.Equal(t, "n must be less than or equal to 1", err.Error())

	var requestParamErr *RequestParamError
	require.ErrorAs(t, err, &requestParamErr)
	require.Equal(t, 400, requestParamErr.StatusCode)
}

func TestValidateImagesEditsRequestRejectsDuplicateN(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("model", "gpt-image-1"))
	require.NoError(t, writer.WriteField("prompt", "test"))
	require.NoError(t, writer.WriteField("n", "1"))
	require.NoError(t, writer.WriteField("n", "100"))
	require.NoError(t, writer.Close())

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/images/edits", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	err := ValidateImagesEditsRequest(c, model.ModelConfig{MaxImageGenerationCount: 1})
	require.Error(t, err)
	require.Equal(t, "duplicate n", err.Error())

	var requestParamErr *RequestParamError
	require.ErrorAs(t, err, &requestParamErr)
	require.Equal(t, 400, requestParamErr.StatusCode)
}

func TestValidateImagesEditsRequestWrapsParseError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequestWithContext(
		context.Background(),
		"POST",
		"/v1/images/edits",
		strings.NewReader("--test\r\n"),
	)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	req.ContentLength = config.MaxRequestBodySize + 1

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	err := ValidateImagesEditsRequest(c, model.ModelConfig{MaxImageGenerationCount: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "request body too large")

	var requestParamErr *RequestParamError
	require.ErrorAs(t, err, &requestParamErr)
	require.Equal(t, 400, requestParamErr.StatusCode)
}

func TestValidateImagesEditsRequestRejectsUnsupportedResolution(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("model", "gpt-image-1"))
	require.NoError(t, writer.WriteField("prompt", "test"))
	require.NoError(t, writer.WriteField("size", "512x512"))
	require.NoError(t, writer.Close())

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/images/edits", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	err := ValidateImagesEditsRequest(c, model.ModelConfig{
		AllowedResolutions: []string{"1024x1024"},
	})
	require.Error(t, err)
	require.Equal(
		t,
		"unsupported image resolution `512x512`, supported resolutions: 1024x1024",
		err.Error(),
	)

	var requestParamErr *RequestParamError
	require.ErrorAs(t, err, &requestParamErr)
	require.Equal(t, 400, requestParamErr.StatusCode)
}

func TestValidateImagesEditsRequestRejectsInvalidResolutionFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("model", "gpt-image-1"))
	require.NoError(t, writer.WriteField("prompt", "test"))
	require.NoError(t, writer.WriteField("size", "1:1"))
	require.NoError(t, writer.Close())

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/images/edits", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	err := ValidateImagesEditsRequest(c, model.ModelConfig{})
	require.Error(t, err)
	require.Equal(
		t,
		"invalid image resolution `1:1`, supported resolutions: auto, <width>x<height>",
		err.Error(),
	)
}

func TestGetImagesEditsRequestUsageReturnsNoPreflightUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("prompt", "edit image"))

	for _, filename := range []string{"input1.png", "input2.png"} {
		part, err := writer.CreateFormFile("image[]", filename)
		require.NoError(t, err)

		_, _ = part.Write([]byte("fake image"))
	}

	require.NoError(t, writer.Close())

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/images/edits",
		body,
	)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	usage, err := GetImagesEditsRequestUsage(c, model.ModelConfig{Model: "gpt-image-1"})
	require.NoError(t, err)
	require.Zero(t, usage.Usage.InputTokens)
	require.Zero(t, usage.Usage.ImageInputTokens)
	require.Zero(t, usage.Usage.OutputTokens)
	require.Zero(t, usage.Usage.ImageOutputTokens)
	require.Zero(t, usage.Usage.TotalTokens)
}
