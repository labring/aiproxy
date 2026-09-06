package vertexai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/gemini"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	vertexgemini "github.com/labring/aiproxy/core/relay/adaptor/vertexai/gemini"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
	log "github.com/sirupsen/logrus"
)

type Adaptor struct{}

func init() {
	registry.Register(model.ChannelTypeVertexAI, &Adaptor{})
}

func (a *Adaptor) DefaultBaseURL() string {
	return ""
}

func (a *Adaptor) SupportMode(mt *meta.Meta) bool {
	m := adaptor.ModeFromMeta(mt)

	if m == mode.AudioSpeech {
		return gemini.IsTTSMetaForAdaptor(mt)
	}

	if m == mode.ImagesGenerations {
		return gemini.IsImageMetaForAdaptor(mt)
	}

	return m == mode.ChatCompletions ||
		m == mode.Anthropic ||
		m == mode.Gemini ||
		m == mode.GeminiFiles ||
		m == mode.GeminiVideo ||
		m == mode.GeminiVideoOperations ||
		m == mode.GeminiTTS ||
		m == mode.GeminiImage ||
		m == mode.VideoGenerationsJobs ||
		m == mode.VideoGenerationsGetJobs ||
		m == mode.VideoGenerationsContent ||
		m == mode.Videos ||
		m == mode.VideosGet ||
		m == mode.VideosContent
}

type Config struct {
	Region    string
	Key       string
	ProjectID string
	ADCJSON   string
}

func resolveFeatureModel(meta *meta.Meta) string {
	if meta == nil {
		return ""
	}

	if modelName := utils.FirstMatchingModelName(
		func(modelName string) bool {
			modelName = strings.ToLower(modelName)
			return strings.Contains(modelName, "gemini") || strings.Contains(modelName, "claude")
		},
		meta.OriginModel,
		meta.ActualModel,
	); modelName != "" {
		return modelName
	}

	return utils.PreferredModelName(meta.OriginModel, meta.ActualModel)
}

func innerAdaptor(meta *meta.Meta) innerAIAdapter {
	if meta == nil {
		return nil
	}

	switch meta.Mode {
	case mode.GeminiVideo,
		mode.GeminiFiles,
		mode.GeminiVideoOperations,
		mode.GeminiTTS,
		mode.GeminiImage,
		mode.AudioSpeech,
		mode.ImagesGenerations,
		mode.VideoGenerationsJobs,
		mode.VideoGenerationsGetJobs,
		mode.VideoGenerationsContent,
		mode.Videos,
		mode.VideosGet,
		mode.VideosContent:
		return &vertexgemini.Adaptor{}
	default:
		return GetAdaptor(resolveFeatureModel(meta))
	}
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	request *http.Request,
) (adaptor.ConvertResult, error) {
	aa := innerAdaptor(meta)
	if aa == nil {
		return adaptor.ConvertResult{}, errors.New("adaptor not found")
	}

	return aa.ConvertRequest(meta, store, request)
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	aa := innerAdaptor(meta)
	if aa == nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			meta.ActualModel+" adaptor not found",
			"adaptor_not_found",
			http.StatusInternalServerError,
		)
	}

	return aa.DoResponse(meta, store, c, resp)
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme:  "Google Vertex AI unified adaptor\nRoutes Gemini and Claude models to Vertex AI publisher endpoints\nSupports OpenAI-compatible chat plus Anthropic-compatible and Gemini-compatible request conversion\nKey format: `region|adcJSON`, `region|apikey`, or `region|project_id|apikey`",
		KeyHelp: "region|adcJSON or region|apikey or region|project_id|apikey",
		Models:  modelList,
	}
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
) (adaptor.RequestURL, error) {
	config, err := getConfigFromKey(meta.Channel.Key)
	if err != nil {
		return adaptor.RequestURL{}, err
	}

	if meta.Mode == mode.GeminiFiles {
		return (&gemini.Adaptor{}).GetRequestURL(meta, store, c)
	}

	featureModel := resolveFeatureModel(meta)

	publisher := "google"
	if strings.Contains(strings.ToLower(featureModel), "claude") {
		publisher = "anthropic"
	}

	if operationID, err := vertexOperationID(meta, store); err != nil {
		return adaptor.RequestURL{}, err
	} else if operationID != "" {
		return a.getOperationRequestURL(meta, config, publisher, operationID)
	}

	suffix := vertexRequestSuffix(meta, c, featureModel)

	return a.getModelActionRequestURL(meta, config, publisher, suffix), nil
}

