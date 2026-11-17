package ali

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"

	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
)

func isMultimodalEmbeddingModel(meta *meta.Meta) bool {
	if meta == nil {
		return false
	}

	return isMultimodalEmbeddingModelName(meta.OriginModel) ||
		isMultimodalEmbeddingModelName(meta.ActualModel)
}

func isMultimodalEmbeddingModelName(modelName string) bool {
	modelName = strings.ToLower(modelName)
	return strings.Contains(modelName, "vl") ||
		strings.Contains(modelName, "multimodal") ||
		strings.Contains(modelName, "vision")
}

func ConvertMultimodalEmbeddingsRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2ValidatedNode(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	if err := patchMultimodalEmbeddingsRequest(meta, &node); err != nil {
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

func patchMultimodalEmbeddingsRequest(meta *meta.Meta, node *ast.Node) error {
	if _, err := node.Unset("encoding_format"); err != nil {
		return err
	}

	if err := moveMultimodalEmbeddingParameter(node, "dimensions", "dimension"); err != nil {
		return err
	}

	for _, key := range []string{
		"dimension",
		"output_type",
		"fps",
		"instruct",
		"enable_fusion",
		"res_level",
		"max_video_frames",
	} {
		if err := moveMultimodalEmbeddingParameter(node, key, key); err != nil {
			return err
		}
	}

	_, err := node.Set("model", ast.NewString(meta.ActualModel))
	if err != nil {
		return err
	}

	inputNode := node.Get("input")
	if !inputNode.Exists() {
		return nil
	}

	if inputNode.TypeSafe() == ast.V_OBJECT {
		contentsNode := inputNode.Get("contents")
		if contentsNode.Exists() {
			return nil
		}
	}

	contentsNode, err := multimodalEmbeddingContents(inputNode)
	if err != nil {
		return err
	}

	_, err = node.Set("input", ast.NewObject([]ast.Pair{
		ast.NewPair("contents", contentsNode),
	}))

	return err
}

func moveMultimodalEmbeddingParameter(node *ast.Node, sourceKey, targetKey string) error {
	sourceNode := node.Get(sourceKey)
	if !sourceNode.Exists() {
		return nil
	}

	parametersNode := node.Get("parameters")
	if !parametersNode.Exists() || parametersNode.TypeSafe() != ast.V_OBJECT {
		if _, err := node.Set("parameters", ast.NewObject(nil)); err != nil {
			return err
		}

		parametersNode = node.Get("parameters")
	}

	if !parametersNode.Get(targetKey).Exists() {
		if _, err := parametersNode.Set(targetKey, *sourceNode); err != nil {
			return err
		}
	}

	_, err := node.Unset(sourceKey)

	return err
}

func multimodalEmbeddingContents(inputNode *ast.Node) (ast.Node, error) {
	switch inputNode.TypeSafe() {
	case ast.V_STRING:
		text, err := inputNode.String()
		if err != nil {
			return ast.Node{}, err
		}

		return ast.NewArray([]ast.Node{newMultimodalTextContent(text)}), nil
	case ast.V_ARRAY:
		var (
			contents []ast.Node
			patchErr error
		)

		err := inputNode.ForEach(func(_ ast.Sequence, item *ast.Node) bool {
			var content ast.Node

			content, patchErr = multimodalEmbeddingContentItem(item)
			if patchErr != nil {
				return false
			}

			contents = append(contents, content)

			return true
		})
		if err != nil {
			return ast.Node{}, err
		}

		if patchErr != nil {
			return ast.Node{}, patchErr
		}

		return ast.NewArray(contents), nil
	case ast.V_OBJECT:
		content, err := multimodalEmbeddingContentItem(inputNode)
		if err != nil {
			return ast.Node{}, err
		}

		return ast.NewArray([]ast.Node{content}), nil
	default:
		return *inputNode, nil
	}
}

func multimodalEmbeddingContentItem(item *ast.Node) (ast.Node, error) {
	switch item.TypeSafe() {
	case ast.V_STRING:
		text, err := item.String()
		if err != nil {
			return ast.Node{}, err
		}

		return newMultimodalTextContent(text), nil
	case ast.V_OBJECT:
		imageURL, ok, err := openAIEmbeddingImageURL(item)
		if err != nil {
			return ast.Node{}, err
		}

		if ok {
			return newMultimodalImageContent(imageURL), nil
		}

		typeNode := item.Get("type")
		if typeNode.Exists() {
			contentType, err := typeNode.String()
			if err != nil {
				return ast.Node{}, err
			}

			switch contentType {
			case "text":
				textNode := item.Get("text")
				if textNode.Exists() && textNode.TypeSafe() == ast.V_STRING {
					text, err := textNode.String()
					if err != nil {
						return ast.Node{}, err
					}

					return newMultimodalTextContent(text), nil
				}
			case "image":
				imageNode := item.Get("image")
				if imageNode.Exists() && imageNode.TypeSafe() == ast.V_STRING {
					image, err := imageNode.String()
					if err != nil {
						return ast.Node{}, err
					}

					return newMultimodalImageContent(image), nil
				}
			case "video":
				videoNode := item.Get("video")
				if videoNode.Exists() && videoNode.TypeSafe() == ast.V_STRING {
					video, err := videoNode.String()
					if err != nil {
						return ast.Node{}, err
					}

					return newMultimodalVideoContent(video), nil
				}
			}
		}

		if content := nativeMultimodalContent(item); content.Exists() {
			return content, nil
		}

		return *item, nil
	default:
		return *item, nil
	}
}

func openAIEmbeddingImageURL(item *ast.Node) (string, bool, error) {
	typeNode := item.Get("type")
	if typeNode.Exists() {
		contentType, err := typeNode.String()
		if err != nil {
			return "", false, err
		}

		if contentType != "image_url" {
			return "", false, nil
		}
	}

	imageURLNode := item.Get("image_url")
	if !imageURLNode.Exists() || imageURLNode.TypeSafe() != ast.V_OBJECT {
		return "", false, nil
	}

	urlNode := imageURLNode.Get("url")
	if !urlNode.Exists() || urlNode.TypeSafe() != ast.V_STRING {
		return "", false, nil
	}

	imageURL, err := urlNode.String()
	if err != nil {
		return "", false, err
	}

	return imageURL, true, nil
}

func nativeMultimodalContent(item *ast.Node) ast.Node {
	pairs := make([]ast.Pair, 0, 4)
	for _, key := range []string{"text", "image", "video", "multi_images"} {
		valueNode := item.Get(key)
		if valueNode.Exists() {
			pairs = append(pairs, ast.NewPair(key, *valueNode))
		}
	}

	if len(pairs) == 0 {
		return ast.Node{}
	}

	return ast.NewObject(pairs)
}

func newMultimodalTextContent(text string) ast.Node {
	return ast.NewObject([]ast.Pair{
		ast.NewPair("text", ast.NewString(text)),
	})
}

func newMultimodalImageContent(image string) ast.Node {
	return ast.NewObject([]ast.Pair{
		ast.NewPair("image", ast.NewString(image)),
	})
}

func newMultimodalVideoContent(video string) ast.Node {
	return ast.NewObject([]ast.Pair{
		ast.NewPair("video", ast.NewString(video)),
	})
}

func EmbeddingsHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	if isMultimodalEmbeddingModel(meta) {
		return openai.EmbeddingsHandler(
			meta,
			c,
			resp,
			multimodalEmbeddingPreHandler,
		)
	}

	return openai.DoResponse(meta, store, c, resp)
}

func multimodalEmbeddingPreHandler(_ *meta.Meta, node *ast.Node) error {
	return patchMultimodalEmbeddingResponse(node)
}

func patchMultimodalEmbeddingResponse(node *ast.Node) error {
	outputNode := node.Get("output")
	if !outputNode.Exists() {
		return nil
	}

	embeddingsNode := outputNode.Get("embeddings")
	if !embeddingsNode.Exists() || embeddingsNode.TypeSafe() != ast.V_ARRAY {
		return nil
	}

	var patchErr error

	err := embeddingsNode.ForEach(func(_ ast.Sequence, item *ast.Node) bool {
		if item.TypeSafe() != ast.V_OBJECT {
			return true
		}

		_, patchErr = item.Set("object", ast.NewString("embedding"))

		return patchErr == nil
	})
	if err != nil {
		return err
	}

	if patchErr != nil {
		return patchErr
	}

	if _, err := node.Set("object", ast.NewString("list")); err != nil {
		return err
	}

	if _, err := node.Set("data", *embeddingsNode); err != nil {
		return err
	}

	if _, err := node.Unset("output"); err != nil {
		return err
	}

	return patchMultimodalEmbeddingUsage(node)
}

func patchMultimodalEmbeddingUsage(node *ast.Node) error {
	usageNode := node.Get("usage")
	if !usageNode.Exists() || usageNode.TypeSafe() != ast.V_OBJECT {
		return nil
	}

	inputTokensNode := usageNode.Get("input_tokens")
	if inputTokensNode.Exists() && !usageNode.Get("prompt_tokens").Exists() {
		if _, err := usageNode.Set("prompt_tokens", *inputTokensNode); err != nil {
			return err
		}
	}

	inputTokensDetailsNode := usageNode.Get("input_tokens_details")
	if inputTokensDetailsNode.Exists() && !usageNode.Get("prompt_tokens_details").Exists() {
		if _, err := usageNode.Set("prompt_tokens_details", *inputTokensDetailsNode); err != nil {
			return err
		}
	}

	return nil
}
