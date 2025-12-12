package minimax

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strconv"

	"github.com/bytedance/sonic"
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
	reqMap, err := utils.UnmarshalMap(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	meta.Set("stream_format", reqMap["stream_format"])

	reqMap["model"] = meta.ActualModel

	reqMap["text"] = reqMap["input"]
	delete(reqMap, "input")

	voice, _ := reqMap["voice"].(string)
	delete(reqMap, "voice")

	if voice == "" {
		voice = "male-qn-qingse"
	}

	voiceSetting, ok := reqMap["voice_setting"].(map[string]any)
	if !ok {
		voiceSetting = map[string]any{}
		reqMap["voice_setting"] = voiceSetting
	}

	if timberWeights, ok := reqMap["timber_weights"].([]any); !ok || len(timberWeights) == 0 {
		voiceSetting["voice_id"] = voice
	}

	speed, ok := reqMap["speed"].(float64)
	if ok {
		voiceSetting["speed"] = int(speed)
	}

	delete(reqMap, "speed")

	audioSetting, ok := reqMap["audio_setting"].(map[string]any)
	if !ok {
		audioSetting = map[string]any{}
		reqMap["audio_setting"] = audioSetting
	}

	responseFormat, _ := reqMap["response_format"].(string)
	if responseFormat == "" {
		responseFormat, _ = reqMap["format"].(string)
	}

	if responseFormat == "" {
		responseFormat = "mp3"
	}

	audioSetting["format"] = responseFormat

	delete(reqMap, "response_format")
	meta.Set("audio_format", responseFormat)

	sampleRate, ok := reqMap["sample_rate"].(float64)
	if ok {
		audioSetting["sample_rate"] = int(sampleRate)
	}

	delete(reqMap, "sample_rate")

	if responseFormat == "wav" {
		reqMap["stream"] = false
	} else {
		reqMap["stream"] = true
		reqMap["stream_options"] = map[string]any{
			"exclude_aggregated_audio": true,
		}
	}

	reqMap["language_boost"] = "auto"

	body, err := sonic.Marshal(reqMap)
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
) (model.Usage, adaptor.Error) {
	if utils.IsStreamResponse(resp) {
		return ttsStreamHandler(meta, c, resp)
	}

	if err := TryErrorHanlder(resp); err != nil {
		return model.Usage{}, err
	}

	defer resp.Body.Close()

	audioFormat := meta.GetString("audio_format")

	log := common.GetLogger(c)

	var result TTSResponse
	if err := common.UnmarshalResponse(resp, &result); err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
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
		return usage, relaymodel.WrapperOpenAIError(
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

	return usage, nil
}

func ttsStreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
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

	return usage.ToModelUsage(), nil
}
