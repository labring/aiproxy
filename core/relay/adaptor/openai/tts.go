package openai

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
	"github.com/labring/aiproxy/core/relay/utils"
)

func ConvertTTSRequest(
	meta *meta.Meta,
	req *http.Request,
	defaultVoice string,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2NodeReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	streamFormat, _ := node.Get("stream_format").String()
	meta.Set("stream_format", streamFormat)

	voice, err := node.Get("voice").String()
	if err != nil && !errors.Is(err, ast.ErrNotExist) {
		return adaptor.ConvertResult{}, err
	}

	if voice == "" && defaultVoice != "" {
		_, err = node.Set("voice", ast.NewString(defaultVoice))
		if err != nil {
			return adaptor.ConvertResult{}, err
		}
	}

	_, err = node.Set("model", ast.NewString(meta.ActualModel))
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	jsonData, err := node.MarshalJSON()
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(jsonData))},
		},
		Body: bytes.NewReader(jsonData),
	}, nil
}

func TTSHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	if utils.IsStreamResponse(resp) {
		return ttsStreamHandler(meta, c, resp)
	}

	defer resp.Body.Close()

	sseFormat := meta.GetString("stream_format") == "sse"

	log := common.GetLogger(c)

	usage := relaymodel.TextToSpeechUsage{
		InputTokens: int64(meta.RequestUsage.InputTokens),
		TotalTokens: int64(meta.RequestUsage.InputTokens),
	}

	if sseFormat {
		_, err := io.Copy(render.NewOpenaiAudioDataWriter(c), resp.Body)
		if err != nil {
			log.Warnf("write response body failed: %v", err)
		}

		render.OpenaiAudioDone(c, usage)

		return adaptor.DoResponseResult{Usage: usage.ToModelUsage()}, nil
	}

	c.Writer.Header().Set("Content-Type", resp.Header.Get("Content-Type"))

	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		c.Writer.Header().Set("Content-Length", contentLength)
	}

	_, err := io.Copy(c.Writer, resp.Body)
	if err != nil {
		log.Warnf("write response body failed: %v", err)
	}

	return adaptor.DoResponseResult{Usage: usage.ToModelUsage()}, nil
}

func ttsStreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	defer resp.Body.Close()

	log := common.GetLogger(c)

	scanner, cleanup := utils.NewScanner(resp.Body)
	defer cleanup()

	var totalUsage *relaymodel.TextToSpeechUsage

	for scanner.Scan() {
		data := scanner.Bytes()
		if !render.IsValidSSEData(data) {
			continue
		}

		data = render.ExtractSSEData(data)
		if render.IsSSEDone(data) {
			break
		}

		var sseResponse relaymodel.TextToSpeechSSEResponse

		err := sonic.Unmarshal(data, &sseResponse)
		if err != nil {
			log.Error("error unmarshalling TTS stream response: " + err.Error())
			continue
		}

		switch sseResponse.Type {
		case relaymodel.TextToSpeechSSEResponseTypeDelta:
			// Stream audio data
			if sseResponse.Audio != "" {
				render.OpenaiAudioData(c, sseResponse.Audio)
			}
		case relaymodel.TextToSpeechSSEResponseTypeDone:
			// Final response with usage
			if sseResponse.Usage != nil {
				totalUsage = sseResponse.Usage
				render.OpenaiAudioDone(c, *totalUsage)
			}
		}
	}

	render.OpenaiDone(c)

	if err := scanner.Err(); err != nil {
		log.Error("error reading TTS stream: " + err.Error())
	}

	// If no usage was provided, use the request usage
	if totalUsage == nil {
		totalUsage = &relaymodel.TextToSpeechUsage{
			InputTokens:  int64(meta.RequestUsage.InputTokens),
			OutputTokens: 0, // TTS doesn't have output tokens in the traditional sense
			TotalTokens:  int64(meta.RequestUsage.InputTokens),
		}
		render.OpenaiAudioDone(c, *totalUsage)
	}

	return adaptor.DoResponseResult{Usage: totalUsage.ToModelUsage()}, nil
}
