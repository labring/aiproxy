package minimax

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

func ConvertTTSRequest(meta *meta.Meta, req *http.Request) (*adaptor.ConvertRequestResult, error) {
	reqMap, err := utils.UnmarshalMap(req)
	if err != nil {
		return nil, err
	}

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

	responseFormat, ok := reqMap["response_format"].(string)
	if ok && responseFormat != "" {
		audioSetting["format"] = responseFormat
	}
	delete(reqMap, "response_format")

	sampleRate, ok := reqMap["sample_rate"].(float64)
	if ok {
		audioSetting["sample_rate"] = int(sampleRate)
	}
	delete(reqMap, "sample_rate")

	if responseFormat == "wav" {
		reqMap["stream"] = false
		meta.Set("stream", false)
	} else {
		stream, _ := reqMap["stream"].(bool)
		meta.Set("stream", stream)
	}

	body, err := sonic.Marshal(reqMap)
	if err != nil {
		return nil, err
	}

	return &adaptor.ConvertRequestResult{
		Method: http.MethodPost,
		Header: nil,
		Body:   bytes.NewReader(body),
	}, nil
}

type TTSExtraInfo struct {
	AudioFormat     string `json:"audio_format"`
	UsageCharacters int64  `json:"usage_characters"`
}

type TTSBaseResp struct {
	StatusMsg  string `json:"status_msg"`
	StatusCode int    `json:"status_code"`
}

type TTSData struct {
	Audio  string `json:"audio"`
	Status int    `json:"status"`
}

type TTSResponse struct {
	BaseResp  *TTSBaseResp `json:"base_resp"`
	ExtraInfo TTSExtraInfo `json:"extra_info"`
	Data      TTSData      `json:"data"`
}

func TTSHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (*model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return nil, openai.ErrorHanlder(resp)
	}

	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") &&
		meta.GetBool("stream") {
		return ttsStreamHandler(meta, c, resp)
	}

	defer resp.Body.Close()

	log := middleware.GetLogger(c)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, relaymodel.WrapperOpenAIError(err, "TTS_ERROR", http.StatusInternalServerError)
	}

	var result TTSResponse
	if err := sonic.Unmarshal(body, &result); err != nil {
		return nil, relaymodel.WrapperOpenAIError(err, "TTS_ERROR", http.StatusInternalServerError)
	}
	if result.BaseResp != nil && result.BaseResp.StatusCode != 0 {
		return nil, relaymodel.WrapperOpenAIErrorWithMessage(
			result.BaseResp.StatusMsg,
			"TTS_ERROR_"+strconv.Itoa(result.BaseResp.StatusCode),
			http.StatusInternalServerError,
		)
	}

	resp.Header.Set("Content-Type", "audio/"+result.ExtraInfo.AudioFormat)

	audioBytes, err := hex.DecodeString(result.Data.Audio)
	if err != nil {
		return nil, relaymodel.WrapperOpenAIError(err, "TTS_ERROR", http.StatusInternalServerError)
	}

	_, err = c.Writer.Write(audioBytes)
	if err != nil {
		log.Warnf("write response body failed: %v", err)
	}

	usageCharacters := meta.RequestUsage.InputTokens
	if result.ExtraInfo.UsageCharacters > 0 {
		usageCharacters = model.ZeroNullInt64(result.ExtraInfo.UsageCharacters)
	}

	return &model.Usage{
		InputTokens: usageCharacters,
		TotalTokens: usageCharacters,
	}, nil
}

func ttsStreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (*model.Usage, adaptor.Error) {
	defer resp.Body.Close()

	resp.Header.Set("Content-Type", "application/octet-stream")

	log := middleware.GetLogger(c)

	scanner := bufio.NewScanner(resp.Body)
	buf := openai.GetScannerBuffer()
	defer openai.PutScannerBuffer(buf)
	scanner.Buffer(*buf, cap(*buf))

	usageCharacters := meta.RequestUsage.InputTokens

	for scanner.Scan() {
		data := scanner.Text()
		if len(data) < openai.DataPrefixLength { // ignore blank line or wrong format
			continue
		}
		if data[:openai.DataPrefixLength] != openai.DataPrefix {
			continue
		}
		data = data[openai.DataPrefixLength:]

		var result TTSResponse
		if err := sonic.UnmarshalString(data, &result); err != nil {
			log.Error("unmarshal tts response failed: " + err.Error())
			continue
		}
		if result.ExtraInfo.UsageCharacters > 0 {
			usageCharacters = model.ZeroNullInt64(result.ExtraInfo.UsageCharacters)
		}

		audioBytes, err := hex.DecodeString(result.Data.Audio)
		if err != nil {
			log.Error("decode audio failed: " + err.Error())
			continue
		}

		_, err = c.Writer.Write(audioBytes)
		if err != nil {
			log.Warnf("write response body failed: %v", err)
		}
	}

	return &model.Usage{
		InputTokens: usageCharacters,
		TotalTokens: usageCharacters,
	}, nil
}
