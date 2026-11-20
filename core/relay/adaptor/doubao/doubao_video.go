package doubao

import (
	"bytes"
	"net/http"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func ConvertDoubaoNativeVideoRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	body, err := common.UnmarshalRequest2ValidatedNode(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	if _, err := body.Set("model", ast.NewString(meta.ActualModel)); err != nil {
		return adaptor.ConvertResult{}, err
	}

	setDoubaoNativeVideoRequestMetadata(meta, &body)

	data, err := body.MarshalJSON()
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(data))},
		},
		Body: bytes.NewReader(data),
	}, nil
}

func DoubaoNativeVideoSubmitHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	body, response, relayErr := readDoubaoNativeVideoTaskResponse(resp, true)
	if relayErr != nil {
		return adaptor.DoResponseResult{}, relayErr
	}

	expiresAt := doubaoVideoExpiresAt(response)
	if err := saveDoubaoVideoStore(meta, store, response.ID, expiresAt); err != nil {
		common.GetLogger(c).Errorf("save doubao native video store failed: %v", err)
	}

	writeDoubaoNativeJSONResponse(c, resp, body)

	return adaptor.DoResponseResult{
		UpstreamID: response.ID,
		AsyncUsage: true,
		UsageContext: doubaoNativeVideoUsageContext(
			&response,
		).WithFallback(doubaoNativeVideoRequestUsageContext(meta)),
	}, nil
}

func DoubaoNativeVideoTaskHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	body, response, relayErr := readDoubaoNativeVideoTaskResponse(resp, false)
	if relayErr != nil {
		return adaptor.DoResponseResult{}, relayErr
	}

	if response.ID == "" {
		response.ID = meta.VideoID
	}

	applyStoredDoubaoVideoMetadata(
		meta,
		store,
		coremodel.VideoGenerationStoreID(response.ID),
		&response,
	)

	if response.ID != "" {
		expiresAt := doubaoVideoExpiresAt(response)
		if err := saveDoubaoVideoStore(meta, store, response.ID, expiresAt); err != nil {
			common.GetLogger(c).Errorf("save doubao native video store failed: %v", err)
		}
	}

	writeDoubaoNativeJSONResponse(c, resp, body)

	return adaptor.DoResponseResult{
		UpstreamID: response.ID,
		UsageContext: doubaoNativeVideoUsageContext(
			&response,
		).WithFallback(doubaoNativeVideoRequestUsageContext(meta)),
	}, nil
}

func setDoubaoNativeVideoRequestMetadata(meta *meta.Meta, body *ast.Node) {
	if meta == nil {
		return
	}

	metadata := doubaoVideoStoreMetadata{
		Prompt:     doubaoVideoPrompt(doubaoNativeVideoContent(body.Get("content"))),
		Resolution: doubaoNativeVideoString(body.Get("resolution")),
		Ratio:      doubaoNativeVideoString(body.Get("ratio")),
		Duration:   doubaoNativeVideoInt(body.Get("duration")),
		ServiceTier: firstNonEmptyString(
			doubaoNativeVideoString(body.Get("service_tier")),
			"default",
		),
		InputVideo:  new(doubaoNativeVideoContentHasVideo(body.Get("content"))),
		OutputAudio: doubaoNativeVideoOutputAudio(body.Get("generate_audio")),
	}

	setDoubaoVideoMetadata(meta, metadata)
}

func doubaoNativeVideoContent(node *ast.Node) []doubaoVideoContent {
	if node == nil || !node.Exists() || node.TypeSafe() == ast.V_NULL {
		return nil
	}

	count, err := node.Len()
	if err != nil || count <= 0 {
		return nil
	}

	content := make([]doubaoVideoContent, 0, count)
	for i := range count {
		item := node.Index(i)
		if item == nil || !item.Exists() || item.TypeSafe() == ast.V_NULL {
			continue
		}

		content = append(content, doubaoVideoContent{
			Type: doubaoNativeVideoString(item.Get("type")),
			Text: doubaoNativeVideoString(item.Get("text")),
		})
	}

	return content
}

func doubaoNativeVideoString(node *ast.Node) string {
	if node == nil || !node.Exists() || node.TypeSafe() == ast.V_NULL {
		return ""
	}

	value, err := node.String()
	if err != nil {
		return ""
	}

	return value
}

func doubaoNativeVideoInt(node *ast.Node) int {
	if node == nil || !node.Exists() || node.TypeSafe() == ast.V_NULL {
		return 0
	}

	value, err := node.Int64()
	if err != nil {
		return 0
	}

	return int(value)
}

func doubaoNativeVideoUsageContext(
	response *relaymodel.DoubaoVideoTaskResponse,
) coremodel.UsageContext {
	usageContext := doubaoVideoUsageContext(response)
	return doubaoNativeVideoUsageContextFromContext(usageContext)
}

