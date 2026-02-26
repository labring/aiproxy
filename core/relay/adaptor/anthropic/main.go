package anthropic

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/image"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
	"github.com/labring/aiproxy/core/relay/utils"
	"golang.org/x/sync/semaphore"
)

func ConvertRequest(
	meta *meta.Meta,
	req *http.Request,
	callbacks ...func(node *ast.Node) error,
) (adaptor.ConvertResult, error) {
	newBody, err := ConvertRequestToBytes(meta, req, callbacks...)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(newBody))},
		},
		Body: bytes.NewReader(newBody),
	}, nil
}

func RemoveToolsExamples(node *ast.Node) {
	toolsNode := node.Get("tools")
	if toolsNode != nil && toolsNode.Check() == nil {
		_ = toolsNode.ForEach(func(path ast.Sequence, toolNode *ast.Node) bool {
			_, _ = toolNode.Unset("input_examples")
			return true
		})
	}
}

func RemoveContextManagenetEdits(
	node *ast.Node,
	isSupportedEditsType ...func(t string) bool,
) {
	contextManagementNode := node.Get("context_management")
	if contextManagementNode.Check() != nil {
		return
	}

	editesNode := contextManagementNode.GetByPath("edits")
	if editesNode.Check() != nil {
		return
	}

	nodeLen, _ := editesNode.Len()
	newEdits := make([]ast.Node, 0, nodeLen)
	_ = editesNode.
		ForEach(func(path ast.Sequence, node *ast.Node) bool {
			t, err := node.Get("type").String()
			if err != nil {
				return true
			}

			for _, v := range isSupportedEditsType {
				if v != nil && !v(t) {
					return true
				}
			}

			newEdits = append(newEdits, *node)

			return true
		})

	if len(newEdits) == 0 {
		_, _ = contextManagementNode.Unset("edits")
		return
	}

	*editesNode = ast.NewArray(newEdits)
}

func ConvertRequestToBytes(
	meta *meta.Meta,
	req *http.Request,
	callbacks ...func(node *ast.Node) error,
) ([]byte, error) {
	// Parse request body into AST node
	node, err := common.UnmarshalRequest2NodeReusable(req)
	if err != nil {
		return nil, err
	}

	return ConvertRequestBodyToBytes(meta, req.Context(), &node, callbacks...)
}

func resetCacheTTLWithContentsNode(contents *ast.Node) error {
	if contents.Check() != nil {
		return nil
	}

	return contents.ForEach(func(_ ast.Sequence, content *ast.Node) bool {
		cacheControl := content.Get("cache_control")
		if cacheControl.Check() != nil {
			return true
		}

		_, _ = cacheControl.Unset("ttl")

		return true
	})
}

