package doubao

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	relayutils "github.com/labring/aiproxy/core/relay/utils"
)

const (
	metaDoubaoVideoMetadata = "doubao_video_metadata"
	doubaoVideoTTL          = 7 * 24 * time.Hour
)

type doubaoVideoRequest struct {
	Model                 string               `json:"model,omitempty"`
	Content               []doubaoVideoContent `json:"content,omitempty"`
	CallbackURL           string               `json:"callback_url,omitempty"`
	ReturnLastFrame       *bool                `json:"return_last_frame,omitempty"`
	ServiceTier           string               `json:"service_tier,omitempty"`
	ExecutionExpiresAfter *int                 `json:"execution_expires_after,omitempty"`
	GenerateAudio         *bool                `json:"generate_audio,omitempty"`
	Draft                 *bool                `json:"draft,omitempty"`
	Tools                 []any                `json:"tools,omitempty"`
	SafetyIdentifier      string               `json:"safety_identifier,omitempty"`
	Priority              *int                 `json:"priority,omitempty"`
	Resolution            string               `json:"resolution,omitempty"`
	Ratio                 string               `json:"ratio,omitempty"`
	Duration              *int                 `json:"duration,omitempty"`
	Frames                *int                 `json:"frames,omitempty"`
	FramesPerSecond       *int                 `json:"framespersecond,omitempty"`
	Seed                  any                  `json:"seed,omitempty"`
	CameraFixed           *bool                `json:"camera_fixed,omitempty"`
	Watermark             *bool                `json:"watermark,omitempty"`
}

type doubaoOpenAIVideoRequest struct {
	Content               []doubaoOpenAIVideoContent `json:"content,omitempty"`
	Prompt                string                     `json:"prompt,omitempty"`
	Model                 string                     `json:"model,omitempty"`
	Width                 doubaoFlexibleInt          `json:"width,omitempty"`
	Height                doubaoFlexibleInt          `json:"height,omitempty"`
	NVariants             doubaoFlexibleInt          `json:"n_variants,omitempty"`
	NSeconds              doubaoFlexibleInt          `json:"n_seconds,omitempty"`
	CallbackURL           string                     `json:"callback_url,omitempty"`
	ServiceTier           string                     `json:"service_tier,omitempty"`
	SafetyIdentifier      string                     `json:"safety_identifier,omitempty"`
	Resolution            string                     `json:"resolution,omitempty"`
	Ratio                 string                     `json:"ratio,omitempty"`
	Size                  string                     `json:"size,omitempty"`
	Seconds               doubaoFlexibleInt          `json:"seconds,omitempty"`
	Seed                  any                        `json:"seed,omitempty"`
	ExecutionExpiresAfter doubaoFlexibleInt          `json:"execution_expires_after,omitempty"`
	GenerateAudio         doubaoFlexibleBool         `json:"generate_audio,omitempty"`
	Draft                 doubaoFlexibleBool         `json:"draft,omitempty"`
	Priority              doubaoFlexibleInt          `json:"priority,omitempty"`
	Frames                doubaoFlexibleInt          `json:"frames,omitempty"`
	FramesPerSecond       doubaoFlexibleInt          `json:"framespersecond,omitempty"`
	FPS                   doubaoFlexibleInt          `json:"fps,omitempty"`
	CameraFixed           doubaoFlexibleBool         `json:"camera_fixed,omitempty"`
	Watermark             doubaoFlexibleBool         `json:"watermark,omitempty"`
	Tools                 []any                      `json:"tools,omitempty"`
	InputReference        doubaoFlexibleString       `json:"input_reference,omitempty"`
	Image                 doubaoFlexibleString       `json:"image,omitempty"`
	ImageURL              doubaoFlexibleString       `json:"image_url,omitempty"`
	FirstFrameURL         doubaoFlexibleString       `json:"first_frame_url,omitempty"`
	LastFrameURL          doubaoFlexibleString       `json:"last_frame_url,omitempty"`
	VideoURL              doubaoFlexibleString       `json:"video_url,omitempty"`
	AudioURL              doubaoFlexibleString       `json:"audio_url,omitempty"`
	InputAudio            *doubaoOpenAIInputAudio    `json:"input_audio,omitempty"`
	DraftTaskID           string                     `json:"draft_task_id,omitempty"`
	VideoID               string                     `json:"video_id,omitempty"`
	Video                 doubaoFlexibleString       `json:"video,omitempty"`
}

type doubaoOpenAIVideoContent struct {
	Type       string                  `json:"type,omitempty"`
	Text       string                  `json:"text,omitempty"`
	Role       string                  `json:"role,omitempty"`
	ImageURL   doubaoFlexibleString    `json:"image_url,omitempty"`
	VideoURL   doubaoFlexibleString    `json:"video_url,omitempty"`
	AudioURL   doubaoFlexibleString    `json:"audio_url,omitempty"`
	InputAudio *doubaoOpenAIInputAudio `json:"input_audio,omitempty"`
	DraftTask  doubaoFlexibleID        `json:"draft_task,omitempty"`
}

type doubaoOpenAIInputAudio struct {
	URL    string `json:"url,omitempty"`
	Data   string `json:"data,omitempty"`
	Format string `json:"format,omitempty"`
}

func (audio *doubaoOpenAIInputAudio) DoubaoURL() *doubaoVideoURLContent {
	if audio == nil {
		return nil
	}

	if url := strings.TrimSpace(audio.URL); url != "" {
		return &doubaoVideoURLContent{URL: url}
	}

	data := strings.TrimSpace(audio.Data)
	if data == "" {
		return nil
	}

	if strings.HasPrefix(data, "data:audio/") {
		return &doubaoVideoURLContent{URL: data}
	}

	if strings.HasPrefix(data, "http://") || strings.HasPrefix(data, "https://") {
		return &doubaoVideoURLContent{URL: data}
	}

	format := strings.TrimSpace(strings.ToLower(audio.Format))
	if format == "" {
		format = "wav"
	}

	return &doubaoVideoURLContent{
		URL: "data:audio/" + format + ";base64," + data,
	}
}

type doubaoFlexibleInt struct {
	Value int
	Set   bool
}

