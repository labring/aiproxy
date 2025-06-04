package openai

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
)

func ConvertTTSRequest(
	meta *meta.Meta,
	req *http.Request,
	defaultVoice string,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalBody2Node(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	voice, err := node.Get("voice").String()
	if err != nil && !errors.Is(err, ast.ErrNotExist) {
		return adaptor.ConvertResult{}, err
	}
	if voice == "" && defaultVoice != "" {
		_, err = node.Set("voice", ast.NewString(defaultVoice))
		if err != nil {
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

func TTSHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := middleware.GetLogger(c)

	for k, v := range resp.Header {
		c.Writer.Header().Set(k, v[0])
	}

	_, err := io.Copy(c.Writer, resp.Body)
	if err != nil {
		log.Warnf("write response body failed: %v", err)
	}
	return model.Usage{
		InputTokens: meta.RequestUsage.InputTokens,
		TotalTokens: meta.RequestUsage.InputTokens,
	}, nil
}
