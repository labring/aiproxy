package aws

import (
	"net/http"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
)

func handleGeminiRequest(meta *meta.Meta, request *http.Request) ([]byte, error) {
	// Convert Gemini format to Claude format
	claudeReq, err := anthropic.ConvertGeminiRequestToStruct(meta, request)
	if err != nil {
		return nil, err
	}

	// AWS Bedrock doesn't use model field in request body
	claudeReq.Model = ""

	// Store stream flag and set it to false for AWS
	meta.Set("stream", claudeReq.Stream)
	claudeReq.Stream = false

	req := Request{
		AnthropicVersion: anthropicVersion,
		ClaudeRequest:    claudeReq,
	}

	return sonic.Marshal(req)
}

func GeminiHandler(meta *meta.Meta, c *gin.Context) (adaptor.DoResponseResult, adaptor.Error) {
	// Get the response output from meta
	resp, ok := meta.Get(ResponseOutput)
	if !ok {
		return adaptor.DoResponseResult{}, relaymodel.WrapperAnthropicErrorWithMessage(
			"response output not found in meta",
			"response_output_not_found",
			http.StatusInternalServerError,
		)
	}

	// Cast to AWS response type
	awsResp, ok := resp.(*bedrockruntime.InvokeModelOutput)
	if !ok {
		return adaptor.DoResponseResult{}, relaymodel.WrapperAnthropicErrorWithMessage(
			"unknown response type",
			"unknown_response_type",
			http.StatusInternalServerError,
		)
	}

	// Parse Claude response
	var claudeResp relaymodel.ClaudeResponse
	if err := sonic.Unmarshal(awsResp.Body, &claudeResp); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperAnthropicError(
			err,
			"unmarshal_response_failed",
			http.StatusInternalServerError,
		)
	}

	// Convert to Gemini format
	geminiResp := anthropic.ConvertClaudeToGeminiResponse(meta, &claudeResp)

	// Marshal and send
	data, err := sonic.Marshal(geminiResp)
	if err != nil {
		return adaptor.DoResponseResult{
				Usage: claudeResp.Usage.ToOpenAIUsage().ToModelUsage(),
			}, relaymodel.WrapperAnthropicError(
				err,
				"marshal_response_failed",
				http.StatusInternalServerError,
			)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	_, _ = c.Writer.Write(data)

	return adaptor.DoResponseResult{
		Usage:      claudeResp.Usage.ToOpenAIUsage().ToModelUsage(),
		UpstreamID: claudeResp.ID,
	}, nil
}

func GeminiStreamHandler(
	meta *meta.Meta,
	c *gin.Context,
) (adaptor.DoResponseResult, adaptor.Error) {
	// Get the response output from meta
	resp, ok := meta.Get(ResponseOutput)
	if !ok {
		return adaptor.DoResponseResult{}, relaymodel.WrapperAnthropicErrorWithMessage(
			"response output not found in meta",
			"response_output_not_found",
			http.StatusInternalServerError,
		)
	}

	// Cast to AWS streaming response type
	awsResp, ok := resp.(*bedrockruntime.InvokeModelWithResponseStreamOutput)
	if !ok {
		return adaptor.DoResponseResult{}, relaymodel.WrapperAnthropicErrorWithMessage(
			"unknown response type",
			"unknown_response_type",
			http.StatusInternalServerError,
		)
	}

	stream := awsResp.GetStream()
	defer stream.Close()

	log := common.GetLogger(c)

	usage := model.Usage{}
	upstreamID := ""

	streamState := anthropic.NewGeminiStreamState()

	for event := range stream.Events() {
		switch v := event.(type) {
		case *types.ResponseStreamMemberChunk:
			// Parse the chunk as Claude stream response
			var claudeResp relaymodel.ClaudeStreamResponse
			if err := sonic.Unmarshal(v.Value.Bytes, &claudeResp); err != nil {
				log.Errorf("error unmarshalling stream chunk: %+v", err)
				continue
			}

			if claudeResp.Message != nil && claudeResp.Message.ID != "" && upstreamID == "" {
				upstreamID = claudeResp.Message.ID
			}

			// Convert to Gemini format
			geminiResp := streamState.ConvertClaudeStreamToGemini(
				meta,
				&claudeResp,
			)
			if geminiResp != nil {
				_ = render.GeminiObjectData(c, geminiResp)

				// Update usage metadata
				if geminiResp.UsageMetadata != nil {
					usage = geminiResp.UsageMetadata.ToModelUsage()
				}
			}

		case *types.UnknownUnionMember:
			log.Error("unknown tag: " + v.Tag)
			continue
		default:
			log.Errorf("union is nil or unknown type: %v", v)
			continue
		}
	}

	return adaptor.DoResponseResult{
		Usage:      usage,
		UpstreamID: upstreamID,
	}, nil
}
