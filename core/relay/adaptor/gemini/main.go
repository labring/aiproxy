package gemini

import (
	"bytes"
	"net/http"
	"strconv"

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

// https://ai.google.dev/docs/gemini_api_overview?hl=zh-cn

// Dummy thought signatures for skipping Gemini's validation when the actual signature is unavailable
// See: https://ai.google.dev/gemini-api/docs/thought-signatures#faqs
const (
	ThoughtSignatureDummySkipValidator = "skip_thought_signature_validator"
	ThoughtSignatureDummyContextEng    = "context_engineering_is_the_way_to_go"
)

func CleanFunctionResponseID(node *ast.Node) error {
	contents := node.Get("contents")
	if !contents.Exists() {
		return nil
	}

	return contents.ForEach(func(_ ast.Sequence, content *ast.Node) bool {
		parts := content.Get("parts")
		if !parts.Exists() {
			return true
		}

		_ = parts.ForEach(func(_ ast.Sequence, part *ast.Node) bool {
			functionResponse := part.Get("functionResponse")
			if functionResponse.Exists() {
				_, _ = functionResponse.Unset("id")
			}

			return true
		})

		return true
	})
}

func ensureThoughtSignature(node *ast.Node) error {
	contents := node.Get("contents")
	if !contents.Exists() {
		return nil
	}

	return contents.ForEach(func(_ ast.Sequence, content *ast.Node) bool {
		parts := content.Get("parts")
		if !parts.Exists() {
			return true
		}

		_ = parts.ForEach(func(_ ast.Sequence, part *ast.Node) bool {
			functionCall := part.Get("functionCall")
			if !functionCall.Exists() {
				return true
			}

			thoughtSignature := part.Get("thoughtSignature")
			if !thoughtSignature.Exists() {
				_, _ = part.Set(
					"thoughtSignature",
					ast.NewString(ThoughtSignatureDummySkipValidator),
				)
			} else {
				val, _ := thoughtSignature.String()
				if val == "" {
					_, _ = part.Set("thoughtSignature", ast.NewString(ThoughtSignatureDummySkipValidator))
				}
			}

			return true
		})

		return true
	})
}

func ensureRole(node *ast.Node) error {
	contents := node.Get("contents")
	if !contents.Exists() {
		return nil
	}

	return contents.ForEach(func(_ ast.Sequence, content *ast.Node) bool {
		role := content.Get("role")
		if !role.Exists() {
			_, _ = content.Set("role", ast.NewString("user"))
		} else {
			val, _ := role.String()
			if val == "" {
				_, _ = content.Set("role", ast.NewString("user"))
			}
		}

		return true
	})
}

func ensureGemini3RemoveToolConfigFunctionCallingAUTOModeConfig(node *ast.Node) error {
	toolConfig := node.Get("toolConfig")
	if !toolConfig.Exists() {
		return nil
	}

	return toolConfig.ForEach(func(_ ast.Sequence, config *ast.Node) bool {
		functionCallingConfig := config.Get("functionCallingConfig")
		if !functionCallingConfig.Exists() {
			return true
		}

		modeNode := functionCallingConfig.Get("mode")
		if !modeNode.Exists() {
			return true
		}

		modeStr, _ := modeNode.String()
		if modeStr == relaymodel.GeminiFunctionCallingModeAuto {
			_, _ = config.Unset("functionCallingConfig")
		}

		return true
	})
}

func NativeConvertRequest(
	meta *meta.Meta,
	req *http.Request,
	callback ...func(node *ast.Node) error,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2NodeReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	err = ensureThoughtSignature(&node)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	err = ensureRole(&node)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	err = ensureGemini3RemoveToolConfigFunctionCallingAUTOModeConfig(&node)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	for _, callback := range callback {
		if callback == nil {
			continue
		}

		err = callback(&node)
		if err != nil {
			return adaptor.ConvertResult{}, err
		}
	}

	body, err := node.MarshalJSON()
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(body))},
		},
		Body: bytes.NewReader(body),
	}, nil
}

// NativeHandler handles non-streaming responses in native Gemini format (passthrough)
func NativeHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	var geminiResponse relaymodel.GeminiChatResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&geminiResponse); err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	// Calculate usage
	usage := model.Usage{}
	if geminiResponse.UsageMetadata != nil {
		usage = geminiResponse.UsageMetadata.ToModelUsage()
	}
	// Get web search count from grounding metadata
	usage.WebSearchCount = model.ZeroNullInt64(geminiResponse.GetWebSearchCount())

	// Pass through the response as-is
	jsonResponse, err := sonic.Marshal(geminiResponse)
	if err != nil {
		return usage, relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonResponse)))
	_, _ = c.Writer.Write(jsonResponse)

	return usage, nil
}

// NativeStreamHandler handles streaming responses in native Gemini format (passthrough)
func NativeStreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	scanner, cleanup := utils.NewStreamScanner(resp.Body, meta.ActualModel)
	defer cleanup()

	usage := model.Usage{}

	var websearchCount int64

	for scanner.Scan() {
		data := scanner.Bytes()
		if !render.IsValidSSEData(data) {
			continue
		}

		data = render.ExtractSSEData(data)

		// Parse to extract usage metadata and web search count
		var geminiResp relaymodel.GeminiChatResponse
		if err := sonic.Unmarshal(data, &geminiResp); err == nil {
			if geminiResp.UsageMetadata != nil {
				usage = geminiResp.UsageMetadata.ToModelUsage()
			}
			// Get web search count from grounding metadata
			if webSearchCount := geminiResp.GetWebSearchCount(); webSearchCount > 0 {
				websearchCount += webSearchCount
			}
		}

		// Pass through the data as-is
		render.GeminiBytesData(c, data)
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	usage.WebSearchCount += model.ZeroNullInt64(websearchCount)

	return usage, nil
}
