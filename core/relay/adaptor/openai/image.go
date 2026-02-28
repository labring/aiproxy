package openai

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/image"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func ConvertImagesRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2NodeReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	responseFormat, err := node.Get("response_format").String()
	if err != nil && !errors.Is(err, ast.ErrNotExist) {
		return adaptor.ConvertResult{}, err
	}

	meta.Set(MetaResponseFormat, responseFormat)

	_, err = node.Set("model", ast.NewString(meta.ActualModel))
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	jsonData, err := node.MarshalJSON()
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(jsonData))},
		},
		Body: bytes.NewReader(jsonData),
	}, nil
}

func ConvertImagesEditsRequest(
	meta *meta.Meta,
	request *http.Request,
) (adaptor.ConvertResult, error) {
	err := request.ParseMultipartForm(1024 * 1024 * 4)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	multipartBody := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(multipartBody)

	for key, values := range request.MultipartForm.Value {
		if len(values) == 0 {
			continue
		}

		value := values[0]

		if key == "model" {
			err = multipartWriter.WriteField(key, meta.ActualModel)
			if err != nil {
				return adaptor.ConvertResult{}, err
			}

			continue
		}

		if key == "response_format" {
			meta.Set(MetaResponseFormat, value)
			continue
		}

		err = multipartWriter.WriteField(key, value)
		if err != nil {
			return adaptor.ConvertResult{}, err
		}
	}

	for _, files := range request.MultipartForm.File {
		if len(files) == 0 {
			continue
		}

		fileHeader := files[0]

		file, err := fileHeader.Open()
		if err != nil {
			return adaptor.ConvertResult{}, err
		}

		w, err := multipartWriter.CreatePart(fileHeader.Header)
		if err != nil {
			file.Close()
			return adaptor.ConvertResult{}, err
		}

		_, err = io.Copy(w, file)
		file.Close()

		if err != nil {
			return adaptor.ConvertResult{}, err
		}
	}

	if err := multipartWriter.Close(); err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type": {multipartWriter.FormDataContentType()},
		},
		Body: multipartBody,
	}, nil
}

func ImagesHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	var imageResponse relaymodel.ImageResponse

	err := common.UnmarshalResponse(resp, &imageResponse)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	usage := model.Usage{
		InputTokens:  meta.RequestUsage.InputTokens,
		OutputTokens: meta.RequestUsage.OutputTokens,
		TotalTokens:  meta.RequestUsage.InputTokens + meta.RequestUsage.OutputTokens,
	}

	if imageResponse.Usage != nil {
		usage = imageResponse.Usage.ToModelUsage()
	}

	if meta.GetString(MetaResponseFormat) == "b64_json" {
		for _, data := range imageResponse.Data {
			if len(data.B64Json) > 0 {
				continue
			}

			_, data.B64Json, err = image.GetImageFromURL(c.Request.Context(), data.URL)
			if err != nil {
				return adaptor.DoResponseResult{Usage: usage}, relaymodel.WrapperOpenAIError(
					err,
					"get_image_from_url_failed",
					http.StatusInternalServerError,
				)
			}
		}
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

	_, err = c.Writer.Write(data)
	if err != nil {
		log.Warnf("write response body failed: %v", err)
	}

	return adaptor.DoResponseResult{Usage: usage}, nil
}
