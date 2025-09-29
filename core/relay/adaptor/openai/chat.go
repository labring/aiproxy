package openai

import (
	"bufio"
	"bytes"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

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

func ConvertCompletionsRequest(
	meta *meta.Meta,
	req *http.Request,
	callback ...func(node *ast.Node) error,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2NodeReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	for _, callback := range callback {
		if callback == nil {
			continue
		}

		if err := callback(&node); err != nil {
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

func ConvertChatCompletionsRequest(
	meta *meta.Meta,
	req *http.Request,
	doNotPatchStreamOptionsIncludeUsage bool,
	callback ...func(node *ast.Node) error,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2NodeReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	for _, callback := range callback {
		if callback == nil {
			continue
		}

		if err := callback(&node); err != nil {
			return adaptor.ConvertResult{}, err
		}
	}

	if !doNotPatchStreamOptionsIncludeUsage {
		if err := patchStreamOptions(&node); err != nil {
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

func patchStreamOptions(node *ast.Node) error {
	streamNode := node.Get("stream")
	if !streamNode.Exists() {
		return nil
	}

	streamBool, err := streamNode.Bool()
	if err != nil {
		return errors.New("stream is not a boolean")
	}

	if !streamBool {
		return nil
	}

	streamOptionsNode := node.Get("stream_options")
	if !streamOptionsNode.Exists() {
		_, err = node.SetAny("stream_options", map[string]any{
			"include_usage": true,
		})
		return err
	}

	if streamOptionsNode.TypeSafe() != ast.V_OBJECT {
		return errors.New("stream_options is not an object")
	}

	_, err = streamOptionsNode.Set("include_usage", ast.NewBool(true))

	return err
}

func GetUsageOrChatChoicesResponseFromNode(
	node *ast.Node,
) (*relaymodel.ChatUsage, []*relaymodel.ChatCompletionsStreamResponseChoice, error) {
	usageNode := node.Get("usage")
	if usageNode != nil && usageNode.TypeSafe() != ast.V_NULL {
		usageRaw, err := usageNode.Raw()
		if err != nil {
			if !errors.Is(err, ast.ErrNotExist) {
				return nil, nil, err
			}
		} else {
			var usage relaymodel.ChatUsage

			err = sonic.UnmarshalString(usageRaw, &usage)
			if err != nil {
				return nil, nil, err
			}

			return &usage, nil, nil
		}
	}

	var choices []*relaymodel.ChatCompletionsStreamResponseChoice

	choicesNode := node.Get("choices")
	if choicesNode != nil && choicesNode.TypeSafe() != ast.V_NULL {
		choicesRaw, err := choicesNode.Raw()
		if err != nil {
			if !errors.Is(err, ast.ErrNotExist) {
				return nil, nil, err
			}
		} else {
			err = sonic.UnmarshalString(choicesRaw, &choices)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	return nil, choices, nil
}

type PreHandler func(meta *meta.Meta, node *ast.Node) error

func StreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
	preHandler PreHandler,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	responseText := strings.Builder{}

	scanner := bufio.NewScanner(resp.Body)

	buf := utils.GetScannerBuffer()
	defer utils.PutScannerBuffer(buf)

	scanner.Buffer(*buf, cap(*buf))

	var usage relaymodel.ChatUsage

	for scanner.Scan() {
		data := scanner.Bytes()
		if !render.IsValidSSEData(data) {
			continue
		}

		data = render.ExtractSSEData(data)
		if render.IsSSEDone(data) {
			break
		}

		node, err := sonic.Get(data)
		if err != nil {
			log.Error("error unmarshalling stream response: " + err.Error())
			continue
		}

		if preHandler != nil {
			err := preHandler(meta, &node)
			if err != nil {
				log.Error("error pre handler: " + err.Error())
				continue
			}
		}

		u, ch, err := GetUsageOrChatChoicesResponseFromNode(&node)
		if err != nil {
			log.Error("error unmarshalling stream response: " + err.Error())
			continue
		}

		if u != nil {
			usage = *u

			responseText.Reset()
		}

		for _, choice := range ch {
			if usage.TotalTokens == 0 {
				if choice.Text != "" {
					responseText.WriteString(choice.Text)
				} else {
					responseText.WriteString(choice.Delta.StringContent())
				}
			}
		}

		_, err = node.Set("model", ast.NewString(meta.OriginModel))
		if err != nil {
			log.Error("error set model: " + err.Error())
		}

		_ = render.OpenaiObjectData(c, &node)
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	if usage.TotalTokens == 0 && responseText.Len() > 0 {
		usage = ResponseText2Usage(
			responseText.String(),
			meta.ActualModel,
			int64(meta.RequestUsage.InputTokens),
		)
		_ = render.OpenaiObjectData(c, &relaymodel.ChatCompletionsStreamResponse{
			ID:      ChatCompletionID(),
			Model:   meta.OriginModel,
			Object:  relaymodel.ChatCompletionChunkObject,
			Created: time.Now().Unix(),
			Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{},
			Usage:   &usage,
		})
	} else if usage.TotalTokens != 0 && usage.PromptTokens == 0 { // some channels don't return prompt tokens & completion tokens
		usage.PromptTokens = int64(meta.RequestUsage.InputTokens)
		usage.CompletionTokens = usage.TotalTokens - int64(meta.RequestUsage.InputTokens)
	}

	render.OpenaiDone(c)

	return usage.ToModelUsage(), nil
}

func GetUsageOrChoicesResponseFromNode(
	node *ast.Node,
) (*relaymodel.ChatUsage, []*relaymodel.TextResponseChoice, error) {
	usageNode := node.Get("usage")
	if usageNode != nil && usageNode.TypeSafe() != ast.V_NULL {
		usageRaw, err := usageNode.Raw()
		if err != nil {
			if !errors.Is(err, ast.ErrNotExist) {
				return nil, nil, err
			}
		} else {
			var usage relaymodel.ChatUsage

			err = sonic.UnmarshalString(usageRaw, &usage)
			if err != nil {
				return nil, nil, err
			}

			return &usage, nil, nil
		}
	}

	var choices []*relaymodel.TextResponseChoice

	choicesNode := node.Get("choices")
	if choicesNode != nil && choicesNode.TypeSafe() != ast.V_NULL {
		choicesRaw, err := choicesNode.Raw()
		if err != nil {
			if !errors.Is(err, ast.ErrNotExist) {
				return nil, nil, err
			}
		} else {
			err = sonic.UnmarshalString(choicesRaw, &choices)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	return nil, choices, nil
}

func Handler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
	preHandler PreHandler,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	node, err := common.UnmarshalResponse2Node(resp)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	if preHandler != nil {
		err := preHandler(meta, &node)
		if err != nil {
			return model.Usage{}, relaymodel.WrapperOpenAIError(
				err,
				"pre_handler_failed",
				http.StatusInternalServerError,
			)
		}
	}

	usage, choices, err := GetUsageOrChoicesResponseFromNode(&node)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	if usage == nil ||
		usage.TotalTokens == 0 ||
		(usage.PromptTokens == 0 && usage.CompletionTokens == 0) {
		var completionTokens int64
		for _, choice := range choices {
			if choice.Text != "" {
				completionTokens += CountTokenText(choice.Text, meta.ActualModel)
				continue
			}

			completionTokens += CountTokenText(choice.Message.StringContent(), meta.ActualModel)
		}

		usage = &relaymodel.ChatUsage{
			PromptTokens:     int64(meta.RequestUsage.InputTokens),
			CompletionTokens: completionTokens,
			TotalTokens:      int64(meta.RequestUsage.InputTokens) + completionTokens,
		}

		_, err = node.Set("usage", ast.NewAny(usage))
		if err != nil {
			return usage.ToModelUsage(), relaymodel.WrapperOpenAIError(
				err,
				"set_usage_failed",
				http.StatusInternalServerError,
			)
		}
	} else if usage.TotalTokens != 0 && usage.PromptTokens == 0 { // some channels don't return prompt tokens & completion tokens
		usage.PromptTokens = int64(meta.RequestUsage.InputTokens)
		usage.CompletionTokens = usage.TotalTokens - int64(meta.RequestUsage.InputTokens)

		_, err = node.Set("usage", ast.NewAny(usage))
		if err != nil {
			return usage.ToModelUsage(), relaymodel.WrapperOpenAIError(err, "set_usage_failed", http.StatusInternalServerError)
		}
	}

	_, err = node.Set("model", ast.NewString(meta.OriginModel))
	if err != nil {
		return usage.ToModelUsage(), relaymodel.WrapperOpenAIError(
			err,
			"set_model_failed",
			http.StatusInternalServerError,
		)
	}

	newData, err := sonic.Marshal(&node)
	if err != nil {
		return usage.ToModelUsage(), relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(newData)))

	_, err = c.Writer.Write(newData)
	if err != nil {
		log.Warnf("write response body failed: %v", err)
	}

	return usage.ToModelUsage(), nil
}
