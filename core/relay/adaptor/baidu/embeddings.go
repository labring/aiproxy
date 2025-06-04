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

type EmbeddingsResponse struct {
	*Error
	Usage relaymodel.Usage `json:"usage"`
}

func EmbeddingsHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
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
	var baiduResponse EmbeddingsResponse
	err = sonic.Unmarshal(body, &baiduResponse)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			nil,
			http.StatusInternalServerError,
		)
	}
	if baiduResponse.Error != nil && baiduResponse.ErrorCode != 0 {
		return baiduResponse.Usage.ToModelUsage(), ErrorHandler(baiduResponse.Error)
	}

	respMap := make(map[string]any)
	err = sonic.Unmarshal(body, &respMap)
	if err != nil {
		return baiduResponse.Usage.ToModelUsage(), relaymodel.WrapperOpenAIErrorWithMessage(
			err.Error(),
			nil,
			http.StatusInternalServerError,
		)
	}
	respMap["model"] = meta.OriginModel
	respMap["object"] = "list"

	data, err := sonic.Marshal(respMap)
	if err != nil {
		return baiduResponse.Usage.ToModelUsage(), relaymodel.WrapperOpenAIErrorWithMessage(
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
	return baiduResponse.Usage.ToModelUsage(), nil
}
