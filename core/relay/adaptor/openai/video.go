package openai

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func ConvertVideoGenerationJobRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	return convertOpenAIVideoRequest(meta, req)
}

func ConvertVideosRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	return convertOpenAIVideoRequest(meta, req)
}

func ConvertVideosRemixRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	return convertOpenAIVideoRequest(meta, req)
}

func ConvertVideosEditRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	return convertOpenAIVideoRequest(meta, req)
}

func ConvertVideosExtensionRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	return convertOpenAIVideoRequest(meta, req)
}

func convertOpenAIVideoRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	if strings.HasPrefix(req.Header.Get("Content-Type"), "multipart/form-data") {
		return convertMultipartOpenAIVideoRequest(meta, req)
	}

	node, err := common.UnmarshalRequest2NodeReusable(req)
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

func convertMultipartOpenAIVideoRequest(
	meta *meta.Meta,
	request *http.Request,
) (adaptor.ConvertResult, error) {
	if err := common.ParseMultipartFormWithLimit(request); err != nil {
		return adaptor.ConvertResult{}, convertRequestError(
			meta,
			fmt.Sprintf("parse multipart form: %s", err),
		)
	}

	multipartBody := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(multipartBody)

	if err := processFormValues(multipartWriter, request.MultipartForm.Value, meta); err != nil {
		return adaptor.ConvertResult{}, fmt.Errorf("process form values: %w", err)
	}

	if err := processFormFiles(multipartWriter, request.MultipartForm.File); err != nil {
		return adaptor.ConvertResult{}, fmt.Errorf("process form files: %w", err)
	}

	if err := multipartWriter.Close(); err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type": {multipartWriter.FormDataContentType()},
		},
		Body: multipartBody,
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

func ConvertVideoNoBodyRequest(
	_ *meta.Meta,
	_ *http.Request,
) (adaptor.ConvertResult, error) {
	return adaptor.ConvertResult{}, nil
}

func ConvertVideosGetRequest(meta *meta.Meta, req *http.Request) (adaptor.ConvertResult, error) {
	return ConvertVideoNoBodyRequest(meta, req)
}

func ConvertVideosContentRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	return ConvertVideoNoBodyRequest(meta, req)
}

func VideoHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusCreated {
		return adaptor.DoResponseResult{}, VideoErrorHanlder(resp)
	}

	defer resp.Body.Close()

	responseBody, err := common.GetResponseBody(resp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	node, err := common.GetJSONNodeNoCopy(responseBody)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	idNode := node.Get("id")

	id, err := idNode.String()
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	err = store.SaveStore(adaptor.StoreCache{
		ID:        model.VideoJobStoreID(id),
		GroupID:   meta.Group.ID,
		TokenID:   meta.Token.ID,
		ChannelID: meta.Channel.ID,
		Model:     meta.OriginModel,
		ExpiresAt: time.Now().Add(time.Hour * 24),
	}, meta.Channel.Scope)
	if err != nil {
		log := common.GetLogger(c)
		log.Errorf("save store failed: %v", err)
	}

	responseBody, err = rewriteTopLevelModelNode(responseBody, &node, responseModelName(meta))
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(responseBody)))
	_, _ = c.Writer.Write(responseBody)

	return adaptor.DoResponseResult{
		UpstreamID: id,
		AsyncUsage: true,
	}, nil
}

func VideosHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	return videosHandler(meta, store, c, resp)
}

func VideosRemixHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	return videosHandler(meta, store, c, resp)
}

func VideosEditHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	return videosHandler(meta, store, c, resp)
}

func VideosExtensionHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	return videosHandler(meta, store, c, resp)
}

func videosHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return adaptor.DoResponseResult{}, VideoErrorHanlder(resp)
	}

	defer resp.Body.Close()

	responseBody, err := common.GetResponseBody(resp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	node, err := common.GetJSONNodeNoCopy(responseBody)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	idNode := node.Get("id")

	id, err := idNode.String()
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	if err := saveOpenAIVideoStore(meta, store, id); err != nil {
		log := common.GetLogger(c)
		log.Errorf("save video store failed: %v", err)
	}

	responseBody, err = rewriteTopLevelModelNode(responseBody, &node, responseModelName(meta))
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(responseBody)))
	_, _ = c.Writer.Write(responseBody)

	return adaptor.DoResponseResult{
		UpstreamID: id,
		AsyncUsage: true,
	}, nil
}

func saveOpenAIVideoStore(meta *meta.Meta, store adaptor.Store, videoID string) error {
	if store == nil || videoID == "" {
		return nil
	}

	return store.SaveStore(adaptor.StoreCache{
		ID:        model.VideoGenerationStoreID(videoID),
		GroupID:   meta.Group.ID,
		TokenID:   meta.Token.ID,
		ChannelID: meta.Channel.ID,
		Model:     meta.OriginModel,
		ExpiresAt: time.Now().Add(time.Hour * 24 * 7),
	}, meta.Channel.Scope)
}

func VideoGetJobsHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, VideoErrorHanlder(resp)
	}

	defer resp.Body.Close()

	responseBody, err := common.GetResponseBody(resp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	node, err := common.GetJSONNodeNoCopy(responseBody)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	expiresAt, err := node.Get("expires_at").Int64()
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
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
			ID:        model.VideoGenerationStoreID(id),
			GroupID:   meta.Group.ID,
			TokenID:   meta.Token.ID,
			ChannelID: meta.Channel.ID,
			Model:     meta.OriginModel,
			ExpiresAt: time.Unix(expiresAt, 0),
		}, meta.Channel.Scope)
		if err != nil {
			log := common.GetLogger(c)
			log.Errorf("save store failed: %v", err)
		}

		return true
	})
	if err == nil {
		err = patchErr
	}

	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	responseBody, err = rewriteTopLevelModelNode(responseBody, &node, responseModelName(meta))
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(responseBody)))
	_, _ = c.Writer.Write(responseBody)

	return adaptor.DoResponseResult{}, nil
}

func VideoGetJobsContentHandler(
	_ *meta.Meta,
	_ adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, VideoErrorHanlder(resp)
	}

	defer resp.Body.Close()

	c.Writer.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	c.Writer.Header().Set("Content-Length", resp.Header.Get("Content-Length"))
	_, _ = io.Copy(c.Writer, resp.Body)

	return adaptor.DoResponseResult{}, nil
}

func VideosGetHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	return videoObjectHandler(meta, c, resp)
}

func VideosContentHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	return videoContentHandler(c, resp)
}

func videoObjectHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, VideoErrorHanlder(resp)
	}

	defer resp.Body.Close()

	responseBody, err := common.GetResponseBody(resp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	responseBody, err = rewriteTopLevelModel(responseBody, responseModelName(meta))
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", firstNonEmptyString(
		resp.Header.Get("Content-Type"),
		"application/json",
	))
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(responseBody)))
	_, _ = c.Writer.Write(responseBody)

	return adaptor.DoResponseResult{}, nil
}

func videoContentHandler(
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, VideoErrorHanlder(resp)
	}

	defer resp.Body.Close()

	c.Writer.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	c.Writer.Header().Set("Content-Length", resp.Header.Get("Content-Length"))
	_, _ = io.Copy(c.Writer, resp.Body)

	return adaptor.DoResponseResult{}, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}

func VideoDeleteHandler(
	_ *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return adaptor.DoResponseResult{}, VideoErrorHanlder(resp)
	}

	defer resp.Body.Close()

	c.Writer.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	c.Writer.Header().Set("Content-Length", resp.Header.Get("Content-Length"))

	if resp.StatusCode == http.StatusNoContent {
		c.Status(http.StatusNoContent)
	}

	_, _ = io.Copy(c.Writer, resp.Body)

	return adaptor.DoResponseResult{}, nil
}
