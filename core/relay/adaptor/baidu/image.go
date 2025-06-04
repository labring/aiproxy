package baidu

import (
	"io"
	"net/http"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

type ImageData struct {
	B64Image string `json:"b64_image"`
}

type ImageResponse struct {
	*Error
	ID      string       `json:"id"`
	Data    []*ImageData `json:"data"`
	Created int64        `json:"created"`
}

func ImageHandler(_ *meta.Meta, c *gin.Context, resp *http.Response) (model.Usage, adaptor.Error) {
	defer resp.Body.Close()

	log := middleware.GetLogger(c)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			nil,
			http.StatusInternalServerError,
		)
	}
	var imageResponse ImageResponse
	err = sonic.Unmarshal(body, &imageResponse)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			nil,
			http.StatusInternalServerError,
		)
	}

	usage := model.Usage{
		InputTokens: model.ZeroNullInt64(len(imageResponse.Data)),
		TotalTokens: model.ZeroNullInt64(len(imageResponse.Data)),
	}

	if imageResponse.Error != nil && imageResponse.ErrorMsg != "" {
		return usage, ErrorHandler(imageResponse.Error)
	}

	openaiResponse := ToOpenAIImageResponse(&imageResponse)
	data, err := sonic.Marshal(openaiResponse)
	if err != nil {
		return usage, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			nil,
			http.StatusInternalServerError,
		)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, err = c.Writer.Write(data)
	if err != nil {
		log.Warnf("write response body failed: %v", err)
	}
	return usage, nil
}

func ToOpenAIImageResponse(imageResponse *ImageResponse) *relaymodel.ImageResponse {
	response := &relaymodel.ImageResponse{
		Created: imageResponse.Created,
	}
	for _, data := range imageResponse.Data {
		response.Data = append(response.Data, &relaymodel.ImageData{
			B64Json: data.B64Image,
		})
	}
	return response
}
