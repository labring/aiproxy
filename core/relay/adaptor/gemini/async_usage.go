package gemini

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	relayutils "github.com/labring/aiproxy/core/relay/utils"
	log "github.com/sirupsen/logrus"
)

var _ adaptor.AsyncUsageFetcher = (*Adaptor)(nil)

func VideoAsyncUsage(
	store adaptor.Store,
	info *model.AsyncUsageInfo,
	operation *relaymodel.GeminiVideoOperation,
) (model.Usage, model.UsageContext) {
	return geminiVideoAsyncUsage(store, info, operation)
}

func (a *Adaptor) FetchAsyncUsage(
	ctx context.Context,
	request adaptor.AsyncUsageRequest,
) (model.Usage, model.UsageContext, bool, error) {
	info := request.Info
	if info == nil {
		return model.Usage{}, model.UsageContext{}, false, errors.New("async usage info is nil")
	}

	switch mode.Mode(info.Mode) {
	case mode.GeminiVideo,
		mode.VideoGenerationsJobs,
		mode.Videos,
		mode.VideosEdits,
		mode.VideosExtensions:
	default:
		return model.Usage{}, model.UsageContext{}, false, fmt.Errorf(
			"unsupported async usage mode: %d",
			info.Mode,
		)
	}

	operation, err := a.fetchVideoOperation(ctx, request.Channel, info)
	if err != nil {
		return model.Usage{}, model.UsageContext{}, false, err
	}

	if operation.Error != nil {
		return model.Usage{}, model.UsageContext{}, true, fmt.Errorf(
			"gemini video operation failed: %s",
			operation.Error.Message,
		)
	}

	if !operation.Done {
		return model.Usage{}, model.UsageContext{}, false, nil
	}

	if reason := geminiOperationFinalFailureMessage(operation); reason != "" {
		return model.Usage{}, model.UsageContext{}, true, fmt.Errorf(
			"gemini video operation failed: %s",
			reason,
		)
	}

	logGeminiOperationPartialRAIFilter(log.Warnf, operation)

	usage, usageContext := geminiVideoAsyncUsage(request.Store, info, operation)

	return usage, usageContext, true, nil
}

func (a *Adaptor) fetchVideoOperation(
	ctx context.Context,
	channel *model.Channel,
	info *model.AsyncUsageInfo,
) (*geminiOperation, error) {
	if info.UpstreamID == "" {
		return nil, errors.New("upstream id is empty")
	}

	requestMeta := meta.NewMeta(
		channel,
		mode.Mode(info.Mode),
		info.Model,
		model.ModelConfig{Model: info.Model},
	)
	if info.BaseURL != "" {
		requestMeta.Channel.BaseURL = info.BaseURL
	}

	requestURL, err := getOperationRequestURL(requestMeta, info.UpstreamID)
	if err != nil {
		return nil, fmt.Errorf("build gemini video operation url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.URL, nil)
	if err != nil {
		return nil, err
	}

	if channel != nil {
		req.Header.Set("X-Goog-Api-Key", channel.Key)
	}

	var (
		proxyURL      string
		skipTLSVerify bool
	)
	if channel != nil {
		proxyURL = channel.ProxyURL
		skipTLSVerify = channel.SkipTLSVerify
	}

	client, err := relayutils.LoadHTTPClientWithTLSConfigE(0, proxyURL, skipTLSVerify)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var operation geminiOperation
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&operation); err != nil {
		return nil, fmt.Errorf("decode gemini video operation: %w", err)
	}

	return &operation, nil
}

func geminiVideoAsyncUsage(
	store adaptor.Store,
	info *model.AsyncUsageInfo,
	operation *geminiOperation,
) (model.Usage, model.UsageContext) {
	metadata := geminiVideoAsyncUsageMetadata(store, info, operation)

	seconds := metadata.Seconds
	if seconds <= 0 {
		seconds = defaultGeminiVideoDurationSeconds
	}

	variants := metadata.Variants
	if variants <= 0 {
		variants = len(geminiVideoURLs(operation))
	}

	if variants <= 0 {
		variants = 1
	}

	tokens := model.ZeroNullInt64(int64(seconds * variants))

	usageContext := info.UsageContext
	if metadata.Resolution != "" {
		usageContext.NativeResolution = metadata.Resolution
	}

	if usageContext.NativeResolution == "" {
		usageContext.NativeResolution = defaultGeminiVideoResolution
	}

	if resolution := geminiVideoAsyncUsageResolution(info, metadata); resolution != "" {
		usageContext.Resolution = resolution
	}

	return model.Usage{
		OutputTokens: tokens,
		TotalTokens:  tokens,
	}, usageContext
}

func geminiVideoAsyncUsageResolution(
	info *model.AsyncUsageInfo,
	metadata geminiVideoStoreMetadata,
) string {
	if mode.Mode(info.Mode) == mode.GeminiVideo {
		return firstNonEmpty(metadata.Resolution, info.UsageContext.NativeResolution)
	}

	return firstNonEmpty(
		videoDimensionsResolution(metadata.Width, metadata.Height),
		info.UsageContext.Resolution,
	)
}

func geminiVideoAsyncUsageMetadata(
	store adaptor.Store,
	info *model.AsyncUsageInfo,
	operation *geminiOperation,
) geminiVideoStoreMetadata {
	if info == nil {
		return geminiVideoStoreMetadata{}
	}

	localID := geminiVideoLocalID(info.UpstreamID)

	nativeOperationID := nativeGeminiVideoStoreID(info.UpstreamID)
	if operation != nil && operation.Name != "" {
		localID = geminiVideoLocalID(operation.Name)
		nativeOperationID = nativeGeminiVideoStoreID(operation.Name)
	}

	var storeIDs []string
	switch mode.Mode(info.Mode) {
	case mode.GeminiVideo:
		if nativeOperationID != "" {
			storeIDs = append(storeIDs, model.VideoJobStoreID(nativeOperationID))
		}

		storeIDs = append(storeIDs, model.VideoJobStoreID(localID))
	case mode.VideoGenerationsJobs:
		storeIDs = append(storeIDs, model.VideoJobStoreID(localID))
	case mode.Videos, mode.VideosEdits, mode.VideosExtensions:
		storeIDs = append(storeIDs, model.VideoGenerationStoreID(localID))
	default:
		storeIDs = append(storeIDs,
			model.VideoJobStoreID(localID),
			model.VideoGenerationStoreID(localID),
		)
	}

	for _, storeID := range storeIDs {
		if store == nil {
			continue
		}

		cache, err := store.GetStoreByScope(
			info.GroupID,
			info.TokenID,
			storeID,
			model.ChannelScopeGlobal,
		)
		if err != nil {
			continue
		}

		metadata := parseGeminiVideoStoreMetadata(cache.Metadata)
		if metadata.OperationName != "" {
			return metadata
		}
	}

	return geminiVideoStoreMetadata{
		Resolution: info.UsageContext.NativeResolution,
	}
}
