// Package aws provides the AWS adaptor for the relay service.
package aws

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
)

func OpenaiHandler(meta *meta.Meta, c *gin.Context) (model.Usage, adaptor.Error) {
	resp, ok := meta.Get(ResponseOutput)
	if !ok {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"missing response",
			nil,
			http.StatusInternalServerError,
		)
	}

	awsResp, ok := resp.(*bedrockruntime.InvokeModelOutput)
	if !ok {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"unknow response type",
			nil,
			http.StatusInternalServerError,
		)
	}

	openaiResp, adaptorErr := anthropic.Response2OpenAI(meta, awsResp.Body)
	if adaptorErr != nil {
		return model.Usage{}, adaptorErr
	}

	jsonBody, err := sonic.Marshal(openaiResp)
	if err != nil {
		return openaiResp.Usage.ToModelUsage(), relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			nil,
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonBody)))
	_, _ = c.Writer.Write(jsonBody)

	return openaiResp.Usage.ToModelUsage(), nil
}

func OpenaiStreamHandler(meta *meta.Meta, c *gin.Context) (model.Usage, adaptor.Error) {
	resp, ok := meta.Get(ResponseOutput)
	if !ok {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"missing response",
			nil,
			http.StatusInternalServerError,
		)
	}

	awsResp, ok := resp.(*bedrockruntime.InvokeModelWithResponseStreamOutput)
	if !ok {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"unknow response type",
			nil,
			http.StatusInternalServerError,
		)
	}

	stream := awsResp.GetStream()
	defer stream.Close()

	responseText := strings.Builder{}

	var (
		usage  *relaymodel.ChatUsage
		writed bool
	)

	streamState := anthropic.NewStreamState()

	log := common.GetLogger(c)

	for event := range stream.Events() {
		switch v := event.(type) {
		case *types.ResponseStreamMemberChunk:
			response, err := streamState.StreamResponse2OpenAI(meta, v.Value.Bytes)
			if err != nil {
				if writed {
					log.Errorf("response error: %+v", err)
					continue
				}

				return usage.ToModelUsage(), err
			}

			if response == nil {
				continue
			}

			switch {
			case response.Usage != nil:
				if usage == nil {
					usage = &relaymodel.ChatUsage{}
				}

				usage.Add(response.Usage)

				if usage.PromptTokens == 0 {
					usage.PromptTokens = int64(meta.RequestUsage.InputTokens)
					usage.TotalTokens += int64(meta.RequestUsage.InputTokens)
				}

				response.Usage = usage

				responseText.Reset()
			case usage == nil:
				for _, choice := range response.Choices {
					responseText.WriteString(choice.Delta.StringContent())
				}
			default:
				response.Usage = usage
			}

			_ = render.OpenaiObjectData(c, response)
			writed = true
		case *types.UnknownUnionMember:
			log.Error("unknown tag: " + v.Tag)
			continue
		default:
			log.Errorf("union is nil or unknown type: %v", v)
			continue
		}
	}

	if usage == nil {
		usage = &relaymodel.ChatUsage{
			PromptTokens:     int64(meta.RequestUsage.InputTokens),
			CompletionTokens: openai.CountTokenText(responseText.String(), meta.OriginModel),
			TotalTokens: int64(
				meta.RequestUsage.InputTokens,
			) + openai.CountTokenText(
				responseText.String(),
				meta.OriginModel,
			),
		}
		_ = render.OpenaiObjectData(c, &relaymodel.ChatCompletionsStreamResponse{
			ID:      openai.ChatCompletionID(),
			Model:   meta.OriginModel,
			Object:  relaymodel.ChatCompletionChunkObject,
			Created: time.Now().Unix(),
			Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{},
			Usage:   usage,
		})
	}

	render.OpenaiDone(c)

	return usage.ToModelUsage(), nil
}
