package ali

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

type RerankResponse struct {
	Usage     *RerankUsage `json:"usage"`
	RequestID string       `json:"request_id"`
	Output    RerankOutput `json:"output"`
}
type RerankOutput struct {
	Results []*relaymodel.RerankResult `json:"results"`
}
type RerankUsage struct {
	TotalTokens int64 `json:"total_tokens"`
}

func ConvertRerankRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2ValidatedNode(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	query, err := aliRerankContentNode(node.Get("query"), false)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	documents, err := aliRerankDocumentsNode(node.Get("documents"))
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	input := ast.NewObject([]ast.Pair{
		ast.NewPair("query", query),
		ast.NewPair("documents", documents),
	})
	parameters := ast.NewObject(nil)
	deleteKeys := make([]string, 0)

	if err := node.ForEach(func(path ast.Sequence, child *ast.Node) bool {
		if path.Key == nil {
			return true
		}

		switch *path.Key {
		case "model", "input", "query", "documents":
			return true
		default:
			_, _ = parameters.Set(*path.Key, cloneAliRerankNode(child))
			deleteKeys = append(deleteKeys, *path.Key)
			return true
		}
	}); err != nil {
		return adaptor.ConvertResult{}, err
	}

	_, _ = node.Unset("query")

	_, _ = node.Unset("documents")
	for _, key := range deleteKeys {
		_, _ = node.Unset(key)
	}

	if _, err := node.Set("model", ast.NewString(meta.ActualModel)); err != nil {
		return adaptor.ConvertResult{}, err
	}

	if _, err := node.Set("input", input); err != nil {
		return adaptor.ConvertResult{}, err
	}

	if _, err := node.Set("parameters", parameters); err != nil {
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

func aliRerankDocumentsNode(documents *ast.Node) (ast.Node, error) {
	if documents == nil || !documents.Exists() {
		return ast.NewNull(), nil
	}

	if documents.TypeSafe() != ast.V_ARRAY {
		return cloneAliRerankNode(documents), nil
	}

	var (
		items    []ast.Node
		patchErr error
	)

	err := documents.ForEach(func(_ ast.Sequence, document *ast.Node) bool {
		var item ast.Node

		item, patchErr = aliRerankContentNode(document, true)
		if patchErr != nil {
			return false
		}

		items = append(items, item)

		return true
	})
	if err != nil {
		return ast.Node{}, err
	}

	if patchErr != nil {
		return ast.Node{}, patchErr
	}

	return ast.NewArray(items), nil
}

func aliRerankContentNode(item *ast.Node, allowVideo bool) (ast.Node, error) {
	if item == nil || !item.Exists() {
		return ast.NewNull(), nil
	}

	switch item.TypeSafe() {
	case ast.V_STRING:
		return cloneAliRerankNode(item), nil
	case ast.V_OBJECT:
		return normalizeAliRerankContentObject(item, allowVideo)
	default:
		return cloneAliRerankNode(item), nil
	}
}

func normalizeAliRerankContentObject(item *ast.Node, allowVideo bool) (ast.Node, error) {
	for _, field := range []aliRerankContentField{
		{source: "text", target: "text"},
		{source: "image", target: "image"},
		{source: "image_url", target: "image"},
		{source: "video", target: "video", video: true},
		{source: "video_url", target: "video", video: true},
	} {
		content, ok, err := aliRerankContentFieldNode(item, field, allowVideo)
		if err != nil || ok {
			return content, err
		}
	}

	return cloneAliRerankObjectWithoutType(item), nil
}

type aliRerankContentField struct {
	source string
	target string
	video  bool
}

func aliRerankContentFieldNode(
	item *ast.Node,
	field aliRerankContentField,
	allowVideo bool,
) (ast.Node, bool, error) {
	if field.video && !allowVideo {
		return ast.Node{}, false, nil
	}

	valueNode := item.Get(field.source)
	if !valueNode.Exists() {
		return ast.Node{}, false, nil
	}

	value, ok, err := aliRerankStringOrURLValue(valueNode)
	if err != nil || !ok {
		return ast.Node{}, false, err
	}

	if field.target == "image" {
		value, err = normalizeAliRerankImage(value)
		if err != nil {
			return ast.Node{}, false, err
		}
	}

	return ast.NewObject([]ast.Pair{
		ast.NewPair(field.target, ast.NewString(value)),
	}), true, nil
}

func aliRerankStringOrURLValue(node *ast.Node) (string, bool, error) {
	switch node.TypeSafe() {
	case ast.V_STRING:
		value, err := node.String()
		return value, true, err
	case ast.V_OBJECT:
		urlNode := node.Get("url")
		if !urlNode.Exists() || urlNode.TypeSafe() != ast.V_STRING {
			return "", false, nil
		}

		value, err := urlNode.String()

		return value, true, err
	default:
		return "", false, nil
	}
}

func normalizeAliRerankImage(image string) (string, error) {
	if strings.HasPrefix(image, "data:image/") ||
		strings.HasPrefix(image, "http://") ||
		strings.HasPrefix(image, "https://") {
		return image, nil
	}

	data, err := base64.StdEncoding.DecodeString(image)
	if err != nil {
		return "", invalidAliRerankImageError()
	}

	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		return "", invalidAliRerankImageError()
	}

	return "data:" + contentType + ";base64," + image, nil
}

func invalidAliRerankImageError() error {
	return relaymodel.NewOpenAIError(http.StatusBadRequest, relaymodel.OpenAIError{
		Code: "InvalidParameter",
		Message: "Image URL or Base64 is invalid. URL must be a valid HTTP/HTTPS link, " +
			"and Base64 must start with 'image/xxx;base64' or be valid raw image base64",
		Type:  relaymodel.ErrorTypeAIPROXY,
		Param: "image",
	})
}

func cloneAliRerankNode(node *ast.Node) ast.Node {
	if node == nil || !node.Exists() {
		return ast.NewNull()
	}

	raw, err := node.Raw()
	if err != nil {
		return ast.NewNull()
	}

	return ast.NewRaw(raw)
}

func cloneAliRerankObjectWithoutType(node *ast.Node) ast.Node {
	if node == nil || !node.Exists() || node.TypeSafe() != ast.V_OBJECT {
		return cloneAliRerankNode(node)
	}

	pairs := make([]ast.Pair, 0)
	if err := node.ForEach(func(path ast.Sequence, child *ast.Node) bool {
		if path.Key == nil || *path.Key == "type" {
			return true
		}

		pairs = append(pairs, ast.NewPair(*path.Key, cloneAliRerankNode(child)))

		return true
	}); err != nil {
		return cloneAliRerankNode(node)
	}

	return ast.NewObject(pairs)
}

func RerankHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	var rerankResponse RerankResponse

	err := common.UnmarshalResponse(resp, &rerankResponse)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	rerankResp := relaymodel.RerankResponse{
		Meta: relaymodel.RerankMeta{
			Tokens: &relaymodel.RerankMetaTokens{
				InputTokens:  rerankResponse.Usage.TotalTokens,
				OutputTokens: 0,
			},
		},
		Results: rerankResponse.Output.Results,
		ID:      rerankResponse.RequestID,
	}

	var usage model.Usage
	if rerankResponse.Usage == nil {
		usage = model.Usage{
			InputTokens: meta.RequestUsage.InputTokens,
			TotalTokens: meta.RequestUsage.InputTokens,
		}
	} else {
		usage = model.Usage{
			InputTokens: model.ZeroNullInt64(rerankResponse.Usage.TotalTokens),
			TotalTokens: model.ZeroNullInt64(rerankResponse.Usage.TotalTokens),
		}
	}

	jsonResponse, err := sonic.Marshal(&rerankResp)
	if err != nil {
		return adaptor.DoResponseResult{Usage: usage}, relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonResponse)))

	_, err = c.Writer.Write(jsonResponse)
	if err != nil {
		log.Warnf("write response body failed: %v", err)
	}

	return adaptor.DoResponseResult{Usage: usage}, nil
}
