package ali

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/image"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
	log "github.com/sirupsen/logrus"
)

const MetaResponseFormat = "response_format"

type qwenImageOpenAIRequest struct {
	relaymodel.ImageRequest
	NegativePrompt   string          `json:"negative_prompt,omitempty"`
	PromptExtend     *bool           `json:"prompt_extend,omitempty"`
	Watermark        *bool           `json:"watermark,omitempty"`
	Seed             *int64          `json:"seed,omitempty"`
	ThinkingMode     *bool           `json:"thinking_mode,omitempty"`
	EnableSequential *bool           `json:"enable_sequential,omitempty"`
	BBoxList         any             `json:"bbox_list,omitempty"`
	RefImage         string          `json:"ref_image,omitempty"`
	RefStrength      *float64        `json:"ref_strength,omitempty"`
	RefMode          string          `json:"ref_mode,omitempty"`
	Ext              json.RawMessage `json:"ext,omitempty"`
	ImageURL         string          `json:"image_url,omitempty"`
	SourceLang       string          `json:"source_lang,omitempty"`
	TargetLang       string          `json:"target_lang,omitempty"`
	ImageSegment     *bool           `json:"imageSegment,omitempty"`
}

func isQwenImageModel(meta *meta.Meta) bool {
	if meta == nil {
		return false
	}

	return isQwenImageModelName(meta.OriginModel) || isQwenImageModelName(meta.ActualModel)
}

func isQwenImageModelName(modelName string) bool {
	modelName = strings.ToLower(modelName)
	return strings.HasPrefix(modelName, "qwen-image")
}

func isAliMultimodalImageModel(meta *meta.Meta) bool {
	if meta == nil {
		return false
	}

	return isAliMultimodalImageModelName(meta.OriginModel) ||
		isAliMultimodalImageModelName(meta.ActualModel)
}

func isAliMultimodalImageModelName(modelName string) bool {
	return isQwenImageModelName(modelName) ||
		isWanMultimodalImageModelName(modelName) ||
		isZImageModelName(modelName)
}

func isWanMultimodalImageModel(meta *meta.Meta) bool {
	if meta == nil {
		return false
	}

	return isWanMultimodalImageModelName(meta.OriginModel) ||
		isWanMultimodalImageModelName(meta.ActualModel)
}

func isWanMultimodalImageModelName(modelName string) bool {
	modelName = strings.ToLower(modelName)
	return modelName == "wan2.6-t2i" || strings.HasPrefix(modelName, "wan2.7-image")
}

func isZImageModelName(modelName string) bool {
	return strings.EqualFold(modelName, "z-image-turbo")
}

func isQwenMTImageModel(meta *meta.Meta) bool {
	if meta == nil {
		return false
	}

	return isQwenMTImageModelName(meta.OriginModel) || isQwenMTImageModelName(meta.ActualModel)
}

func isQwenMTImageModelName(modelName string) bool {
	return strings.EqualFold(modelName, "qwen-mt-image")
}

func ConvertImageRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	if isAliMultimodalImageModel(meta) {
		return ConvertMultimodalImageGenerationRequest(meta, req)
	}

	request, err := unmarshalAliImageRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	request.Model = meta.ActualModel

	var imageRequest ImageRequest

	imageRequest.Input.Prompt = request.Prompt
	imageRequest.Input.NegativePrompt = request.NegativePrompt
	imageRequest.Input.RefImage = request.RefImage
	imageRequest.Model = request.Model
	imageRequest.Parameters.Size = strings.ReplaceAll(request.Size, "x", "*")
	imageRequest.Parameters.N = request.N
	imageRequest.Parameters.PromptExtend = request.PromptExtend
	imageRequest.Parameters.Watermark = request.Watermark
	imageRequest.Parameters.Seed = request.Seed
	imageRequest.Parameters.Style = request.Style
	imageRequest.Parameters.RefStrength = request.RefStrength
	imageRequest.Parameters.RefMode = request.RefMode
	imageRequest.ResponseFormat = request.ResponseFormat

	meta.Set(MetaResponseFormat, request.ResponseFormat)

	data, err := sonic.Marshal(&imageRequest)
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

func ConvertMultimodalImageGenerationRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	request, err := unmarshalAliImageRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	meta.Set(MetaResponseFormat, request.ResponseFormat)

	imageRequest, err := buildMultimodalImageRequest(meta, request.Prompt, request)
	if err != nil {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	data, err := sonic.Marshal(&imageRequest)
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

func ConvertAliImageEditRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	if isQwenMTImageModel(meta) {
		return ConvertQwenMTImageRequest(meta, req)
	}

	err := common.ParseMultipartFormWithLimit(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	imageRequest := &qwenImageOpenAIRequest{
		ImageRequest: relaymodel.ImageRequest{
			Model:          req.PostFormValue("model"),
			Prompt:         req.PostFormValue("prompt"),
			Size:           req.PostFormValue("size"),
			ResponseFormat: req.PostFormValue("response_format"),
		},
		NegativePrompt: req.PostFormValue("negative_prompt"),
		RefImage:       req.PostFormValue("ref_image"),
		RefMode:        req.PostFormValue("ref_mode"),
	}

	if err := parseAliImageFormFields(req, imageRequest); err != nil {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	meta.Set(MetaResponseFormat, imageRequest.ResponseFormat)

	qwenRequest, err := buildMultimodalImageRequest(meta, imageRequest.Prompt, imageRequest)
	if err != nil {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	userMessage := &qwenRequest.Input.Messages[0]

	fileHeaders := qwenImageEditFileHeaders(req.MultipartForm.File)
	if len(fileHeaders) == 0 {
		return adaptor.ConvertResult{}, convertRequestError(meta, "image is required")
	}

	if maxImages := multimodalImageEditMaxImages(
		meta,
	); maxImages > 0 &&
		len(fileHeaders) > maxImages {
		return adaptor.ConvertResult{}, convertRequestError(
			meta,
			fmt.Sprintf("image supports at most %d files", maxImages),
		)
	}

	imageContents := make([]map[string]any, 0, len(fileHeaders))
	for _, fileHeader := range fileHeaders {
		imageData, err := multipartImageFileToDataURL(fileHeader)
		if err != nil {
			return adaptor.ConvertResult{}, err
		}

		imageContents = append(imageContents, map[string]any{"image": imageData})
	}

	userMessage.Content = append(imageContents, userMessage.Content...)

	data, err := sonic.Marshal(&qwenRequest)
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

func ConvertQwenMTImageRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	err := common.ParseMultipartFormWithLimit(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	imageRequest := &qwenImageOpenAIRequest{
		ImageRequest: relaymodel.ImageRequest{
			Model:          req.PostFormValue("model"),
			Prompt:         req.PostFormValue("prompt"),
			ResponseFormat: req.PostFormValue("response_format"),
		},
		ImageURL:   req.PostFormValue("image_url"),
		SourceLang: req.PostFormValue("source_lang"),
		TargetLang: req.PostFormValue("target_lang"),
	}

	if err := parseAliImageFormFields(req, imageRequest); err != nil {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	if imageRequest.ImageURL == "" {
		imageRequest.ImageURL = req.PostFormValue("image")
	}

	if imageRequest.ImageURL == "" {
		fileHeaders := qwenImageEditFileHeaders(req.MultipartForm.File)
		if len(fileHeaders) > 1 {
			return adaptor.ConvertResult{}, convertRequestError(
				meta,
				"qwen-mt-image supports at most 1 image",
			)
		}

		if len(fileHeaders) == 1 {
			imageRequest.ImageURL, err = multipartImageFileToDataURL(fileHeaders[0])
			if err != nil {
				return adaptor.ConvertResult{}, err
			}
		}
	}

	if imageRequest.ImageURL == "" {
		return adaptor.ConvertResult{}, convertRequestError(
			meta,
			"image_url is required for qwen-mt-image",
		)
	}

	if imageRequest.SourceLang == "" {
		return adaptor.ConvertResult{}, convertRequestError(
			meta,
			"source_lang is required for qwen-mt-image",
		)
	}

	if imageRequest.TargetLang == "" {
		return adaptor.ConvertResult{}, convertRequestError(
			meta,
			"target_lang is required for qwen-mt-image",
		)
	}

	body := map[string]any{
		"model": meta.ActualModel,
	}

	input := map[string]any{
		"image_url":   imageRequest.ImageURL,
		"source_lang": imageRequest.SourceLang,
		"target_lang": imageRequest.TargetLang,
	}
	body["input"] = input

	if imageRequest.Ext != nil {
		var ext any
		if err := sonic.Unmarshal(imageRequest.Ext, &ext); err != nil {
			return adaptor.ConvertResult{}, convertRequestError(
				meta,
				fmt.Sprintf("invalid ext: %s", err),
			)
		}

		input["ext"] = ext
	} else if imageRequest.ImageSegment != nil {
		input["ext"] = map[string]any{
			"config": map[string]any{
				"imageSegment": *imageRequest.ImageSegment,
			},
		}
	}

	meta.Set(MetaResponseFormat, imageRequest.ResponseFormat)

	data, err := sonic.Marshal(body)
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

func buildMultimodalImageRequest(
	meta *meta.Meta,
	prompt string,
	request *qwenImageOpenAIRequest,
) (MultimodalImageRequest, error) {
	var imageRequest MultimodalImageRequest

	if strings.TrimSpace(prompt) == "" {
		return imageRequest, errors.New("prompt is required")
	}

	imageRequest.Model = meta.ActualModel
	imageRequest.Input.Messages = []MultimodalImageMessage{
		{
			Role:    "user",
			Content: []map[string]any{{"text": prompt}},
		},
	}

	parameters := make(map[string]any)
	if request != nil {
		if size := aliImageSize(request.Size); size != "" && multimodalImageSupportsSize(meta) {
			parameters["size"] = size
		}

		if request.N > 0 && multimodalImageSupportsN(meta) {
			if err := validateMultimodalImageN(meta, request.N); err != nil {
				return imageRequest, err
			}

			parameters["n"] = request.N
		} else if request.N > 1 {
			return imageRequest, errors.New("n must be 1 for this model")
		}

		if request.NegativePrompt != "" {
			parameters["negative_prompt"] = request.NegativePrompt
		}

		if request.PromptExtend != nil && multimodalImageSupportsPromptExtend(meta) {
			parameters["prompt_extend"] = *request.PromptExtend
		}

		if request.Watermark != nil && multimodalImageSupportsWatermark(meta) {
			parameters["watermark"] = *request.Watermark
		}

		if request.Seed != nil && multimodalImageSupportsSeed(meta) {
			parameters["seed"] = *request.Seed
		}

		if request.ThinkingMode != nil && isWanMultimodalImageModel(meta) {
			parameters["thinking_mode"] = *request.ThinkingMode
		}

		if request.EnableSequential != nil && isWanMultimodalImageModel(meta) {
			parameters["enable_sequential"] = *request.EnableSequential
		}

		if request.BBoxList != nil && isWanMultimodalImageModel(meta) {
			parameters["bbox_list"] = request.BBoxList
		}
	}

	if len(parameters) > 0 {
		imageRequest.Parameters = parameters
	}

	return imageRequest, nil
}

func aliImageSize(size string) string {
	size = strings.TrimSpace(size)
	if size == "" || strings.EqualFold(size, "auto") {
		return ""
	}

	return normalizeAliSizeToStar(size)
}

func normalizeAliSizeToStar(size string) string {
	size = strings.TrimSpace(size)
	size = strings.ReplaceAll(size, "×", "*")
	size = strings.ReplaceAll(size, "X", "*")
	size = strings.ReplaceAll(size, "x", "*")

	return size
}

func normalizeAliSizeToX(size string) string {
	size = strings.TrimSpace(strings.ToLower(size))
	size = strings.ReplaceAll(size, "×", "x")
	size = strings.ReplaceAll(size, "*", "x")

	return size
}

func unmarshalAliImageRequest(req *http.Request) (*qwenImageOpenAIRequest, error) {
	var request qwenImageOpenAIRequest

	err := common.UnmarshalRequestReusable(req, &request)
	if err != nil {
		return nil, err
	}

	return &request, nil
}

func parseAliImageFormFields(req *http.Request, imageRequest *qwenImageOpenAIRequest) error {
	var err error

	if n := req.PostFormValue("n"); n != "" {
		imageRequest.N, err = strconv.Atoi(n)
		if err != nil {
			return fmt.Errorf("invalid n: %w", err)
		}
	}

	if promptExtend := req.PostFormValue("prompt_extend"); promptExtend != "" {
		value, err := strconv.ParseBool(promptExtend)
		if err != nil {
			return fmt.Errorf("invalid prompt_extend: %w", err)
		}

		imageRequest.PromptExtend = &value
	}

	if watermark := req.PostFormValue("watermark"); watermark != "" {
		value, err := strconv.ParseBool(watermark)
		if err != nil {
			return fmt.Errorf("invalid watermark: %w", err)
		}

		imageRequest.Watermark = &value
	}

	if seed := req.PostFormValue("seed"); seed != "" {
		value, err := strconv.ParseInt(seed, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid seed: %w", err)
		}

		imageRequest.Seed = &value
	}

	if thinkingMode := req.PostFormValue("thinking_mode"); thinkingMode != "" {
		value, err := strconv.ParseBool(thinkingMode)
		if err != nil {
			return fmt.Errorf("invalid thinking_mode: %w", err)
		}

		imageRequest.ThinkingMode = &value
	}

	if enableSequential := req.PostFormValue("enable_sequential"); enableSequential != "" {
		value, err := strconv.ParseBool(enableSequential)
		if err != nil {
			return fmt.Errorf("invalid enable_sequential: %w", err)
		}

		imageRequest.EnableSequential = &value
	}

	if refStrength := req.PostFormValue("ref_strength"); refStrength != "" {
		value, err := strconv.ParseFloat(refStrength, 64)
		if err != nil {
			return fmt.Errorf("invalid ref_strength: %w", err)
		}

		imageRequest.RefStrength = &value
	}

	if bboxList := req.PostFormValue("bbox_list"); bboxList != "" {
		var value any
		if err := sonic.UnmarshalString(bboxList, &value); err != nil {
			return fmt.Errorf("invalid bbox_list: %w", err)
		}

		imageRequest.BBoxList = value
	}

	if ext := req.PostFormValue("ext"); ext != "" {
		if !json.Valid([]byte(ext)) {
			return errors.New("invalid ext")
		}

		imageRequest.Ext = json.RawMessage(ext)
	}

	imageSegment := firstNonEmpty(
		req.PostFormValue("imageSegment"),
		req.PostFormValue("image_segment"),
	)
	if imageSegment != "" {
		value, err := strconv.ParseBool(imageSegment)
		if err != nil {
			return fmt.Errorf("invalid imageSegment: %w", err)
		}

		imageRequest.ImageSegment = &value
	}

	if style := req.PostFormValue("style"); style != "" {
		imageRequest.Style = style
	}

	if refImage := req.PostFormValue("ref_image"); refImage != "" {
		imageRequest.RefImage = refImage
	}

	if refMode := req.PostFormValue("ref_mode"); refMode != "" {
		imageRequest.RefMode = refMode
	}

	return nil
}

func qwenImageRuleModel(meta *meta.Meta) string {
	if meta == nil {
		return ""
	}

	if isQwenImageModelName(meta.ActualModel) {
		return strings.ToLower(meta.ActualModel)
	}

	return strings.ToLower(meta.OriginModel)
}

func qwenImageSupportsN(meta *meta.Meta) bool {
	modelName := qwenImageRuleModel(meta)
	if strings.HasPrefix(modelName, "qwen-image-2.0") {
		return true
	}

	if meta != nil && meta.Mode == mode.ImagesEdits {
		return strings.HasPrefix(modelName, "qwen-image-edit-max") ||
			strings.HasPrefix(modelName, "qwen-image-edit-plus")
	}

	return false
}

func qwenImageSupportsSize(meta *meta.Meta) bool {
	return meta == nil ||
		meta.Mode != mode.ImagesEdits ||
		qwenImageRuleModel(meta) != "qwen-image-edit"
}

func qwenImageSupportsPromptExtend(meta *meta.Meta) bool {
	return qwenImageSupportsSize(meta)
}

func multimodalImageSupportsSize(meta *meta.Meta) bool {
	if isQwenImageModel(meta) {
		return qwenImageSupportsSize(meta)
	}

	return true
}

func multimodalImageSupportsN(meta *meta.Meta) bool {
	if isQwenImageModel(meta) {
		return qwenImageSupportsN(meta)
	}

	return isWanMultimodalImageModel(meta)
}

func multimodalImageSupportsPromptExtend(meta *meta.Meta) bool {
	if isQwenImageModel(meta) {
		return qwenImageSupportsPromptExtend(meta)
	}

	return !isWan27ImageModel(meta)
}

func multimodalImageSupportsWatermark(meta *meta.Meta) bool {
	return isQwenImageModel(meta) || isWanMultimodalImageModel(meta)
}

func multimodalImageSupportsSeed(meta *meta.Meta) bool {
	return isQwenImageModel(meta) || isWan26ImageModel(meta)
}

func validateMultimodalImageN(meta *meta.Meta, n int) error {
	if n < 1 {
		return errors.New("n must be greater than or equal to 1")
	}

	maxN := 1
	switch {
	case isQwenImageModel(meta):
		maxN = 6
	case isWanMultimodalImageModel(meta):
		maxN = 4
	}

	if n > maxN {
		return fmt.Errorf("n must be between 1 and %d", maxN)
	}

	return nil
}

func multimodalImageEditMaxImages(meta *meta.Meta) int {
	if isWan27ImageModel(meta) {
		return 10
	}

	return 3
}

func isWan27ImageModel(meta *meta.Meta) bool {
	if meta == nil {
		return false
	}

	return isWan27ImageModelName(meta.OriginModel) || isWan27ImageModelName(meta.ActualModel)
}

func isWan27ImageModelName(modelName string) bool {
	return strings.HasPrefix(strings.ToLower(modelName), "wan2.7-image")
}

func isWan26ImageModel(meta *meta.Meta) bool {
	if meta == nil {
		return false
	}

	return strings.EqualFold(meta.OriginModel, "wan2.6-t2i") ||
		strings.EqualFold(meta.ActualModel, "wan2.6-t2i")
}

func qwenImageEditFileHeaders(files map[string][]*multipart.FileHeader) []*multipart.FileHeader {
	fileHeaders := make([]*multipart.FileHeader, 0, len(files["image"])+len(files["image[]"]))
	fileHeaders = append(fileHeaders, files["image"]...)
	fileHeaders = append(fileHeaders, files["image[]"]...)

	return fileHeaders
}

func multipartImageFileToDataURL(fileHeader *multipart.FileHeader) (string, error) {
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
		return "", relaymodel.WrapperOpenAIErrorWithMessage(
			fmt.Sprintf("image too large: max: %d", image.MaxImageSize),
			"invalid_request_error",
			http.StatusBadRequest,
		)
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	if !image.IsImageURL(contentType) {
		if ext := strings.ToLower(filepath.Ext(fileHeader.Filename)); ext != "" {
			if detected := mime.TypeByExtension(ext); detected != "" {
				contentType = detected
			}
		}
	}

	if !image.IsImageURL(contentType) {
		return "", relaymodel.WrapperOpenAIErrorWithMessage(
			"image file is not an image",
			"invalid_request_error",
			http.StatusBadRequest,
		)
	}

	contentType = image.TrimImageContentType(contentType)

	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func ImageHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if isAliMultimodalImageModel(meta) {
		return MultimodalImageHandler(meta, c, resp)
	}

	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	responseFormat, _ := meta.MustGet(MetaResponseFormat).(string)

	var aliTaskResponse TaskResponse

	err := common.UnmarshalResponse(resp, &aliTaskResponse)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	if aliTaskResponse.Message != "" {
		log.Error("aliAsyncTask err: " + aliTaskResponse.Message)

		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			errors.New(aliTaskResponse.Message),
			"ali_async_task_failed",
			http.StatusInternalServerError,
		)
	}

	aliResponse, err := asyncTaskWait(
		c.Request.Context(),
		meta,
		aliTaskResponse.Output.TaskID,
	)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"ali_async_task_wait_failed",
			http.StatusInternalServerError,
		)
	}

	if aliResponse.Output.TaskStatus != "SUCCEEDED" {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			aliResponse.Output.Message,
			"ali_error",
			resp.StatusCode,
		)
	}

	fullTextResponse := responseAli2OpenAIImage(c.Request.Context(), aliResponse, responseFormat)

	jsonResponse, err := sonic.Marshal(fullTextResponse)
	if err != nil {
		responseErr := relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)

		return adaptor.DoResponseResult{
			Usage: fullTextResponse.Usage.ToModelUsage(),
		}, responseErr
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonResponse)))
	_, _ = c.Writer.Write(jsonResponse)

	return adaptor.DoResponseResult{Usage: fullTextResponse.Usage.ToModelUsage()}, nil
}

func MultimodalImageHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	var aliResponse MultimodalImageResponse
	if err := common.UnmarshalResponse(resp, &aliResponse); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	if aliResponse.Code != "" || aliResponse.Message != "" {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			aliResponse.Message,
			aliResponse.Code,
			http.StatusInternalServerError,
		)
	}

	imageResponse := responseQwenImage2OpenAI(
		c.Request.Context(),
		&aliResponse,
		meta.GetString(MetaResponseFormat),
	)

	jsonResponse, err := sonic.Marshal(imageResponse)
	if err != nil {
		responseErr := relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)

		return adaptor.DoResponseResult{
			Usage: imageResponse.Usage.ToModelUsage(),
		}, responseErr
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonResponse)))
	_, _ = c.Writer.Write(jsonResponse)

	return adaptor.DoResponseResult{
		Usage:      imageResponse.Usage.ToModelUsage(),
		UpstreamID: aliResponse.RequestID,
	}, nil
}

func asyncTask(ctx context.Context, m *meta.Meta, taskID string) (*TaskResponse, error) {
	var aliResponse TaskResponse

	taskURL, err := url.JoinPath(m.Channel.BaseURL, "/api/v1/tasks", taskID)
	if err != nil {
		return &aliResponse, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, taskURL, nil)
	if err != nil {
		return &aliResponse, err
	}

	req.Header.Set("Authorization", "Bearer "+m.Channel.Key)

	resp, err := utils.DoRequestWithMeta(req, m)
	if err != nil {
		return &aliResponse, err
	}
	defer resp.Body.Close()

	var response TaskResponse

	err = sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return &aliResponse, err
	}

	return &response, nil
}

