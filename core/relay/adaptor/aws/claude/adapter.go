package aws

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/adaptor/aws/utils"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
)

const (
	anthropicVersion = "bedrock-2023-05-31"
	ConvertedRequest = "convertedRequest"
	ResponseOutput   = "responseOutput"
)

type Adaptor struct{}

type Request struct {
	AnthropicBeta    []string `json:"anthropic_beta,omitempty"`
	AnthropicVersion string   `json:"anthropic_version"`
	*relaymodel.ClaudeRequest
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	request *http.Request,
) (adaptor.ConvertResult, error) {
	var (
		data []byte
		err  error
	)

	switch meta.Mode {
	case mode.ChatCompletions:
		data, err = handleChatCompletionsRequest(meta, request)
	case mode.Anthropic:
		data, err = handleAnthropicRequest(meta, request)
	case mode.Gemini:
		data, err = handleGeminiRequest(meta, request)
	default:
		return adaptor.ConvertResult{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}

	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	meta.Set(ConvertedRequest, data)

	return adaptor.ConvertResult{
		Header: nil,
		Body:   nil,
	}, nil
}

var unsupportedBetas = map[string]struct{}{
	"tool-examples-2025-10-29": {},
}

func fixBetas(model string, betas []string) []string {
	return anthropic.FixBetasWithModel(model, betas, func(e string) bool {
		_, ok := unsupportedBetas[e]
		return ok
	})
}

var supportedContextManagementEditsType = map[string]struct{}{
	"clear_tool_uses_20250919": {},
}

func handleChatCompletionsRequest(meta *meta.Meta, request *http.Request) ([]byte, error) {
	claudeReq, err := anthropic.OpenAIConvertRequest(meta, request)
	if err != nil {
		return nil, err
	}

	claudeReq.Model = ""

	meta.Set("stream", claudeReq.Stream)
	claudeReq.Stream = false

	req := Request{
		AnthropicVersion: anthropicVersion,
		ClaudeRequest:    claudeReq,
	}

	if betas := request.Header.Get(anthropic.AnthropicBeta); betas != "" {
		req.AnthropicBeta = fixBetas(meta.ActualModel, strings.Split(betas, ","))
	}

	return sonic.Marshal(req)
}

func handleAnthropicRequest(meta *meta.Meta, request *http.Request) ([]byte, error) {
	return anthropic.ConvertRequestToBytes(meta, request, func(node *ast.Node) error {
		if _, err := node.Unset("model"); err != nil {
			return err
		}

		if betas := request.Header.Get(anthropic.AnthropicBeta); betas != "" {
			_, _ = node.SetAny(
				"anthropic_beta",
				fixBetas(meta.ActualModel, strings.Split(betas, ",")),
			)
		}

		anthropic.RemoveContextManagenetEdits(node, func(t string) bool {
			_, ok := supportedContextManagementEditsType[t]
			return ok
		})
		anthropic.RemoveToolsExamples(node)

		stream, _ := node.Get("stream").Bool()
		meta.Set("stream", stream)

		_, _ = node.Unset("stream")

		if _, err := node.Set("anthropic_version", ast.NewString(anthropicVersion)); err != nil {
			return err
		}

		return nil
	})
}

func (a *Adaptor) DoRequest(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	req *http.Request,
) (*http.Response, error) {
	convReq, ok := meta.Get(ConvertedRequest)
	if !ok {
		return nil, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			"request not found",
		)
	}

	body, ok := convReq.([]byte)
	if !ok {
		return nil, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			fmt.Sprintf("claude request type error: %T", convReq),
		)
	}

	region, err := utils.AwsRegionFromMeta(meta)
	if err != nil {
		return nil, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			err.Error(),
		)
	}

	awsModelID, err := awsModelID(meta.ActualModel, region)
	if err != nil {
		return nil, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			err.Error(),
		)
	}

	awsClient, err := utils.AwsClientFromMeta(meta)
	if err != nil {
		return nil, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			err.Error(),
		)
	}

	if meta.GetBool("stream") {
		awsReq := &bedrockruntime.InvokeModelWithResponseStreamInput{
			ModelId:     aws.String(awsModelID),
			ContentType: aws.String("application/json"),
			Body:        body,
		}

		awsResp, err := awsClient.InvokeModelWithResponseStream(c.Request.Context(), awsReq)
		if err != nil {
			code, errmessage := UnwrapInvokeError(err)

			return nil, relaymodel.WrapperErrorWithMessage(
				meta.Mode,
				code,
				errmessage,
			)
		}

		meta.Set(ResponseOutput, awsResp)
	} else {
		awsReq := &bedrockruntime.InvokeModelInput{
			ModelId:     aws.String(awsModelID),
			ContentType: aws.String("application/json"),
			Body:        body,
		}

		awsResp, err := awsClient.InvokeModel(c.Request.Context(), awsReq)
		if err != nil {
			code, errmessage := UnwrapInvokeError(err)

			return nil, relaymodel.WrapperErrorWithMessage(
				meta.Mode,
				code,
				errmessage,
			)
		}

		meta.Set(ResponseOutput, awsResp)
	}

	return &http.Response{
		StatusCode: http.StatusOK,
	}, nil
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	_ adaptor.Store,
	c *gin.Context,
) (usage model.Usage, err adaptor.Error) {
	switch meta.Mode {
	case mode.Anthropic:
		if meta.GetBool("stream") {
			usage, err = StreamHandler(meta, c)
		} else {
			usage, err = Handler(meta, c)
		}
	case mode.Gemini:
		if meta.GetBool("stream") {
			usage, err = GeminiStreamHandler(meta, c)
		} else {
			usage, err = GeminiHandler(meta, c)
		}
	default:
		if meta.GetBool("stream") {
			usage, err = OpenaiStreamHandler(meta, c)
		} else {
			usage, err = OpenaiHandler(meta, c)
		}
	}

	return usage, err
}

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