func (a *Adaptor) getOperationRequestURL(
	meta *meta.Meta,
	config Config,
	publisher string,
	operationName string,
) (adaptor.RequestURL, error) {
	if operationName == "" {
		return adaptor.RequestURL{}, errors.New("operation name is empty")
	}

	if strings.HasPrefix(operationName, "projects/") ||
		strings.HasPrefix(operationName, "publishers/") {
		if meta.Channel.BaseURL != "" {
			return adaptor.RequestURL{
				Method: http.MethodGet,
				URL:    fmt.Sprintf("%s/v1/%s", meta.Channel.BaseURL, operationName),
			}, nil
		}

		requestDomain := "aiplatform.googleapis.com"
		if config.Region != "" && config.Region != "global" {
			requestDomain = config.Region + "-aiplatform.googleapis.com"
		}

		return adaptor.RequestURL{
			Method: http.MethodGet,
			URL:    fmt.Sprintf("https://%s/v1/%s", requestDomain, operationName),
		}, nil
	}

	operationName = vertexModelScopedOperationName(operationName)

	if meta.Channel.BaseURL != "" {
		if config.ProjectID == "" || config.Region == "" {
			return adaptor.RequestURL{
				Method: http.MethodGet,
				URL: fmt.Sprintf(
					"%s/v1/publishers/%s/models/%s/%s",
					meta.Channel.BaseURL,
					publisher,
					meta.ActualModel,
					operationName,
				),
			}, nil
		}

		return adaptor.RequestURL{
			Method: http.MethodGet,
			URL: fmt.Sprintf(
				"%s/v1/projects/%s/locations/%s/publishers/%s/models/%s/%s",
				meta.Channel.BaseURL,
				config.ProjectID,
				config.Region,
				publisher,
				meta.ActualModel,
				operationName,
			),
		}, nil
	}

	requestDomain := "aiplatform.googleapis.com"
	if config.Region != "" && config.Region != "global" {
		requestDomain = config.Region + "-aiplatform.googleapis.com"
	}

	if config.ProjectID == "" || config.Region == "" {
		return adaptor.RequestURL{
			Method: http.MethodGet,
			URL: fmt.Sprintf(
				"https://%s/v1/publishers/%s/models/%s/%s",
				requestDomain,
				publisher,
				meta.ActualModel,
				operationName,
			),
		}, nil
	}

	return adaptor.RequestURL{
		Method: http.MethodGet,
		URL: fmt.Sprintf(
			"https://%s/v1/projects/%s/locations/%s/publishers/%s/models/%s/%s",
			requestDomain,
			config.ProjectID,
			config.Region,
			publisher,
			meta.ActualModel,
			operationName,
		),
	}, nil
}

func vertexModelScopedOperationName(operationName string) string {
	operationName = strings.TrimPrefix(operationName, "/")
	if !strings.HasPrefix(operationName, "models/") {
		return operationName
	}

	parts := strings.SplitN(operationName, "/", 3)
	if len(parts) != 3 || !strings.HasPrefix(parts[2], "operations/") {
		return operationName
	}

	return parts[2]
}

