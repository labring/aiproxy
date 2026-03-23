package openai

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertImagesRequest_DefaultIncludesModel(t *testing.T) {
	meta := meta.NewMeta(nil, mode.ImagesGenerations, "gpt-image-1", model.ModelConfig{})

	req, err := http.NewRequest(
		http.MethodPost,
		"http://example.com/v1/images/generations",
		strings.NewReader(`{"prompt":"test","response_format":"b64_json"}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	result, err := ConvertImagesRequest(meta, req)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(body, &payload)
	require.NoError(t, err)

	assert.Equal(t, "gpt-image-1", payload["model"])
	assert.Equal(t, "b64_json", meta.GetString(MetaResponseFormat))
}

func TestConvertImagesRequest_CanRemoveModelDynamically(t *testing.T) {
	meta := meta.NewMeta(nil, mode.ImagesGenerations, "gpt-image-1", model.ModelConfig{})

	req, err := http.NewRequest(
		http.MethodPost,
		"http://example.com/v1/images/generations",
		strings.NewReader(`{"model":"ignored","prompt":"test","response_format":"b64_json"}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	result, err := ConvertImagesRequest(meta, req, ImagesRequestRemoveModel)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(body, &payload)
	require.NoError(t, err)

	_, ok := payload["model"]
	assert.False(t, ok)
	assert.Equal(t, "b64_json", meta.GetString(MetaResponseFormat))
}

func TestConvertImagesEditsRequest_DefaultIncludesModel(t *testing.T) {
	meta := meta.NewMeta(nil, mode.ImagesEdits, "gpt-image-1", model.ModelConfig{})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "ignored"))
	require.NoError(t, writer.WriteField("prompt", "edit prompt"))
	part, err := writer.CreateFormFile("image", "test.png")
	require.NoError(t, err)
	_, err = part.Write([]byte("png-bytes"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req, err := http.NewRequest(http.MethodPost, "http://example.com/v1/images/edits", bytes.NewReader(body.Bytes()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.ContentLength = int64(body.Len())

	result, err := ConvertImagesEditsRequest(meta, req)
	require.NoError(t, err)

	convertedBody, err := io.ReadAll(result.Body)
	require.NoError(t, err)

	convertedReq, err := http.NewRequest(http.MethodPost, "http://example.com", bytes.NewReader(convertedBody))
	require.NoError(t, err)
	convertedReq.Header.Set("Content-Type", result.Header.Get("Content-Type"))
	convertedReq.ContentLength = int64(len(convertedBody))

	err = convertedReq.ParseMultipartForm(1024 * 1024 * 4)
	require.NoError(t, err)

	assert.Equal(t, "gpt-image-1", convertedReq.MultipartForm.Value["model"][0])
	assert.Equal(t, "edit prompt", convertedReq.MultipartForm.Value["prompt"][0])
}

func TestConvertImagesEditsRequest_CanRemoveModelDynamically(t *testing.T) {
	meta := meta.NewMeta(nil, mode.ImagesEdits, "gpt-image-1", model.ModelConfig{})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "ignored"))
	require.NoError(t, writer.WriteField("prompt", "edit prompt"))
	require.NoError(t, writer.WriteField("response_format", "b64_json"))
	part, err := writer.CreateFormFile("image", "test.png")
	require.NoError(t, err)
	_, err = part.Write([]byte("png-bytes"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req, err := http.NewRequest(http.MethodPost, "http://example.com/v1/images/edits", bytes.NewReader(body.Bytes()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.ContentLength = int64(body.Len())

	result, err := ConvertImagesEditsRequest(meta, req, ImagesRequestRemoveModel)
	require.NoError(t, err)

	convertedBody, err := io.ReadAll(result.Body)
	require.NoError(t, err)

	convertedReq, err := http.NewRequest(http.MethodPost, "http://example.com", bytes.NewReader(convertedBody))
	require.NoError(t, err)
	convertedReq.Header.Set("Content-Type", result.Header.Get("Content-Type"))
	convertedReq.ContentLength = int64(len(convertedBody))

	err = convertedReq.ParseMultipartForm(1024 * 1024 * 4)
	require.NoError(t, err)

	assert.Nil(t, convertedReq.MultipartForm.Value["model"])
	assert.Equal(t, "edit prompt", convertedReq.MultipartForm.Value["prompt"][0])
	assert.Equal(t, "b64_json", meta.GetString(MetaResponseFormat))
}
