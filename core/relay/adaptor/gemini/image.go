package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
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
	"github.com/labring/aiproxy/core/common/config"
	commonimage "github.com/labring/aiproxy/core/common/image"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
	"github.com/labring/aiproxy/core/relay/utils"
)

const geminiImageEditFetchTimeout = 30 * time.Second

const (
	gemini25FlashImageDefaultOutputTokens = int64(1290)
	gemini3ImageDefaultOutputTokens       = int64(1120)
	gemini3Image4KOutputTokens            = int64(2000)
)

func ConvertImageRequest(meta *meta.Meta, req *http.Request) (adaptor.ConvertResult, error) {
	imageRequest, err := utils.UnmarshalImageRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	if meta != nil {
		meta.Set(openai.MetaResponseFormat, imageRequest.ResponseFormat)
		meta.Set("stream", imageRequest.Stream)
	}

	if imageRequest.Prompt == "" {
		return adaptor.ConvertResult{}, convertRequestError(meta, "prompt is required")
	}

	geminiRequest := relaymodel.GeminiChatRequest{
		Contents: []*relaymodel.GeminiChatContent{
			{
				Role:  relaymodel.GeminiRoleUser,
				Parts: []*relaymodel.GeminiPart{{Text: imageRequest.Prompt}},
			},
		},
		GenerationConfig: buildImageGenerationConfig(imageRequest.Size),
	}

	data, err := sonic.Marshal(geminiRequest)
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

func ConvertImageEditRequest(meta *meta.Meta, req *http.Request) (adaptor.ConvertResult, error) {
	if err := common.ParseMultipartFormWithLimit(req); err != nil {
		return adaptor.ConvertResult{}, err
	}

	prompt := strings.TrimSpace(req.PostFormValue("prompt"))
	if prompt == "" {
		return adaptor.ConvertResult{}, convertRequestError(meta, "prompt is required")
	}

	if meta != nil {
		meta.Set(openai.MetaResponseFormat, req.PostFormValue("response_format"))
		meta.Set("stream", strings.EqualFold(req.PostFormValue("stream"), "true"))
	}

	cfg, err := loadConfig(meta)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	imageParts, err := geminiImageEditParts(
		meta,
		req.Context(),
		req.MultipartForm,
		autoImageURLToBase64Disabled(meta, cfg),
	)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	parts := make([]*relaymodel.GeminiPart, 0, len(imageParts)+1)
	parts = append(parts, imageParts...)
	parts = append(parts, &relaymodel.GeminiPart{Text: prompt})

	geminiRequest := relaymodel.GeminiChatRequest{
		Contents: []*relaymodel.GeminiChatContent{
			{
				Role:  relaymodel.GeminiRoleUser,
				Parts: parts,
			},
		},
		GenerationConfig: buildImageGenerationConfig(req.PostFormValue("size")),
	}

	data, err := sonic.Marshal(geminiRequest)
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

func buildImageGenerationConfig(
	size string,
) *relaymodel.GeminiChatGenerationConfig {
	config := &relaymodel.GeminiChatGenerationConfig{
		ResponseModalities: []string{
			relaymodel.GeminiModalityImage,
		},
		ImageConfig: &relaymodel.GeminiImageConfig{},
	}

	if aspectRatio := geminiImageAspectRatioFromSize(size); aspectRatio != "" {
		config.ImageConfig.AspectRatio = aspectRatio
	}

	if imageSize := geminiImageSizeFromSize(size); imageSize != "" {
		config.ImageConfig.ImageSize = imageSize
	}

	if config.ImageConfig.AspectRatio == "" && config.ImageConfig.ImageSize == "" {
		config.ImageConfig = nil
	}

	return config
}

func geminiImageEditParts(
	meta *meta.Meta,
	ctx context.Context,
	form *multipart.Form,
	disableAutoImageURLToBase64 bool,
) ([]*relaymodel.GeminiPart, error) {
	if form == nil {
		return nil, convertRequestError(meta, "image is required")
	}

	parts := []*relaymodel.GeminiPart{}

	for _, value := range firstFormValues(form.Value, "image", "image_url") {
		part, err := geminiImagePartFromString(meta, ctx, value, disableAutoImageURLToBase64)
		if err != nil {
			return nil, err
		}

		if part != nil {
			parts = append(parts, part)
		}
	}

	fileParts, err := geminiImagePartsFromFiles(meta, geminiImageEditFiles(form.File))
	if err != nil {
		return nil, err
	}

	parts = append(parts, fileParts...)

	if len(parts) == 0 {
		return nil, convertRequestError(meta, "image is required")
	}

	return parts, nil
}

func geminiImageEditFiles(files map[string][]*multipart.FileHeader) []*multipart.FileHeader {
	imageFiles := files["image"]
	imageArrayFiles := files["image[]"]

	result := make([]*multipart.FileHeader, 0, len(imageFiles)+len(imageArrayFiles))
	result = append(result, imageFiles...)
	result = append(result, imageArrayFiles...)

	return result
}

func firstFormValues(values map[string][]string, names ...string) []string {
	for _, name := range names {
		if len(values[name]) > 0 {
			return values[name]
		}
	}

	return nil
}

func geminiImagePartFromString(
	meta *meta.Meta,
	ctx context.Context,
	value string,
	disableAutoImageURLToBase64 bool,
) (*relaymodel.GeminiPart, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	if mimeType, data, ok := parseMediaDataURL(value, "image"); ok {
		return &relaymodel.GeminiPart{
			InlineData: &relaymodel.GeminiInlineData{
				MimeType: mimeType,
				Data:     data,
			},
		}, nil
	}

	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		if disableAutoImageURLToBase64 {
			return &relaymodel.GeminiPart{
				FileData: &relaymodel.GeminiFileData{
					FileURI: value,
				},
			}, nil
		}

		fetchCtx, cancel := context.WithTimeout(ctx, geminiImageEditFetchTimeout)
		defer cancel()

		mimeType, data, err := commonimage.GetImageFromURL(fetchCtx, value)
		if err != nil {
			return nil, err
		}

		return &relaymodel.GeminiPart{
			InlineData: &relaymodel.GeminiInlineData{
				MimeType: mimeType,
				Data:     data,
			},
		}, nil
	}

	return nil, convertRequestError(meta, "unsupported image value: "+value)
}

func geminiImagePartsFromFiles(
	meta *meta.Meta,
	files []*multipart.FileHeader,
) ([]*relaymodel.GeminiPart, error) {
	parts := make([]*relaymodel.GeminiPart, 0, len(files))

	for _, fileHeader := range files {
		if fileHeader == nil {
			continue
		}

		part, err := geminiImagePartFromFile(meta, fileHeader)
		if err != nil {
			return nil, err
		}

		parts = append(parts, part)
	}

	return parts, nil
}

func geminiImagePartFromFile(
	meta *meta.Meta,
	fileHeader *multipart.FileHeader,
) (*relaymodel.GeminiPart, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(common.LimitReader(file, config.MaxImageSize+1))
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > config.MaxImageSize {
		return nil, convertRequestError(
			meta,
			fmt.Sprintf("image too large: max: %d", config.MaxImageSize),
		)
	}

	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}

	if !strings.HasPrefix(mimeType, "image/") {
		if extensionMimeType := mime.TypeByExtension(
			filepath.Ext(fileHeader.Filename),
		); extensionMimeType != "" {
			mimeType = extensionMimeType
		}
	}

	if !strings.HasPrefix(mimeType, "image/") {
		return nil, convertRequestError(
			meta,
			"unsupported image content type: "+mimeType,
		)
	}

	return &relaymodel.GeminiPart{
		InlineData: &relaymodel.GeminiInlineData{
			MimeType: commonimage.TrimImageContentType(mimeType),
			Data:     base64.StdEncoding.EncodeToString(data),
		},
	}, nil
}