func (value *doubaoFlexibleInt) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		return nil
	}

	if strings.HasPrefix(text, `"`) {
		var raw string
		if err := sonic.Unmarshal(data, &raw); err != nil {
			return nil
		}

		text = strings.TrimSpace(raw)
		if text == "" {
			return nil
		}
	}

	number := json.Number(text)

	parsed, err := number.Int64()
	if err != nil {
		floatValue, floatErr := number.Float64()
		if floatErr != nil {
			return nil
		}

		parsed = int64(floatValue)
	}

	value.Value = int(parsed)
	value.Set = true

	return nil
}

func (value doubaoFlexibleInt) Ptr() *int {
	if !value.Set {
		return nil
	}

	return &value.Value
}

type doubaoFlexibleBool struct {
	Value bool
	Set   bool
}

func (value *doubaoFlexibleBool) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		return nil
	}

	if strings.HasPrefix(text, `"`) {
		var raw string
		if err := sonic.Unmarshal(data, &raw); err != nil {
			return nil
		}

		text = strings.TrimSpace(raw)
	}

	parsed, err := strconv.ParseBool(text)
	if err != nil {
		return nil
	}

	value.Value = parsed
	value.Set = true

	return nil
}

func (value doubaoFlexibleBool) Ptr() *bool {
	if !value.Set {
		return nil
	}

	return &value.Value
}

type doubaoFlexibleString string

func (value *doubaoFlexibleString) UnmarshalJSON(data []byte) error {
	var text string
	if err := sonic.Unmarshal(data, &text); err == nil {
		*value = doubaoFlexibleString(strings.TrimSpace(text))
		return nil
	}

	var object struct {
		URL string `json:"url,omitempty"`
	}
	if err := sonic.Unmarshal(data, &object); err == nil {
		*value = doubaoFlexibleString(strings.TrimSpace(object.URL))
	}

	return nil
}

func (value doubaoFlexibleString) String() string {
	return strings.TrimSpace(string(value))
}

type doubaoFlexibleID string

func (value *doubaoFlexibleID) UnmarshalJSON(data []byte) error {
	var text string
	if err := sonic.Unmarshal(data, &text); err == nil {
		*value = doubaoFlexibleID(strings.TrimSpace(text))
		return nil
	}

	var object struct {
		ID     string `json:"id,omitempty"`
		TaskID string `json:"task_id,omitempty"`
	}
	if err := sonic.Unmarshal(data, &object); err == nil {
		*value = doubaoFlexibleID(firstNonEmptyString(object.ID, object.TaskID))
	}

	return nil
}

func (value doubaoFlexibleID) String() string {
	return strings.TrimSpace(string(value))
}

type doubaoOpenAIVideoMode string

const (
	doubaoOpenAIVideoModeCreate doubaoOpenAIVideoMode = "create"
	doubaoOpenAIVideoModeEdit   doubaoOpenAIVideoMode = "edit"
	doubaoOpenAIVideoModeExtend doubaoOpenAIVideoMode = "extend"
)

type doubaoVideoContent struct {
	Type      string                 `json:"type,omitempty"`
	Text      string                 `json:"text,omitempty"`
	ImageURL  *doubaoVideoURLContent `json:"image_url,omitempty"`
	VideoURL  *doubaoVideoURLContent `json:"video_url,omitempty"`
	AudioURL  *doubaoVideoURLContent `json:"audio_url,omitempty"`
	DraftTask *doubaoDraftTask       `json:"draft_task,omitempty"`
	Role      string                 `json:"role,omitempty"`
}

type doubaoVideoURLContent struct {
	URL string `json:"url,omitempty"`
}

type doubaoDraftTask struct {
	ID string `json:"id,omitempty"`
}

type doubaoVideoStoreMetadata struct {
	Prompt      string `json:"prompt,omitempty"`
	Resolution  string `json:"resolution,omitempty"`
	Ratio       string `json:"ratio,omitempty"`
	Duration    int    `json:"duration,omitempty"`
	ServiceTier string `json:"service_tier,omitempty"`
	InputVideo  *bool  `json:"input_video,omitempty"`
	OutputAudio *bool  `json:"output_audio,omitempty"`
}

func ConvertVideoGenerationJobRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	return convertDoubaoVideoGenerationJobRequest(meta, req)
}

func ConvertVideosRequest(meta *meta.Meta, req *http.Request) (adaptor.ConvertResult, error) {
	return convertDoubaoVideosRequest(meta, req)
}

func ConvertVideosEditRequest(meta *meta.Meta, req *http.Request) (adaptor.ConvertResult, error) {
	return convertDoubaoVideosEditRequest(meta, req)
}

func ConvertVideosExtensionRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	return convertDoubaoVideosExtensionRequest(meta, req)
}

func ConvertVideoGenerationsGetJobsRequest(
	_ *meta.Meta,
	_ *http.Request,
) (adaptor.ConvertResult, error) {
	return adaptor.ConvertResult{}, nil
}

func ConvertVideoGenerationsContentRequest(
	_ *meta.Meta,
	_ *http.Request,
) (adaptor.ConvertResult, error) {
	return adaptor.ConvertResult{}, nil
}

func ConvertVideosGetRequest(_ *meta.Meta, _ *http.Request) (adaptor.ConvertResult, error) {
	return adaptor.ConvertResult{}, nil
}

func ConvertVideosContentRequest(_ *meta.Meta, _ *http.Request) (adaptor.ConvertResult, error) {
	return adaptor.ConvertResult{}, nil
}

func convertDoubaoVideoGenerationJobRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	request, err := parseDoubaoVideoGenerationJobRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	return convertDoubaoVideoRequest(meta, request)
}

func convertDoubaoVideosRequest(meta *meta.Meta, req *http.Request) (adaptor.ConvertResult, error) {
	request, err := parseDoubaoVideosRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	return convertDoubaoVideoRequest(meta, request)
}

func convertDoubaoVideosEditRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	request, err := parseDoubaoVideosEditRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	return convertDoubaoVideoRequest(meta, request)
}

func convertDoubaoVideosExtensionRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	request, err := parseDoubaoVideosExtensionRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	return convertDoubaoVideoRequest(meta, request)
}

