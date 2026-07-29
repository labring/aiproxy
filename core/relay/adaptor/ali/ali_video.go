package ali

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func ConvertAliNativeVideoRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	body, err := common.UnmarshalRequest2NodeReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	if _, err := body.Set("model", ast.NewString(meta.ActualModel)); err != nil {
		return adaptor.ConvertResult{}, err
	}

	setAliNativeVideoRequestMetadata(meta, &body)

	data, err := body.MarshalJSON()
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"X-Dashscope-Async": {"enable"},
			"Content-Type":      {"application/json"},
			"Content-Length":    {strconv.Itoa(len(data))},
		},
		Body: bytes.NewReader(data),
	}, nil
}

func AliNativeVideoHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	body, relayErr := readAliNativeVideoResponseBody(resp)
	if relayErr != nil {
		return adaptor.DoResponseResult{}, relayErr
	}

	var aliResponse relaymodel.AliVideoTaskResponse
	if err := sonic.Unmarshal(body, &aliResponse); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			relaymodel.ErrorCodeBadResponse,
			http.StatusInternalServerError,
			relaymodel.ErrorTypeUpstream,
		)
	}

	if aliResponse.Code != "" || aliResponse.Message != "" {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			firstNonEmpty(aliResponse.Message, aliResponse.Code),
			relaymodel.ErrorCodeBadResponse,
			http.StatusInternalServerError,
			relaymodel.ErrorTypeUpstream,
		)
	}

	taskID := strings.TrimSpace(aliResponse.Output.TaskID)
	if taskID == "" {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"missing output.task_id in ali video response",
			relaymodel.ErrorCodeBadResponse,
			http.StatusInternalServerError,
			relaymodel.ErrorTypeUpstream,
		)
	}

	if err := saveAliNativeVideoStore(meta, store, taskID, aliVideoTaskExpiresAt(nil)); err != nil {
		common.GetLogger(c).Errorf("save ali native video store failed: %v", err)
	}

	writeAliNativeJSONResponse(c, resp, body)

	return adaptor.DoResponseResult{
		UpstreamID: taskID,
		AsyncUsage: true,
		UsageContext: aliNativeVideoUsageContext(aliResponse.Usage).
			WithFallback(aliNativeVideoRequestUsageContext(meta)),
	}, nil
}

func AliNativeVideoTaskHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	body, relayErr := readAliNativeVideoResponseBody(resp)
	if relayErr != nil {
		return adaptor.DoResponseResult{}, relayErr
	}

	var aliResponse relaymodel.AliVideoTaskResponse
	if err := sonic.Unmarshal(body, &aliResponse); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			relaymodel.ErrorCodeBadResponse,
			http.StatusInternalServerError,
			relaymodel.ErrorTypeUpstream,
		)
	}

	if aliResponse.Output.TaskID == "" {
		aliResponse.Output.TaskID = meta.VideoID
	}

	taskID := strings.TrimSpace(aliResponse.Output.TaskID)
	if taskID == "" {
		taskID = meta.VideoID
	}

	applyStoredAliVideoRequestMetadata(
		meta,
		store,
		coremodel.VideoGenerationStoreID(taskID),
	)

	if taskID != "" {
		if err := saveAliNativeVideoStore(
			meta,
			store,
			taskID,
			aliVideoTaskExpiresAt(&aliResponse),
		); err != nil {
			common.GetLogger(c).Errorf("save ali native video store failed: %v", err)
		}
	}

	writeAliNativeJSONResponse(c, resp, body)

	return adaptor.DoResponseResult{
		UpstreamID: firstNonEmpty(taskID, meta.VideoID),
		UsageContext: aliNativeVideoUsageContext(aliResponse.Usage).
			WithFallback(aliNativeVideoRequestUsageContext(meta)),
	}, nil
}

