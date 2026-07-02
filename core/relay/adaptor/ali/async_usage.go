package ali

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
	channel := request.Channel

	info := request.Info
	if info == nil {
		return coremodel.Usage{}, coremodel.UsageContext{}, false, errors.New(
			"async usage info is nil",
		)
	}

	switch mode.Mode(info.Mode) {
	case mode.AliVideo:
		return a.fetchAliNativeVideoUsage(ctx, channel, request.Store, info)
	case mode.VideoGenerationsJobs,
		mode.Videos,
		mode.VideosRemix,
		mode.VideosEdits,
		mode.VideosExtensions:
		return a.fetchAliVideoJobUsage(ctx, channel, request.Store, info)
	default:
		return coremodel.Usage{}, coremodel.UsageContext{}, false, fmt.Errorf(
			"unsupported async usage mode: %d",
			info.Mode,
		)
	}
}

func (a *Adaptor) fetchAliNativeVideoUsage(
	ctx context.Context,
	channel *coremodel.Channel,
	store adaptor.Store,
	info *coremodel.AsyncUsageInfo,
) (coremodel.Usage, coremodel.UsageContext, bool, error) {
	usage, usageContext, completed, err := a.fetchAliVideoJobUsage(ctx, channel, store, info)
	if !completed || err != nil {
		return usage, usageContext, completed, err
	}

	return usage,
		aliNativeVideoUsageContextFromContext(usageContext).
			WithFallback(aliNativeVideoUsageContextFromContext(info.UsageContext)),
		completed,
		nil
}

func (a *Adaptor) fetchAliVideoJobUsage(
	ctx context.Context,
	channel *coremodel.Channel,
	store adaptor.Store,
	info *coremodel.AsyncUsageInfo,
) (coremodel.Usage, coremodel.UsageContext, bool, error) {
	if info.UpstreamID == "" {
		return coremodel.Usage{}, coremodel.UsageContext{}, false, errors.New(
			"upstream id is empty",
		)
	}

	response, err := fetchAliVideoTask(ctx, channel, info.BaseURL, info.UpstreamID)
	if err != nil {
		return coremodel.Usage{}, coremodel.UsageContext{}, false, err
	}

	switch strings.ToUpper(response.Output.TaskStatus) {
	case "SUCCEEDED":
		usage := aliVideoUsageToModelUsage(response.Usage)

		usageContext := aliVideoAsyncUsageContext(response.Usage, store, info).
			WithFallback(info.UsageContext)

		return usage, usageContext, true, nil
	case "PENDING", "RUNNING", "":
		return coremodel.Usage{}, coremodel.UsageContext{}, false, nil
	case "FAILED", "CANCELED", "CANCELLED", "UNKNOWN":
		return coremodel.Usage{}, coremodel.UsageContext{}, true, fmt.Errorf(
			"ali video task ended with status %q: %s",
			response.Output.TaskStatus,
			firstNonEmpty(response.Output.Message, response.Message),
		)
	default:
		return coremodel.Usage{}, coremodel.UsageContext{}, false, nil
	}
}

func aliNativeVideoUsageContextFromContext(
	usageContext coremodel.UsageContext,
) coremodel.UsageContext {
	nativeResolution := usageContext.NativeResolution
	if nativeResolution == "" {
		nativeResolution = usageContext.Resolution
	}

	if nativeResolution == "" {
		return coremodel.UsageContext{}
	}

	return coremodel.UsageContext{
		Resolution:       nativeResolution,
		NativeResolution: nativeResolution,
		ServiceTier:      usageContext.ServiceTier,
		Quality:          usageContext.Quality,
	}
}

func aliVideoAsyncUsageContext(
	usage relaymodel.AliVideoUsage,
	store adaptor.Store,
	info *coremodel.AsyncUsageInfo,
) coremodel.UsageContext {
	storedContext := aliVideoAsyncUsageContextFromStore(store, info)
	if strings.TrimSpace(usage.Ratio) == "" && storedContext.Resolution != "" {
		return coremodel.UsageContext{
			Resolution:       storedContext.Resolution,
			NativeResolution: aliVideoNativeResolution(usage),
		}.WithFallback(storedContext)
	}

	usageContext := coremodel.UsageContext{}
	if width, height := aliVideoDimensions(usage); width > 0 && height > 0 {
		usageContext.Resolution = aliVideoSize(width, height)
	}

	usageContext.NativeResolution = aliVideoNativeResolution(usage)

	return usageContext.WithFallback(storedContext)
}

func aliVideoAsyncUsageContextFromStore(
	store adaptor.Store,
	info *coremodel.AsyncUsageInfo,
) coremodel.UsageContext {
	if store == nil || info == nil || info.UpstreamID == "" {
		return coremodel.UsageContext{}
	}

	storeIDs := []string{}
	switch mode.Mode(info.Mode) {
	case mode.AliVideo:
		storeIDs = append(storeIDs, coremodel.VideoGenerationStoreID(info.UpstreamID))
	case mode.VideoGenerationsJobs:
		storeIDs = append(storeIDs, coremodel.VideoJobStoreID(info.UpstreamID))
	case mode.Videos, mode.VideosRemix, mode.VideosEdits, mode.VideosExtensions:
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

		var metadata aliVideoStoreMetadata
		if err := sonic.UnmarshalString(cache.Metadata, &metadata); err != nil {
			continue
		}

		if metadata.Size != "" {
			return coremodel.UsageContext{Resolution: metadata.Size}
		}
	}

	return coremodel.UsageContext{}
}

func fetchAliVideoTask(
	ctx context.Context,
	channel *coremodel.Channel,
	baseURL string,
	taskID string,
) (*relaymodel.AliVideoTaskResponse, error) {
	if baseURL == "" && channel != nil {
		baseURL = channel.BaseURL
	}

	if baseURL == "" {
		baseURL = (&Adaptor{}).DefaultBaseURL()
	}

	taskURL, err := url.JoinPath(baseURL, "/api/v1/tasks", taskID)
	if err != nil {
		return nil, fmt.Errorf("build ali video task url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, taskURL, nil)
	if err != nil {
		return nil, fmt.Errorf("new ali video task request: %w", err)
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

	client, err := relayutils.LoadHTTPClientWithTLSConfigE(
		0,
		proxyURL,
		skipTLSVerify,
	)
	if err != nil {
		return nil, fmt.Errorf("load http client: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do ali video task request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response relaymodel.AliVideoTaskResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode ali video task: %w", err)
	}

	return &response, nil
}