func convertDoubaoVideoRequest(
	meta *meta.Meta,
	request doubaoVideoRequest,
) (adaptor.ConvertResult, error) {
	if len(request.Content) == 0 {
		return adaptor.ConvertResult{}, convertRequestError(meta, "content is required")
	}

	request.Model = meta.ActualModel
	setDoubaoVideoMetadata(meta, doubaoVideoMetadataFromRequest(request))

	data, err := sonic.Marshal(&request)
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

func parseDoubaoVideoGenerationJobRequest(req *http.Request) (doubaoVideoRequest, error) {
	if strings.HasPrefix(req.Header.Get("Content-Type"), "multipart/form-data") {
		return parseDoubaoMultipartVideoGenerationJobRequest(req)
	}

	var raw doubaoOpenAIVideoRequest
	if err := common.UnmarshalRequestReusable(req, &raw); err != nil {
		return doubaoVideoRequest{}, err
	}

	return parseDoubaoJSONVideoGenerationJobRequest(raw), nil
}

func parseDoubaoVideosRequest(req *http.Request) (doubaoVideoRequest, error) {
	if strings.HasPrefix(req.Header.Get("Content-Type"), "multipart/form-data") {
		return parseDoubaoMultipartVideosRequest(req, doubaoOpenAIVideoModeCreate)
	}

	var raw doubaoOpenAIVideoRequest
	if err := common.UnmarshalRequestReusable(req, &raw); err != nil {
		return doubaoVideoRequest{}, err
	}

	return parseDoubaoJSONVideosRequest(raw), nil
}

func parseDoubaoVideosEditRequest(req *http.Request) (doubaoVideoRequest, error) {
	return parseDoubaoVideosModeRequest(req, doubaoOpenAIVideoModeEdit)
}

func parseDoubaoVideosExtensionRequest(req *http.Request) (doubaoVideoRequest, error) {
	return parseDoubaoVideosModeRequest(req, doubaoOpenAIVideoModeExtend)
}

func parseDoubaoVideosModeRequest(
	req *http.Request,
	openAIMode doubaoOpenAIVideoMode,
) (doubaoVideoRequest, error) {
	if strings.HasPrefix(req.Header.Get("Content-Type"), "multipart/form-data") {
		return parseDoubaoMultipartVideosRequest(req, openAIMode)
	}

	var raw doubaoOpenAIVideoRequest
	if err := common.UnmarshalRequestReusable(req, &raw); err != nil {
		return doubaoVideoRequest{}, err
	}

	request := parseDoubaoJSONVideosRequest(raw)
	addDoubaoOpenAIVideoField(&request.Content, raw.Video.String(), openAIMode)

	return request, nil
}

func parseDoubaoJSONVideoGenerationJobRequest(raw doubaoOpenAIVideoRequest) doubaoVideoRequest {
	request := parseDoubaoJSONOpenAIVideoCommonRequest(raw, doubaoVideoJobSizeFromJSON(raw))
	request.Duration = raw.NSeconds.Ptr()

	return request
}

func parseDoubaoJSONVideosRequest(raw doubaoOpenAIVideoRequest) doubaoVideoRequest {
	request := parseDoubaoJSONOpenAIVideoCommonRequest(raw, raw.Size)
	request.Duration = raw.Seconds.Ptr()

	return request
}

func parseDoubaoJSONOpenAIVideoCommonRequest(
	raw doubaoOpenAIVideoRequest,
	size string,
) doubaoVideoRequest {
	request := doubaoVideoRequest{
		Content:          doubaoVideoContentFromOpenAIContent(raw.Content),
		CallbackURL:      strings.TrimSpace(raw.CallbackURL),
		ServiceTier:      strings.TrimSpace(raw.ServiceTier),
		SafetyIdentifier: strings.TrimSpace(raw.SafetyIdentifier),
		Resolution: firstNonEmptyString(
			raw.Resolution,
			doubaoVideoResolutionFromSize(size),
		),
		Ratio: firstNonEmptyString(
			raw.Ratio,
			ratioFromSize(size),
		),
		Seed:                  raw.Seed,
		ExecutionExpiresAfter: raw.ExecutionExpiresAfter.Ptr(),
		GenerateAudio:         raw.GenerateAudio.Ptr(),
		Draft:                 raw.Draft.Ptr(),
		Priority:              raw.Priority.Ptr(),
		Frames:                raw.Frames.Ptr(),
		FramesPerSecond:       firstFlexibleIntPtr(raw.FramesPerSecond, raw.FPS),
		CameraFixed:           raw.CameraFixed.Ptr(),
		Watermark:             raw.Watermark.Ptr(),
		Tools:                 raw.Tools,
	}

	if request.Content == nil {
		request.Content = doubaoVideoContentFromOpenAIRequest(raw)
	}

	return request
}

func parseDoubaoMultipartVideoGenerationJobRequest(req *http.Request) (doubaoVideoRequest, error) {
	request, err := parseDoubaoMultipartOpenAIVideoCommonRequest(
		req,
		doubaoVideoJobSizeFromForm,
	)
	if err != nil {
		return doubaoVideoRequest{}, err
	}

	setOptionalInt(&request.Duration, req.PostFormValue("n_seconds"))

	return request, nil
}

func parseDoubaoMultipartVideosRequest(
	req *http.Request,
	openAIMode doubaoOpenAIVideoMode,
) (doubaoVideoRequest, error) {
	request, err := parseDoubaoMultipartOpenAIVideoCommonRequest(req, doubaoVideoSizeFromForm)
	if err != nil {
		return doubaoVideoRequest{}, err
	}

	setOptionalInt(&request.Duration, req.PostFormValue("seconds"))

	addDoubaoOpenAIVideoField(
		&request.Content,
		req.PostFormValue("video"),
		openAIMode,
	)

	return request, nil
}

func parseDoubaoMultipartOpenAIVideoCommonRequest(
	req *http.Request,
	sizeFromForm func(*http.Request) string,
) (doubaoVideoRequest, error) {
	if err := common.ParseMultipartFormWithLimit(req); err != nil {
		return doubaoVideoRequest{}, fmt.Errorf("parse multipart form: %w", err)
	}

	size := ""
	if sizeFromForm != nil {
		size = sizeFromForm(req)
	}

	request := doubaoVideoRequest{
		CallbackURL:      req.PostFormValue("callback_url"),
		ServiceTier:      req.PostFormValue("service_tier"),
		SafetyIdentifier: req.PostFormValue("safety_identifier"),
		Resolution: firstNonEmptyString(
			req.PostFormValue("resolution"),
			doubaoVideoResolutionFromSize(size),
		),
		Ratio: firstNonEmptyString(
			req.PostFormValue("ratio"),
			ratioFromSize(size),
		),
		Content: []doubaoVideoContent{},
	}

	if prompt := req.PostFormValue("prompt"); prompt != "" {
		request.Content = append(request.Content, doubaoVideoContent{Type: "text", Text: prompt})
	}

	setOptionalInt(&request.Frames, req.PostFormValue("frames"))
	setOptionalInt(
		&request.FramesPerSecond,
		req.PostFormValue("framespersecond"),
		req.PostFormValue("fps"),
	)
	setOptionalInt(&request.ExecutionExpiresAfter, req.PostFormValue("execution_expires_after"))
	setOptionalInt(&request.Priority, req.PostFormValue("priority"))

	request.GenerateAudio = boolPtrFromString(req.PostFormValue("generate_audio"))
	request.Draft = boolPtrFromString(req.PostFormValue("draft"))
	request.CameraFixed = boolPtrFromString(req.PostFormValue("camera_fixed"))
	request.Watermark = boolPtrFromString(req.PostFormValue("watermark"))

	if seed := strings.TrimSpace(req.PostFormValue("seed")); seed != "" {
		request.Seed = seed
	}

	addFormURLContents(&request.Content, req.MultipartForm.Value)

	if err := addMultipartFileContents(&request.Content, req.MultipartForm.File); err != nil {
		return doubaoVideoRequest{}, err
	}

	return request, nil
}

func doubaoVideoSizeFromForm(req *http.Request) string {
	return req.PostFormValue("size")
}

func doubaoVideoJobSizeFromJSON(raw doubaoOpenAIVideoRequest) string {
	width := raw.Width.Value

	height := raw.Height.Value
	if width <= 0 || height <= 0 {
		return ""
	}

	return fmt.Sprintf("%dx%d", width, height)
}

func doubaoVideoJobSizeFromForm(req *http.Request) string {
	width := intFromPtr(intPtrFromAny(req.PostFormValue("width")))

	height := intFromPtr(intPtrFromAny(req.PostFormValue("height")))
	if width <= 0 || height <= 0 {
		return ""
	}

	return fmt.Sprintf("%dx%d", width, height)
}

func firstFlexibleIntPtr(values ...doubaoFlexibleInt) *int {
	for _, value := range values {
		if value.Set {
			return &value.Value
		}
	}

	return nil
}

func doubaoVideoContentFromOpenAIContent(items []doubaoOpenAIVideoContent) []doubaoVideoContent {
	if len(items) == 0 {
		return nil
	}

	content := make([]doubaoVideoContent, 0, len(items))
	for _, item := range items {
		content = append(content, doubaoVideoContentFromOpenAIContentItem(item))
	}

	if len(content) == 0 {
		return nil
	}

	return content
}

func doubaoVideoContentFromOpenAIContentItem(raw doubaoOpenAIVideoContent) doubaoVideoContent {
	item := doubaoVideoContent{
		Type: strings.TrimSpace(raw.Type),
		Text: strings.TrimSpace(raw.Text),
		Role: strings.TrimSpace(raw.Role),
	}

	if item.Type == "" && item.Text != "" {
		item.Type = "text"
	}

	switch item.Type {
	case "image_url":
		item.ImageURL = &doubaoVideoURLContent{URL: raw.ImageURL.String()}
	case "video_url":
		item.VideoURL = &doubaoVideoURLContent{URL: raw.VideoURL.String()}
	case "audio_url":
		item.AudioURL = &doubaoVideoURLContent{URL: raw.AudioURL.String()}
	case "input_audio":
		item.Type = "audio_url"

		item.AudioURL = raw.InputAudio.DoubaoURL()
		if item.Role == "" {
			item.Role = "reference_audio"
		}
	case "draft_task":
		item.DraftTask = &doubaoDraftTask{ID: raw.DraftTask.String()}
	}

	return item
}

func doubaoVideoContentFromOpenAIRequest(raw doubaoOpenAIVideoRequest) []doubaoVideoContent {
	content := []doubaoVideoContent{}

	if prompt := strings.TrimSpace(raw.Prompt); prompt != "" {
		content = append(content, doubaoVideoContent{Type: "text", Text: prompt})
	}

	addStringContent := func(contentType, urlValue, role string) {
		urlValue = strings.TrimSpace(urlValue)
		if urlValue == "" {
			return
		}

		item := doubaoVideoContent{Type: contentType, Role: role}
		switch contentType {
		case "image_url":
			item.ImageURL = &doubaoVideoURLContent{URL: urlValue}
		case "video_url":
			item.VideoURL = &doubaoVideoURLContent{URL: urlValue}
		case "audio_url":
			item.AudioURL = &doubaoVideoURLContent{URL: urlValue}
		}

		content = append(content, item)
	}

	addStringContent("image_url", firstNonEmptyString(
		raw.InputReference.String(),
		raw.Image.String(),
		raw.ImageURL.String(),
	), "")
	addStringContent("image_url", raw.FirstFrameURL.String(), "first_frame")
	addStringContent("image_url", raw.LastFrameURL.String(), "last_frame")
	addStringContent("video_url", raw.VideoURL.String(), "reference_video")
	addStringContent("audio_url", raw.AudioURL.String(), "reference_audio")

	if inputAudio := raw.InputAudio.DoubaoURL(); inputAudio != nil {
		content = append(content, doubaoVideoContent{
			Type:     "audio_url",
			AudioURL: inputAudio,
			Role:     "reference_audio",
		})
	}

	if draftTaskID := firstNonEmptyString(raw.DraftTaskID, raw.VideoID); draftTaskID != "" {
		addDoubaoDraftTaskContent(&content, draftTaskID)
	}

	return content
}

func addDoubaoOpenAIVideoField(
	content *[]doubaoVideoContent,
	value string,
	openAIMode doubaoOpenAIVideoMode,
) {
	if openAIMode == doubaoOpenAIVideoModeCreate {
		return
	}

	videoURL := strings.TrimSpace(value)
	if videoURL == "" {
		return
	}

	if strings.HasPrefix(videoURL, "video_") || strings.HasPrefix(videoURL, "doubao_") {
		addDoubaoDraftTaskContent(content, videoURL)
		return
	}

	roleForMode := func() string {
		if openAIMode == doubaoOpenAIVideoModeExtend {
			return "first_video"
		}

		return "reference_video"
	}

	for i := range *content {
		item := &(*content)[i]
		if item.Type != "video_url" || item.Role != "reference_video" {
			continue
		}

		urlValue := ""
		if item.VideoURL != nil {
			urlValue = item.VideoURL.URL
		}

		if urlValue == videoURL {
			item.Role = roleForMode()
			return
		}
	}

	*content = append(*content, doubaoVideoContent{
		Type:     "video_url",
		VideoURL: &doubaoVideoURLContent{URL: videoURL},
		Role:     roleForMode(),
	})
}

func addDoubaoDraftTaskContent(content *[]doubaoVideoContent, draftTaskID string) {
	draftTaskID = strings.TrimSpace(draftTaskID)
	if draftTaskID == "" {
		return
	}

	for _, item := range *content {
		if item.Type == "draft_task" && item.DraftTask != nil && item.DraftTask.ID == draftTaskID {
			return
		}
	}

	*content = append(*content, doubaoVideoContent{
		Type:      "draft_task",
		DraftTask: &doubaoDraftTask{ID: draftTaskID},
	})
}

func addFormURLContents(content *[]doubaoVideoContent, values map[string][]string) {
	add := func(contentType, key, role string) {
		for _, value := range values[key] {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}

			item := doubaoVideoContent{Type: contentType, Role: role}
			switch contentType {
			case "image_url":
				item.ImageURL = &doubaoVideoURLContent{URL: value}
			case "video_url":
				item.VideoURL = &doubaoVideoURLContent{URL: value}
			case "audio_url":
				item.AudioURL = &doubaoVideoURLContent{URL: value}
			}

			*content = append(*content, item)
		}
	}

	add("image_url", "input_reference", "")
	add("image_url", "image", "")
	add("image_url", "first_frame_url", "first_frame")
	add("image_url", "last_frame_url", "last_frame")
	add("video_url", "video_url", "reference_video")
	add("video_url", "video", "reference_video")
	add("audio_url", "audio_url", "reference_audio")
}