func setAliNativeVideoRequestMetadata(meta *meta.Meta, body *ast.Node) {
	if meta == nil {
		return
	}

	if input := body.Get(
		"input",
	); input != nil && input.Exists() &&
		input.TypeSafe() != ast.V_NULL {
		if prompt := aliNativeVideoString(input.Get("prompt")); prompt != "" {
			meta.Set(metaAliVideoPrompt, prompt)
		}
	}

	parameters := body.Get("parameters")
	if parameters == nil || !parameters.Exists() || parameters.TypeSafe() == ast.V_NULL {
		return
	}

	size := firstNonEmpty(
		aliNativeVideoString(parameters.Get("size")),
		aliNativeVideoString(parameters.Get("resolution")),
	)
	if size != "" {
		meta.Set(metaAliVideoSize, size)
	}

	if duration := aliNativeVideoInt(parameters.Get("duration")); duration > 0 {
		meta.Set(metaAliVideoSeconds, duration)
	}
}

func aliNativeVideoString(node *ast.Node) string {
	if node == nil || !node.Exists() || node.TypeSafe() == ast.V_NULL {
		return ""
	}

	value, err := node.String()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(value)
}

func aliNativeVideoInt(node *ast.Node) int {
	if node == nil || !node.Exists() || node.TypeSafe() == ast.V_NULL {
		return 0
	}

	if node.TypeSafe() == ast.V_STRING {
		value, err := node.String()
		if err != nil {
			return 0
		}

		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || parsed <= 0 {
			return 0
		}

		return parsed
	}

	value, err := node.Int64()
	if err != nil || value <= 0 {
		return 0
	}

	return int(value)
}

func aliNativeVideoUsageContext(usage relaymodel.AliVideoUsage) coremodel.UsageContext {
	nativeResolution := aliVideoNativeResolution(usage)
	if nativeResolution == "" {
		return coremodel.UsageContext{}
	}

	return coremodel.UsageContext{
		Resolution:       nativeResolution,
		NativeResolution: nativeResolution,
	}
}

func aliNativeVideoRequestUsageContext(meta *meta.Meta) coremodel.UsageContext {
	if meta == nil {
		return coremodel.UsageContext{}
	}

	nativeResolution := strings.TrimSpace(meta.GetString(metaAliVideoSize))
	if nativeResolution == "" {
		return coremodel.UsageContext{}
	}

	return coremodel.UsageContext{
		Resolution:       nativeResolution,
		NativeResolution: nativeResolution,
	}
}

func readAliNativeVideoResponseBody(resp *http.Response) ([]byte, adaptor.Error) {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	body, err := common.GetResponseBody(resp)
	if err != nil {
		return nil, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			relaymodel.ErrorCodeBadResponse,
			resp.StatusCode,
			relaymodel.ErrorTypeUpstream,
		)
	}

	return body, nil
}

func saveAliNativeVideoStore(
	meta *meta.Meta,
	store adaptor.Store,
	taskID string,
	expiresAt time.Time,
) error {
	if store == nil || meta == nil || taskID == "" {
		return nil
	}

	return store.SaveStore(adaptor.StoreCache{
		ID:        coremodel.VideoGenerationStoreID(taskID),
		GroupID:   meta.Group.ID,
		TokenID:   meta.Token.ID,
		ChannelID: meta.Channel.ID,
		Model:     meta.OriginModel,
		Metadata:  aliVideoStoreMetadataString(meta, taskID),
		ExpiresAt: expiresAt,
	}, meta.Channel.Scope)
}

func aliVideoTaskExpiresAt(response *relaymodel.AliVideoTaskResponse) time.Time {
	if response != nil && response.Output.EndTime != "" {
		if end, err := time.Parse(time.RFC3339, response.Output.EndTime); err == nil {
			return end.Add(aliVideoTaskTTL)
		}
	}

	return time.Now().Add(aliVideoTaskTTL)
}

func writeAliNativeJSONResponse(c *gin.Context, resp *http.Response, body []byte) {
	contentType := "application/json"
	if resp != nil && resp.Header.Get("Content-Type") != "" {
		contentType = resp.Header.Get("Content-Type")
	}

	c.Writer.Header().Set("Content-Type", contentType)
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = c.Writer.Write(body)
}
