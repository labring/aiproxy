package openai

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
	"github.com/labring/aiproxy/core/relay/utils"
)

// ConvertSTTRequest converts multipart form request for STT
func ConvertSTTRequest(
	meta *meta.Meta,
	request *http.Request,
) (adaptor.ConvertResult, error) {
	if err := common.ParseMultipartFormWithLimit(request); err != nil {
		return adaptor.ConvertResult{}, convertRequestError(
			meta,
			fmt.Sprintf("parse multipart form: %s", err),
		)
	}

	multipartBody := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(multipartBody)

	// Process form values
	if err := processFormValues(multipartWriter, request.MultipartForm.Value, meta); err != nil {
		return adaptor.ConvertResult{}, fmt.Errorf("process form values: %w", err)
	}

	// Process form files
	if err := processFormFiles(multipartWriter, request.MultipartForm.File); err != nil {
		return adaptor.ConvertResult{}, fmt.Errorf("process form files: %w", err)
	}

	multipartWriter.Close()

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type": {multipartWriter.FormDataContentType()},
		},
		Body: multipartBody,
	}, nil
}

// processFormValues processes form values and handles special cases
func processFormValues(
	writer *multipart.Writer,
	formValues map[string][]string,
	meta *meta.Meta,
) error {
	for key, values := range formValues {
		if len(values) == 0 {
			continue
		}

		value := values[0]

		switch key {
		case "model":
			if err := writer.WriteField(key, meta.ActualModel); err != nil {
				return fmt.Errorf("write model field: %w", err)
			}
		case "response_format":
			meta.Set(MetaResponseFormat, value)
		default:
			if err := writer.WriteField(key, value); err != nil {
				return fmt.Errorf("write field %s: %w", key, err)
			}
		}
	}

	return nil
}

// processFormFiles processes form files
func processFormFiles(
	writer *multipart.Writer,
	formFiles map[string][]*multipart.FileHeader,
) error {
	for key, files := range formFiles {
		if len(files) == 0 {
			continue
		}

		fileHeader := files[0]
		if err := copyFileToWriter(writer, key, fileHeader); err != nil {
			return fmt.Errorf("copy file %s: %w", key, err)
		}
	}

	return nil
}

// copyFileToWriter copies a file to multipart writer
func copyFileToWriter(
	writer *multipart.Writer,
	key string,
	fileHeader *multipart.FileHeader,
) error {
	file, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	w, err := writer.CreateFormFile(key, fileHeader.Filename)
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}

	if _, err := io.Copy(w, file); err != nil {
		return fmt.Errorf("copy file content: %w", err)
	}

	return nil
}

// STTHandler handles STT response
func STTHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	if utils.IsStreamResponse(resp) {
		return handleSTTStream(meta, c, resp)
	}

	return handleSTTNonStream(meta, c, resp)
}

// handleSTTNonStream handles non-streaming STT response
func handleSTTNonStream(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	defer resp.Body.Close()

	responseBody, err := common.GetResponseBody(resp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	_, err = extractTextFromResponse(responseBody, meta.GetString(MetaResponseFormat))
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"extract_text_failed",
			http.StatusInternalServerError,
		)
	}

	usage := measuredSTTDurationUsage(meta)

	// Handle JSON response with usage injection
	if strings.Contains(resp.Header.Get("Content-Type"), "json") {
		node, err := common.GetJSONNodeNoCopy(responseBody)
		if err != nil {
			responseErr := relaymodel.WrapperOpenAIError(
				err,
				"get_node_from_body_err",
				http.StatusInternalServerError,
			)

			return adaptor.DoResponseResult{
				Usage: usage.ToModelUsage(),
			}, responseErr
		}

		usageNode := node.Get("usage")
		if usageNode != nil && usageNode.Exists() && usageNode.TypeSafe() != ast.V_NULL {
			usageStr, err := usageNode.Raw()
			if err != nil {
				responseErr := relaymodel.WrapperOpenAIError(
					err,
					"unmarshal_response_err",
					http.StatusInternalServerError,
				)

				return adaptor.DoResponseResult{
					Usage: usage.ToModelUsage(),
				}, responseErr
			}

			normalized, normalizeErr := relaymodel.NormalizeSTTTokenUsage([]byte(usageStr))
			if normalizeErr != nil {
				err = normalizeErr
			} else {
				// Upstream components start at zero; neither totals nor text can
				// supply missing input/output usage.
				usage = &relaymodel.SttUsage{
					Type:    relaymodel.SttUsageTypeTokens,
					Seconds: int64(meta.RequestUsage.AudioInputTokens),
				}

				err = sonic.Unmarshal(normalized, usage)
				if err == nil {
					_, err = node.SetAny("usage", sonic.NoCopyRawMessage(normalized))
					if err == nil {
						responseBody, err = node.MarshalJSON()
					}
				}
			}

			if err != nil {
				responseErr := relaymodel.WrapperOpenAIError(
					err,
					"unmarshal_response_err",
					http.StatusInternalServerError,
				)

				return adaptor.DoResponseResult{
					Usage: usage.ToModelUsage(),
				}, responseErr
			}
		} else {
			responseBody, err = injectUsageIntoJSON(&node, usage)
			if err != nil {
				responseErr := relaymodel.WrapperOpenAIError(
					err,
					"inject_usage_failed",
					http.StatusInternalServerError,
				)

				return adaptor.DoResponseResult{
					Usage: usage.ToModelUsage(),
				}, responseErr
			}
		}

		responseBody, err = injectSTTSeconds(
			responseBody,
			int64(meta.RequestUsage.AudioInputTokens),
		)
		if err != nil {
			return adaptor.DoResponseResult{
				Usage: usage.ToModelUsage(),
			}, relaymodel.WrapperOpenAIError(
				err,
				"inject_usage_seconds_failed",
				http.StatusInternalServerError,
			)
		}

		c.Writer.Header().Set("Content-Type", "application/json")
	}

	log := common.GetLogger(c)
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(responseBody)))

	if _, err := c.Writer.Write(responseBody); err != nil {
		log.Warnf("write response body failed: %v", err)
	}

	return adaptor.DoResponseResult{Usage: usage.ToModelUsage()}, nil
}