func (a *Adaptor) getModelActionRequestURL(
	meta *meta.Meta,
	config Config,
	publisher string,
	suffix string,
) adaptor.RequestURL {
	if meta.Channel.BaseURL != "" {
		if config.ProjectID == "" || config.Region == "" {
			return adaptor.RequestURL{
				Method: http.MethodPost,
				URL: fmt.Sprintf(
					"%s/v1/publishers/%s/models/%s:%s",
					meta.Channel.BaseURL,
					publisher,
					meta.ActualModel,
					suffix,
				),
			}
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL: fmt.Sprintf(
				"%s/v1/projects/%s/locations/%s/publishers/%s/models/%s:%s",
				meta.Channel.BaseURL,
				config.ProjectID,
				config.Region,
				publisher,
				meta.ActualModel,
				suffix,
			),
		}
	}

	var requestDoamin string
	if config.Region == "" || config.Region == "global" {
		requestDoamin = "aiplatform.googleapis.com"
	} else {
		requestDoamin = config.Region + "-aiplatform.googleapis.com"
	}

	if config.ProjectID == "" || config.Region == "" {
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL: fmt.Sprintf(
				"https://%s/v1/publishers/%s/models/%s:%s",
				requestDoamin,
				publisher,
				meta.ActualModel,
				suffix,
			),
		}
	}

	return adaptor.RequestURL{
		Method: http.MethodPost,
		URL: fmt.Sprintf(
			"https://%s/v1/projects/%s/locations/%s/publishers/%s/models/%s:%s",
			requestDoamin,
			config.ProjectID,
			config.Region,
			publisher,
			meta.ActualModel,
			suffix,
		),
	}
}

func vertexOperationID(meta *meta.Meta, store adaptor.Store) (string, error) {
	switch meta.Mode {
	case mode.GeminiVideoOperations:
		return gemini.NativeGeminiVideoUpstreamOperationName(meta, store), nil
	case mode.VideoGenerationsJobs, mode.Videos:
		return meta.JobID, nil
	case mode.VideoGenerationsGetJobs:
		return gemini.ResolveVideoJobOperationID(meta, store, meta.JobID)
	case mode.VideoGenerationsContent:
		return gemini.ResolveVideoGenerationOperationID(meta, store, meta.GenerationID)
	case mode.VideosGet:
		return gemini.ResolveVideoGenerationOperationID(meta, store, meta.VideoID)
	case mode.VideosContent:
		return gemini.ResolveVideoGenerationOperationID(meta, store, meta.VideoID)
	default:
		return "", nil
	}
}

func vertexRequestSuffix(meta *meta.Meta, c *gin.Context, featureModel string) string {
	if meta.Mode == mode.GeminiVideo ||
		meta.Mode == mode.VideoGenerationsJobs ||
		meta.Mode == mode.Videos {
		return "predictLongRunning"
	}

	isStream := meta.GetBool("stream")
	if meta.Mode == mode.Gemini && c != nil {
		isStream = strings.Contains(c.Request.URL.Path, ":stream")
	}

	if strings.HasPrefix(strings.ToLower(featureModel), "gemini") {
		if isStream {
			return "streamGenerateContent?alt=sse"
		}

		return "generateContent"
	}

	if isStream {
		return "streamRawPredict?alt=sse"
	}

	return "rawPredict"
}

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	req *http.Request,
) error {
	aa := innerAdaptor(meta)
	if aa == nil {
		return relaymodel.WrapperOpenAIErrorWithMessage(
			meta.ActualModel+" adaptor not found",
			"adaptor_not_found",
			http.StatusInternalServerError,
		)
	}

	err := aa.SetupRequestHeader(meta, store, c, req)
	if err != nil {
		return err
	}

	config, err := getConfigFromKey(meta.Channel.Key)
	if err != nil {
		return err
	}

	if config.Key != "" {
		req.Header.Set("X-Goog-Api-Key", config.Key)
		return nil
	}

	token, err := getToken(
		req.Context(),
		config.ADCJSON,
		meta.Channel.ProxyURL,
		meta.Channel.SkipTLSVerify,
	)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	return nil
}