func asyncTaskWait(ctx context.Context, m *meta.Meta, taskID string) (*TaskResponse, error) {
	waitSeconds := 2
	step := 0
	maxStep := 20

	for {
		step++

		rsp, err := asyncTask(ctx, m, taskID)
		if err != nil {
			return nil, err
		}

		if rsp.Output.TaskStatus == "" {
			return rsp, nil
		}

		switch rsp.Output.TaskStatus {
		case "FAILED":
			fallthrough
		case "CANCELED":
			fallthrough
		case "SUCCEEDED":
			fallthrough
		case "UNKNOWN":
			return rsp, nil
		}

		if step >= maxStep {
			break
		}

		time.Sleep(time.Duration(waitSeconds) * time.Second)
	}

	return nil, errors.New("aliAsyncTaskWait timeout")
}

func responseAli2OpenAIImage(
	ctx context.Context,
	response *TaskResponse,
	responseFormat string,
) *relaymodel.ImageResponse {
	imageResponse := relaymodel.ImageResponse{
		Created: time.Now().Unix(),
	}

	for _, data := range response.Output.Results {
		appendAliImageData(ctx, &imageResponse, data.URL, data.B64Image, responseFormat)
	}

	if response.Output.ImageURL != "" || response.Output.B64Image != "" {
		appendAliImageData(
			ctx,
			&imageResponse,
			response.Output.ImageURL,
			response.Output.B64Image,
			responseFormat,
		)
	}

	imageResponse.Usage = aliImageUsageToOpenAI(response.Usage, int64(len(imageResponse.Data)))

	return &imageResponse
}