// handleSTTStream handles streaming STT response
func handleSTTStream(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	defer resp.Body.Close()

	scanner, cleanup := utils.NewScanner(resp.Body)
	defer cleanup()

	return adaptor.DoResponseResult{Usage: processSTTStreamChunks(scanner, c, meta)}, nil
}

// processSTTStreamChunks processes streaming chunks and returns final usage
func processSTTStreamChunks(
	scanner *bufio.Scanner,
	c *gin.Context,
	meta *meta.Meta,
) model.Usage {
	log := common.GetLogger(c)

	var usage *relaymodel.SttUsage

	for scanner.Scan() {
		data := scanner.Bytes()
		if !render.IsValidSSEData(data) {
			continue
		}

		data = render.ExtractSSEData(data)
		if render.IsSSEDone(data) {
			break
		}

		normalized, normalizeErr := normalizeSTTResponseTokenUsage(data)
		if normalizeErr != nil {
			log.Error("error normalizing STT stream usage: " + normalizeErr.Error())
			continue
		}

		data = normalized

		var sseResponse relaymodel.SttSSEResponse

		err := sonic.Unmarshal(data, &sseResponse)
		if err != nil {
			log.Error("error unmarshalling STT stream response: " + err.Error())
			continue
		}

		data, totalUsage := processSSEResponse(sseResponse, meta, data)
		if totalUsage != nil {
			usage = totalUsage
		}

		render.OpenaiBytesData(c, data)
	}

	render.OpenaiDone(c)

	if usage == nil {
		usage = measuredSTTDurationUsage(meta)
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading STT stream: " + err.Error())
	}

	return usage.ToModelUsage()
}

// processSSEResponse processes individual SSE response and returns usage if complete
func processSSEResponse(
	sseResponse relaymodel.SttSSEResponse,
	meta *meta.Meta,
	data []byte,
) ([]byte, *relaymodel.SttUsage) {
	switch sseResponse.Type {
	case relaymodel.SttSSEResponseTypeTranscriptTextDelta:
		return data, nil

	case relaymodel.SttSSEResponseTypeTranscriptTextDone:
		if sseResponse.Usage != nil {
			// Expose server-measured seconds independently of upstream input/output counters.
			withSeconds, err := injectSTTSeconds(data, int64(meta.RequestUsage.AudioInputTokens))
			if err == nil {
				data = withSeconds
			}

			return data, sseResponse.Usage
		}

		usage := measuredSTTDurationUsage(meta)

		return injectUsageIntoSSE(data, usage), usage

	default:
		return data, nil
	}
}

