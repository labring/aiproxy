package azure

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	relayutils "github.com/labring/aiproxy/core/relay/utils"
	log "github.com/sirupsen/logrus"
)

var _ adaptor.AsyncUsageFetcher = (*Adaptor)(nil)

func (a *Adaptor) FetchAsyncUsage(
	ctx context.Context,
	channel *model.Channel,
	info *model.AsyncUsageInfo,
) (model.Usage, bool, error) {
	switch mode.Mode(info.Mode) {
	case mode.VideoGenerationsJobs:
		return a.fetchVideoJobUsage(ctx, channel, info)
	case mode.Responses, mode.ChatCompletions, mode.Anthropic, mode.Gemini:
		return a.fetchResponseUsage(ctx, channel, info)
	default:
		return model.Usage{}, false, fmt.Errorf("unsupported async usage mode: %d", info.Mode)
	}
}

func (a *Adaptor) fetchVideoJobUsage(
	ctx context.Context,
	channel *model.Channel,
	info *model.AsyncUsageInfo,
) (model.Usage, bool, error) {
	resp, err := a.fetchAsyncUsageObject(
		ctx,
		channel,
		info,
		"/openai/v1/video/generations/jobs",
		false,
	)
	if err != nil {
		return model.Usage{}, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var job relaymodel.VideoGenerationJob
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&job); err != nil {
		return model.Usage{}, false, fmt.Errorf("decode video job: %w", err)
	}

	switch job.Status {
	case relaymodel.VideoGenerationJobStatusSucceeded:
		totalSeconds := job.NSeconds * job.NVariants

		return model.Usage{
			OutputTokens: model.ZeroNullInt64(totalSeconds),
			TotalTokens:  model.ZeroNullInt64(totalSeconds),
		}, true, nil
	case relaymodel.VideoGenerationJobStatusQueued,
		relaymodel.VideoGenerationJobStatusProcessing,
		relaymodel.VideoGenerationJobStatusRunning:
		return model.Usage{}, false, nil
	default:
		return model.Usage{}, true, fmt.Errorf("video job ended with status %q", job.Status)
	}
}

func (a *Adaptor) fetchResponseUsage(
	ctx context.Context,
	channel *model.Channel,
	info *model.AsyncUsageInfo,
) (model.Usage, bool, error) {
	resp, err := a.fetchAsyncUsageObject(ctx, channel, info, "/openai/v1/responses", true)
	if err != nil {
		return model.Usage{}, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response relaymodel.Response
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&response); err != nil {
		return model.Usage{}, false, fmt.Errorf("decode response: %w", err)
	}

	switch response.Status {
	case relaymodel.ResponseStatusInProgress, relaymodel.ResponseStatusQueued:
		return model.Usage{}, false, nil
	case relaymodel.ResponseStatusFailed,
		relaymodel.ResponseStatusIncomplete,
		relaymodel.ResponseStatusCancelled:
		return model.Usage{}, true, fmt.Errorf("response ended with status %q", response.Status)
	default:
		return response.ToModelUsage(), true, nil
	}
}

func (a *Adaptor) fetchAsyncUsageObject(
	ctx context.Context,
	channel *model.Channel,
	info *model.AsyncUsageInfo,
	path string,
	responsesPreview bool,
) (*http.Response, error) {
	if info.UpstreamID == "" {
		return nil, errors.New("upstream id is empty")
	}

	token, apiVersion, err := GetTokenAndAPIVersion(channel.Key)
	if err != nil {
		return nil, fmt.Errorf("parse azure key: %w", err)
	}

	if responsesPreview {
		apiVersion = "preview"
	}

	baseURL := info.BaseURL
	if baseURL == "" {
		baseURL = channel.BaseURL
	}

	if baseURL == "" {
		baseURL = a.DefaultBaseURL()
	}

	requestURL, err := url.JoinPath(baseURL, path, info.UpstreamID)
	if err != nil {
		return nil, fmt.Errorf("build async usage url: %w", err)
	}

	requestURL = fmt.Sprintf("%s?api-version=%s", requestURL, apiVersion)

	log.Debugf(
		"async usage fetch url: id=%d request_id=%s upstream_id=%s mode=%d channel_id=%d url=%s",
		info.ID,
		info.RequestID,
		info.UpstreamID,
		info.Mode,
		info.ChannelID,
		requestURL,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("new async usage request: %w", err)
	}

	req.Header.Set("Api-Key", token)
	req.Header.Set("Content-Type", "application/json")

	client, err := relayutils.LoadHTTPClientWithTLSConfigE(
		0,
		channel.ProxyURL,
		channel.SkipTLSVerify,
	)
	if err != nil {
		return nil, fmt.Errorf("load http client: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do async usage request: %w", err)
	}

	return resp, nil
}
