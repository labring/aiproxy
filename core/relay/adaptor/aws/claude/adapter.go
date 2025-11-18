package aws

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/adaptor/aws/utils"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
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
	default:
		if meta.GetBool("stream") {
			usage, err = OpenaiStreamHandler(meta, c)
		} else {
			usage, err = OpenaiHandler(meta, c)
		}
	}

	return usage, err
}
