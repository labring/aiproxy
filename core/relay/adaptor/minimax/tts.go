package minimax

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
	"github.com/labring/aiproxy/core/relay/utils"
)

func ConvertTTSRequest(meta *meta.Meta, req *http.Request) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2ValidatedNode(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	meta.Set("stream_format", stringFromTTSNode(node.Get("stream_format")))

	responseFormat, err := patchTTSRequestNode(&node, meta.ActualModel)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	meta.Set("audio_format", responseFormat)

	body, err := node.MarshalJSON()
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(body))},
		},
		Body: bytes.NewReader(body),
	}, nil
}

func patchTTSRequestNode(node *ast.Node, actualModel string) (string, error) {
	if err := patchTTSModelAndText(node, actualModel); err != nil {
		return "", err
	}

	if err := patchTTSVoice(node); err != nil {
		return "", err
	}

	responseFormat, err := patchTTSAudio(node)
	if err != nil {
		return "", err
	}

	if err := patchTTSStreamOptions(node, responseFormat); err != nil {
		return "", err
	}

	if _, err := node.Set("language_boost", ast.NewString("auto")); err != nil {
		return "", err
	}

	return responseFormat, nil
}

func patchTTSModelAndText(node *ast.Node, actualModel string) error {
	if _, err := node.Set("model", ast.NewString(actualModel)); err != nil {
		return err
	}

	inputNode := node.Get("input")
	if inputNode.Exists() {
		if _, err := node.Set("text", *inputNode); err != nil {
			return err
		}
	} else if _, err := node.Set("text", ast.NewNull()); err != nil {
		return err
	}

	if _, err := node.Unset("input"); err != nil {
		return err
	}

	return nil
}

func patchTTSVoice(node *ast.Node) error {
	voice := stringFromTTSNode(node.Get("voice"))

	if _, err := node.Unset("voice"); err != nil {
		return err
	}

	if voice == "" {
		voice = "male-qn-qingse"
	}

	voiceSetting, err := ttsObjectNode(node, "voice_setting")
	if err != nil {
		return err
	}

	timberWeightsNode := node.Get("timber_weights")
	if !timberWeightsNode.Exists() || timberWeightsNode.TypeSafe() != ast.V_ARRAY ||
		ttsArrayLen(timberWeightsNode) == 0 {
		if _, err := voiceSetting.Set("voice_id", ast.NewString(voice)); err != nil {
			return err
		}
	}

	if speed, ok := floatFromTTSNode(node.Get("speed")); ok {
		if _, err := voiceSetting.Set(
			"speed",
			ast.NewNumber(strconv.Itoa(int(speed))),
		); err != nil {
			return err
		}
	}

	if _, err := node.Unset("speed"); err != nil {
		return err
	}

	return nil
}

func patchTTSAudio(node *ast.Node) (string, error) {
	audioSetting, err := ttsObjectNode(node, "audio_setting")
	if err != nil {
		return "", err
	}

	responseFormat := stringFromTTSNode(node.Get("response_format"))
	if responseFormat == "" {
		responseFormat = stringFromTTSNode(node.Get("format"))
	}

	if responseFormat == "" {
		responseFormat = "mp3"
	}

	if _, err := audioSetting.Set("format", ast.NewString(responseFormat)); err != nil {
		return "", err
	}

	if _, err := node.Unset("response_format"); err != nil {
		return "", err
	}

	if sampleRate, ok := floatFromTTSNode(node.Get("sample_rate")); ok {
		if _, err := audioSetting.Set(
			"sample_rate",
			ast.NewNumber(strconv.Itoa(int(sampleRate))),
		); err != nil {
			return "", err
		}
	}

	if _, err := node.Unset("sample_rate"); err != nil {
		return "", err
	}

	return responseFormat, nil
}

func patchTTSStreamOptions(node *ast.Node, responseFormat string) error {
	if responseFormat == "wav" {
		if _, err := node.Set("stream", ast.NewBool(false)); err != nil {
			return err
		}
	} else {
		if _, err := node.Set("stream", ast.NewBool(true)); err != nil {
			return err
		}

		if _, err := node.Set("stream_options", ast.NewObject([]ast.Pair{
			ast.NewPair("exclude_aggregated_audio", ast.NewBool(true)),
		})); err != nil {
			return err
		}
	}

	return nil
}

func ttsObjectNode(node *ast.Node, key string) (*ast.Node, error) {
	value := node.Get(key)
	if value.Exists() && value.TypeSafe() == ast.V_OBJECT {
		return value, nil
	}

	if _, err := node.Set(key, ast.NewObject(nil)); err != nil {
		return nil, err
	}

	return node.Get(key), nil
}

func stringFromTTSNode(node *ast.Node) string {
	if node == nil || !node.Exists() || node.TypeSafe() != ast.V_STRING {
		return ""
	}

	value, err := node.String()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(value)
}