func ConvertRequestBodyToBytes(
	meta *meta.Meta,
	ctx context.Context,
	node *ast.Node,
	callbacks ...func(node *ast.Node) error,
) ([]byte, error) { // Process image content if present
	adaptorConfig := Config{}
	if err := meta.ChannelConfigs.LoadConfig(&adaptorConfig); err != nil {
		return nil, err
	}

	if adaptorConfig.DisableContextManagement {
		_, _ = node.Unset("context_management")
	} else if len(adaptorConfig.SupportedContextManagementEditsType) > 0 {
		supported := make(
			map[string]struct{},
			len(adaptorConfig.SupportedContextManagementEditsType),
		)
		for _, t := range adaptorConfig.SupportedContextManagementEditsType {
			supported[t] = struct{}{}
		}

		RemoveContextManagenetEdits(node, func(t string) bool {
			_, ok := supported[t]
			return ok
		})
	}

	if adaptorConfig.RemoveToolsExamples {
		RemoveToolsExamples(node)
	}

	err := ConvertImage2Base64(ctx, node)
	if err != nil {
		return nil, err
	}

	// Set the actual model in the request
	_, err = node.Set("model", ast.NewString(meta.ActualModel))
	if err != nil {
		return nil, err
	}

	err = resetCacheTTLWithContentsNode(node.Get("system"))
	if err != nil {
		return nil, err
	}

	messagesNode := node.Get("messages")
	if messagesNode.Check() == nil {
		_ = messagesNode.ForEach(func(_ ast.Sequence, messages *ast.Node) bool {
			_ = resetCacheTTLWithContentsNode(messages.Get("content"))
			return true
		})
	}

	maxTokensNode := node.Get("max_tokens")
	if maxTokensNode == nil || !maxTokensNode.Exists() {
		_, _ = node.Set(
			"max_tokens",
			ast.NewNumber(strconv.Itoa(ModelDefaultMaxTokens(meta.ActualModel))),
		)
	}

	// Handle thinking budget tokens adjustment
	thinkingNode := node.Get("thinking")
	if thinkingNode != nil && thinkingNode.Exists() {
		// Only adjust budget_tokens for "enabled" type
		// Opus 4.6's "adaptive" type doesn't support budget_tokens
		thinkingType, _ := thinkingNode.Get("type").String()
		if thinkingType == relaymodel.ClaudeThinkingTypeEnabled {
			maxTokens, err := node.Get("max_tokens").Int64()
			if err == nil {
				budgetTokens, _ := thinkingNode.Get("budget_tokens").Int64()
				maxTokensInt := int(maxTokens)
				budgetTokensInt := int(budgetTokens)
				adjustThinkingBudgetTokens(&maxTokensInt, &budgetTokensInt)

				// Update the nodes with adjusted values
				_, _ = node.Set("max_tokens", ast.NewNumber(strconv.Itoa(maxTokensInt)))
				_, _ = thinkingNode.Set(
					"budget_tokens",
					ast.NewNumber(strconv.Itoa(budgetTokensInt)),
				)
			}
		}

		// Remove temperature when thinking is enabled
		_, _ = node.Unset("temperature")
	}

	if node.Get("temperature").Exists() && node.Get("top_p").Exists() {
		// Claude does not allow both temperature and top_p to be specified
		_, _ = node.Unset("top_p")
	}

	for _, callback := range callbacks {
		if callback == nil {
			continue
		}

		if err := callback(node); err != nil {
			return nil, err
		}
	}

	return node.MarshalJSON()
}

// ConvertImage2Base64 handles converting image URLs to base64 encoded data
func ConvertImage2Base64(ctx context.Context, node *ast.Node) error {
	messagesNode := node.Get("messages")
	if messagesNode == nil || messagesNode.TypeSafe() != ast.V_ARRAY {
		return nil
	}

	var imageItems []*ast.Node

	err := messagesNode.ForEach(func(_ ast.Sequence, msgNode *ast.Node) bool {
		contentNode := msgNode.Get("content")
		if contentNode == nil || contentNode.TypeSafe() != ast.V_ARRAY {
			return true
		}

		err := contentNode.ForEach(func(_ ast.Sequence, contentItem *ast.Node) bool {
			contentType, err := contentItem.Get("type").String()
			if err == nil && contentType == relaymodel.ClaudeContentTypeImage {
				sourceNode := contentItem.Get("source")
				if sourceNode != nil {
					imageType, err := sourceNode.Get("type").String()
					if err == nil && imageType == "url" {
						imageItems = append(imageItems, contentItem)
					}
				}
			}

			return true
		})

		return err == nil
	})
	if err != nil {
		return err
	}

	if len(imageItems) == 0 {
		return nil
	}

	sem := semaphore.NewWeighted(3)

	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		processErrs []error
	)

	for _, item := range imageItems {
		wg.Add(1)

		go func(contentItem *ast.Node) {
			defer wg.Done()

			_ = sem.Acquire(ctx, 1)
			defer sem.Release(1)

			err := convertImageURLToBase64(ctx, contentItem)
			if err != nil {
				mu.Lock()

				processErrs = append(processErrs, err)

				mu.Unlock()
			}
		}(item)
	}

	wg.Wait()

	if len(processErrs) != 0 {
		return errors.Join(processErrs...)
	}

	return nil
}

