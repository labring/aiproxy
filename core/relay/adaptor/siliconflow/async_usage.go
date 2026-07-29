package siliconflow

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	relayutils "github.com/labring/aiproxy/core/relay/utils"
)

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
	case mode.VideoGenerationsJobs, mode.Videos:
	default:
		return model.Usage{}, model.UsageContext{}, false, fmt.Errorf(
			"unsupported async usage mode: %d",
			info.Mode,
		)
	}

	response, err := a.fetchVideoStatus(ctx, request.Channel, info)
	if err != nil {
		return model.Usage{}, model.UsageContext{}, false, err
	}

	switch siliconFlowVideoStatusToOpenAI(response.Status) {
	case relaymodel.VideoStatusCompleted:
		outputTokens := siliconFlowVideoOutputTokens(response)

		return model.Usage{
				OutputTokens: model.ZeroNullInt64(outputTokens),
				TotalTokens:  model.ZeroNullInt64(outputTokens),
			}, siliconFlowVideoAsyncUsageContextFromStore(request.Store, info).
				WithFallback(info.UsageContext), true, nil
	case relaymodel.VideoStatusQueued, relaymodel.VideoStatusInProgress:
		return model.Usage{}, model.UsageContext{}, false, nil
	default:
		return model.Usage{}, model.UsageContext{}, true, fmt.Errorf(
			"siliconflow video task ended with status %q: %s",
			response.Status,
			response.Reason,
		)
	}
}

func siliconFlowVideoAsyncUsageContextFromStore(
	store adaptor.Store,
	info *model.AsyncUsageInfo,
) model.UsageContext {
	metadata := siliconFlowVideoAsyncMetadataFromStore(store, info)
	if metadata.ImageSize == "" {
		return model.UsageContext{}
	}

	return model.UsageContext{Resolution: metadata.ImageSize}
}

func siliconFlowVideoAsyncMetadataFromStore(
	store adaptor.Store,
	info *model.AsyncUsageInfo,
) videoStoreMetadata {
	if store == nil || info == nil || info.UpstreamID == "" {
		return videoStoreMetadata{}
	}

	storeIDs := []string{}
	switch mode.Mode(info.Mode) {
	case mode.VideoGenerationsJobs:
		storeIDs = append(storeIDs, model.VideoJobStoreID(info.UpstreamID))
	case mode.Videos:
		storeIDs = append(storeIDs, model.VideoGenerationStoreID(info.UpstreamID))
	default:
		storeIDs = append(
			storeIDs,
			model.VideoJobStoreID(info.UpstreamID),
			model.VideoGenerationStoreID(info.UpstreamID),
		)
	}

	for _, storeID := range storeIDs {
		cache, err := store.GetStoreByScope(
			info.GroupID,
			info.TokenID,
			storeID,
			model.ChannelScopeGlobal,
		)
		if err != nil || cache.Metadata == "" {
			continue
		}

		var metadata videoStoreMetadata
		if err := sonic.UnmarshalString(cache.Metadata, &metadata); err == nil {
			return metadata
		}
	}

	return videoStoreMetadata{}
}

func (a *Adaptor) fetchVideoStatus(
	ctx context.Context,
	channel *model.Channel,
	info *model.AsyncUsageInfo,
) (*videoStatusResponse, error) {
	if info.UpstreamID == "" {
		return nil, errors.New("upstream id is empty")
	}

	baseURL := a.DefaultBaseURL()
	if info.BaseURL != "" {
		baseURL = info.BaseURL
	} else if channel != nil && channel.BaseURL != "" {
		baseURL = channel.BaseURL
	}

	requestURL, err := url.JoinPath(baseURL, "/video/status")
	if err != nil {
		return nil, fmt.Errorf("build video status url: %w", err)
	}

	body, err := sonic.Marshal(videoStatusRequest{RequestID: info.UpstreamID})
	if err != nil {
		return nil, fmt.Errorf("marshal video status request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

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
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response videoStatusResponse
	if err := common.UnmarshalResponse(resp, &response); err != nil {
		return nil, fmt.Errorf("decode video status response: %w", err)
	}

	return &response, nil
}

func siliconFlowVideoOutputTokens(response *videoStatusResponse) int64 {
	if response == nil {
		return 1
	}

	var count int64
	for _, video := range response.Results.Videos {
		if video.URL != "" {
			count++
		}
	}

	if count > 0 {
		return count
	}

	return 1
}
