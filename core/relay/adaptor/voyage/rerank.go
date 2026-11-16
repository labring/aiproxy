package voyage

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

var (
	errMissingData  = errors.New("voyage rerank response has no data array")
	errMissingIndex = errors.New("voyage rerank result has no integer index")
	errMissingScore = errors.New("voyage rerank result has no relevance_score")
)

// ConvertRerankRequest adapts a rerank request with string documents to POST
// /v1/rerank (Voyage reranks text only). `top_n` becomes Voyage `top_k` unless
// `top_k` is set; `query`, `documents`, `return_documents` and `truncation` are
// passed through.
func ConvertRerankRequest(
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

	topN := node.Get("top_n")
	if topN.Exists() {
		if !node.Get("top_k").Exists() && topN.TypeSafe() != ast.V_NULL {
			if _, err := node.Set("top_k", *topN); err != nil {
				return adaptor.ConvertResult{}, err
			}
		}

		if _, err := node.Unset("top_n"); err != nil {
			return adaptor.ConvertResult{}, err
		}
	}

	return marshalConvertResult(&node)
}

// RerankHandler converts the Voyage rerank response
// {"data":[{"index","relevance_score","document"?}],"usage":{"total_tokens"}} to the
// Cohere/Jina shape the relay consumers expect:
// {"results":[{"index","relevance_score","document":{"text"}?}],"meta":{"tokens":{...}}}.
// A response without usage.total_tokens fails instead of being billed as zero.
func RerankHandler(
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
		return adaptor.DoResponseResult{}, missingUsageError("rerank")
	}

	modelUsage := model.Usage{
		InputTokens: model.ZeroNullInt64(totalTokens),
		TotalTokens: model.ZeroNullInt64(totalTokens),
	}

	results, err := buildRerankResults(&node)
	if err != nil {
		return adaptor.DoResponseResult{Usage: modelUsage}, relaymodel.WrapperOpenAIError(
			err, "invalid_rerank_response", http.StatusBadGateway,
		)
	}

	out := ast.NewObject([]ast.Pair{
		ast.NewPair("model", ast.NewString(meta.OriginModel)),
		ast.NewPair("results", ast.NewArray(results)),
		ast.NewPair("meta", ast.NewObject([]ast.Pair{
			ast.NewPair("tokens", ast.NewAny(modelUsage)),
		})),
	})

	data, err := out.MarshalJSON()
	if err != nil {
		return adaptor.DoResponseResult{Usage: modelUsage}, relaymodel.WrapperOpenAIError(
			err, "marshal_response_failed", http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(data)))

	if _, err := c.Writer.Write(data); err != nil {
		common.GetLogger(c).Warnf("write response body failed: %v", err)
	}

	return adaptor.DoResponseResult{Usage: modelUsage}, nil
}

func buildRerankResults(node *ast.Node) ([]ast.Node, error) {
	data := node.Get("data")
	if !data.Exists() || data.TypeSafe() != ast.V_ARRAY {
		return nil, errMissingData
	}

	var results []ast.Node

	var walkErr error

	err := data.ForEach(func(_ ast.Sequence, item *ast.Node) bool {
		index, err := item.Get("index").Int64()
		if err != nil {
			walkErr = errMissingIndex
			return false
		}

		score, err := item.Get("relevance_score").Float64()
		if err != nil {
			walkErr = errMissingScore
			return false
		}

		pairs := []ast.Pair{
			ast.NewPair("index", ast.NewNumber(strconv.FormatInt(index, 10))),
			ast.NewPair("relevance_score", ast.NewNumber(strconv.FormatFloat(score, 'g', -1, 64))),
		}

		if doc := item.Get("document"); doc.Exists() && doc.TypeSafe() == ast.V_STRING {
			text, _ := doc.String()
			pairs = append(pairs, ast.NewPair("document", ast.NewObject([]ast.Pair{
				ast.NewPair("text", ast.NewString(text)),
			})))
		}

		results = append(results, ast.NewObject(pairs))

		return true
	})
	if err != nil {
		return nil, err
	}

	if walkErr != nil {
		return nil, walkErr
	}

	return results, nil
}
