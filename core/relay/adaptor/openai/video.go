package openai

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func ConvertVideoRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalBody2Node(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	_, err = node.Set("model", ast.NewString(meta.ActualModel))
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	jsonData, err := sonic.Marshal(&node)
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

func ConvertVideoGetJobsRequest(
	_ *meta.Meta,
	_ *http.Request,
) (adaptor.ConvertResult, error) {
	return adaptor.ConvertResult{}, nil
}

func ConvertVideoGetJobsContentRequest(
	_ *meta.Meta,
	_ *http.Request,
) (adaptor.ConvertResult, error) {
	return adaptor.ConvertResult{}, nil
}

func VideoHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusCreated {
		return model.Usage{}, VideoErrorHanlder(resp)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	node, err := sonic.Get(responseBody)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}
	idNode := node.Get("id")
	id, err := idNode.String()
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	err = store.SaveStore(adaptor.StoreCache{
		ID:        id,
		GroupID:   meta.Group.ID,
		TokenID:   meta.Token.ID,
		ChannelID: meta.Channel.ID,
		Model:     meta.ActualModel,
		ExpiresAt: time.Now().Add(time.Hour * 24),
	})
	if err != nil {
		log := middleware.GetLogger(c)
		log.Errorf("save store failed: %v", err)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(responseBody)))
	_, _ = c.Writer.Write(responseBody)
	return model.Usage{}, nil
}

func VideoGetJobsHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, VideoErrorHanlder(resp)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	node, err := sonic.Get(responseBody)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	expiresAt, err := node.Get("expires_at").Int64()
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	generationsNode := node.Get("generations")
	var patchErr error
	err = generationsNode.ForEach(func(_ ast.Sequence, node *ast.Node) bool {
		idNode := node.Get("id")
		id, err := idNode.String()
		if err != nil {
			patchErr = err
			return false
		}
		err = store.SaveStore(adaptor.StoreCache{
			ID:        id,
			GroupID:   meta.Group.ID,
			TokenID:   meta.Token.ID,
			ChannelID: meta.Channel.ID,
			Model:     meta.ActualModel,
			ExpiresAt: time.Unix(expiresAt, 0),
		})
		if err != nil {
			log := middleware.GetLogger(c)
			log.Errorf("save store failed: %v", err)
		}
		return true
	})
	if err == nil {
		err = patchErr
	}
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(responseBody)))
	_, _ = c.Writer.Write(responseBody)
	return model.Usage{}, nil
}

func VideoGetJobsContentHandler(
	_ *meta.Meta,
	_ adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, VideoErrorHanlder(resp)
	}

	c.Writer.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	_, _ = io.Copy(c.Writer, resp.Body)
	return model.Usage{}, nil
}
