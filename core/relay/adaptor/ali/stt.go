package ali

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

const (
	sttRealtime      = "dashscope_realtime"
	sttChatAudio     = "openai_chat_audio"
	sttTranscription = "openai_transcription"
)

type sttConfig struct {
	Protocol string            `json:"stt_upstream_protocol"`
	Models   map[string]string `json:"stt_model_protocols"`
}

// resolveSTTProtocol reads the selected channel on every attempt. Overrides use
// the public model first, then the mapped model, so aliases and channel retries
// can select different protocols without changing the global model definition.
func resolveSTTProtocol(mt *meta.Meta) (string, error) {
	var cfg sttConfig
	if err := mt.ChannelConfigs.LoadConfig(&cfg); err != nil {
		return "", fmt.Errorf("load STT protocol configuration: %w", err)
	}

	protocol, ok := cfg.Models[mt.OriginModel]
	if !ok {
		protocol, ok = cfg.Models[mt.ActualModel]
	}

	if !ok {
		protocol = cfg.Protocol
	}

	switch protocol {
	case sttRealtime, sttChatAudio, sttTranscription:
		return protocol, nil
	case "", "auto":
		name := strings.ToLower(mt.ActualModel)
		if strings.HasPrefix(name, "qwen3-asr-") && strings.Contains(name, "realtime") {
			return "", sttInputError(
				"Qwen realtime ASR requires the session-based realtime API, not DashScope task-based realtime",
			)
		}

		if strings.HasPrefix(name, "qwen3-asr-flash") && !strings.Contains(name, "filetrans") &&
			!strings.Contains(name, "realtime") {
			return sttChatAudio, nil
		}

		if strings.Contains(name, "filetrans") || name == "fun-asr" ||
			strings.HasPrefix(name, "fun-asr-20") ||
			(strings.HasPrefix(name, "fun-asr-mtl") && !strings.Contains(name, "realtime")) ||
			(strings.HasPrefix(name, "paraformer-") && !strings.Contains(name, "realtime")) ||
			name == "sensevoicesmall" {
			return "", sttInputError(
				"model requires the DashScope asynchronous file transcription API; it cannot use the synchronous audio/transcriptions endpoint",
			)
		}

		if strings.HasPrefix(name, "fun-asr-flash") && !strings.Contains(name, "realtime") {
			return "", sttInputError(
				"Fun-ASR-Flash requires the native DashScope multimodal generation API",
			)
		}
		// Preserve the realtime path for existing/custom DashScope deployments.
		return sttRealtime, nil
	default:
		return "", sttInputError(fmt.Sprintf("invalid STT upstream protocol %q", protocol))
	}
}

// sttConfigSchema extends the existing channel editor without a database migration.
func sttConfigSchema() map[string]any {
	schema := openai.ConfigSchema()
	properties, _ := schema["properties"].(map[string]any)
	protocols := []string{"auto", sttRealtime, sttChatAudio, sttTranscription}
	properties["stt_upstream_protocol"] = map[string]any{
		"type":        "string",
		"title":       "STT upstream protocol",
		"enum":        protocols,
		"default":     "auto",
		"description": "Protocol for incoming audio/transcriptions requests. Auto detects Qwen3-ASR-Flash and preserves DashScope realtime for existing models. Asynchronous file transcription models require a separate API.",
	}
	properties["stt_model_protocols"] = map[string]any{
		"type":                 "object",
		"title":                "STT protocol overrides by model",
		"description":          "Map a public or mapped model name to auto, dashscope_realtime, openai_chat_audio or openai_transcription. Public model overrides take precedence. Example: {\"my-asr\":\"openai_chat_audio\"}.",
		"additionalProperties": map[string]any{"type": "string", "enum": protocols},
	}

	return schema
}

func sttInputError(message string) error {
	return relaymodel.WrapperOpenAIErrorWithMessage(
		message,
		"invalid_stt_request",
		http.StatusBadRequest,
	)
}

// sttRequestURL routes STT independently of the incoming Whisper-compatible path.
func sttRequestURL(mt *meta.Meta) (adaptor.RequestURL, error) {
	protocol, err := resolveSTTProtocol(mt)
	if err != nil {
		return adaptor.RequestURL{}, err
	}

	path := "/api-ws/v1/inference"
	if protocol == sttChatAudio {
		path = "/compatible-mode/v1/chat/completions"
	}

	if protocol == sttTranscription {
		path = "/compatible-mode/v1/audio/transcriptions"
	}
	// A full OpenAI-compatible base URL can be used for channel overrides.
	if protocol != sttRealtime {
		if base, parseErr := url.Parse(
			mt.Channel.BaseURL,
		); parseErr == nil &&
			strings.HasSuffix(strings.TrimRight(base.Path, "/"), "/v1") {
			path = strings.TrimPrefix(path, "/compatible-mode/v1")
		}
	}

	target, err := url.JoinPath(mt.Channel.BaseURL, path)

	return adaptor.RequestURL{Method: http.MethodPost, URL: target}, err
}

