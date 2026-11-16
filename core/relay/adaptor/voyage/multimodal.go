package voyage

import (
	"net/http"
	"strings"

	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

// pixelsPerImageToken is the Voyage conversion between image pixels and tokens:
// every 560 pixels of an image count as one token.
const pixelsPerImageToken = 560

// ConvertMultimodalEmbeddingsRequest adapts an OpenAI-style embeddings request
// to POST /v1/multimodalembeddings. Each element of `input` becomes one
// `{"content": [...]}` entry and therefore one embedding:
//   - a string is a text content item;
//   - {"type":"text","text":...} is a text content item;
//   - {"type":"image_url","image_url":{"url":...}} is an image; `data:` URLs
//     are sent as `image_base64`, everything else as `image_url`;
//   - {"content":[...]} is forwarded as is, for text and image mixed in one embedding.
//
// Anything Voyage rejects (unreachable image URL, unsupported dimension, ...) is
// reported through ErrorHandler.
func ConvertMultimodalEmbeddingsRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2NodeReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, convertRequestError(err.Error())
	}

	if _, err := node.Set("model", ast.NewString(meta.ActualModel)); err != nil {
		return adaptor.ConvertResult{}, err
	}

	if err := normalizeEncodingFormat(&node); err != nil {
		return adaptor.ConvertResult{}, err
	}

	if err := mapDimensions(&node); err != nil {
		return adaptor.ConvertResult{}, err
	}

	inputNode := node.Get("input")
	if !inputNode.Exists() {
		return adaptor.ConvertResult{}, convertRequestError("input is required")
	}

	var items []ast.Node

	switch inputNode.TypeSafe() {
	case ast.V_STRING:
		items = []ast.Node{*inputNode}
	case ast.V_ARRAY:
		if err := inputNode.ForEach(func(_ ast.Sequence, item *ast.Node) bool {
			items = append(items, *item)
			return true
		}); err != nil {
			return adaptor.ConvertResult{}, err
		}
	default:
		return adaptor.ConvertResult{}, convertRequestError("input must be a string or an array")
	}

	if len(items) == 0 {
		return adaptor.ConvertResult{}, convertRequestError("input must not be empty")
	}

	inputs := make([]ast.Node, 0, len(items))

	for i := range items {
		converted, err := toMultimodalInput(&items[i])
		if err != nil {
			return adaptor.ConvertResult{}, err
		}

		inputs = append(inputs, converted)
	}

	if _, err := node.Unset("input"); err != nil {
		return adaptor.ConvertResult{}, err
	}

	if _, err := node.Set("inputs", ast.NewArray(inputs)); err != nil {
		return adaptor.ConvertResult{}, err
	}

	return marshalConvertResult(&node)
}

func toMultimodalInput(item *ast.Node) (ast.Node, error) {
	switch item.TypeSafe() {
	case ast.V_STRING:
		text, err := item.String()
		if err != nil {
			return ast.Node{}, err
		}

		return wrapContent(textContent(text)), nil
	case ast.V_OBJECT:
		if content := item.Get("content"); content.Exists() {
			if content.TypeSafe() != ast.V_ARRAY {
				return ast.Node{}, convertRequestError("input content must be an array")
			}

			if err := checkContentTypes(content); err != nil {
				return ast.Node{}, err
			}

			return ast.NewObject([]ast.Pair{ast.NewPair("content", *content)}), nil
		}

		content, err := toContentItem(item)
		if err != nil {
			return ast.Node{}, err
		}

		return wrapContent(content), nil
	default:
		return ast.Node{}, convertRequestError(
			"each input must be a string or an object",
		)
	}
}

// checkContentTypes rejects content items other than text and images. Video is
// not relayed: its pixels would be billed at the text rate because only image
// pixels are split out of usage.total_tokens.
func checkContentTypes(content *ast.Node) error {
	var checkErr error

	err := content.ForEach(func(_ ast.Sequence, item *ast.Node) bool {
		contentType, err := item.Get("type").String()
		if err != nil {
			checkErr = convertRequestError("each content item needs a string type")
			return false
		}

		switch contentType {
		case "text", "image_url", "image_base64":
			return true
		default:
			checkErr = convertRequestError("unsupported content type: " + contentType)
			return false
		}
	})
	if err != nil {
		return err
	}

	return checkErr
}

func toContentItem(item *ast.Node) (ast.Node, error) {
	typeNode := item.Get("type")
	if !typeNode.Exists() {
		return ast.Node{}, convertRequestError("input object needs a type or a content array")
	}

	contentType, err := typeNode.String()
	if err != nil {
		return ast.Node{}, convertRequestError("input type must be a string")
	}

	switch contentType {
	case "text":
		text, err := item.Get("text").String()
		if err != nil {
			return ast.Node{}, convertRequestError("text input needs a string `text`")
		}

		return textContent(text), nil
	case "image_url":
		imageURL, err := imageURLValue(item.Get("image_url"))
		if err != nil {
			return ast.Node{}, err
		}

		if strings.HasPrefix(imageURL, "data:") {
			return ast.NewObject([]ast.Pair{
				ast.NewPair("type", ast.NewString("image_base64")),
				ast.NewPair("image_base64", ast.NewString(imageURL)),
			}), nil
		}

		return ast.NewObject([]ast.Pair{
			ast.NewPair("type", ast.NewString("image_url")),
			ast.NewPair("image_url", ast.NewString(imageURL)),
		}), nil
	default:
		return ast.Node{}, convertRequestError("unsupported input type: " + contentType)
	}
}

// imageURLValue accepts both the OpenAI object form {"url": "..."} and a bare string.
func imageURLValue(node *ast.Node) (string, error) {
	if node == nil || !node.Exists() {
		return "", convertRequestError("image_url input needs `image_url`")
	}

	if node.TypeSafe() == ast.V_OBJECT {
		node = node.Get("url")
	}

	value, err := node.String()
	if err != nil || value == "" {
		return "", convertRequestError("image_url must be a non-empty URL string")
	}

	return value, nil
}

func textContent(text string) ast.Node {
	return ast.NewObject([]ast.Pair{
		ast.NewPair("type", ast.NewString("text")),
		ast.NewPair("text", ast.NewString(text)),
	})
}

func wrapContent(content ast.Node) ast.Node {
	return ast.NewObject([]ast.Pair{
		ast.NewPair("content", ast.NewArray([]ast.Node{content})),
	})
}

// MultimodalEmbeddingsHandler relays the multimodal embeddings response and
// splits the usage into text and image tokens so that image pixels are billed at
// the image price. A response without usage.total_tokens fails instead of being
// billed as zero.
func MultimodalEmbeddingsHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	node, err := common.UnmarshalResponse2Node(resp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	totalTokens, ok := readTotalTokens(&node)
	if !ok {
		return adaptor.DoResponseResult{}, missingUsageError("multimodal embeddings")
	}

	textTokens, _ := node.GetByPath("usage", "text_tokens").Int64()
	imagePixels, _ := node.GetByPath("usage", "image_pixels").Int64()

	imageTokens := min(imagePixels/pixelsPerImageToken, totalTokens)

	usage := relaymodel.EmbeddingUsage{
		PromptTokens: totalTokens,
		TotalTokens:  totalTokens,
		PromptTokensDetails: &relaymodel.EmbeddingPromptTokensDetails{
			TextTokens:  textTokens,
			ImageTokens: imageTokens,
		},
	}

	return writeEmbeddingsResponse(meta, c, &node, usage)
}