func addMultipartFileContents(
	content *[]doubaoVideoContent,
	files map[string][]*multipart.FileHeader,
) error {
	for _, key := range []string{"input_reference", "image"} {
		for _, fileHeader := range files[key] {
			dataURL, err := multipartMediaDataURL(fileHeader, "image")
			if err != nil {
				return err
			}

			*content = append(*content, doubaoVideoContent{
				Type:     "image_url",
				ImageURL: &doubaoVideoURLContent{URL: dataURL},
			})
		}
	}

	for _, fileHeader := range files["video"] {
		dataURL, err := multipartMediaDataURL(fileHeader, "video")
		if err != nil {
			return err
		}

		*content = append(*content, doubaoVideoContent{
			Type:     "video_url",
			VideoURL: &doubaoVideoURLContent{URL: dataURL},
			Role:     "reference_video",
		})
	}

	for _, fileHeader := range files["audio"] {
		dataURL, err := multipartMediaDataURL(fileHeader, "audio")
		if err != nil {
			return err
		}

		*content = append(*content, doubaoVideoContent{
			Type:     "audio_url",
			AudioURL: &doubaoVideoURLContent{URL: dataURL},
			Role:     "reference_audio",
		})
	}

	return nil
}

func multipartMediaDataURL(fileHeader *multipart.FileHeader, mediaType string) (string, error) {
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
		return "", fmt.Errorf("%s too large: max: %d", mediaType, common.MaxRequestBodySize)
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	if ext := strings.ToLower(filepath.Ext(fileHeader.Filename)); ext != "" {
		if detected := mime.TypeByExtension(ext); detected != "" &&
			!strings.HasPrefix(contentType, mediaType+"/") {
			contentType = detected
		}
	}

	if !strings.HasPrefix(contentType, mediaType+"/") {
		return "", fmt.Errorf("%s file is not %s", mediaType, mediaType)
	}

	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func VideoGenerationJobSubmitHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	response, relayErr := readDoubaoVideoTaskResponse(resp)
	if relayErr != nil {
		return adaptor.DoResponseResult{}, relayErr
	}

	expiresAt := doubaoVideoExpiresAt(response)
	if err := saveDoubaoVideoJobStore(meta, store, response.ID, expiresAt); err != nil {
		common.GetLogger(c).Errorf("save doubao video job store failed: %v", err)
	}

	job := buildDoubaoVideoJob(meta, response.ID, &response)
	if job.Status == relaymodel.VideoGenerationJobStatusSucceeded {
		for _, generation := range job.Generations {
			if err := saveDoubaoVideoStore(meta, store, generation.ID, expiresAt); err != nil {
				common.GetLogger(c).
					Errorf("save doubao video generation store failed: %v", err)
			}
		}
	}

	return writeDoubaoVideoObject(c, job, adaptor.DoResponseResult{
		UpstreamID: response.ID,
		AsyncUsage: true,
		UsageContext: doubaoVideoUsageContext(
			&response,
		).WithFallback(doubaoVideoRequestUsageContext(meta)),
	})
}

func VideosSubmitHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	response, relayErr := readDoubaoVideoTaskResponse(resp)
	if relayErr != nil {
		return adaptor.DoResponseResult{}, relayErr
	}

	expiresAt := doubaoVideoExpiresAt(response)
	if err := saveDoubaoVideoStore(meta, store, response.ID, expiresAt); err != nil {
		common.GetLogger(c).Errorf("save doubao video store failed: %v", err)
	}

	video := buildDoubaoVideo(meta, response.ID, &response)

	return writeDoubaoVideoObject(c, video, adaptor.DoResponseResult{
		UpstreamID: response.ID,
		AsyncUsage: true,
		UsageContext: doubaoVideoUsageContext(
			&response,
		).WithFallback(doubaoVideoRequestUsageContext(meta)),
	})
}

func readDoubaoVideoTaskResponse(
	resp *http.Response,
) (relaymodel.DoubaoVideoTaskResponse, adaptor.Error) {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return relaymodel.DoubaoVideoTaskResponse{}, OpenAIVideoErrorHandler(resp)
	}

	defer resp.Body.Close()

	var response relaymodel.DoubaoVideoTaskResponse
	if err := common.UnmarshalResponse(resp, &response); err != nil {
		return relaymodel.DoubaoVideoTaskResponse{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	if response.ID == "" {
		return relaymodel.DoubaoVideoTaskResponse{}, relaymodel.WrapperOpenAIVideoErrorWithMessage(
			"missing id in doubao video response",
			http.StatusInternalServerError,
		)
	}

	if response.Status == "" {
		response.Status = "queued"
	}

	return response, nil
}

func VideoGenerationJobStatusHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, OpenAIVideoErrorHandler(resp)
	}

	defer resp.Body.Close()

	var response relaymodel.DoubaoVideoTaskResponse
	if err := common.UnmarshalResponse(resp, &response); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	if response.ID == "" {
		response.ID = meta.JobID
	}

	applyStoredDoubaoVideoMetadata(
		meta,
		store,
		coremodel.VideoJobStoreID(response.ID),
		&response,
	)

	expiresAt := doubaoVideoExpiresAt(response)
	job := buildDoubaoVideoJob(meta, response.ID, &response)

	if job.Status == relaymodel.VideoGenerationJobStatusSucceeded {
		for _, generation := range job.Generations {
			if err := saveDoubaoVideoStore(meta, store, generation.ID, expiresAt); err != nil {
				common.GetLogger(c).Errorf("save doubao video generation store failed: %v", err)
			}
		}
	}

	return writeDoubaoVideoObject(c, job, adaptor.DoResponseResult{
		UsageContext: doubaoVideoUsageContext(
			&response,
		).WithFallback(doubaoVideoRequestUsageContext(meta)),
	})
}

func VideosStatusHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, OpenAIVideoErrorHandler(resp)
	}

	defer resp.Body.Close()

	var response relaymodel.DoubaoVideoTaskResponse
	if err := common.UnmarshalResponse(resp, &response); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
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

	expiresAt := doubaoVideoExpiresAt(response)
	if response.Content.VideoURL != "" || response.Content.FileURL != "" {
		if err := saveDoubaoVideoStore(meta, store, response.ID, expiresAt); err != nil {
			common.GetLogger(c).Errorf("save doubao video store failed: %v", err)
		}
	}

	return writeDoubaoVideoObject(
		c,
		buildDoubaoVideo(meta, response.ID, &response),
		adaptor.DoResponseResult{
			UpstreamID: response.ID,
			UsageContext: doubaoVideoUsageContext(
				&response,
			).WithFallback(doubaoVideoRequestUsageContext(meta)),
		},
	)
}

func VideoGenerationJobContentHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	return fetchDoubaoVideoContentHandler(meta, c, resp, meta.GenerationID)
}

func VideosContentHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	return fetchDoubaoVideoContentHandler(meta, c, resp, meta.VideoID)
}

func fetchDoubaoVideoContentHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
	id string,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, OpenAIVideoErrorHandler(resp)
	}

	defer resp.Body.Close()

	var response relaymodel.DoubaoVideoTaskResponse
	if err := common.UnmarshalResponse(resp, &response); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoError(
			err,
			http.StatusInternalServerError,
		)
	}

	videoURL := firstNonEmptyString(response.Content.VideoURL, response.Content.FileURL)
	if videoURL == "" {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIVideoErrorWithMessage(
			"video url is empty",
			http.StatusInternalServerError,
		)
	}

	videoResp, err := fetchDoubaoVideoContent(c.Request.Context(), meta, videoURL)
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
		Set("Content-Type", firstNonEmptyString(videoResp.Header.Get("Content-Type"), "video/mp4"))
	c.Writer.Header().Set("Content-Length", videoResp.Header.Get("Content-Length"))
	_, _ = io.Copy(c.Writer, videoResp.Body)

	return adaptor.DoResponseResult{UpstreamID: id}, nil
}

func buildDoubaoVideoJob(
	meta *meta.Meta,
	id string,
	response *relaymodel.DoubaoVideoTaskResponse,
) relaymodel.VideoGenerationJob {
	now := time.Now().Unix()
	createdAt := firstPositiveInt64(response.CreatedAt, now)
	expiresAt := doubaoVideoExpiresAt(*response).Unix()
	metadata := doubaoVideoMetadataFromMeta(meta)
	status := doubaoVideoJobStatus(response.Status)

	job := relaymodel.VideoGenerationJob{
		Object:      relaymodel.VideoGenerationJobObject,
		ID:          id,
		Status:      status,
		CreatedAt:   createdAt,
		ExpiresAt:   &expiresAt,
		Generations: []relaymodel.VideoGenerations{},
		Prompt:      metadata.Prompt,
		Model:       meta.OriginModel,
		NVariants:   1,
		NSeconds:    firstPositiveInt(response.Duration, metadata.Duration),
	}

	resolution, ratio := doubaoVideoResolutionAndRatio(response, metadata)
	job.Width, job.Height = doubaoVideoDimensions(resolution, ratio)

	if status == relaymodel.VideoGenerationJobStatusSucceeded ||
		status == relaymodel.VideoGenerationJobStatus("failed") {
		finishedAt := firstPositiveInt64(response.UpdatedAt, now)
		job.FinishedAt = &finishedAt
	}

	if response.Error != nil && response.Error.Message != "" {
		reason := response.Error.Message
		job.FinishReason = &reason
	}

	if status == relaymodel.VideoGenerationJobStatusSucceeded &&
		firstNonEmptyString(response.Content.VideoURL, response.Content.FileURL) != "" {
		job.Generations = append(job.Generations, relaymodel.VideoGenerations{
			Object:    relaymodel.VideoGenerationObject,
			ID:        id,
			JobID:     id,
			CreatedAt: firstPositiveInt64(response.UpdatedAt, now),
			Width:     job.Width,
			Height:    job.Height,
			Prompt:    job.Prompt,
			NSeconds:  job.NSeconds,
		})
	}

	return job
}