// convertSTTRequest keeps the public multipart contract while adapting the
// selected upstream. Unsupported response modes fail before a paid request.
func convertSTTRequest(mt *meta.Meta, req *http.Request) (adaptor.ConvertResult, error) {
	protocol, err := resolveSTTProtocol(mt)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	if protocol == sttTranscription {
		return openai.ConvertSTTRequest(mt, req)
	}

	if err := common.ParseMultipartFormWithLimit(req); err != nil {
		return adaptor.ConvertResult{}, err
	}

	switch req.FormValue("response_format") {
	case "", "json", "text":
	default:
		return adaptor.ConvertResult{}, sttInputError(
			"this STT protocol supports response_format=json or text",
		)
	}

	if stream := req.FormValue("stream"); stream != "" && stream != "false" {
		return adaptor.ConvertResult{}, sttInputError(
			"this STT protocol does not support streaming transcription responses",
		)
	}

	for _, key := range []string{"timestamp_granularities", "timestamp_granularities[]"} {
		if len(req.MultipartForm.Value[key]) > 0 {
			return adaptor.ConvertResult{}, sttInputError(
				"this STT protocol does not support timestamp_granularities",
			)
		}
	}

	if protocol == sttRealtime {
		return ConvertSTTRequest(mt, req)
	}

	audio, format, _, err := prepareSTTAudio(mt, req, false)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	if len(audio) > 7*1024*1024 {
		return adaptor.ConvertResult{}, sttInputError(
			"Qwen-ASR audio is too large for a 10 MB Base64 request",
		)
	}

	mime := "audio/" + format
	if format == "mp3" {
		mime = "audio/mpeg"
	}

	messages := []any{}
	if prompt := req.FormValue("prompt"); prompt != "" {
		messages = append(messages, map[string]any{"role": "system", "content": prompt})
	}

	messages = append(messages, map[string]any{"role": "user", "content": []any{
		map[string]any{
			"type": "input_audio",
			"input_audio": map[string]any{
				"data": "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(audio),
			},
		},
	}})

	options := map[string]any{}
	if language := req.FormValue("language"); language != "" {
		options["language"] = language
	}

	data, err := sonic.Marshal(
		map[string]any{
			"model":       mt.ActualModel,
			"messages":    messages,
			"stream":      false,
			"asr_options": options,
		},
	)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{"Content-Type": {"application/json"}},
		Body:   bytes.NewReader(data),
	}, nil
}

// sttResponse normalizes chat audio results, preserving token usage separately
// from seconds. Silence is a valid empty transcript; missing choices are not.
func sttResponse(
	mt *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	protocol, err := resolveSTTProtocol(mt)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"stt_protocol_error",
			http.StatusBadRequest,
		)
	}

	if protocol == sttRealtime {
		return STTDoResponse(mt, c, resp)
	}

	if protocol == sttTranscription {
		return openai.STTHandler(mt, c, resp)
	}

	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	data, err := common.GetResponseBody(resp)

	var result struct {
		ID      string `json:"id"`
		Choices []struct {
			Message struct {
				Content *string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage json.RawMessage `json:"usage"`
	}
	if err == nil {
		err = sonic.Unmarshal(data, &result)
	}

	if err == nil && (len(result.Choices) == 0 || result.Choices[0].Message.Content == nil) {
		err = errors.New("missing transcription content in upstream chat response")
	}

	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"invalid_stt_response",
			http.StatusBadGateway,
		)
	}

	usage := relaymodel.SttUsage{Type: relaymodel.SttUsageTypeTokens}
	if len(result.Usage) > 0 {
		normalized, usageErr := relaymodel.NormalizeSTTTokenUsage(result.Usage)
		if usageErr == nil {
			var decoded struct {
				*relaymodel.SttUsage
				Seconds float64 `json:"seconds"`
			}

			decoded.SttUsage = &usage
			usageErr = sonic.Unmarshal(normalized, &decoded)
			usage.Seconds = int64(math.Ceil(decoded.Seconds))
		}

		if usageErr != nil {
			return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
				usageErr,
				"invalid_stt_usage",
				http.StatusBadGateway,
			)
		}
	}

	if usage.Seconds <= 0 {
		usage.Seconds = int64(mt.RequestUsage.AudioInputTokens)
	}

	writeSTTResponse(c, *result.Choices[0].Message.Content, usage)

	return adaptor.DoResponseResult{Usage: usage.ToModelUsage(), UpstreamID: result.ID}, nil
}

// writeSTTResponse supports the synchronous Whisper JSON and text contracts.
func writeSTTResponse(c *gin.Context, text string, usage relaymodel.SttUsage) {
	if c.Request.FormValue("response_format") == "text" {
		c.String(http.StatusOK, "%s", text)
		return
	}

	c.JSON(http.StatusOK, gin.H{"text": text, "usage": usage})
}

func sttDoRequest(mt *meta.Meta, req *http.Request) (*http.Response, error) {
	protocol, err := resolveSTTProtocol(mt)
	if err != nil {
		return nil, err
	}

	if protocol == sttRealtime {
		return STTDoRequest(mt, req)
	}

	return utils.DoRequestWithMeta(req, mt)
}