func GeminiHandler(meta *meta.Meta, c *gin.Context) (model.Usage, adaptor.Error) {
	// Get the response output from meta
	resp, ok := meta.Get(ResponseOutput)
	if !ok {
		return model.Usage{}, relaymodel.WrapperAnthropicErrorWithMessage(
			"response output not found in meta",
			"response_output_not_found",
			http.StatusInternalServerError,
		)
	}

	// Cast to AWS response type
	awsResp, ok := resp.(*bedrockruntime.InvokeModelOutput)
	if !ok {
		return model.Usage{}, relaymodel.WrapperAnthropicErrorWithMessage(
			"unknown response type",
			"unknown_response_type",
			http.StatusInternalServerError,
		)
	}

	// Parse Claude response
	var claudeResp relaymodel.ClaudeResponse
	if err := sonic.Unmarshal(awsResp.Body, &claudeResp); err != nil {
		return model.Usage{}, relaymodel.WrapperAnthropicError(
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
		return claudeResp.Usage.ToOpenAIUsage().ToModelUsage(), relaymodel.WrapperAnthropicError(
			err,
			"marshal_response_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	_, _ = c.Writer.Write(data)

	return claudeResp.Usage.ToOpenAIUsage().ToModelUsage(), nil
}

func GeminiStreamHandler(meta *meta.Meta, c *gin.Context) (model.Usage, adaptor.Error) {
	// Get the response output from meta
	resp, ok := meta.Get(ResponseOutput)
	if !ok {
		return model.Usage{}, relaymodel.WrapperAnthropicErrorWithMessage(
			"response output not found in meta",
			"response_output_not_found",
			http.StatusInternalServerError,
		)
	}

	// Cast to AWS streaming response type
	awsResp, ok := resp.(*bedrockruntime.InvokeModelWithResponseStreamOutput)
	if !ok {
		return model.Usage{}, relaymodel.WrapperAnthropicErrorWithMessage(
			"unknown response type",
			"unknown_response_type",
			http.StatusInternalServerError,
		)
	}

	stream := awsResp.GetStream()
	defer stream.Close()

	log := common.GetLogger(c)
	usage := model.Usage{}

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

			// Convert to Gemini format
			geminiResp := streamState.ConvertClaudeStreamToGemini(
				meta,
				&claudeResp,
			)
			if geminiResp != nil {
				_ = render.GeminiObjectData(c, geminiResp)

				// Update usage metadata
				if geminiResp.UsageMetadata != nil {
					usage = geminiResp.UsageMetadata.ToUsage().ToModelUsage()
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

	return usage, nil
}
