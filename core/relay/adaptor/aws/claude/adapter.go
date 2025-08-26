package aws

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
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
)

const (
	anthropicVersion = "bedrock-2023-05-31"
	ConvertedRequest = "convertedRequest"
	ResponseOutput   = "responseOutput"
)

type Adaptor struct{}

type Request struct {
	AnthropicVersion string `json:"anthropic_version"`
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

func handleChatCompletionsRequest(meta *meta.Meta, request *http.Request) ([]byte, error) {
	claudeReq, err := anthropic.OpenAIConvertRequest(meta, request)
	if err != nil {
		return nil, err
	}

	meta.Set("stream", claudeReq.Stream)

	req := Request{
		AnthropicVersion: anthropicVersion,
		ClaudeRequest:    claudeReq,
	}

	return sonic.Marshal(req)
}

func handleAnthropicRequest(meta *meta.Meta, request *http.Request) ([]byte, error) {
	reqBody, err := common.GetRequestBodyReusable(request)
	if err != nil {
		return nil, err
	}

	node, err := sonic.Get(reqBody)
	if err != nil {
		return nil, err
	}

	if err = anthropic.ConvertImage2Base64(context.Background(), &node); err != nil {
		return nil, err
	}

	stream, _ := node.Get("stream").Bool()
	meta.Set("stream", stream)

	if _, err = node.Set("anthropic_version", ast.NewString(anthropicVersion)); err != nil {
		return nil, err
	}

	return node.MarshalJSON()
}

func (a *Adaptor) DoRequest(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	req *http.Request,
) (*http.Response, error) {
	convReq, ok := meta.Get(ConvertedRequest)
	if !ok {
		return nil, relaymodel.WrapperOpenAIErrorWithMessage(
			"request not found",
			nil,
			http.StatusInternalServerError,
		)
	}

	body, ok := convReq.([]byte)
	if !ok {
		return nil, relaymodel.WrapperOpenAIErrorWithMessage(
			fmt.Sprintf("claude request type error: %T", convReq),
			nil,
			http.StatusInternalServerError,
		)
	}

	awsModelID, err := awsModelID(meta.ActualModel)
	if err != nil {
		return nil, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			nil,
			http.StatusInternalServerError,
		)
	}

	awsClient, err := utils.AwsClientFromMeta(meta)
	if err != nil {
		return nil, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			nil,
			http.StatusInternalServerError,
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
			return nil, relaymodel.WrapperOpenAIErrorWithMessage(
				err.Error(),
				nil,
				http.StatusInternalServerError,
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
			return nil, relaymodel.WrapperOpenAIErrorWithMessage(
				err.Error(),
				nil,
				http.StatusInternalServerError,
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

	return
}
