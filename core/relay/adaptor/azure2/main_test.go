package azure2_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/azure2"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertRequest_ImagesGenerationsRemovesModel(t *testing.T) {
	adaptor := &azure2.Adaptor{}
	meta := meta.NewMeta(nil, mode.ImagesGenerations, "gpt-image-1.0", model.ModelConfig{})

	body := `{"model":"gpt-image-1.0","prompt":"test prompt","response_format":"b64_json"}`
	req, err := http.NewRequest(http.MethodPost, "http://example.com/v1/images/generations", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	result, err := adaptor.ConvertRequest(meta, nil, req)
	require.NoError(t, err)

	convertedBody, err := io.ReadAll(result.Body)
	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(convertedBody, &payload)
	require.NoError(t, err)

	_, ok := payload["model"]
	assert.False(t, ok)
	assert.Equal(t, "test prompt", payload["prompt"])
	assert.Equal(t, "b64_json", meta.GetString(openai.MetaResponseFormat))
}

func TestConvertRequest_ImagesEditsRemovesModel(t *testing.T) {
	adaptor := &azure2.Adaptor{}
	meta := meta.NewMeta(nil, mode.ImagesEdits, "gpt-image-1.0", model.ModelConfig{})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-1.0"))
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

	result, err := adaptor.ConvertRequest(meta, nil, req)
	require.NoError(t, err)

	convertedBody, err := io.ReadAll(result.Body)
	require.NoError(t, err)

	convertedReq, err := http.NewRequest(http.MethodPost, "http://example.com", bytes.NewReader(convertedBody))
	require.NoError(t, err)
	convertedReq.Header.Set("Content-Type", result.Header.Get("Content-Type"))
	convertedReq.ContentLength = int64(len(convertedBody))

	err = convertedReq.ParseMultipartForm(1024 * 1024 * 4)
	require.NoError(t, err)

	assert.Equal(t, "edit prompt", convertedReq.MultipartForm.Value["prompt"][0])
	assert.Nil(t, convertedReq.MultipartForm.Value["model"])
	assert.Equal(t, "b64_json", meta.GetString(openai.MetaResponseFormat))

	files := convertedReq.MultipartForm.File["image"]
	require.Len(t, files, 1)
	file, err := files[0].Open()
	require.NoError(t, err)
	defer file.Close()

	fileContent, err := io.ReadAll(file)
	require.NoError(t, err)
	assert.Equal(t, []byte("png-bytes"), fileContent)
}
