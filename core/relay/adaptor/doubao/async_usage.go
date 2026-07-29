package doubao

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/bytedance/sonic"
	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	relayutils "github.com/labring/aiproxy/core/relay/utils"
)

var _ adaptor.AsyncUsageFetcher = (*Adaptor)(nil)

func (a *Adaptor) FetchAsyncUsage(
	ctx context.Context,
	request adaptor.AsyncUsageRequest,
) (coremodel.Usage, coremodel.UsageContext, bool, error) {
	info := request.Info
	if info == nil {
		return coremodel.Usage{}, coremodel.UsageContext{}, false, errors.New(
			"async usage info is nil",
		)
	}

	switch mode.Mode(info.Mode) {
	case mode.DoubaoVideo,
		mode.VideoGenerationsJobs,
		mode.Videos,
		mode.VideosEdits,
		mode.VideosExtensions:
	default:
		return coremodel.Usage{}, coremodel.UsageContext{}, false, fmt.Errorf(
			"unsupported async usage mode: %d",
			info.Mode,
		)
	}

	response, err := a.fetchVideoTask(ctx, request.Channel, info)
	if err != nil {
		return coremodel.Usage{}, coremodel.UsageContext{}, false, err
	}

	switch strings.ToLower(response.Status) {
	case "succeeded":
		usageContext := doubaoVideoAsyncUsageContext(response, request.Store, info)
		if mode.Mode(info.Mode) == mode.DoubaoVideo {
			usageContext = doubaoNativeVideoUsageContextFromContext(usageContext).
				WithFallback(doubaoNativeVideoUsageContextFromContext(info.UsageContext))
		} else {
			usageContext = usageContext.WithFallback(info.UsageContext)
		}

		return doubaoVideoUsageToModelUsage(response.Usage),
			usageContext,
			true,
			nil
	case "queued", "running", "":
		return coremodel.Usage{}, coremodel.UsageContext{}, false, nil
	default:
		return coremodel.Usage{}, coremodel.UsageContext{}, true, fmt.Errorf(
			"doubao video task ended with status %q: %s",
			response.Status,
			doubaoVideoErrorMessage(response),
		)
	}
}

func doubaoVideoAsyncUsageContext(
	response *relaymodel.DoubaoVideoTaskResponse,
	store adaptor.Store,
	info *coremodel.AsyncUsageInfo,
) coremodel.UsageContext {
	if response == nil {
		return doubaoVideoAsyncUsageContextFromStore(store, info)
	}

	metadata := doubaoVideoAsyncMetadataFromStore(store, info)
	if metadata == (doubaoVideoStoreMetadata{}) {
		return doubaoVideoUsageContext(response)
	}

	merged := *response
	merged.Resolution = firstNonEmptyString(response.Resolution, metadata.Resolution)
	merged.Ratio = firstNonEmptyString(response.Ratio, metadata.Ratio)

	merged.ServiceTier = firstNonEmptyString(response.ServiceTier, metadata.ServiceTier)
	if merged.GenerateAudio == nil {
		merged.GenerateAudio = metadata.OutputAudio
	}

	return doubaoVideoUsageContext(&merged).WithFallback(coremodel.UsageContext{
		InputVideo:  metadata.InputVideo,
		OutputAudio: metadata.OutputAudio,
	})
}

func doubaoVideoAsyncUsageContextFromStore(
	store adaptor.Store,
	info *coremodel.AsyncUsageInfo,
) coremodel.UsageContext {
	metadata := doubaoVideoAsyncMetadataFromStore(store, info)
	if metadata == (doubaoVideoStoreMetadata{}) {
		return coremodel.UsageContext{}
	}

	response := &relaymodel.DoubaoVideoTaskResponse{
		Resolution:    metadata.Resolution,
		Ratio:         metadata.Ratio,
		ServiceTier:   metadata.ServiceTier,
		GenerateAudio: metadata.OutputAudio,
	}

	return doubaoVideoUsageContext(response).WithFallback(coremodel.UsageContext{
		InputVideo:  metadata.InputVideo,
		OutputAudio: metadata.OutputAudio,
	})
}

func doubaoVideoAsyncMetadataFromStore(
	store adaptor.Store,
	info *coremodel.AsyncUsageInfo,
) doubaoVideoStoreMetadata {
	if store == nil || info == nil || info.UpstreamID == "" {
		return doubaoVideoStoreMetadata{}
	}

	storeIDs := []string{}
	switch mode.Mode(info.Mode) {
	case mode.DoubaoVideo:
		storeIDs = append(storeIDs, coremodel.VideoGenerationStoreID(info.UpstreamID))
	case mode.VideoGenerationsJobs:
		storeIDs = append(storeIDs, coremodel.VideoJobStoreID(info.UpstreamID))
	case mode.Videos, mode.VideosEdits, mode.VideosExtensions:
		storeIDs = append(storeIDs, coremodel.VideoGenerationStoreID(info.UpstreamID))
	default:
		storeIDs = append(
			storeIDs,
			coremodel.VideoJobStoreID(info.UpstreamID),
			coremodel.VideoGenerationStoreID(info.UpstreamID),
		)
	}

	for _, storeID := range storeIDs {
		cache, err := store.GetStoreByScope(
			info.GroupID,
			info.TokenID,
			storeID,
			coremodel.ChannelScopeGlobal,
		)
		if err != nil || cache.Metadata == "" {
			continue
		}

		var metadata doubaoVideoStoreMetadata
		if err := sonic.UnmarshalString(cache.Metadata, &metadata); err == nil {
			return metadata
		}
	}

	return doubaoVideoStoreMetadata{}
}

func (a *Adaptor) fetchVideoTask(
	ctx context.Context,
	channel *coremodel.Channel,
	info *coremodel.AsyncUsageInfo,
) (*relaymodel.DoubaoVideoTaskResponse, error) {
	if info.UpstreamID == "" {
		return nil, errors.New("upstream id is empty")
	}

	baseURL := a.DefaultBaseURL()
	if info.BaseURL != "" {
		baseURL = info.BaseURL
	} else if channel != nil && channel.BaseURL != "" {
		baseURL = channel.BaseURL
	}

	requestURL, err := url.JoinPath(baseURL, "/api/v3/contents/generations/tasks", info.UpstreamID)
	if err != nil {
		return nil, fmt.Errorf("build doubao video task url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("new doubao video task request: %w", err)
	}

	if channel != nil {
		req.Header.Set("Authorization", "Bearer "+channel.Key)
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
		return nil, fmt.Errorf("do doubao video task request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response relaymodel.DoubaoVideoTaskResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode doubao video task: %w", err)
	}

	return &response, nil
}

func doubaoVideoErrorMessage(response *relaymodel.DoubaoVideoTaskResponse) string {
	if response == nil || response.Error == nil {
		return ""
	}

	return response.Error.Message
}