// convertImageURLToBase64 converts an image URL to base64 encoded data
func convertImageURLToBase64(ctx context.Context, contentItem *ast.Node) error {
	sourceNode := contentItem.Get("source")
	if sourceNode == nil {
		return nil
	}

	url, err := sourceNode.Get("url").String()
	if err != nil {
		return nil
	}

	mimeType, data, err := image.GetImageFromURL(ctx, url)
	if err != nil {
		return nil
	}

	patches := []func() (bool, error){
		func() (bool, error) { return sourceNode.Set("type", ast.NewString("base64")) },
		func() (bool, error) { return sourceNode.Set("media_type", ast.NewString(mimeType)) },
		func() (bool, error) { return sourceNode.Set("data", ast.NewString(data)) },
		func() (bool, error) { return sourceNode.Unset("url") },
	}

	for _, patch := range patches {
		if _, err := patch(); err != nil {
			return err
		}
	}

	return nil
}

func StreamHandler(
	m *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	scanner, cleanup := utils.NewStreamScanner(resp.Body, m.ActualModel)
	defer cleanup()

	responseText := strings.Builder{}

	var (
		usage      *relaymodel.ChatUsage
		writed     bool
		upstreamID string
	)

	streamState := NewStreamState()

	for scanner.Scan() {
		data := scanner.Bytes()
		if !render.IsValidSSEData(data) {
			continue
		}

		data = render.ExtractSSEData(data)
		if render.IsSSEDone(data) {
			break
		}

		response, err := streamState.StreamResponse2OpenAI(m, data)
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
					usage.PromptTokens = int64(m.RequestUsage.InputTokens)
					usage.TotalTokens += int64(m.RequestUsage.InputTokens)
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

		node, parseErr := sonic.Get(data)
		if parseErr != nil {
			log.Error("error unmarshalling stream response: " + parseErr.Error())
		} else {
			// Check if model field exists in message.model (for message_start events)
			messageNode := node.Get("message")
			if messageNode != nil && messageNode.Exists() {
				modelNode := messageNode.Get("model")
				if modelNode != nil && modelNode.Exists() {
					_, setErr := messageNode.Set("model", ast.NewString(m.OriginModel))
					if setErr != nil {
						log.Error("error set response model in message: " + setErr.Error())
					} else {
						newData, marshalErr := node.MarshalJSON()
						if marshalErr != nil {
							log.Error("error marshalling stream response: " + marshalErr.Error())
						} else {
							data = newData
						}
					}
				}
			}
		}

		render.ClaudeData(c, data)

		writed = true
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	if usage == nil {
		usage = &relaymodel.ChatUsage{
			PromptTokens:     int64(m.RequestUsage.InputTokens),
			CompletionTokens: openai.CountTokenText(responseText.String(), m.OriginModel),
			TotalTokens: int64(
				m.RequestUsage.InputTokens,
			) + openai.CountTokenText(
				responseText.String(),
				m.OriginModel,
			),
		}
	}

	return adaptor.DoResponseResult{
		Usage:      usage.ToModelUsage(),
		UpstreamID: upstreamID,
	}, nil
}

func Handler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	respBody, err := common.GetResponseBody(resp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperAnthropicError(
			err,
			"read_response_failed",
			http.StatusInternalServerError,
		)
	}

	fullTextResponse, adaptorErr := Response2OpenAI(meta, respBody)
	if adaptorErr != nil {
		return adaptor.DoResponseResult{}, adaptorErr
	}

	log := common.GetLogger(c)

	// Set model to OriginModel in response body
	node, err := sonic.Get(respBody)
	if err != nil {
		log.Error("error unmarshalling stream response: " + err.Error())
	} else {
		_, err = node.Set("model", ast.NewString(meta.OriginModel))
		if err != nil {
			log.Error("error set response model: " + err.Error())
		} else {
			newRespBody, err := node.MarshalJSON()
			if err != nil {
				log.Error("error marshalling response: " + err.Error())
			} else {
				respBody = newRespBody
			}
		}
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(respBody)))
	_, _ = c.Writer.Write(respBody)

	return adaptor.DoResponseResult{
		Usage:      fullTextResponse.Usage.ToModelUsage(),
		UpstreamID: fullTextResponse.ID,
	}, nil
}
