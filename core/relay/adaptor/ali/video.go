package ali

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/image"
	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	relayutils "github.com/labring/aiproxy/core/relay/utils"
)

var _ adaptor.AsyncUsageFetcher = (*Adaptor)(nil)

const aliVideoTaskTTL = 24 * time.Hour

const (
	metaAliVideoPrompt  = "ali_video_prompt"
	metaAliVideoSeconds = "ali_video_seconds"
	metaAliVideoWidth   = "ali_video_width"
	metaAliVideoHeight  = "ali_video_height"
	metaAliVideoSize    = "ali_video_size"
)

type aliVideoOpenAIRequest struct {
	relaymodel.VideoGenerationJobRequest
	Seconds            any             `json:"seconds,omitempty"`
	InputReference     string          `json:"input_reference,omitempty"`
	ImageURL           string          `json:"image_url,omitempty"`
	ImgURL             string          `json:"img_url,omitempty"`
	FirstFrameURL      string          `json:"first_frame_url,omitempty"`
	LastFrameURL       string          `json:"last_frame_url,omitempty"`
	FirstClipURL       string          `json:"first_clip_url,omitempty"`
	AudioURL           string          `json:"audio_url,omitempty"`
	VideoURL           string          `json:"video_url,omitempty"`
	NegativePrompt     string          `json:"negative_prompt,omitempty"`
	Size               string          `json:"size,omitempty"`
	Ratio              string          `json:"ratio,omitempty"`
	PromptExtend       *bool           `json:"prompt_extend,omitempty"`
	Watermark          *bool           `json:"watermark,omitempty"`
	Seed               *int64          `json:"seed,omitempty"`
	ShotType           string          `json:"shot_type,omitempty"`
	Audio              *bool           `json:"audio,omitempty"`
	Template           string          `json:"template,omitempty"`
	Video              any             `json:"video,omitempty"`
	Media              []aliVideoMedia `json:"media,omitempty"`
	ReferenceURLs      []string        `json:"reference_urls,omitempty"`
	ReferenceImageURLs []string        `json:"reference_image_urls,omitempty"`
	Input              map[string]any  `json:"input,omitempty"`
	Parameters         map[string]any  `json:"parameters,omitempty"`
	Metadata           map[string]any  `json:"metadata,omitempty"`
	Ext                map[string]any  `json:"ext,omitempty"`
	ForceVideoMedia    bool            `json:"-"`
}

type aliVideoParsedRequest struct {
	request *aliVideoOpenAIRequest
	seconds int
	width   int
	height  int
	size    string
	isRemix bool
}