func appendAliImageData(
	ctx context.Context,
	imageResponse *relaymodel.ImageResponse,
	url string,
	b64Image string,
	responseFormat string,
) {
	if url == "" && b64Image == "" {
		return
	}

	b64Json := b64Image
	if responseFormat == "b64_json" && b64Json == "" && url != "" {
		_, imageData, err := image.GetImageFromURL(ctx, url)
		if err != nil {
			log.Error("getImageData Error getting image data: " + err.Error())
		} else {
			b64Json = imageData
		}
	}

	imageResponse.Data = append(imageResponse.Data, &relaymodel.ImageData{
		URL:     imageDataURLForResponseFormat(url, b64Json, responseFormat),
		B64Json: b64Json,
	})
}

func imageDataURLForResponseFormat(url, b64JSON, responseFormat string) string {
	if responseFormat == "b64_json" && b64JSON != "" {
		return ""
	}

	return url
}

func responseQwenImage2OpenAI(
	ctx context.Context,
	response *MultimodalImageResponse,
	responseFormat string,
) *relaymodel.ImageResponse {
	imageResponse := relaymodel.ImageResponse{
		Created: time.Now().Unix(),
	}

	appendData := func(url, b64Image, revisedPrompt string) {
		if url == "" && b64Image == "" {
			return
		}

		b64Json := b64Image
		if responseFormat == "b64_json" && b64Json == "" && url != "" {
			_, imageData, err := image.GetImageFromURL(ctx, url)
			if err != nil {
				log.Error("getImageData Error getting image data: " + err.Error())
			} else {
				b64Json = imageData
			}
		}

		imageResponse.Data = append(imageResponse.Data, &relaymodel.ImageData{
			URL:           imageDataURLForResponseFormat(url, b64Json, responseFormat),
			B64Json:       b64Json,
			RevisedPrompt: revisedPrompt,
		})
	}

	for _, choice := range response.Output.Choices {
		for _, content := range choice.Message.Content {
			appendData(
				firstNonEmpty(content.Image, content.URL),
				content.B64Image,
				content.ActualPrompt,
			)
		}
	}

	for _, result := range response.Output.Results {
		appendData(
			firstNonEmpty(result.Image, result.URL),
			result.B64Image,
			result.ActualPrompt,
		)
	}

	imageResponse.Usage = aliImageUsageToOpenAI(response.Usage, int64(len(imageResponse.Data)))

	return &imageResponse
}

func aliImageUsageToOpenAI(usage AliImageUsage, fallbackImageCount int64) *relaymodel.ImageUsage {
	imageCount := usage.ImageCount
	if imageCount == 0 {
		imageCount = fallbackImageCount
	}

	return &relaymodel.ImageUsage{
		OutputTokens: imageCount,
		TotalTokens:  imageCount,
		OutputTokensDetails: &relaymodel.ImageOutputTokensDetails{
			ImageTokens: imageCount,
		},
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