func geminiImageAspectRatioFromSize(size string) string {
	size = normalizeGeminiImageSize(size)
	switch size {
	case "1:1", "3:4", "4:3", "9:16", "16:9":
		return size
	}

	width, height, ok := relaymodel.ParseVideoDimensions(size)
	if !ok || width <= 0 || height <= 0 {
		return ""
	}

	return closestGeminiImageAspectRatio(width, height)
}

func absFloat64(value float64) float64 {
	if value < 0 {
		return -value
	}

	return value
}

func geminiImageSizeFromSize(size string) string {
	size = normalizeGeminiImageSize(size)
	switch size {
	case "512":
		return "512"
	case "1k":
		return "1K"
	case "2k":
		return "2K"
	case "4k":
		return "4K"
	}

	width, height, ok := relaymodel.ParseVideoDimensions(size)
	if !ok || width <= 0 || height <= 0 {
		return ""
	}

	longSide := max(width, height)
	switch {
	case longSide >= 3500:
		return "4K"
	case longSide >= 1500:
		return "2K"
	case longSide >= 900:
		return "1K"
	default:
		return "512"
	}
}

func normalizeGeminiImageSize(size string) string {
	size = strings.ToLower(strings.TrimSpace(size))
	size = strings.ReplaceAll(size, "×", "x")
	size = strings.ReplaceAll(size, "*", "x")

	return size
}