func floatFromTTSNode(node *ast.Node) (float64, bool) {
	if node == nil || !node.Exists() || node.TypeSafe() == ast.V_NULL {
		return 0, false
	}

	if node.TypeSafe() == ast.V_STRING {
		value, err := node.String()
		if err != nil {
			return 0, false
		}

		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return 0, false
		}

		return parsed, true
	}

	value, err := node.Float64()
	if err != nil {
		return 0, false
	}

	return value, true
}

func ttsArrayLen(node *ast.Node) int {
	count := 0
	_ = node.ForEach(func(_ ast.Sequence, _ *ast.Node) bool {
		count++
		return true
	})

	return count
}

type TTSExtraInfo struct {
	AudioFormat     string `json:"audio_format"`
	UsageCharacters int64  `json:"usage_characters"`
}

type TTSData struct {
	Audio  string `json:"audio"`
	Status int    `json:"status"`
}

type TTSResponse struct {
	ExtraInfo TTSExtraInfo `json:"extra_info"`
	Data      TTSData      `json:"data"`
}

func TTSHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if utils.IsStreamResponse(resp) {
		return ttsStreamHandler(meta, c, resp)
	}

	if err := TryErrorHanlder(resp); err != nil {
		return adaptor.DoResponseResult{}, err
	}

	defer resp.Body.Close()

	audioFormat := meta.GetString("audio_format")

	log := common.GetLogger(c)

	var result TTSResponse
	if err := common.UnmarshalResponse(resp, &result); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"TTS_ERROR",
			http.StatusInternalServerError,
		)
	}

	usageCharacters := meta.RequestUsage.InputTokens
	if result.ExtraInfo.UsageCharacters > 0 {
		usageCharacters = model.ZeroNullInt64(result.ExtraInfo.UsageCharacters)
	}

	usage := model.Usage{
		InputTokens: usageCharacters,
		TotalTokens: usageCharacters,
	}

	audioBytes, err := hex.DecodeString(result.Data.Audio)
	if err != nil {
		return adaptor.DoResponseResult{Usage: usage}, relaymodel.WrapperOpenAIError(
			err,
			"TTS_ERROR",
			http.StatusInternalServerError,
		)
	}

	if result.ExtraInfo.AudioFormat != "" {
		audioFormat = result.ExtraInfo.AudioFormat
	}

	if audioFormat == "" {
		c.Writer.Header().Set("Content-Type", http.DetectContentType(audioBytes))
	} else {
		c.Writer.Header().Set("Content-Type", "audio/"+audioFormat)
	}

	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(audioBytes)))

	_, err = c.Writer.Write(audioBytes)
	if err != nil {
		log.Warnf("write response body failed: %v", err)
	}

	return adaptor.DoResponseResult{Usage: usage}, nil
}

func ttsStreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	sseFormat := meta.GetString("stream_format") == "sse"
	audioFormat := meta.GetString("audio_format")

	defer resp.Body.Close()

	contextTypeWritten := false

	if !sseFormat && audioFormat != "" {
		c.Writer.Header().Set("Content-Type", "audio/"+audioFormat)

		contextTypeWritten = true
	}

	log := common.GetLogger(c)

	scanner, cleanup := utils.NewScanner(resp.Body)
	defer cleanup()

	usageCharacters := meta.RequestUsage.InputTokens

	for scanner.Scan() {
		data := scanner.Bytes()
		if !render.IsValidSSEData(data) {
			continue
		}

		data = render.ExtractSSEData(data)
		if render.IsSSEDone(data) {
			break
		}

		var result TTSResponse
		if err := sonic.Unmarshal(data, &result); err != nil {
			log.Error("unmarshal tts response failed: " + err.Error())
			continue
		}

		if result.ExtraInfo.UsageCharacters > 0 {
			usageCharacters = model.ZeroNullInt64(result.ExtraInfo.UsageCharacters)
		}

		if result.Data.Audio == "" {
			continue
		}

		audioBytes, err := hex.DecodeString(result.Data.Audio)
		if err != nil {
			log.Error("decode audio failed: " + err.Error())
			continue
		}

		if sseFormat {
			render.OpenaiAudioData(c, base64.StdEncoding.EncodeToString(audioBytes))
			continue
		}

		// do not write content type for sse format
		if !contextTypeWritten {
			c.Writer.Header().Set("Content-Type", http.DetectContentType(audioBytes))

			contextTypeWritten = true
		}

		_, err = c.Writer.Write(audioBytes)
		if err != nil {
			log.Warnf("write response body failed: %v", err)
		}

		c.Writer.Flush()
	}

	usage := relaymodel.TextToSpeechUsage{
		InputTokens: int64(usageCharacters),
		TotalTokens: int64(usageCharacters),
	}

	if sseFormat {
		render.OpenaiAudioDone(c, usage)
	}

	return adaptor.DoResponseResult{Usage: usage.ToModelUsage()}, nil
}
