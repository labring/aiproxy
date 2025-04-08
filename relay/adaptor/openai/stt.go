package openai

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/common/conv"
	"github.com/labring/aiproxy/middleware"
	"github.com/labring/aiproxy/model"
	"github.com/labring/aiproxy/relay/meta"
	relaymodel "github.com/labring/aiproxy/relay/model"
)

func ConvertSTTRequest(meta *meta.Meta, request *http.Request) (string, http.Header, io.Reader, error) {
	err := request.ParseMultipartForm(1024 * 1024 * 4)
	if err != nil {
		return "", nil, nil, err
	}

	multipartBody := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(multipartBody)

	for key, values := range request.MultipartForm.Value {
		if len(values) == 0 {
			continue
		}
		value := values[0]
		if key == "model" {
			err = multipartWriter.WriteField(key, meta.ActualModel)
			if err != nil {
				return "", nil, nil, err
			}
			continue
		}
		if key == "response_format" {
			meta.Set(MetaResponseFormat, value)
			continue
		}
		err = multipartWriter.WriteField(key, value)
		if err != nil {
			return "", nil, nil, err
		}
	}

	for key, files := range request.MultipartForm.File {
		if len(files) == 0 {
			continue
		}
		fileHeader := files[0]
		file, err := fileHeader.Open()
		if err != nil {
			return "", nil, nil, err
		}
		w, err := multipartWriter.CreateFormFile(key, fileHeader.Filename)
		if err != nil {
			file.Close()
			return "", nil, nil, err
		}
		_, err = io.Copy(w, file)
		file.Close()
		if err != nil {
			return "", nil, nil, err
		}
	}

	multipartWriter.Close()
	ContentType := multipartWriter.FormDataContentType()
	return http.MethodPost, http.Header{
		"Content-Type": {ContentType},
	}, multipartBody, nil
}

func STTHandler(meta *meta.Meta, c *gin.Context, resp *http.Response) (*model.Usage, *relaymodel.ErrorWithStatusCode) {
	if resp.StatusCode != http.StatusOK {
		return nil, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := middleware.GetLogger(c)

	responseFormat := meta.GetString(MetaResponseFormat)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}

	var text string
	switch responseFormat {
	case "text":
		text = getTextFromText(responseBody)
	case "srt":
		text, err = getTextFromSRT(responseBody)
	case "verbose_json":
		text, err = getTextFromVerboseJSON(responseBody)
	case "vtt":
		text, err = getTextFromVTT(responseBody)
	case "json":
		fallthrough
	default:
		text, err = getTextFromJSON(responseBody)
	}
	if err != nil {
		return nil, ErrorWrapper(err, "get_text_from_body_err", http.StatusInternalServerError)
	}
	var promptTokens int64
	if meta.InputTokens > 0 {
		promptTokens = meta.InputTokens
	} else {
		promptTokens = CountTokenText(text, meta.ActualModel)
	}

	for k, v := range resp.Header {
		c.Writer.Header().Set(k, v[0])
	}
	_, err = c.Writer.Write(responseBody)
	if err != nil {
		log.Warnf("write response body failed: %v", err)
	}

	return &model.Usage{
		InputTokens: promptTokens,
		TotalTokens: promptTokens,
	}, nil
}

func getTextFromVTT(body []byte) (string, error) {
	return getTextFromSRT(body)
}

func getTextFromVerboseJSON(body []byte) (string, error) {
	var whisperResponse relaymodel.SttVerboseJSONResponse
	if err := sonic.Unmarshal(body, &whisperResponse); err != nil {
		return "", fmt.Errorf("unmarshal_response_body_failed err :%w", err)
	}
	return whisperResponse.Text, nil
}

func getTextFromSRT(body []byte) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	var builder strings.Builder
	var textLine bool
	for scanner.Scan() {
		line := scanner.Text()
		if textLine {
			builder.WriteString(line)
			textLine = false
			continue
		} else if strings.Contains(line, "-->") {
			textLine = true
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return builder.String(), nil
}

func getTextFromText(body []byte) string {
	return strings.TrimSuffix(conv.BytesToString(body), "\n")
}

func getTextFromJSON(body []byte) (string, error) {
	var whisperResponse relaymodel.SttJSONResponse
	if err := sonic.Unmarshal(body, &whisperResponse); err != nil {
		return "", fmt.Errorf("unmarshal_response_body_failed err :%w", err)
	}
	return whisperResponse.Text, nil
}