func (a *Adaptor) DoRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) (*http.Response, error) {
	return utils.DoRequestWithMeta(req, meta)
}

var _ adaptor.AsyncUsageFetcher = (*Adaptor)(nil)

func (a *Adaptor) FetchAsyncUsage(
	ctx context.Context,
	request adaptor.AsyncUsageRequest,
) (model.Usage, model.UsageContext, bool, error) {
	info := request.Info
	if info == nil {
		return model.Usage{}, model.UsageContext{}, false, errors.New("async usage info is nil")
	}

	switch mode.Mode(info.Mode) {
	case mode.GeminiVideo, mode.VideoGenerationsJobs, mode.Videos:
	default:
		return model.Usage{}, model.UsageContext{}, false, fmt.Errorf(
			"unsupported async usage mode: %d",
			info.Mode,
		)
	}

	channel := request.Channel
	if channel == nil {
		return model.Usage{}, model.UsageContext{}, false, errors.New("channel is nil")
	}

	requestMeta := meta.NewMeta(
		channel,
		mode.Mode(info.Mode),
		info.Model,
		model.ModelConfig{Model: info.Model},
		meta.WithJobID(info.UpstreamID),
	)
	if info.BaseURL != "" {
		requestMeta.Channel.BaseURL = info.BaseURL
	}

	config, err := getConfigFromKey(channel.Key)
	if err != nil {
		return model.Usage{}, model.UsageContext{}, false, err
	}

	publisher := "google"
	if strings.Contains(strings.ToLower(resolveFeatureModel(requestMeta)), "claude") {
		publisher = "anthropic"
	}

	requestURL, err := a.getOperationRequestURL(requestMeta, config, publisher, info.UpstreamID)
	if err != nil {
		return model.Usage{}, model.UsageContext{}, false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.URL, nil)
	if err != nil {
		return model.Usage{}, model.UsageContext{}, false, err
	}

	if config.Key != "" {
		req.Header.Set("X-Goog-Api-Key", config.Key)
	} else {
		token, err := getToken(
			ctx,
			config.ADCJSON,
			requestMeta.Channel.ProxyURL,
			requestMeta.Channel.SkipTLSVerify,
		)
		if err != nil {
			return model.Usage{}, model.UsageContext{}, false, err
		}

		req.Header.Set("Authorization", "Bearer "+token)
	}

	var (
		proxyURL      string
		skipTLSVerify bool
	)
	if channel != nil {
		proxyURL = channel.ProxyURL
		skipTLSVerify = channel.SkipTLSVerify
	}

	client, err := utils.LoadHTTPClientWithTLSConfigE(0, proxyURL, skipTLSVerify)
	if err != nil {
		return model.Usage{}, model.UsageContext{}, false, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return model.Usage{}, model.UsageContext{}, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, model.UsageContext{}, false, fmt.Errorf(
			"unexpected status code: %d",
			resp.StatusCode,
		)
	}

	var operation relaymodel.GeminiVideoOperation
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&operation); err != nil {
		return model.Usage{}, model.UsageContext{}, false, fmt.Errorf(
			"decode vertexai gemini video operation: %w",
			err,
		)
	}

	if operation.Error != nil {
		return model.Usage{}, model.UsageContext{}, true, fmt.Errorf(
			"vertexai gemini video operation failed: %s",
			operation.Error.Message,
		)
	}

	if !operation.Done {
		return model.Usage{}, model.UsageContext{}, false, nil
	}

	if reason := gemini.GeminiOperationFinalFailureMessage(&operation); reason != "" {
		return model.Usage{}, model.UsageContext{}, true, fmt.Errorf(
			"vertexai gemini video operation failed: %s",
			reason,
		)
	}

	gemini.LogGeminiOperationPartialRAIFilter(log.Warnf, &operation)

	usage, usageContext := gemini.VideoAsyncUsage(request.Store, info, &operation)

	return usage, usageContext, true, nil
}