func buildDoubaoVideo(
	meta *meta.Meta,
	id string,
	response *relaymodel.DoubaoVideoTaskResponse,
) relaymodel.Video {
	now := time.Now().Unix()
	metadata := doubaoVideoMetadataFromMeta(meta)
	resolution, ratio := doubaoVideoResolutionAndRatio(response, metadata)
	video := relaymodel.Video{
		ID:        id,
		Object:    relaymodel.VideoObject,
		CreatedAt: firstPositiveInt64(response.CreatedAt, now),
		Status:    doubaoVideoStatus(response.Status),
		Model:     meta.OriginModel,
		Prompt:    metadata.Prompt,
		Seconds:   firstPositiveInt(response.Duration, metadata.Duration),
		Size:      doubaoVideoSize(resolution, ratio),
	}

	switch video.Status {
	case relaymodel.VideoStatusCompleted:
		video.Progress = 100
	case relaymodel.VideoStatusInProgress:
		video.Progress = 50
	case relaymodel.VideoStatusQueued:
		video.Progress = 0
	}

	if response.Error != nil && response.Error.Message != "" {
		video.Error = map[string]any{"message": response.Error.Message}
	}

	return video
}

func doubaoVideoUsageToModelUsage(usage relaymodel.DoubaoVideoUsage) coremodel.Usage {
	// Seedance video usage is returned as completion/total tokens by Ark, and
	// model pricing uses those tokens directly rather than generated seconds.
	output := usage.CompletionTokens
	if output == 0 {
		output = usage.TotalTokens
	}

	total := usage.TotalTokens
	if total == 0 {
		total = output
	}

	return coremodel.Usage{
		OutputTokens:   coremodel.ZeroNullInt64(output),
		TotalTokens:    coremodel.ZeroNullInt64(total),
		WebSearchCount: coremodel.ZeroNullInt64(usage.ToolUsage.WebSearch),
	}
}

func doubaoVideoUsageContext(response *relaymodel.DoubaoVideoTaskResponse) coremodel.UsageContext {
	if response == nil {
		return coremodel.UsageContext{}
	}

	resolution := strings.TrimSpace(response.Resolution)
	ratio := strings.TrimSpace(response.Ratio)

	return coremodel.UsageContext{
		Resolution:       doubaoVideoSize(resolution, ratio),
		NativeResolution: resolution,
		ServiceTier:      response.ServiceTier,
		OutputAudio:      response.GenerateAudio,
	}
}

func doubaoVideoRequestUsageContext(meta *meta.Meta) coremodel.UsageContext {
	metadata := doubaoVideoMetadataFromMeta(meta)

	return coremodel.UsageContext{
		Resolution:       doubaoVideoSize(metadata.Resolution, metadata.Ratio),
		NativeResolution: metadata.Resolution,
		ServiceTier:      metadata.ServiceTier,
		InputVideo:       metadata.InputVideo,
		OutputAudio:      metadata.OutputAudio,
	}
}

