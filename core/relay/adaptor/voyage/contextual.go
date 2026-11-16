package voyage

import (
	"net/http"
	"strconv"

	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

// contextualChunkSize is the auto-chunking window sent for every document. It
// equals the context length of voyage-context-3/4, the largest value Voyage
// accepts (40000 is rejected with a 400).
const contextualChunkSize = 32000

// ConvertContextualizedEmbeddingsRequest adapts an OpenAI-style embeddings
// request to POST /v1/contextualizedembeddings:
//   - `input_type` is required and must be `document` or `query`;
//   - document mode sends a flat list of strings (one string is one document)
//     with `enable_auto_chunking: true` and `chunk_size: 32000`; Voyage rejects
//     list[list[str]] together with auto-chunking;
//   - query mode never auto-chunks and wraps every query in its own inner list;
//   - `input` becomes `inputs`, `dimensions` becomes `output_dimension`, and
//     `encoding_format: "float"` is dropped.
//
// Every input must be a plain string; token arrays, nested lists and objects are
// rejected instead of being reinterpreted.
func ConvertContextualizedEmbeddingsRequest(
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

	inputType, err := contextualInputType(&node)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	if err := checkChunkingFields(&node, inputType); err != nil {
		return adaptor.ConvertResult{}, err
	}

	texts, err := contextualInputTexts(node.Get("input"))
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	if _, err := node.Unset("input"); err != nil {
		return adaptor.ConvertResult{}, err
	}

	inputs := make([]ast.Node, 0, len(texts))

	for _, text := range texts {
		item := ast.NewString(text)
		if inputType == inputTypeQuery {
			item = ast.NewArray([]ast.Node{item})
		}

		inputs = append(inputs, item)
	}

	if _, err := node.Set("inputs", ast.NewArray(inputs)); err != nil {
		return adaptor.ConvertResult{}, err
	}

	meta.Set(metaInputCountKey, len(texts))

	if inputType == inputTypeDocument {
		if _, err := node.Set("enable_auto_chunking", ast.NewBool(true)); err != nil {
			return adaptor.ConvertResult{}, err
		}

		if _, err := node.Set(
			"chunk_size",
			ast.NewNumber(strconv.Itoa(contextualChunkSize)),
		); err != nil {
			return adaptor.ConvertResult{}, err
		}
	}

	return marshalConvertResult(&node)
}

// metaInputCountKey carries the number of inputs from the request conversion to
// the response handler, which must return exactly one vector for each of them.
const metaInputCountKey = "voyage_contextual_input_count"

const (
	inputTypeDocument = "document"
	inputTypeQuery    = "query"
)

// checkChunkingFields keeps the chunking settings under the adaptor's control.
// Query mode never auto-chunks, so the fields a client may have sent are
// removed (Voyage answers 400 to them). Document mode always uses auto-chunking
// at the fixed size: a client asking for something else is rejected instead of
// being silently overridden.
func checkChunkingFields(node *ast.Node, inputType string) error {
	if inputType == inputTypeQuery {
		for _, key := range []string{"enable_auto_chunking", "chunk_size", "chunk_fn"} {
			if _, err := node.Unset(key); err != nil {
				return err
			}
		}

		return nil
	}

	if enabled := node.Get("enable_auto_chunking"); enabled != nil && enabled.Exists() {
		if enabled.TypeSafe() != ast.V_TRUE {
			return convertRequestError(
				"enable_auto_chunking is always true for document inputs",
			)
		}
	}

	if size := node.Get("chunk_size"); size != nil && size.Exists() {
		if value, err := size.Int64(); size.TypeSafe() != ast.V_NUMBER || err != nil ||
			value != contextualChunkSize {
			return convertRequestError(
				"chunk_size is fixed at " + strconv.Itoa(contextualChunkSize),
			)
		}
	}

	if _, err := node.Unset("chunk_fn"); err != nil {
		return err
	}

	return nil
}

func contextualInputType(node *ast.Node) (string, error) {
	typeNode := node.Get("input_type")
	if !typeNode.Exists() || typeNode.TypeSafe() == ast.V_NULL {
		return "", convertRequestError(
			"input_type is required for contextualized embeddings (document or query)",
		)
	}

	value, err := typeNode.String()
	if err != nil || (value != inputTypeDocument && value != inputTypeQuery) {
		return "", convertRequestError("input_type must be document or query")
	}

	return value, nil
}

// contextualInputTexts returns the non-empty strings of `input`, which may be a
// single string or a list of strings.
func contextualInputTexts(input *ast.Node) ([]string, error) {
	if !input.Exists() {
		return nil, convertRequestError("input is required")
	}

	if input.TypeSafe() == ast.V_STRING {
		text, err := input.String()
		if err != nil || text == "" {
			return nil, convertRequestError("input must not be empty")
		}

		return []string{text}, nil
	}

	if input.TypeSafe() != ast.V_ARRAY {
		return nil, convertRequestError("input must be a string or a list of strings")
	}

	list, err := input.ArrayUseNode()
	if err != nil || len(list) == 0 {
		return nil, convertRequestError("input must not be empty")
	}

	texts := make([]string, 0, len(list))

	for _, item := range list {
		if item.TypeSafe() != ast.V_STRING {
			return nil, convertRequestError(
				"contextualized embeddings take one string per document; " +
					"pre-chunked lists are not supported with auto-chunking",
			)
		}

		text, err := item.String()
		if err != nil || text == "" {
			return nil, convertRequestError("input items must be non-empty strings")
		}

		texts = append(texts, text)
	}

	return texts, nil
}

// ContextualizedEmbeddingsHandler converts the Voyage response, a list of
// documents each holding a list of chunk embeddings, to the flat OpenAI list.
// The OpenAI shape has one vector per input, so a document that Voyage split
// into several chunks fails the request (the usage is still billed, because
// Voyage already charged it). A chunk's `text`, which Voyage returns only when
// it auto-chunked, is kept on the embedding item. A response without
// usage.total_tokens fails instead of being billed as zero.
func ContextualizedEmbeddingsHandler(
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
		return adaptor.DoResponseResult{}, missingUsageError("contextualized embeddings")
	}

	usage := relaymodel.EmbeddingUsage{PromptTokens: totalTokens, TotalTokens: totalTokens}

	items, convertErr := flattenContextualData(node.Get("data"), meta.GetInt(metaInputCountKey))
	if convertErr != nil {
		return adaptor.DoResponseResult{Usage: usage.ToModelUsage()}, convertErr
	}

	if _, err := node.Set("data", ast.NewArray(items)); err != nil {
		return adaptor.DoResponseResult{Usage: usage.ToModelUsage()}, relaymodel.WrapperOpenAIError(
			err, "set_data_failed", http.StatusInternalServerError,
		)
	}

	return writeEmbeddingsResponse(meta, c, &node, usage)
}