func closestGeminiImageAspectRatio(width, height int) string {
	type candidate struct {
		label string
		ratio float64
	}

	ratio := float64(width) / float64(height)
	candidates := []candidate{
		{"1:1", 1},
		{"1:4", 1.0 / 4.0},
		{"1:8", 1.0 / 8.0},
		{"2:3", 2.0 / 3.0},
		{"3:2", 3.0 / 2.0},
		{"3:4", 3.0 / 4.0},
		{"4:3", 4.0 / 3.0},
		{"4:1", 4.0},
		{"4:5", 4.0 / 5.0},
		{"5:4", 5.0 / 4.0},
		{"8:1", 8.0},
		{"9:16", 9.0 / 16.0},
		{"16:9", 16.0 / 9.0},
		{"21:9", 21.0 / 9.0},
	}

	best := candidates[0]
	bestDelta := absFloat64(ratio - best.ratio)

	for _, item := range candidates[1:] {
		delta := absFloat64(ratio - item.ratio)
		if delta < bestDelta {
			best = item
			bestDelta = delta
		}
	}

	return best.label
}

func ImageHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHandler(resp)
	}

	if utils.IsStreamResponse(resp) {
		return imageStreamHandler(meta, c, resp)
	}

	defer resp.Body.Close()

	var geminiResponse relaymodel.GeminiChatResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&geminiResponse); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	imageResponse, usage := geminiImageResponseToOpenAI(meta, &geminiResponse)
	if len(imageResponse.Data) == 0 {
		message := geminiImageEmptyResponseMessage(&geminiResponse)

		return adaptor.DoResponseResult{Usage: usage}, relaymodel.WrapperOpenAIErrorWithMessage(
			message,
			"empty_image",
			http.StatusInternalServerError,
		)
	}

	data, err := sonic.Marshal(imageResponse)
	if err != nil {
		return adaptor.DoResponseResult{Usage: usage}, relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = c.Writer.Write(data)

	return adaptor.DoResponseResult{Usage: usage}, nil
}

func geminiImageEmptyResponseMessage(response *relaymodel.GeminiChatResponse) string {
	const baseMessage = "gemini image response image is empty"

	if response == nil {
		return baseMessage
	}

	details := make([]string, 0, len(response.Candidates))

	for _, candidate := range response.Candidates {
		if candidate == nil {
			continue
		}

		if candidate.FinishReason != "" {
			details = append(details, "finish_reason="+candidate.FinishReason)
		}

		for _, part := range candidate.Content.Parts {
			if text := strings.TrimSpace(part.Text); text != "" {
				details = append(details, text)
			}
		}
	}

	if len(details) == 0 {
		return baseMessage
	}

	return baseMessage + ": " + strings.Join(details, "; ")
}

func geminiImageResponseToOpenAI(
	meta *meta.Meta,
	response *relaymodel.GeminiChatResponse,
) (relaymodel.ImageResponse, model.Usage) {
	imageResponse := relaymodel.ImageResponse{
		Created: time.Now().Unix(),
		Data:    make([]*relaymodel.ImageData, 0),
	}

	for _, candidate := range response.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.InlineData == nil ||
				!strings.HasPrefix(part.InlineData.MimeType, "image/") {
				continue
			}

			imageResponse.Data = append(imageResponse.Data, geminiImageData(meta, part.InlineData))
		}
	}

	usage := geminiImageUsageFromResponse(
		meta,
		response.UsageMetadata,
		int64(len(imageResponse.Data)),
	)
	imageResponse.Usage = usageToImageUsagePtr(usage)

	return imageResponse, usage
}

func geminiImageData(
	meta *meta.Meta,
	inlineData *relaymodel.GeminiInlineData,
) *relaymodel.ImageData {
	if inlineData == nil {
		return &relaymodel.ImageData{}
	}

	if meta != nil && meta.GetString(openai.MetaResponseFormat) == "url" {
		return &relaymodel.ImageData{
			URL: "data:" + inlineData.MimeType + ";base64," + inlineData.Data,
		}
	}

	return &relaymodel.ImageData{B64Json: inlineData.Data}
}

func geminiImageUsageFromResponse(
	meta *meta.Meta,
	usage *relaymodel.GeminiUsageMetadata,
	imageCount int64,
) model.Usage {
	if usage == nil {
		return geminiImageCountUsage(meta, imageCount)
	}

	return usage.ToModelUsage()
}

func imageStreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	defer resp.Body.Close()

	log := common.GetLogger(c)

	scanner, cleanup := utils.NewStreamScanner(resp.Body, meta.ActualModel)
	defer cleanup()

	usage := model.Usage{}
	imageIndex := 0

	var completedB64JSON string

	for scanner.Scan() {
		data := scanner.Bytes()
		if !render.IsValidSSEData(data) {
			continue
		}

		data = render.ExtractSSEData(data)
		if render.IsSSEDone(data) {
			break
		}

		var geminiResponse relaymodel.GeminiChatResponse
		if err := sonic.Unmarshal(data, &geminiResponse); err != nil {
			log.Error("error unmarshalling gemini image stream response: " + err.Error())
			continue
		}

		if geminiResponse.UsageMetadata != nil {
			usage = geminiImageUsageFromResponse(
				meta,
				geminiResponse.UsageMetadata,
				int64(imageIndex),
			)
		}

		for _, candidate := range geminiResponse.Candidates {
			for _, part := range candidate.Content.Parts {
				if part.InlineData == nil ||
					!strings.HasPrefix(part.InlineData.MimeType, "image/") {
					continue
				}

				index := imageIndex
				imageIndex++
				completedB64JSON = part.InlineData.Data

				err := render.ResponsesObjectData(c, relaymodel.ImageStreamEvent{
					Type:              relaymodel.ImageStreamEventPartialImage,
					PartialImageIndex: &index,
					B64Json:           part.InlineData.Data,
				})
				if err != nil {
					log.Warnf("write gemini image stream chunk failed: %v", err)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading gemini image stream: " + err.Error())
	}

	if usage.OutputTokens == 0 && imageIndex > 0 {
		usage = geminiImageCountUsage(meta, int64(imageIndex))
	}

	imageUsage := usageToImageUsage(usage)

	completed := relaymodel.ImageStreamEvent{
		Type:  relaymodel.ImageStreamEventCompleted,
		Usage: &imageUsage,
	}
	if completedB64JSON != "" {
		completed.B64Json = completedB64JSON
	}

	if err := render.ResponsesObjectData(c, completed); err != nil {
		log.Warnf("write gemini image stream completed failed: %v", err)
	}

	return adaptor.DoResponseResult{Usage: usage}, nil
}

func usageToImageUsage(usage model.Usage) relaymodel.ImageUsage {
	inputTokens := int64(usage.InputTokens)
	imageInputTokens := int64(usage.ImageInputTokens)
	outputTokens := int64(usage.OutputTokens)
	imageOutputTokens := int64(usage.ImageOutputTokens)

	return relaymodel.ImageUsage{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  int64(usage.TotalTokens),
		InputTokensDetails: relaymodel.ImageInputTokensDetails{
			TextTokens:  maxInt64(inputTokens-imageInputTokens, 0),
			ImageTokens: imageInputTokens,
		},
		OutputTokensDetails: &relaymodel.ImageOutputTokensDetails{
			ImageTokens: imageOutputTokens,
		},
	}
}

func usageToImageUsagePtr(usage model.Usage) *relaymodel.ImageUsage {
	imageUsage := usageToImageUsage(usage)
	return &imageUsage
}

func geminiImageCountUsage(meta *meta.Meta, imageCount int64) model.Usage {
	outputTokens := geminiImageOutputTokensFromContext(meta) * imageCount

	return model.Usage{
		OutputTokens:      model.ZeroNullInt64(outputTokens),
		ImageOutputTokens: model.ZeroNullInt64(outputTokens),
		TotalTokens:       model.ZeroNullInt64(outputTokens),
	}
}

func geminiImageOutputTokensFromContext(meta *meta.Meta) int64 {
	if meta == nil {
		return gemini3ImageDefaultOutputTokens
	}

	if strings.Contains(meta.ActualModel, "2.5-flash-image") ||
		strings.Contains(meta.OriginModel, "2.5-flash-image") ||
		strings.Contains(meta.ModelConfig.Model, "2.5-flash-image") {
		return gemini25FlashImageDefaultOutputTokens
	}

	if geminiImageSizeFromSize(meta.RequestUsageContext.Resolution) == "4K" {
		return gemini3Image4KOutputTokens
	}

	return gemini3ImageDefaultOutputTokens
}

func maxInt64(values ...int64) int64 {
	maxValue := int64(0)
	for _, value := range values {
		if value > maxValue {
			maxValue = value
		}
	}

	return maxValue
}
