package openai

import (
	"bytes"
	"io"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
)

// ConvertWebSearchRequest strips the "model" field from the JSON body
// before forwarding to the upstream web-search endpoint.
func ConvertWebSearchRequest(
	_ *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	body, err := common.GetRequestBodyReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	node, err := sonic.Get(body)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	// Upstream API does not accept a "model" field — remove it.
	existed, _ := node.Unset("model")
	if !existed {
		return adaptor.ConvertResult{
			Body: bytes.NewReader(body),
		}, nil
	}

	out, err := node.MarshalJSON()
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{"Content-Type": {"application/json"}},
		Body:   bytes.NewReader(out),
	}, nil
}

// WebSearchHandler passes the upstream response through to the client unchanged.
func WebSearchHandler(
	_ *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	c.Writer.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		c.Writer.Header().Set("Content-Length", cl)
	}

	_, _ = io.Copy(c.Writer, resp.Body)

	return adaptor.DoResponseResult{}, nil
}
