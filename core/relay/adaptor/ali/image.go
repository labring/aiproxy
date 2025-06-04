package ali

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/image"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
	log "github.com/sirupsen/logrus"
)

const MetaResponseFormat = "response_format"

func ConvertImageRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	request, err := utils.UnmarshalImageRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}
	request.Model = meta.ActualModel

	var imageRequest ImageRequest
	imageRequest.Input.Prompt = request.Prompt
	imageRequest.Model = request.Model
	imageRequest.Parameters.Size = strings.ReplaceAll(request.Size, "x", "*")
	imageRequest.Parameters.N = request.N
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

func ImageHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, openai.ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := middleware.GetLogger(c)

	responseFormat, _ := meta.MustGet(MetaResponseFormat).(string)

	var aliTaskResponse TaskResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}
	err = sonic.Unmarshal(responseBody, &aliTaskResponse)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	if aliTaskResponse.Message != "" {
		log.Error("aliAsyncTask err: " + aliTaskResponse.Message)
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			errors.New(aliTaskResponse.Message),
			"ali_async_task_failed",
			http.StatusInternalServerError,
		)
	}

	aliResponse, err := asyncTaskWait(c, aliTaskResponse.Output.TaskID, meta.Channel.Key)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"ali_async_task_wait_failed",
			http.StatusInternalServerError,
		)
	}

	if aliResponse.Output.TaskStatus != "SUCCEEDED" {
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			aliResponse.Output.Message,
			"ali_error",
			resp.StatusCode,
		)
	}

	fullTextResponse := responseAli2OpenAIImage(c.Request.Context(), aliResponse, responseFormat)
	jsonResponse, err := sonic.Marshal(fullTextResponse)
	if err != nil {
		return fullTextResponse.Usage.ToModelUsage(), relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonResponse)))
	_, _ = c.Writer.Write(jsonResponse)
	return fullTextResponse.Usage.ToModelUsage(), nil
}

func asyncTask(ctx context.Context, taskID, key string) (*TaskResponse, error) {
	url := "https://dashscope.aliyuncs.com/api/v1/tasks/" + taskID

	var aliResponse TaskResponse

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return &aliResponse, err
	}

	req.Header.Set("Authorization", "Bearer "+key)

	client := &http.Client{}
	resp, err := client.Do(req)
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

func asyncTaskWait(ctx context.Context, taskID, key string) (*TaskResponse, error) {
	waitSeconds := 2
	step := 0
	maxStep := 20

	for {
		step++
		rsp, err := asyncTask(ctx, taskID, key)
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
		var b64Json string
		if responseFormat == "b64_json" {
			// 读取 data.Url 的图片数据并转存到 b64Json
			_, imageData, err := image.GetImageFromURL(ctx, data.URL)
			if err != nil {
				// 处理获取图片数据失败的情况
				log.Error("getImageData Error getting image data: " + err.Error())
				continue
			}

			// 将图片数据转为 Base64 编码的字符串
			b64Json = imageData
		} else {
			// 如果 responseFormat 不是 "b64_json"，则直接使用 data.B64Image
			b64Json = data.B64Image
		}

		imageResponse.Data = append(imageResponse.Data, &relaymodel.ImageData{
			URL:           data.URL,
			B64Json:       b64Json,
			RevisedPrompt: "",
		})
	}
	imageResponse.Usage = &relaymodel.ImageUsage{
		OutputTokens: int64(len(imageResponse.Data)),
		TotalTokens:  int64(len(imageResponse.Data)),
	}
	return &imageResponse
}
