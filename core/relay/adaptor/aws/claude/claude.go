// Package aws provides the AWS adaptor for the relay service.
package aws

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
)

func Handler(meta *meta.Meta, c *gin.Context) (adaptor.DoResponseResult, adaptor.Error) {
	resp, ok := meta.Get(ResponseOutput)
	if !ok {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"missing response",
			nil,
			http.StatusInternalServerError,
		)
	}

	awsResp, ok := resp.(*bedrockruntime.InvokeModelOutput)
	if !ok {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"unknow response type",
			nil,
			http.StatusInternalServerError,
		)
	}

	openaiResp, adaptorErr := anthropic.Response2OpenAI(meta, awsResp.Body)
	if adaptorErr != nil {
		return adaptor.DoResponseResult{}, adaptorErr
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(awsResp.Body)))
	_, _ = c.Writer.Write(awsResp.Body)

	return adaptor.DoResponseResult{
		Usage:      openaiResp.Usage.ToModelUsage(),
		UpstreamID: openaiResp.ID,
	}, nil
}

func StreamHandler(meta *meta.Meta, c *gin.Context) (adaptor.DoResponseResult, adaptor.Error) {
	resp, ok := meta.Get(ResponseOutput)
	if !ok {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"missing response",
			nil,
			http.StatusInternalServerError,
		)
	}

	awsResp, ok := resp.(*bedrockruntime.InvokeModelWithResponseStreamOutput)
	if !ok {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"unknow response type",
			nil,
			http.StatusInternalServerError,
		)
	}

	stream := awsResp.GetStream()
	defer stream.Close()

	responseText := strings.Builder{}

	var (
		usage       *relaymodel.ChatUsage
		writed      bool
		upstreamID  string
	)

	streamState := anthropic.NewStreamState()

	log := common.GetLogger(c)

	for event := range stream.Events() {
		switch v := event.(type) {
		case *types.ResponseStreamMemberChunk:
			data := v.Value.Bytes

			response, err := streamState.StreamResponse2OpenAI(meta, v.Value.Bytes)
			if err != nil {
				if writed {
					log.Errorf("response error: %+v", err)
				} else {
					if usage == nil {
						usage = &relaymodel.ChatUsage{}
					}

					if response != nil && response.Usage != nil {
						usage.Add(response.Usage)
					}

					return adaptor.DoResponseResult{Usage: usage.ToModelUsage()}, err
				}
			}

			// Capture upstream ID from response ID
			if response != nil && response.ID != "" && upstreamID == "" {
				upstreamID = response.ID
			}

			if response != nil {
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
			}

			render.ClaudeData(c, data)

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
	}

	return adaptor.DoResponseResult{
		Usage:      usage.ToModelUsage(),
		UpstreamID: upstreamID,
	}, nil
}
