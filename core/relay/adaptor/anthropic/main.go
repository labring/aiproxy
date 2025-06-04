package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/common/image"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"golang.org/x/sync/semaphore"
)

func ConvertRequest(meta *meta.Meta, req *http.Request) (adaptor.ConvertResult, error) {
	// Parse request body into AST node
	node, err := common.UnmarshalBody2Node(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	// Set the actual model in the request
	_, err = node.Set("model", ast.NewString(meta.ActualModel))
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	// Process image content if present
	err = ConvertImage2Base64(req.Context(), &node)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	// Serialize the modified node
	newBody, err := node.MarshalJSON()
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
			if err == nil && contentType == conetentTypeImage {
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
	var wg sync.WaitGroup
	var mu sync.Mutex
	var processErrs []error

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
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	log := middleware.GetLogger(c)

	scanner := bufio.NewScanner(resp.Body)
	buf := openai.GetScannerBuffer()
	defer openai.PutScannerBuffer(buf)
	scanner.Buffer(*buf, cap(*buf))

	responseText := strings.Builder{}

	var usage *relaymodel.Usage
	var writed bool

	for scanner.Scan() {
		data := scanner.Bytes()
		if len(data) < 6 || conv.BytesToString(data[:6]) != "data: " {
			continue
		}
		data = data[6:]

		response, err := StreamResponse2OpenAI(m, data)
		if err != nil {
			if writed {
				log.Errorf("response error: %+v", err)
			} else {
				return usage.ToModelUsage(), err
			}
		}
		if response != nil {
			switch {
			case response.Usage != nil:
				if usage == nil {
					usage = &relaymodel.Usage{}
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

		Data(c, data)
		writed = true
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	if usage == nil {
		usage = &relaymodel.Usage{
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

	return usage.ToModelUsage(), nil
}

func Handler(meta *meta.Meta, c *gin.Context, resp *http.Response) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperAnthropicError(
			err,
			"read_response_failed",
			http.StatusInternalServerError,
		)
	}

	fullTextResponse, adaptorErr := Response2OpenAI(meta, respBody)
	if adaptorErr != nil {
		return model.Usage{}, adaptorErr
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(respBody)))
	_, _ = c.Writer.Write(respBody)
	return fullTextResponse.ToModelUsage(), nil
}