func doubaoNativeVideoRequestUsageContext(meta *meta.Meta) coremodel.UsageContext {
	usageContext := doubaoVideoRequestUsageContext(meta)
	return doubaoNativeVideoUsageContextFromContext(usageContext)
}

func doubaoNativeVideoUsageContextFromContext(
	usageContext coremodel.UsageContext,
) coremodel.UsageContext {
	nativeResolution := usageContext.NativeResolution
	if nativeResolution == "" {
		nativeResolution = usageContext.Resolution
	}

	if nativeResolution == "" &&
		usageContext.ServiceTier == "" &&
		usageContext.Quality == "" &&
		usageContext.InputVideo == nil &&
		usageContext.OutputAudio == nil {
		return coremodel.UsageContext{}
	}

	return coremodel.UsageContext{
		Resolution:       nativeResolution,
		NativeResolution: nativeResolution,
		ServiceTier:      usageContext.ServiceTier,
		Quality:          usageContext.Quality,
		InputVideo:       usageContext.InputVideo,
		OutputAudio:      usageContext.OutputAudio,
	}
}

func doubaoNativeVideoContentHasVideo(node *ast.Node) bool {
	if node == nil || !node.Exists() || node.TypeSafe() != ast.V_ARRAY {
		return false
	}

	hasVideo := false

	_ = node.ForEach(func(_ ast.Sequence, item *ast.Node) bool {
		if item == nil || !item.Exists() || item.TypeSafe() != ast.V_OBJECT {
			return true
		}

		typeNode := item.Get("type")
		if typeNode.Exists() && typeNode.TypeSafe() == ast.V_STRING {
			itemType, err := typeNode.String()
			if err == nil && (itemType == "video_url" || itemType == "draft_task") {
				hasVideo = true
				return false
			}
		}

		if videoNode := item.Get(
			"video_url",
		); videoNode.Exists() &&
			videoNode.TypeSafe() != ast.V_NULL {
			hasVideo = true
			return false
		}

		if draftTaskNode := item.Get(
			"draft_task",
		); draftTaskNode.Exists() &&
			draftTaskNode.TypeSafe() != ast.V_NULL {
			hasVideo = true
			return false
		}

		return true
	})

	return hasVideo
}

func doubaoNativeVideoOutputAudio(node *ast.Node) *bool {
	if node == nil {
		return new(true)
	}

	value, err := node.Bool()
	if err != nil {
		return new(true)
	}

	return &value
}

func DoubaoNativeVideoTaskDeleteHandler(
	_ *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return adaptor.DoResponseResult{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	body, err := common.GetResponseBody(resp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			relaymodel.ErrorCodeBadResponse,
			resp.StatusCode,
			relaymodel.ErrorTypeUpstream,
		)
	}

	if resp.StatusCode == http.StatusNoContent {
		c.Writer.WriteHeader(http.StatusNoContent)
		return adaptor.DoResponseResult{}, nil
	}

	writeDoubaoNativeJSONResponse(c, resp, body)

	return adaptor.DoResponseResult{}, nil
}

func readDoubaoNativeVideoTaskResponse(
	resp *http.Response,
	requireID bool,
) ([]byte, relaymodel.DoubaoVideoTaskResponse, adaptor.Error) {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, relaymodel.DoubaoVideoTaskResponse{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	body, err := common.GetResponseBody(resp)
	if err != nil {
		return nil, relaymodel.DoubaoVideoTaskResponse{}, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			relaymodel.ErrorCodeBadResponse,
			resp.StatusCode,
			relaymodel.ErrorTypeUpstream,
		)
	}

	var response relaymodel.DoubaoVideoTaskResponse
	if err := sonic.Unmarshal(body, &response); err != nil {
		return nil, relaymodel.DoubaoVideoTaskResponse{}, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			relaymodel.ErrorCodeBadResponse,
			http.StatusInternalServerError,
			relaymodel.ErrorTypeUpstream,
		)
	}

	if requireID && response.ID == "" {
		return nil, relaymodel.DoubaoVideoTaskResponse{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"missing id in doubao video response",
			relaymodel.ErrorCodeBadResponse,
			http.StatusInternalServerError,
			relaymodel.ErrorTypeUpstream,
		)
	}

	return body, response, nil
}

func writeDoubaoNativeJSONResponse(c *gin.Context, resp *http.Response, body []byte) {
	contentType := "application/json"
	if resp != nil && resp.Header.Get("Content-Type") != "" {
		contentType = resp.Header.Get("Content-Type")
	}

	c.Writer.Header().Set("Content-Type", contentType)
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = c.Writer.Write(body)
}
