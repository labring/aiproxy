package siliconflow

import (
	"bytes"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/image"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

type ImageRequest struct {
	Model             string `json:"model"`
	Prompt            string `json:"prompt"`
	NegativePrompt    string `json:"negative_prompt"`
	ImageSize         string `json:"image_size"`
	BatchSize         int    `json:"batch_size"`
	Seed              int64  `json:"seed"`
	NumInferenceSteps int    `json:"num_inference_steps"`
	GuidanceScale     int    `json:"guidance_scale"`
	PromptEnhancement bool   `json:"prompt_enhancement"`
}

type imageResponse struct {
	Images  []imageResponseImage `json:"images"`
	Timings map[string]any       `json:"timings,omitempty"`
	Seed    int64                `json:"seed,omitempty"`
}

type imageResponseImage struct {
	URL string `json:"url"`
}

func ConvertImageRequest(meta *meta.Meta, request *http.Request) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2ValidatedNode(request)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	responseFormat, err := node.Get("response_format").String()
	if err != nil && !errors.Is(err, ast.ErrNotExist) {
		return adaptor.ConvertResult{}, err
	}

	meta.Set(openai.MetaResponseFormat, responseFormat)

	if _, err := node.Set("model", ast.NewString(meta.ActualModel)); err != nil {
		return adaptor.ConvertResult{}, err
	}

	if err := renameImageRequestField(&node, "n", "batch_size"); err != nil {
		return adaptor.ConvertResult{}, err
	}

	if err := renameImageRequestField(&node, "steps", "num_inference_steps"); err != nil {
		return adaptor.ConvertResult{}, err
	}

	if err := renameImageRequestField(&node, "scale", "guidance_scale"); err != nil {
		return adaptor.ConvertResult{}, err
	}

	if err := renameImageRequestSize(&node); err != nil {
		return adaptor.ConvertResult{}, err
	}

	data, err := node.MarshalJSON()
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

func renameImageRequestField(node *ast.Node, oldKey, newKey string) error {
	value := node.Get(oldKey)
	if !value.Exists() {
		return nil
	}

	if _, err := node.Set(newKey, *value); err != nil {
		return err
	}

	_, err := node.Unset(oldKey)

	return err
}

func renameImageRequestSize(node *ast.Node) error {
	value := node.Get("size")
	if !value.Exists() {
		return nil
	}

	size, err := value.String()
	if err != nil {
		return renameImageRequestField(node, "size", "image_size")
	}

	if _, err := node.Set("image_size", ast.NewString(normalizeSiliconFlowSize(size))); err != nil {
		return err
	}

	_, err = node.Unset("size")

	return err
}

func ImageHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	var sfResponse imageResponse
	if err := common.UnmarshalResponse(resp, &sfResponse); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	openaiResponse := relaymodel.ImageResponse{
		Created: time.Now().Unix(),
		Data:    make([]*relaymodel.ImageData, 0, len(sfResponse.Images)),
	}

	for _, img := range sfResponse.Images {
		openaiResponse.Data = append(openaiResponse.Data, &relaymodel.ImageData{
			URL: img.URL,
		})
	}

	var err error

	if meta.GetString(openai.MetaResponseFormat) == "b64_json" {
		for i := range openaiResponse.Data {
			data := openaiResponse.Data[i]
			if data.B64Json != "" || data.URL == "" {
				continue
			}

			_, data.B64Json, err = image.GetImageFromURL(c.Request.Context(), data.URL)
			if err != nil {
				log.Warnf(
					"convert siliconflow image url to b64_json failed, keep original url: %v",
					err,
				)

				continue
			}

			data.URL = ""
		}
	}

	data, err := sonic.Marshal(openaiResponse)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(data)))

	_, err = c.Writer.Write(data)
	if err != nil {
		log.Warnf("write response body failed: %v", err)
	}

	usage := model.Usage{
		ImageOutputTokens: model.ZeroNullInt64(len(openaiResponse.Data)),
	}
	usage.OutputTokens = usage.ImageOutputTokens
	usage.TotalTokens = usage.OutputTokens

	return adaptor.DoResponseResult{Usage: usage}, nil
}