// injectSTTSeconds fills missing/invalid response seconds from server-measured audio duration.
// It preserves upstream zero, token fields, unknown fields and numeric precision. The response
// copy preserves upstream input/output counters; duration never supplies token quantities.
func injectSTTSeconds(data []byte, seconds int64) ([]byte, error) {
	node, err := common.GetJSONNodeNoCopy(data)
	if err != nil {
		return nil, fmt.Errorf("parse STT response: %w", err)
	}

	usageNode := node.Get("usage")
	if usageNode == nil || !usageNode.Exists() || usageNode.TypeSafe() == ast.V_NULL {
		_, err = node.SetAny("usage", map[string]int64{"seconds": max(0, seconds)})
	} else {
		secondsNode := usageNode.Get("seconds")
		if secondsNode != nil && secondsNode.TypeSafe() == ast.V_NUMBER {
			value, valueErr := secondsNode.Float64()
			if valueErr == nil && value >= 0 && !math.IsInf(value, 0) && !math.IsNaN(value) {
				return data, nil
			}
		}

		_, err = usageNode.SetAny("seconds", max(0, seconds))
	}

	if err != nil {
		return nil, fmt.Errorf("set STT seconds: %w", err)
	}

	return node.MarshalJSON()
}

// measuredSTTDurationUsage supplies only the server-measured duration when
// upstream usage is absent. Token quantities must always come from upstream.
func measuredSTTDurationUsage(meta *meta.Meta) *relaymodel.SttUsage {
	return &relaymodel.SttUsage{
		Type:    relaymodel.SttUsageTypeDuration,
		Seconds: int64(meta.RequestUsage.AudioInputTokens),
	}
}

// normalizeSTTResponseTokenUsage enriches JSON/SSE usage while preserving all
// other event fields. Missing/null usage remains absent for duration fallback.
func normalizeSTTResponseTokenUsage(data []byte) ([]byte, error) {
	node, err := common.GetJSONNodeNoCopy(data)
	if err != nil {
		return nil, err
	}

	usageNode := node.Get("usage")
	if usageNode == nil || !usageNode.Exists() || usageNode.TypeSafe() == ast.V_NULL {
		return data, nil
	}

	raw, err := usageNode.Raw()
	if err != nil {
		return nil, err
	}

	normalized, err := relaymodel.NormalizeSTTTokenUsage([]byte(raw))
	if err != nil {
		return nil, err
	}

	if _, err = node.SetAny("usage", sonic.NoCopyRawMessage(normalized)); err != nil {
		return nil, err
	}

	return node.MarshalJSON()
}

// injectUsageIntoJSON injects usage into JSON response
func injectUsageIntoJSON(node *ast.Node, usage *relaymodel.SttUsage) ([]byte, error) {
	_, err := node.SetAny("usage", usage)
	if err != nil {
		return nil, fmt.Errorf("set usage: %w", err)
	}

	return node.MarshalJSON()
}

// injectUsageIntoSSE injects usage into SSE response data
func injectUsageIntoSSE(data []byte, usage *relaymodel.SttUsage) []byte {
	node, err := common.GetJSONNodeNoCopy(data)
	if err != nil {
		return nil
	}

	_, err = node.SetAny("usage", usage)
	if err != nil {
		return nil
	}

	result, err := node.MarshalJSON()
	if err != nil {
		return nil
	}

	return result
}

// extractTextFromResponse extracts text based on response format
func extractTextFromResponse(body []byte, responseFormat string) (string, error) {
	switch responseFormat {
	case "text":
		return getTextFromText(body), nil
	case "srt":
		return getTextFromSRT(body)
	case "verbose_json":
		return getTextFromVerboseJSON(body)
	case "vtt":
		return getTextFromVTT(body)
	case "json":
		fallthrough
	default:
		return getTextFromJSON(body)
	}
}

// Text extraction functions (unchanged)
func getTextFromVTT(body []byte) (string, error) {
	return getTextFromSRT(body)
}

func getTextFromVerboseJSON(body []byte) (string, error) {
	var whisperResponse relaymodel.SttVerboseJSONResponse
	if err := sonic.Unmarshal(body, &whisperResponse); err != nil {
		return "", fmt.Errorf("unmarshal verbose JSON: %w", err)
	}

	return whisperResponse.Text, nil
}

func getTextFromSRT(body []byte) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(body))

	var (
		builder  strings.Builder
		textLine bool
	)

	for scanner.Scan() {
		line := scanner.Text()
		if textLine {
			builder.WriteString(line)

			textLine = false
		} else if strings.Contains(line, "-->") {
			textLine = true
		}
	}

	return builder.String(), scanner.Err()
}

func getTextFromText(body []byte) string {
	return strings.TrimSuffix(conv.BytesToString(body), "\n")
}

func getTextFromJSON(body []byte) (string, error) {
	var whisperResponse relaymodel.SttJSONResponse
	if err := sonic.Unmarshal(body, &whisperResponse); err != nil {
		return "", fmt.Errorf("unmarshal JSON: %w", err)
	}

	return whisperResponse.Text, nil
}