func flattenContextualData(data *ast.Node, inputCount int) ([]ast.Node, adaptor.Error) {
	if data == nil || data.TypeSafe() != ast.V_ARRAY {
		return nil, malformedContextualError("data is not a list")
	}

	documents, err := data.ArrayUseNode()
	if err != nil || len(documents) == 0 {
		return nil, malformedContextualError("data is empty")
	}

	if len(documents) != inputCount {
		return nil, malformedContextualError(
			strconv.Itoa(len(documents)) + " documents returned for " +
				strconv.Itoa(inputCount) + " inputs",
		)
	}

	items := make([]ast.Node, 0, len(documents))

	for i := range documents {
		if position, err := documents[i].Get("index").Int64(); err != nil || position != int64(i) {
			return nil, malformedContextualError("documents are out of order or unindexed")
		}

		chunks := documents[i].Get("data")
		if chunks == nil || !chunks.Exists() || chunks.TypeSafe() != ast.V_ARRAY {
			return nil, malformedContextualError("document has no chunk list")
		}

		chunkList, err := chunks.ArrayUseNode()
		if err != nil || len(chunkList) == 0 {
			return nil, malformedContextualError("document has no embedding")
		}

		if chunkCount := len(chunkList); chunkCount != 1 {
			return nil, relaymodel.WrapperOpenAIErrorWithMessage(
				"input "+strconv.Itoa(i)+" was split into "+strconv.Itoa(chunkCount)+
					" chunks; send one chunk per input or shorten it",
				"multiple_chunks",
				http.StatusUnprocessableEntity,
			)
		}

		chunk := chunkList[0]

		embedding := chunk.Get("embedding")
		if embedding == nil || !embedding.Exists() ||
			(embedding.TypeSafe() != ast.V_ARRAY && embedding.TypeSafe() != ast.V_STRING) {
			return nil, malformedContextualError("chunk embedding is not a vector")
		}

		pairs := []ast.Pair{
			ast.NewPair("object", ast.NewString("embedding")),
			ast.NewPair("index", ast.NewNumber(strconv.Itoa(i))),
			ast.NewPair("embedding", *embedding),
		}

		if text := chunk.Get("text"); text != nil && text.Exists() {
			pairs = append(pairs, ast.NewPair("text", *text))
		}

		items = append(items, ast.NewObject(pairs))
	}

	return items, nil
}

func malformedContextualError(reason string) adaptor.Error {
	return relaymodel.WrapperOpenAIErrorWithMessage(
		"voyage contextualized embeddings response is malformed: "+reason,
		"invalid_upstream_response",
		http.StatusBadGateway,
	)
}