func writeDoubaoVideoObject(
	c *gin.Context,
	value any,
	result adaptor.DoResponseResult,
) (adaptor.DoResponseResult, adaptor.Error) {
	data, err := sonic.Marshal(value)
	if err != nil {
		return result, relaymodel.WrapperOpenAIVideoError(err, http.StatusInternalServerError)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = c.Writer.Write(data)

	return result, nil
}

func fetchDoubaoVideoContent(
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

	client, err := relayutils.LoadHTTPClientWithTLSConfigE(0, proxyURL, skipTLSVerify)
	if err != nil {
		return nil, err
	}

	return client.Do(req)
}

func saveDoubaoVideoJobStore(
	meta *meta.Meta,
	store adaptor.Store,
	jobID string,
	expiresAt time.Time,
) error {
	if store == nil || jobID == "" {
		return nil
	}

	return store.SaveStore(adaptor.StoreCache{
		ID:        coremodel.VideoJobStoreID(jobID),
		GroupID:   meta.Group.ID,
		TokenID:   meta.Token.ID,
		ChannelID: meta.Channel.ID,
		Model:     meta.OriginModel,
		Metadata:  doubaoVideoStoreMetadataString(meta),
		ExpiresAt: expiresAt,
	})
}

func saveDoubaoVideoStore(
	meta *meta.Meta,
	store adaptor.Store,
	videoID string,
	expiresAt time.Time,
) error {
	if store == nil || videoID == "" {
		return nil
	}

	return store.SaveStore(adaptor.StoreCache{
		ID:        coremodel.VideoGenerationStoreID(videoID),
		GroupID:   meta.Group.ID,
		TokenID:   meta.Token.ID,
		ChannelID: meta.Channel.ID,
		Model:     meta.OriginModel,
		Metadata:  doubaoVideoStoreMetadataString(meta),
		ExpiresAt: expiresAt,
	})
}

func doubaoVideoStoreMetadataString(meta *meta.Meta) string {
	metadata := doubaoVideoMetadataFromMeta(meta)

	data, err := sonic.MarshalString(metadata)
	if err != nil {
		return ""
	}

	return data
}

func applyStoredDoubaoVideoMetadata(
	meta *meta.Meta,
	store adaptor.Store,
	storeID string,
	response *relaymodel.DoubaoVideoTaskResponse,
) {
	if meta == nil || store == nil || storeID == "" || response == nil {
		return
	}

	cache, err := store.GetStore(meta.Group.ID, meta.Token.ID, storeID)
	if err != nil || cache.Metadata == "" {
		return
	}

	var metadata doubaoVideoStoreMetadata
	if err := sonic.UnmarshalString(cache.Metadata, &metadata); err != nil {
		return
	}

	metadata = doubaoVideoMetadataFromMeta(meta).WithFallback(metadata)
	setDoubaoVideoMetadata(meta, metadata)

	if response.Resolution == "" {
		response.Resolution = metadata.Resolution
	}

	if response.Ratio == "" {
		response.Ratio = metadata.Ratio
	}

	if response.Duration == 0 {
		response.Duration = metadata.Duration
	}

	if response.ServiceTier == "" {
		response.ServiceTier = metadata.ServiceTier
	}

	if response.GenerateAudio == nil {
		response.GenerateAudio = metadata.OutputAudio
	}
}

func doubaoVideoMetadataFromMeta(meta *meta.Meta) doubaoVideoStoreMetadata {
	if meta == nil {
		return doubaoVideoStoreMetadata{}
	}

	if value, ok := meta.Get(metaDoubaoVideoMetadata); ok {
		metadata, _ := value.(doubaoVideoStoreMetadata)
		return metadata
	}

	return doubaoVideoStoreMetadata{}
}

func setDoubaoVideoMetadata(meta *meta.Meta, metadata doubaoVideoStoreMetadata) {
	if meta == nil {
		return
	}

	if metadata == (doubaoVideoStoreMetadata{}) {
		return
	}

	meta.Set(metaDoubaoVideoMetadata, metadata)
}

func doubaoVideoMetadataFromRequest(request doubaoVideoRequest) doubaoVideoStoreMetadata {
	return doubaoVideoStoreMetadata{
		Prompt:      doubaoVideoPrompt(request.Content),
		Resolution:  request.Resolution,
		Ratio:       request.Ratio,
		Duration:    intFromPtr(request.Duration),
		ServiceTier: firstNonEmptyString(request.ServiceTier, "default"),
		InputVideo:  new(doubaoVideoContentHasVideo(request.Content)),
		OutputAudio: doubaoVideoOutputAudioFromRequest(request),
	}
}

func (metadata doubaoVideoStoreMetadata) WithFallback(
	fallback doubaoVideoStoreMetadata,
) doubaoVideoStoreMetadata {
	if metadata.Prompt == "" {
		metadata.Prompt = fallback.Prompt
	}

	if metadata.Resolution == "" {
		metadata.Resolution = fallback.Resolution
	}

	if metadata.Ratio == "" {
		metadata.Ratio = fallback.Ratio
	}

	if metadata.Duration == 0 {
		metadata.Duration = fallback.Duration
	}

	if metadata.ServiceTier == "" {
		metadata.ServiceTier = fallback.ServiceTier
	}

	if metadata.InputVideo == nil {
		metadata.InputVideo = fallback.InputVideo
	}

	if metadata.OutputAudio == nil {
		metadata.OutputAudio = fallback.OutputAudio
	}

	return metadata
}

func doubaoVideoPrompt(content []doubaoVideoContent) string {
	for _, item := range content {
		if item.Type == "text" && item.Text != "" {
			return item.Text
		}
	}

	return ""
}

func doubaoVideoContentHasVideo(content []doubaoVideoContent) bool {
	for _, item := range content {
		if item.Type == "video_url" || (item.VideoURL != nil && item.VideoURL.URL != "") {
			return true
		}

		if item.Type == "draft_task" || (item.DraftTask != nil && item.DraftTask.ID != "") {
			return true
		}
	}

	return false
}

func doubaoVideoOutputAudioFromRequest(request doubaoVideoRequest) *bool {
	if request.GenerateAudio != nil {
		return request.GenerateAudio
	}

	// Ark Seedance 2.0 and 1.5 default generate_audio to true.
	return new(true)
}

func doubaoVideoExpiresAt(response relaymodel.DoubaoVideoTaskResponse) time.Time {
	if response.CreatedAt > 0 && response.ExecutionExpiresAfter > 0 {
		return time.Unix(response.CreatedAt+response.ExecutionExpiresAfter, 0)
	}

	return time.Now().Add(doubaoVideoTTL)
}

func doubaoVideoDimensions(resolution, ratio string) (int, int) {
	var height int
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "1080p":
		height = 1080
	case "720p":
		height = 720
	case "480p":
		height = 480
	default:
		return 0, 0
	}

	switch strings.TrimSpace(ratio) {
	case "9:16":
		return height, height * 16 / 9
	case "1:1":
		return height, height
	case "4:3":
		return height * 4 / 3, height
	case "3:4":
		return height, height * 4 / 3
	case "21:9":
		return height * 21 / 9, height
	default:
		return height * 16 / 9, height
	}
}

func doubaoVideoResolutionAndRatio(
	response *relaymodel.DoubaoVideoTaskResponse,
	metadata doubaoVideoStoreMetadata,
) (string, string) {
	if response == nil {
		return metadata.Resolution, metadata.Ratio
	}

	return firstNonEmptyString(response.Resolution, metadata.Resolution),
		firstNonEmptyString(response.Ratio, metadata.Ratio)
}

func doubaoVideoSize(resolution, ratio string) string {
	width, height := doubaoVideoDimensions(resolution, ratio)
	if width <= 0 || height <= 0 {
		return ""
	}

	return fmt.Sprintf("%dx%d", width, height)
}

func doubaoVideoJobStatus(status string) relaymodel.VideoGenerationJobStatus {
	switch strings.ToLower(status) {
	case "succeeded":
		return relaymodel.VideoGenerationJobStatusSucceeded
	case "running":
		return relaymodel.VideoGenerationJobStatusRunning
	case "failed":
		return relaymodel.VideoGenerationJobStatus("failed")
	case "expired":
		return relaymodel.VideoGenerationJobStatus("expired")
	case "cancelled", "canceled":
		return relaymodel.VideoGenerationJobStatus("cancelled")
	default:
		return relaymodel.VideoGenerationJobStatusQueued
	}
}

func doubaoVideoStatus(status string) relaymodel.VideoStatus {
	switch strings.ToLower(status) {
	case "succeeded":
		return relaymodel.VideoStatusCompleted
	case "running":
		return relaymodel.VideoStatusInProgress
	case "failed":
		return relaymodel.VideoStatusFailed
	case "cancelled", "canceled":
		return relaymodel.VideoStatusCancelled
	default:
		return relaymodel.VideoStatusQueued
	}
}