type aliVideoMedia struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type aliVideoRequest struct {
	Model      string         `json:"model"`
	Input      map[string]any `json:"input"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

type aliVideoStoreMetadata struct {
	Prompt     string `json:"prompt,omitempty"`
	Seconds    int    `json:"seconds,omitempty"`
	Size       string `json:"size,omitempty"`
	UpstreamID string `json:"upstream_id,omitempty"`
}

func getAliVideoRequestURL(
	baseURL string,
	meta *meta.Meta,
	store adaptor.Store,
) (adaptor.RequestURL, error) {
	path := "/api/v1/services/aigc/video-generation/video-synthesis"
	method := http.MethodPost

	taskID, err := aliVideoTaskID(meta, store)
	if err != nil {
		return adaptor.RequestURL{}, err
	}

	if taskID != "" {
		path = "/api/v1/tasks/" + taskID
		method = http.MethodGet
	}

	targetURL, err := url.JoinPath(baseURL, path)
	if err != nil {
		return adaptor.RequestURL{}, err
	}

	return adaptor.RequestURL{
		Method: method,
		URL:    targetURL,
	}, nil
}

func aliVideoTaskID(meta *meta.Meta, store adaptor.Store) (string, error) {
	if meta == nil {
		return "", nil
	}

	switch meta.Mode {
	case mode.AliVideoTasks:
		return meta.VideoID, nil
	case mode.VideoGenerationsGetJobs:
		return meta.JobID, nil
	case mode.VideoGenerationsContent:
		return meta.GenerationID, nil
	case mode.VideosGet:
		return meta.VideoID, nil
	case mode.VideosContent:
		return aliVideoUpstreamTaskID(meta, store, meta.VideoID)
	default:
		return "", nil
	}
}

func aliVideoUpstreamTaskID(meta *meta.Meta, store adaptor.Store, videoID string) (string, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return "", nil
	}

	if store == nil || meta == nil {
		return videoID, nil
	}

	cache, err := store.GetStoreByScope(
		meta.Group.ID,
		meta.Token.ID,
		coremodel.VideoGenerationStoreID(videoID),
		meta.Channel.Scope,
	)
	if err != nil || cache.ID == "" {
		return videoID, nil
	}

	metadata, err := parseAliVideoStoreMetadata(cache.Metadata)
	if err != nil {
		trimmed := strings.TrimSpace(cache.Metadata)
		if trimmed != "" {
			return trimmed, nil
		}

		return "", err
	}

	if metadata.UpstreamID != "" {
		return metadata.UpstreamID, nil
	}

	return videoID, nil
}

func parseAliVideoStoreMetadata(value string) (aliVideoStoreMetadata, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return aliVideoStoreMetadata{}, nil
	}

	var metadata aliVideoStoreMetadata
	if err := sonic.UnmarshalString(value, &metadata); err != nil {
		return aliVideoStoreMetadata{}, err
	}

	return metadata, nil
}

func ConvertAliVideoGenerationJobRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	parsed, err := parseAliVideoGenerationJobRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	if parsed.request.NVariants > 1 {
		return adaptor.ConvertResult{}, convertRequestError(
			meta,
			"n_variants must be 1 for Ali video models",
		)
	}

	return convertAliVideoRequest(meta, req, parsed)
}

func ConvertAliVideosRequest(meta *meta.Meta, req *http.Request) (adaptor.ConvertResult, error) {
	parsed, err := parseAliVideosRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	return convertAliVideoRequest(meta, req, parsed)
}

func ConvertAliVideosRemixRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	parsed, err := parseAliVideosRemixRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	return convertAliVideoRequest(meta, req, parsed)
}

func ConvertAliVideosEditRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	parsed, err := parseAliVideosEditRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	return convertAliVideoRequest(meta, req, parsed)
}

func ConvertAliVideosExtensionRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	parsed, err := parseAliVideosExtensionRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	return convertAliVideoRequest(meta, req, parsed)
}

func ConvertAliVideoGenerationGetJobsRequest(
	_ *meta.Meta,
	_ *http.Request,
) (adaptor.ConvertResult, error) {
	return adaptor.ConvertResult{}, nil
}

func ConvertAliVideoGenerationContentRequest(
	_ *meta.Meta,
	_ *http.Request,
) (adaptor.ConvertResult, error) {
	return adaptor.ConvertResult{}, nil
}

func ConvertAliVideosGetRequest(_ *meta.Meta, _ *http.Request) (adaptor.ConvertResult, error) {
	return adaptor.ConvertResult{}, nil
}

func ConvertAliVideosContentRequest(_ *meta.Meta, _ *http.Request) (adaptor.ConvertResult, error) {
	return adaptor.ConvertResult{}, nil
}

func convertAliVideoRequest(
	meta *meta.Meta,
	req *http.Request,
	parsed aliVideoParsedRequest,
) (adaptor.ConvertResult, error) {
	if err := hydrateAliVideoRemixReference(meta, req, parsed); err != nil {
		return adaptor.ConvertResult{}, err
	}

	if meta != nil && meta.Mode == mode.VideosEdits {
		if err := hydrateAliVideosEditReference(meta, req, parsed); err != nil {
			return adaptor.ConvertResult{}, err
		}
	}

	if meta != nil && meta.Mode == mode.VideosExtensions {
		if err := hydrateAliVideosExtensionReference(meta, req, parsed); err != nil {
			return adaptor.ConvertResult{}, err
		}
	}

	setAliVideoRequestMetadata(meta, parsed)

	body := aliVideoRequest{
		Model: meta.ActualModel,
		Input: buildAliVideoInput(meta, parsed.request),
	}

	parameters := buildAliVideoParameters(
		meta,
		parsed,
		aliVideoRequestHasRatioBlockingMedia(body.Input),
	)
	if len(parameters) > 0 {
		body.Parameters = parameters
	}

	data, err := sonic.Marshal(&body)
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

func parseAliVideoGenerationJobRequest(req *http.Request) (aliVideoParsedRequest, error) {
	request, err := unmarshalAliVideoRequest(req)
	if err != nil {
		return aliVideoParsedRequest{}, err
	}

	return aliVideoParsedRequest{
		request: request,
		seconds: request.NSeconds,
		width:   request.Width,
		height:  request.Height,
		size:    aliVideoJobSize(request),
	}, nil
}

func parseAliVideosRequest(req *http.Request) (aliVideoParsedRequest, error) {
	request, err := unmarshalAliVideoRequest(req)
	if err != nil {
		return aliVideoParsedRequest{}, err
	}

	width, height := dimensionsFromAliVideoSize(request.Size)

	return aliVideoParsedRequest{
		request: request,
		seconds: aliVideoVideosSeconds(request),
		width:   width,
		height:  height,
		size:    request.Size,
	}, nil
}

func parseAliVideosRemixRequest(req *http.Request) (aliVideoParsedRequest, error) {
	parsed, err := parseAliVideosRequest(req)
	if err != nil {
		return aliVideoParsedRequest{}, err
	}

	parsed.isRemix = true

	return parsed, nil
}

func parseAliVideosEditRequest(req *http.Request) (aliVideoParsedRequest, error) {
	parsed, err := parseAliVideosRequest(req)
	if err != nil {
		return aliVideoParsedRequest{}, err
	}

	if err := setAliVideoVideoURLFromOpenAIVideoField(parsed.request); err != nil {
		return aliVideoParsedRequest{}, err
	}

	return parsed, nil
}

func parseAliVideosExtensionRequest(req *http.Request) (aliVideoParsedRequest, error) {
	parsed, err := parseAliVideosRequest(req)
	if err != nil {
		return aliVideoParsedRequest{}, err
	}

	if err := setAliVideoFirstClipURLFromOpenAIVideoField(parsed.request); err != nil {
		return aliVideoParsedRequest{}, err
	}

	return parsed, nil
}

func setAliVideoRequestMetadata(meta *meta.Meta, parsed aliVideoParsedRequest) {
	request := parsed.request
	if request.Prompt != "" {
		meta.Set(metaAliVideoPrompt, request.Prompt)
	}

	if parsed.seconds > 0 {
		meta.Set(metaAliVideoSeconds, parsed.seconds)
	}

	if parsed.width > 0 && parsed.height > 0 {
		meta.Set(metaAliVideoWidth, parsed.width)
		meta.Set(metaAliVideoHeight, parsed.height)
		meta.Set(metaAliVideoSize, fmt.Sprintf("%dx%d", parsed.width, parsed.height))
	} else if parsed.size != "" {
		meta.Set(metaAliVideoSize, parsed.size)
	}
}

func hydrateAliVideoRemixReference(
	meta *meta.Meta,
	req *http.Request,
	parsed aliVideoParsedRequest,
) error {
	request := parsed.request
	if !parsed.isRemix || requestHasAliVideoReference(request) {
		return nil
	}

	if meta.VideoID == "" {
		return convertRequestError(meta, "video_id is required")
	}

	channel := &coremodel.Channel{
		BaseURL:       meta.Channel.BaseURL,
		Key:           meta.Channel.Key,
		ProxyURL:      meta.Channel.ProxyURL,
		SkipTLSVerify: meta.Channel.SkipTLSVerify,
	}

	task, err := fetchAliVideoTask(req.Context(), channel, meta.Channel.BaseURL, meta.VideoID)
	if err != nil {
		return fmt.Errorf("fetch remix source video: %w", err)
	}

	videoURL := firstNonEmpty(task.Output.VideoURL, task.Output.OutputVideoURL)
	if videoURL == "" {
		return errors.New("remix source video url is empty")
	}

	request.VideoURL = videoURL

	return nil
}

func requestHasAliVideoReference(request *aliVideoOpenAIRequest) bool {
	return request.InputReference != "" ||
		request.ImageURL != "" ||
		request.ImgURL != "" ||
		request.FirstFrameURL != "" ||
		request.FirstClipURL != "" ||
		request.VideoURL != "" ||
		len(request.Media) > 0 ||
		len(request.ReferenceURLs) > 0 ||
		len(request.ReferenceImageURLs) > 0
}

func unmarshalAliVideoRequest(req *http.Request) (*aliVideoOpenAIRequest, error) {
	var request aliVideoOpenAIRequest

	contentType := req.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		if err := common.UnmarshalRequestReusable(req, &request); err != nil {
			return nil, err
		}

		if err := rejectAliVideoDangerousAliases(req, &request); err != nil {
			return nil, err
		}

		return &request, nil
	}

	if err := common.ParseMultipartFormWithLimit(req); err != nil {
		return nil, err
	}

	request.Model = req.PostFormValue("model")
	request.Prompt = req.PostFormValue("prompt")
	request.InputReference = req.PostFormValue("input_reference")
	request.Size = req.PostFormValue("size")
	request.Ratio = req.PostFormValue("ratio")
	request.NegativePrompt = req.PostFormValue("negative_prompt")
	request.ImageURL = req.PostFormValue("image_url")
	request.ImgURL = req.PostFormValue("img_url")
	request.FirstFrameURL = req.PostFormValue("first_frame_url")
	request.LastFrameURL = req.PostFormValue("last_frame_url")
	request.FirstClipURL = req.PostFormValue("first_clip_url")
	request.AudioURL = req.PostFormValue("audio_url")
	request.VideoURL = req.PostFormValue("video_url")
	request.Video = req.PostFormValue("video")
	request.ShotType = req.PostFormValue("shot_type")
	request.Template = req.PostFormValue("template")

	if err := parseAliVideoFormFields(req, &request); err != nil {
		return nil, err
	}

	if err := rejectAliVideoDangerousAliases(req, &request); err != nil {
		return nil, err
	}

	if request.InputReference == "" {
		value, err := multipartVideoReferenceToDataURL(req.MultipartForm.File)
		if err != nil {
			return nil, err
		}

		request.InputReference = value
	}

	if video, _ := aliOpenAIVideoFieldURL(request.Video); video == "" {
		value, err := multipartAliVideoFileToDataURL(req.MultipartForm.File)
		if err != nil {
			return nil, err
		}

		if value != "" {
			request.Video = value
		}
	}

	return &request, nil
}

func rejectAliVideoDangerousAliases(req *http.Request, request *aliVideoOpenAIRequest) error {
	if request.Parameters != nil {
		if err := rejectAliVideoDangerousParameterAliases(request.Parameters); err != nil {
			return err
		}
	}

	if err := rejectAliVideoDangerousMetadataAliases(request.Metadata); err != nil {
		return err
	}

	if err := rejectAliVideoDangerousMetadataAliases(request.Ext); err != nil {
		return err
	}

	if request.Input != nil {
		if _, ok := request.Input["resolution"]; ok {
			return errors.New("resolution is not supported, use size")
		}
	}

	contentType := req.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if req.PostFormValue("duration") != "" {
			return errors.New("duration is not supported, use seconds")
		}

		if req.PostFormValue("resolution") != "" {
			return errors.New("resolution is not supported, use size")
		}

		return nil
	}

	node, err := common.UnmarshalRequest2NodeReusable(req)
	if err != nil {
		return err
	}

	if value := node.Get("duration"); value != nil &&
		value.Exists() &&
		value.TypeSafe() != ast.V_NULL {
		return errors.New("duration is not supported, use seconds")
	}

	if value := node.Get("resolution"); value != nil &&
		value.Exists() &&
		value.TypeSafe() != ast.V_NULL {
		return errors.New("resolution is not supported, use size")
	}

	inputNode := node.Get("input")
	if inputNode != nil && inputNode.Exists() && inputNode.TypeSafe() != ast.V_NULL {
		if value := inputNode.Get("resolution"); value != nil &&
			value.Exists() &&
			value.TypeSafe() != ast.V_NULL {
			return errors.New("resolution is not supported, use size")
		}
	}

	return nil
}

func rejectAliVideoDangerousMetadataAliases(metadata map[string]any) error {
	if len(metadata) == 0 {
		return nil
	}

	if rawParameters, ok := metadata["parameters"].(map[string]any); ok {
		if err := rejectAliVideoDangerousParameterAliases(rawParameters); err != nil {
			return err
		}
	}

	return nil
}

func rejectAliVideoDangerousParameterAliases(parameters map[string]any) error {
	if _, ok := parameters["duration"]; ok {
		return errors.New("duration is not supported in parameters, use seconds")
	}

	if _, ok := parameters["resolution"]; ok {
		return errors.New("resolution is not supported in parameters, use size")
	}

	return nil
}

func parseAliVideoFormFields(req *http.Request, request *aliVideoOpenAIRequest) error {
	var err error

	if seconds := req.PostFormValue("seconds"); seconds != "" {
		request.Seconds = seconds
	}

	if nSeconds := req.PostFormValue("n_seconds"); nSeconds != "" {
		request.NSeconds, err = strconv.Atoi(nSeconds)
		if err != nil {
			return fmt.Errorf("invalid n_seconds: %w", err)
		}
	}

	if nVariants := req.PostFormValue("n_variants"); nVariants != "" {
		request.NVariants, err = strconv.Atoi(nVariants)
		if err != nil {
			return fmt.Errorf("invalid n_variants: %w", err)
		}
	}

	if width := req.PostFormValue("width"); width != "" {
		request.Width, err = strconv.Atoi(width)
		if err != nil {
			return fmt.Errorf("invalid width: %w", err)
		}
	}

	if height := req.PostFormValue("height"); height != "" {
		request.Height, err = strconv.Atoi(height)
		if err != nil {
			return fmt.Errorf("invalid height: %w", err)
		}
	}

	if promptExtend := req.PostFormValue("prompt_extend"); promptExtend != "" {
		value, err := strconv.ParseBool(promptExtend)
		if err != nil {
			return fmt.Errorf("invalid prompt_extend: %w", err)
		}

		request.PromptExtend = &value
	}

	if watermark := req.PostFormValue("watermark"); watermark != "" {
		value, err := strconv.ParseBool(watermark)
		if err != nil {
			return fmt.Errorf("invalid watermark: %w", err)
		}

		request.Watermark = &value
	}

	if audio := req.PostFormValue("audio"); audio != "" {
		value, err := strconv.ParseBool(audio)
		if err != nil {
			return fmt.Errorf("invalid audio: %w", err)
		}

		request.Audio = &value
	}

	if seed := req.PostFormValue("seed"); seed != "" {
		value, err := strconv.ParseInt(seed, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid seed: %w", err)
		}

		request.Seed = &value
	}

	for _, field := range []struct {
		name   string
		target *map[string]any
	}{
		{name: "input", target: &request.Input},
		{name: "parameters", target: &request.Parameters},
		{name: "metadata", target: &request.Metadata},
		{name: "ext", target: &request.Ext},
	} {
		if err := parseAliVideoJSONFormField(req, field.name, field.target); err != nil {
			return err
		}
	}

	return nil
}

func parseAliVideoJSONFormField(req *http.Request, name string, target *map[string]any) error {
	value := req.PostFormValue(name)
	if value == "" {
		return nil
	}

	var parsed map[string]any
	if err := sonic.UnmarshalString(value, &parsed); err != nil {
		return fmt.Errorf("invalid %s: %w", name, err)
	}

	*target = parsed

	return nil
}

func setAliVideoVideoURLFromOpenAIVideoField(request *aliVideoOpenAIRequest) error {
	videoURL, err := aliOpenAIVideoFieldURL(request.Video)
	if err != nil {
		return err
	}

	if request.VideoURL == "" {
		request.VideoURL = videoURL
	}

	if videoURL != "" {
		request.ForceVideoMedia = true
	}

	return nil
}

func hydrateAliVideosEditReference(
	meta *meta.Meta,
	req *http.Request,
	parsed aliVideoParsedRequest,
) error {
	request := parsed.request

	videoID := strings.TrimSpace(meta.VideoID)
	if videoID == "" {
		return nil
	}

	if videoURL := strings.TrimSpace(request.VideoURL); videoURL != "" && videoURL != videoID {
		return nil
	}

	videoURL, err := fetchAliVideoURLForReference(req.Context(), meta, videoID)
	if err != nil {
		return err
	}

	request.VideoURL = videoURL
	request.ForceVideoMedia = true

	return nil
}

func hydrateAliVideosExtensionReference(
	meta *meta.Meta,
	req *http.Request,
	parsed aliVideoParsedRequest,
) error {
	request := parsed.request

	videoID := strings.TrimSpace(meta.VideoID)
	if videoID == "" {
		return nil
	}

	if videoURL := strings.TrimSpace(request.FirstClipURL); videoURL != "" && videoURL != videoID {
		return nil
	}

	videoURL, err := fetchAliVideoURLForReference(req.Context(), meta, videoID)
	if err != nil {
		return err
	}

	request.FirstClipURL = videoURL

	return nil
}

func fetchAliVideoURLForReference(
	ctx context.Context,
	meta *meta.Meta,
	videoID string,
) (string, error) {
	channel := &coremodel.Channel{
		BaseURL:       meta.Channel.BaseURL,
		Key:           meta.Channel.Key,
		ProxyURL:      meta.Channel.ProxyURL,
		SkipTLSVerify: meta.Channel.SkipTLSVerify,
	}

	task, err := fetchAliVideoTask(ctx, channel, meta.Channel.BaseURL, videoID)
	if err != nil {
		return "", fmt.Errorf("fetch source video: %w", err)
	}

	videoURL := firstNonEmpty(task.Output.VideoURL, task.Output.OutputVideoURL)
	if videoURL == "" {
		return "", errors.New("source video url is empty")
	}

	return videoURL, nil
}

func setAliVideoFirstClipURLFromOpenAIVideoField(request *aliVideoOpenAIRequest) error {
	videoURL, err := aliOpenAIVideoFieldURL(request.Video)
	if err != nil {
		return err
	}

	if request.FirstClipURL == "" {
		request.FirstClipURL = videoURL
	}

	return nil
}

func aliOpenAIVideoFieldURL(value any) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return strings.TrimSpace(typed), nil
	default:
		return "", fmt.Errorf("unsupported video field type %T", value)
	}
}

func multipartVideoReferenceToDataURL(
	files map[string][]*multipart.FileHeader,
) (string, error) {
	fileHeaders := files["input_reference"]

	fileHeaders = append(fileHeaders, files["input_reference[]"]...)
	if len(fileHeaders) == 0 {
		return "", nil
	}

	if len(fileHeaders) > 1 {
		return "", errors.New("input_reference supports at most 1 file")
	}

	return multipartVideoReferenceFileToDataURL(fileHeaders[0])
}

func multipartAliVideoFileToDataURL(
	files map[string][]*multipart.FileHeader,
) (string, error) {
	fileHeaders := files["video"]
	if len(fileHeaders) == 0 {
		return "", nil
	}

	if len(fileHeaders) > 1 {
		return "", errors.New("video supports at most 1 file")
	}

	fileHeader := fileHeaders[0]

	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	data, err := io.ReadAll(common.LimitReader(file, common.MaxRequestBodySize+1))
	if err != nil {
		return "", err
	}

	if len(data) > common.MaxRequestBodySize {
		return "", fmt.Errorf("video too large: max: %d", common.MaxRequestBodySize)
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	if !strings.HasPrefix(contentType, "video/") {
		return "", errors.New("video file is not a video")
	}

	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func multipartVideoReferenceFileToDataURL(fileHeader *multipart.FileHeader) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	data, err := io.ReadAll(common.LimitReader(file, image.MaxImageSize+1))
	if err != nil {
		return "", err
	}

	if len(data) > image.MaxImageSize {
		return "", fmt.Errorf("input_reference too large: max: %d", image.MaxImageSize)
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	if !image.IsImageURL(contentType) {
		return "", errors.New("input_reference file is not an image")
	}

	contentType = image.TrimImageContentType(contentType)

	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func buildAliVideoInput(meta *meta.Meta, request *aliVideoOpenAIRequest) map[string]any {
	input := cloneMap(request.Input)
	mergeAliVideoMetadataInput(input, request.Metadata)
	mergeAliVideoMetadataInput(input, request.Ext)

	if request.Prompt != "" {
		input["prompt"] = request.Prompt
	}

	if request.NegativePrompt != "" {
		input["negative_prompt"] = request.NegativePrompt
	}

	if request.AudioURL != "" {
		input["audio_url"] = request.AudioURL
	}

	inputReference := strings.TrimSpace(request.InputReference)

	referenceURLs := append([]string(nil), request.ReferenceURLs...)
	if len(referenceURLs) == 0 {
		referenceURLs = stringSliceFromAny(
			firstMapValue("reference_urls", request.Metadata, request.Ext),
		)
	}

	media := append([]aliVideoMedia(nil), request.Media...)
	if len(media) == 0 {
		media = mediaFromAny(firstMapValue("media", request.Metadata, request.Ext))
	}

	if len(media) == 0 {
		media = buildAliVideoMedia(meta, request, inputReference, referenceURLs)
	}

	if len(media) > 0 && videoModelUsesMedia(meta) {
		input["media"] = media
	} else {
		if url := firstNonEmpty(
			request.ImgURL,
			request.ImageURL,
			inputReference,
			request.FirstFrameURL,
		); url != "" {
			input["img_url"] = url
		}

		if request.FirstFrameURL != "" && request.FirstFrameURL != input["img_url"] {
			input["first_frame_url"] = request.FirstFrameURL
		}

		if request.LastFrameURL != "" {
			input["last_frame_url"] = request.LastFrameURL
		}

		if len(referenceURLs) > 0 {
			input["reference_urls"] = referenceURLs
		}
	}

	return input
}

func buildAliVideoMedia(
	meta *meta.Meta,
	request *aliVideoOpenAIRequest,
	inputReference string,
	referenceURLs []string,
) []aliVideoMedia {
	media := make([]aliVideoMedia, 0, 4+len(referenceURLs)+len(request.ReferenceImageURLs))

	if url := firstNonEmpty(
		inputReference,
		request.ImgURL,
		request.ImageURL,
		request.FirstFrameURL,
	); url != "" {
		media = append(media, aliVideoMedia{Type: aliVideoImageMediaType(meta), URL: url})
	}

	if request.LastFrameURL != "" && !aliVideoModelUsesReferenceMedia(meta) {
		media = append(media, aliVideoMedia{Type: "last_frame", URL: request.LastFrameURL})
	}

	if request.AudioURL != "" && videoModelUsesMedia(meta) {
		media = append(media, aliVideoMedia{Type: "driving_audio", URL: request.AudioURL})
	}

	if request.FirstClipURL != "" {
		media = append(
			media,
			aliVideoMedia{Type: aliVideoClipMediaType(meta), URL: request.FirstClipURL},
		)
	} else if request.VideoURL != "" {
		media = append(
			media,
			aliVideoMedia{Type: aliVideoURLMediaType(meta, request), URL: request.VideoURL},
		)
	}

	for _, url := range request.ReferenceImageURLs {
		if url != "" {
			media = append(media, aliVideoMedia{Type: "reference_image", URL: url})
		}
	}

	if aliVideoModelUsesReferenceImages(meta) {
		for _, url := range referenceURLs {
			if url != "" {
				media = append(media, aliVideoMedia{Type: "reference_image", URL: url})
			}
		}
	}

	return media
}

func buildAliVideoParameters(
	meta *meta.Meta,
	parsed aliVideoParsedRequest,
	hasRatioBlockingMedia bool,
) map[string]any {
	request := parsed.request
	parameters := cloneMap(request.Parameters)
	mergeAliVideoMetadataParameters(parameters, request.Metadata)
	mergeAliVideoMetadataParameters(parameters, request.Ext)

	if parsed.seconds > 0 {
		parameters["duration"] = parsed.seconds
	}

	if meta != nil && meta.Mode == mode.VideoGenerationsJobs {
		setAliVideoJobResolutionParameters(meta, parameters, parsed, hasRatioBlockingMedia)
	} else if request.Size != "" {
		setAliVideoSize(meta, parameters, request.Size)
	} else if parsed.size != "" {
		setAliVideoSize(meta, parameters, parsed.size)
	} else if resolution := resolutionFromJobDimensions(parsed.width, parsed.height); resolution != "" {
		parameters["resolution"] = resolution
	}

	if request.Ratio != "" {
		parameters["ratio"] = request.Ratio
	}

	if request.PromptExtend != nil {
		parameters["prompt_extend"] = *request.PromptExtend
	}

	if request.Watermark != nil {
		parameters["watermark"] = *request.Watermark
	}

	if request.Seed != nil {
		parameters["seed"] = *request.Seed
	}

	if request.ShotType != "" {
		parameters["shot_type"] = request.ShotType
	}

	if request.Audio != nil {
		parameters["audio"] = *request.Audio
	}

	if request.Template != "" {
		parameters["template"] = request.Template
	}

	return parameters
}

func setAliVideoJobResolutionParameters(
	meta *meta.Meta,
	parameters map[string]any,
	parsed aliVideoParsedRequest,
	hasRatioBlockingMedia bool,
) {
	request := parsed.request
	if videoModelUsesResolution(meta) && parsed.width > 0 && parsed.height > 0 {
		if resolution := resolutionFromJobDimensions(
			parsed.width,
			parsed.height,
		); resolution != "" {
			parameters["resolution"] = resolution
		}

		if request.Ratio == "" && !hasRatioBlockingMedia {
			if ratio := relaymodel.ClosestVideoAspectRatio(
				parsed.width,
				parsed.height,
			); ratio != "" {
				parameters["ratio"] = ratio
			}
		}
	} else if parsed.size != "" {
		setAliVideoSize(meta, parameters, parsed.size)
	} else if resolution := resolutionFromJobDimensions(parsed.width, parsed.height); resolution != "" {
		parameters["resolution"] = resolution
	}
}

func aliVideoRequestHasRatioBlockingMedia(input map[string]any) bool {
	media, ok := input["media"].([]aliVideoMedia)
	if !ok {
		return false
	}

	for _, item := range media {
		switch item.Type {
		case "first_frame", "video":
			return true
		}
	}

	return false
}

func aliVideoVideosSeconds(request *aliVideoOpenAIRequest) int {
	switch value := request.Seconds.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		seconds, _ := strconv.Atoi(value.String())
		return seconds
	case string:
		seconds, _ := strconv.Atoi(value)
		return seconds
	default:
		return 0
	}
}

func dimensionsFromAliVideoSize(size string) (int, int) {
	size = normalizeAliSizeToX(size)

	if !strings.Contains(size, "x") {
		return 0, 0
	}

	widthText, heightText, ok := strings.Cut(size, "x")
	if !ok {
		return 0, 0
	}

	width, widthErr := strconv.Atoi(widthText)
	height, heightErr := strconv.Atoi(heightText)

	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return 0, 0
	}

	return width, height
}

func setAliVideoSize(meta *meta.Meta, parameters map[string]any, size string) {
	size = normalizeAliSizeToStar(size)
	if videoModelUsesResolution(meta) && strings.HasSuffix(strings.ToUpper(size), "P") {
		parameters["resolution"] = strings.ToUpper(size)
		return
	}

	parameters["size"] = size
}

func resolutionFromJobDimensions(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	switch {
	case min(width, height) >= 1000:
		return "1080P"
	default:
		return "720P"
	}
}

func aliVideoJobSize(request *aliVideoOpenAIRequest) string {
	if request.Width <= 0 || request.Height <= 0 {
		return ""
	}

	return fmt.Sprintf("%d*%d", request.Width, request.Height)
}

func mergeAliVideoMetadataInput(input, metadata map[string]any) {
	if len(metadata) == 0 {
		return
	}

	if rawInput, ok := metadata["input"].(map[string]any); ok {
		maps.Copy(input, rawInput)
	}
}

func mergeAliVideoMetadataParameters(parameters, metadata map[string]any) {
	if len(metadata) == 0 {
		return
	}

	if rawParameters, ok := metadata["parameters"].(map[string]any); ok {
		maps.Copy(parameters, rawParameters)
	}

	for key, value := range metadata {
		if key == "input" || key == "parameters" || isAliVideoMetadataInputKey(key) {
			continue
		}

		parameters[key] = value
	}
}

func isAliVideoMetadataInputKey(key string) bool {
	switch key {
	case "media",
		"reference_urls",
		"img_url",
		"image_url",
		"first_frame_url",
		"last_frame_url",
		"first_clip_url",
		"audio_url",
		"video_url",
		"negative_prompt":
		return true
	default:
		return false
	}
}

func cloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	maps.Copy(output, input)

	return output
}

func firstMapValue(key string, maps ...map[string]any) any {
	for _, values := range maps {
		if values == nil {
			continue
		}

		if value, ok := values[key]; ok {
			return value
		}
	}

	return nil
}

func stringSliceFromAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if itemString, ok := item.(string); ok && itemString != "" {
				values = append(values, itemString)
			}
		}

		return values
	default:
		return nil
	}
}

func mediaFromAny(value any) []aliVideoMedia {
	items, ok := value.([]any)
	if !ok {
		return nil
	}

	media := make([]aliVideoMedia, 0, len(items))
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		mediaType, _ := itemMap["type"].(string)

		mediaURL, _ := itemMap["url"].(string)
		if mediaType != "" && mediaURL != "" {
			media = append(media, aliVideoMedia{Type: mediaType, URL: mediaURL})
		}
	}

	return media
}

func videoModelUsesMedia(meta *meta.Meta) bool {
	return isHappyHorseVideoModel(meta) || isWan27VideoModel(meta)
}

func videoModelUsesResolution(meta *meta.Meta) bool {
	return isHappyHorseVideoModel(meta) || isWan27VideoModel(meta)
}

func isHappyHorseVideoModel(meta *meta.Meta) bool {
	return isHappyHorseVideoModelName(meta.OriginModel) ||
		isHappyHorseVideoModelName(meta.ActualModel)
}

func isHappyHorseVideoModelName(modelName string) bool {
	return strings.HasPrefix(strings.ToLower(modelName), "happyhorse-")
}

func isWan27VideoModel(meta *meta.Meta) bool {
	return strings.HasPrefix(strings.ToLower(meta.OriginModel), "wan2.7-") ||
		strings.HasPrefix(strings.ToLower(meta.ActualModel), "wan2.7-")
}

func isAliVideoEditModel(meta *meta.Meta) bool {
	return aliVideoModelNameMatches(meta, func(modelName string) bool {
		return strings.Contains(modelName, "videoedit") ||
			strings.Contains(modelName, "video-edit")
	})
}

func isAliVideoReferenceModel(meta *meta.Meta) bool {
	return aliVideoModelNameMatches(meta, func(modelName string) bool {
		return strings.Contains(modelName, "r2v")
	})
}

func isAliImageToVideoModel(meta *meta.Meta) bool {
	return aliVideoModelNameMatches(meta, func(modelName string) bool {
		return strings.Contains(modelName, "i2v")
	})
}

func aliVideoModelNameMatches(meta *meta.Meta, match func(string) bool) bool {
	if meta == nil {
		return false
	}

	return match(strings.ToLower(meta.OriginModel)) ||
		match(strings.ToLower(meta.ActualModel))
}

func aliVideoModelUsesReferenceMedia(meta *meta.Meta) bool {
	return isAliVideoReferenceModel(meta)
}

func aliVideoModelUsesReferenceImages(meta *meta.Meta) bool {
	return aliVideoModelUsesReferenceMedia(meta) || isAliVideoEditModel(meta)
}

func aliVideoImageMediaType(meta *meta.Meta) string {
	switch {
	case aliVideoModelUsesReferenceMedia(meta), isAliVideoEditModel(meta):
		return "reference_image"
	case isAliImageToVideoModel(meta):
		return "first_frame"
	default:
		return "first_frame"
	}
}

func aliVideoClipMediaType(meta *meta.Meta) string {
	switch {
	case isAliVideoEditModel(meta):
		return "video"
	case aliVideoModelUsesReferenceMedia(meta):
		return "reference_video"
	default:
		return "first_clip"
	}
}

func aliVideoURLMediaType(meta *meta.Meta, request *aliVideoOpenAIRequest) string {
	switch {
	case isAliVideoEditModel(meta):
		return "video"
	case aliVideoModelUsesReferenceMedia(meta):
		return "reference_video"
	case request != nil && request.ForceVideoMedia:
		return "video"
	default:
		return "first_clip"
	}
}

func AliVideoHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return adaptor.DoResponseResult{}, OpenAIVideoErrorHandler(resp)
	}

	defer resp.Body.Close()

	var aliResponse relaymodel.AliVideoTaskResponse
	if err := common.UnmarshalResponse(resp, &aliResponse); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	if aliResponse.Code != "" || aliResponse.Message != "" {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoErrorWithMessage(
			firstNonEmpty(aliResponse.Message, aliResponse.Code),
			http.StatusInternalServerError,
		)
	}

	job := aliVideoTaskToOpenAIJob(meta, &aliResponse)
	if err := saveAliVideoJobStore(meta, store, job.ID); err != nil {
		common.GetLogger(c).Errorf("save video job store failed: %v", err)
	}

	jsonResponse, err := sonic.Marshal(job)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonResponse)))
	_, _ = c.Writer.Write(jsonResponse)

	return adaptor.DoResponseResult{
		UpstreamID:   job.ID,
		AsyncUsage:   true,
		UsageContext: aliVideoUsageContext(meta, aliResponse.Usage),
	}, nil
}

func AliVideoGetJobsHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, OpenAIVideoErrorHandler(resp)
	}

	defer resp.Body.Close()

	var aliResponse relaymodel.AliVideoTaskResponse
	if err := common.UnmarshalResponse(resp, &aliResponse); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	applyStoredAliVideoRequestMetadata(
		meta,
		store,
		coremodel.VideoJobStoreID(aliResponse.Output.TaskID),
	)

	job := aliVideoTaskToOpenAIJob(meta, &aliResponse)
	if job.Status == relaymodel.VideoGenerationJobStatusSucceeded {
		for _, generation := range job.Generations {
			if err := saveAliVideoGenerationStore(
				meta,
				store,
				generation.ID,
				aliResponse.Output.TaskID,
				job.ExpiresAt,
			); err != nil {
				common.GetLogger(c).Errorf("save video generation store failed: %v", err)
			}
		}
	}

	jsonResponse, err := sonic.Marshal(job)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonResponse)))
	_, _ = c.Writer.Write(jsonResponse)

	return adaptor.DoResponseResult{
		UpstreamID:   job.ID,
		UsageContext: aliVideoUsageContext(meta, aliResponse.Usage),
	}, nil
}

func AliVideoContentHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	return aliVideoContentHandler(meta, c, resp)
}

func AliVideosContentHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	return aliVideoContentHandler(meta, c, resp)
}

func aliVideoContentHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, OpenAIVideoErrorHandler(resp)
	}

	defer resp.Body.Close()

	var aliResponse relaymodel.AliVideoTaskResponse
	if err := common.UnmarshalResponse(resp, &aliResponse); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	videoURL := firstNonEmpty(aliResponse.Output.VideoURL, aliResponse.Output.OutputVideoURL)
	if videoURL == "" {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoErrorWithMessage(
			"video url is empty",
			http.StatusInternalServerError,
		)
	}

	videoResp, err := fetchAliVideoContent(c.Request.Context(), meta, videoURL)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}
	defer videoResp.Body.Close()

	if videoResp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoErrorWithMessage(
			fmt.Sprintf("unexpected video status code: %d", videoResp.StatusCode),
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().
		Set("Content-Type", firstNonEmpty(videoResp.Header.Get("Content-Type"), "video/mp4"))
	c.Writer.Header().Set("Content-Length", videoResp.Header.Get("Content-Length"))
	_, _ = io.Copy(c.Writer, videoResp.Body)

	return adaptor.DoResponseResult{UpstreamID: aliResponse.Output.TaskID}, nil
}

func fetchAliVideoContent(
	ctx context.Context,
	meta *meta.Meta,
	videoURL string,
) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		return nil, err
	}

	var (
		proxyURL      string
		skipTLSVerify bool
	)
	if meta != nil {
		proxyURL = meta.Channel.ProxyURL
		skipTLSVerify = meta.Channel.SkipTLSVerify
	}

	client, err := relayutils.LoadHTTPClientWithTLSConfigE(
		0,
		proxyURL,
		skipTLSVerify,
	)
	if err != nil {
		return nil, err
	}

	return client.Do(req)
}

func aliVideoTaskToOpenAIJob(
	meta *meta.Meta,
	response *relaymodel.AliVideoTaskResponse,
) *relaymodel.VideoGenerationJob {
	now := time.Now()
	createdAt := parseAliVideoTime(response.Output.SubmitTime, now).Unix()
	finishedAt := parseAliVideoOptionalTime(response.Output.EndTime)

	expiresAtTime := now.Add(aliVideoTaskTTL)
	if finishedAt != nil {
		expiresAtTime = time.Unix(*finishedAt, 0).Add(aliVideoTaskTTL)
	}

	expiresAt := expiresAtTime.Unix()
	taskID := response.Output.TaskID
	status := aliVideoStatusToOpenAI(response.Output.TaskStatus)
	nSeconds := int(aliVideoOutputSeconds(response.Usage))

	nVariants := int(response.Usage.VideoCount)
	if nVariants == 0 {
		nVariants = 1
	}

	width, height := aliVideoDimensionsWithStoredRequestSize(meta, response.Usage)

	prompt := firstNonEmpty(response.Output.OrigPrompt, meta.GetString(metaAliVideoPrompt))
	if nSeconds == 0 {
		nSeconds = meta.GetInt(metaAliVideoSeconds)
	}

	job := &relaymodel.VideoGenerationJob{
		Object:      relaymodel.VideoGenerationJobObject,
		ID:          taskID,
		Status:      status,
		CreatedAt:   createdAt,
		FinishedAt:  finishedAt,
		ExpiresAt:   &expiresAt,
		Generations: []relaymodel.VideoGenerations{},
		Prompt:      prompt,
		Model:       meta.OriginModel,
		NVariants:   nVariants,
		NSeconds:    nSeconds,
		Width:       width,
		Height:      height,
	}

	if status == relaymodel.VideoGenerationJobStatusSucceeded &&
		firstNonEmpty(response.Output.VideoURL, response.Output.OutputVideoURL) != "" {
		job.Generations = append(job.Generations, relaymodel.VideoGenerations{
			Object:    relaymodel.VideoGenerationObject,
			ID:        taskID,
			JobID:     taskID,
			CreatedAt: createdAt,
			Width:     width,
			Height:    height,
			Prompt:    prompt,
			NSeconds:  nSeconds,
		})
	}

	if response.Output.Message != "" {
		finishReason := response.Output.Message
		job.FinishReason = &finishReason
	}

	return job
}

func AliVideosHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	return aliVideosHandler(meta, store, c, resp)
}

func AliVideosRemixHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	return aliVideosHandler(meta, store, c, resp)
}

func AliVideosEditHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	return aliVideosHandler(meta, store, c, resp)
}

func AliVideosExtensionHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	return aliVideosHandler(meta, store, c, resp)
}

func aliVideosHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return adaptor.DoResponseResult{}, OpenAIVideoErrorHandler(resp)
	}

	defer resp.Body.Close()

	var aliResponse relaymodel.AliVideoTaskResponse
	if err := common.UnmarshalResponse(resp, &aliResponse); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	if aliResponse.Code != "" || aliResponse.Message != "" {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoErrorWithMessage(
			firstNonEmpty(aliResponse.Message, aliResponse.Code),
			http.StatusInternalServerError,
		)
	}

	video := aliVideoTaskToOpenAIVideo(meta, &aliResponse)
	if err := saveAliVideoGenerationStore(meta, store, video.ID, video.ID, nil); err != nil {
		common.GetLogger(c).Errorf("save video store failed: %v", err)
	}

	jsonResponse, err := sonic.Marshal(video)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonResponse)))
	_, _ = c.Writer.Write(jsonResponse)

	return adaptor.DoResponseResult{
		UpstreamID:   video.ID,
		AsyncUsage:   true,
		UsageContext: aliVideoUsageContext(meta, aliResponse.Usage),
	}, nil
}

func AliVideoGetHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, OpenAIVideoErrorHandler(resp)
	}

	defer resp.Body.Close()

	var aliResponse relaymodel.AliVideoTaskResponse
	if err := common.UnmarshalResponse(resp, &aliResponse); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	applyStoredAliVideoRequestMetadata(
		meta,
		store,
		coremodel.VideoGenerationStoreID(aliResponse.Output.TaskID),
	)

	video := aliVideoTaskToOpenAIVideo(meta, &aliResponse)
	if video.Status == relaymodel.VideoStatusCompleted {
		if err := saveAliVideoGenerationStore(meta, store, video.ID, video.ID, nil); err != nil {
			common.GetLogger(c).Errorf("save video store failed: %v", err)
		}
	}

	jsonResponse, err := sonic.Marshal(video)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonResponse)))
	_, _ = c.Writer.Write(jsonResponse)

	return adaptor.DoResponseResult{
		UpstreamID:   video.ID,
		UsageContext: aliVideoUsageContext(meta, aliResponse.Usage),
	}, nil
}

func aliVideoStatusToOpenAI(status string) relaymodel.VideoGenerationJobStatus {
	switch strings.ToUpper(status) {
	case "PENDING":
		return relaymodel.VideoGenerationJobStatusQueued
	case "RUNNING":
		return relaymodel.VideoGenerationJobStatusRunning
	case "SUCCEEDED":
		return relaymodel.VideoGenerationJobStatusSucceeded
	case "FAILED":
		return "failed"
	case "CANCELED", "CANCELLED":
		return "cancelled"
	case "UNKNOWN":
		return "unknown"
	default:
		if status == "" {
			return relaymodel.VideoGenerationJobStatusQueued
		}

		return strings.ToLower(status)
	}
}

func aliVideoStatusToOpenAIVideo(status string) relaymodel.VideoStatus {
	switch strings.ToUpper(status) {
	case "PENDING":
		return relaymodel.VideoStatusQueued
	case "RUNNING":
		return relaymodel.VideoStatusInProgress
	case "SUCCEEDED":
		return relaymodel.VideoStatusCompleted
	case "FAILED":
		return relaymodel.VideoStatusFailed
	case "CANCELED", "CANCELLED":
		return relaymodel.VideoStatusCancelled
	default:
		if status == "" {
			return relaymodel.VideoStatusQueued
		}

		return strings.ToLower(status)
	}
}

func aliVideoTaskToOpenAIVideo(
	meta *meta.Meta,
	response *relaymodel.AliVideoTaskResponse,
) *relaymodel.Video {
	now := time.Now()
	createdAt := parseAliVideoTime(response.Output.SubmitTime, now).Unix()
	width, height := aliVideoDimensionsWithStoredRequestSize(meta, response.Usage)

	seconds := int(aliVideoOutputSeconds(response.Usage))
	if seconds == 0 {
		seconds = meta.GetInt(metaAliVideoSeconds)
	}

	size := aliVideoSize(width, height)
	if size == "" {
		size = meta.GetString(metaAliVideoSize)
	}

	video := &relaymodel.Video{
		ID:        response.Output.TaskID,
		Object:    relaymodel.VideoObject,
		CreatedAt: createdAt,
		Status:    aliVideoStatusToOpenAIVideo(response.Output.TaskStatus),
		Model:     meta.OriginModel,
		Prompt:    firstNonEmpty(response.Output.OrigPrompt, meta.GetString(metaAliVideoPrompt)),
		Seconds:   seconds,
		Size:      size,
	}

	switch video.Status {
	case relaymodel.VideoStatusCompleted:
		video.Progress = 100
	case relaymodel.VideoStatusQueued:
		video.Progress = 0
	case relaymodel.VideoStatusInProgress:
		video.Progress = 50
	}

	message := firstNonEmpty(response.Output.Message, response.Message)
	if message != "" && video.Status == relaymodel.VideoStatusFailed {
		video.Error = map[string]any{"message": message}
	}

	return video
}

func aliVideoSize(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	return fmt.Sprintf("%dx%d", width, height)
}

func aliVideoUsageContext(meta *meta.Meta, usage relaymodel.AliVideoUsage) coremodel.UsageContext {
	usageContext := coremodel.UsageContext{}
	if width, height := aliVideoDimensionsWithStoredRequestSize(
		meta,
		usage,
	); width > 0 &&
		height > 0 {
		usageContext.Resolution = aliVideoSize(width, height)
	}

	usageContext.NativeResolution = aliVideoNativeResolution(usage)

	return usageContext
}

func aliVideoOutputSeconds(usage relaymodel.AliVideoUsage) int64 {
	if usage.OutputVideoDuration > 0 {
		return usage.OutputVideoDuration
	}

	if usage.VideoDuration > 0 {
		return usage.VideoDuration
	}

	if usage.Duration > usage.InputVideoDuration {
		return usage.Duration - usage.InputVideoDuration
	}

	return usage.Duration
}

func aliVideoUsageToModelUsage(usage relaymodel.AliVideoUsage) coremodel.Usage {
	input := usage.InputVideoDuration
	output := aliVideoOutputSeconds(usage)

	total := usage.Duration
	if total == 0 {
		total = input + output
	}

	return coremodel.Usage{
		VideoInputTokens: coremodel.ZeroNullInt64(input),
		OutputTokens:     coremodel.ZeroNullInt64(output),
		TotalTokens:      coremodel.ZeroNullInt64(total),
	}
}

func aliVideoDimensions(usage relaymodel.AliVideoUsage) (int, int) {
	resolution := intFromAny(usage.SR)
	if resolution == 0 {
		return 0, 0
	}

	widthRatio, heightRatio := aliVideoUsageRatio(usage.Ratio)
	if widthRatio <= 0 || heightRatio <= 0 {
		widthRatio, heightRatio = 16, 9
	}

	if widthRatio >= heightRatio {
		return resolution * widthRatio / heightRatio, resolution
	}

	return resolution, resolution * heightRatio / widthRatio
}

func aliVideoUsageRatio(ratio string) (int, int) {
	ratio = strings.TrimSpace(strings.ToLower(ratio))
	if ratio == "" {
		return 0, 0
	}

	ratio = strings.ReplaceAll(ratio, "：", ":")
	ratio = strings.ReplaceAll(ratio, "×", "x")
	ratio = strings.ReplaceAll(ratio, "*", "x")
	ratio = strings.ReplaceAll(ratio, "/", ":")
	ratio = strings.ReplaceAll(ratio, " ", "")

	width, height, ok := relaymodel.ParseVideoDimensions(ratio)
	if !ok {
		parts := strings.Split(ratio, ":")
		if len(parts) != 2 {
			return 0, 0
		}

		var err error

		width, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0
		}

		height, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0
		}
	}

	if width <= 0 || height <= 0 {
		return 0, 0
	}

	return width, height
}

func aliVideoNativeResolution(usage relaymodel.AliVideoUsage) string {
	resolution := intFromAny(usage.SR)
	if resolution <= 0 {
		return ""
	}

	return fmt.Sprintf("%dP", resolution)
}

func aliVideoDimensionsWithStoredRequestSize(
	meta *meta.Meta,
	usage relaymodel.AliVideoUsage,
) (int, int) {
	storedWidth, storedHeight := storedAliVideoRequestDimensions(meta)
	if strings.TrimSpace(usage.Ratio) == "" && storedWidth > 0 && storedHeight > 0 {
		return storedWidth, storedHeight
	}

	width, height := aliVideoDimensions(usage)
	if width > 0 && height > 0 {
		return width, height
	}

	return storedWidth, storedHeight
}

func storedAliVideoRequestDimensions(meta *meta.Meta) (int, int) {
	if meta == nil {
		return 0, 0
	}

	width := meta.GetInt(metaAliVideoWidth)

	height := meta.GetInt(metaAliVideoHeight)
	if width > 0 && height > 0 {
		return width, height
	}

	width, height, _ = relaymodel.ParseVideoDimensions(meta.GetString(metaAliVideoSize))

	return width, height
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		value, _ := strconv.Atoi(typed)
		return value
	default:
		return 0
	}
}

func parseAliVideoTime(value string, fallback time.Time) time.Time {
	if parsed := parseAliVideoOptionalTime(value); parsed != nil {
		return time.Unix(*parsed, 0)
	}

	return fallback
}

func parseAliVideoOptionalTime(value string) *int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	for _, layout := range []string{
		"2006-01-02 15:04:05.000",
		time.DateTime,
		time.RFC3339Nano,
		time.RFC3339,
	} {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			unix := parsed.Unix()
			return &unix
		}
	}

	return nil
}

func saveAliVideoJobStore(meta *meta.Meta, store adaptor.Store, jobID string) error {
	if store == nil || jobID == "" {
		return nil
	}

	return store.SaveStore(adaptor.StoreCache{
		ID:        coremodel.VideoJobStoreID(jobID),
		GroupID:   meta.Group.ID,
		TokenID:   meta.Token.ID,
		ChannelID: meta.Channel.ID,
		Model:     meta.OriginModel,
		Metadata:  aliVideoStoreMetadataString(meta),
		ExpiresAt: time.Now().Add(aliVideoTaskTTL),
	}, meta.Channel.Scope)
}

func saveAliVideoGenerationStore(
	meta *meta.Meta,
	store adaptor.Store,
	generationID string,
	upstreamID string,
	expiresAt *int64,
) error {
	if store == nil || generationID == "" {
		return nil
	}

	expiresAtTime := time.Now().Add(aliVideoTaskTTL)
	if expiresAt != nil {
		expiresAtTime = time.Unix(*expiresAt, 0)
	}

	return store.SaveStore(adaptor.StoreCache{
		ID:        coremodel.VideoGenerationStoreID(generationID),
		GroupID:   meta.Group.ID,
		TokenID:   meta.Token.ID,
		ChannelID: meta.Channel.ID,
		Model:     meta.OriginModel,
		Metadata:  aliVideoStoreMetadataString(meta, upstreamID),
		ExpiresAt: expiresAtTime,
	}, meta.Channel.Scope)
}

func aliVideoStoreMetadataString(meta *meta.Meta, upstreamID ...string) string {
	if meta == nil {
		return ""
	}

	metadata := aliVideoStoreMetadata{
		Prompt:  meta.GetString(metaAliVideoPrompt),
		Seconds: meta.GetInt(metaAliVideoSeconds),
		Size:    meta.GetString(metaAliVideoSize),
	}
	if len(upstreamID) > 0 {
		metadata.UpstreamID = upstreamID[0]
	}

	data, err := sonic.MarshalString(metadata)
	if err != nil {
		return ""
	}

	return data
}

func applyStoredAliVideoRequestMetadata(meta *meta.Meta, store adaptor.Store, storeID string) {
	if meta == nil || store == nil || storeID == "" {
		return
	}

	cache, err := store.GetStoreByScope(meta.Group.ID, meta.Token.ID, storeID, meta.Channel.Scope)
	if err != nil || cache.Metadata == "" {
		return
	}

	metadata, err := parseAliVideoStoreMetadata(cache.Metadata)
	if err != nil {
		return
	}

	if meta.GetString(metaAliVideoPrompt) == "" && metadata.Prompt != "" {
		meta.Set(metaAliVideoPrompt, metadata.Prompt)
	}

	if meta.GetInt(metaAliVideoSeconds) == 0 && metadata.Seconds > 0 {
		meta.Set(metaAliVideoSeconds, metadata.Seconds)
	}

	if meta.GetString(metaAliVideoSize) == "" && metadata.Size != "" {
		meta.Set(metaAliVideoSize, metadata.Size)
	}

	if metadata.Size == "" {
		return
	}

	width, height, ok := relaymodel.ParseVideoDimensions(metadata.Size)
	if !ok {
		return
	}

	if meta.GetInt(metaAliVideoWidth) == 0 {
		meta.Set(metaAliVideoWidth, width)
	}

	if meta.GetInt(metaAliVideoHeight) == 0 {
		meta.Set(metaAliVideoHeight, height)
	}
}
