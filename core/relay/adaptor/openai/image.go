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
	"github.com/labring/aiproxy/core/relay/render"
	"github.com/labring/aiproxy/core/relay/utils"
)

func ConvertImagesRequest(
	meta *meta.Meta,
	req *http.Request,
	callbacks ...func(node *ast.Node) error,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2ValidatedNode(req)
	if err != nil {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	responseFormat, err := node.Get("response_format").String()
	if err != nil && !errors.Is(err, ast.ErrNotExist) {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	meta.Set(MetaResponseFormat, responseFormat)

	_, err = node.Set("model", ast.NewString(meta.ActualModel))
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	for _, callback := range callbacks {
		if callback == nil {
			continue
		}

		err = callback(&node)
		if err != nil {
			return adaptor.ConvertResult{}, err
		}
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
	includeModel bool,
) (adaptor.ConvertResult, error) {
	err := common.ParseMultipartFormWithLimit(request)
	if err != nil {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	multipartBody := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(multipartBody)

	for key, values := range request.MultipartForm.Value {
		if len(values) == 0 {
			continue
		}

		value := values[0]

		if key == "model" {
			if includeModel {
				err = multipartWriter.WriteField(key, meta.ActualModel)
				if err != nil {
					return adaptor.ConvertResult{}, err
				}
			}

			continue
		}

		if key == "response_format" {
			meta.Set(MetaResponseFormat, value)
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

func ImagesRequestRemoveModel(node *ast.Node) error {
	_, err := node.Unset("model")
	if err != nil && !errors.Is(err, ast.ErrNotExist) {
		return err
	}

	return nil
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

	usage := model.Usage{}
	if imageResponse.Usage != nil {
		usage = imageResponse.Usage.ToModelUsage()
	} else if imageCount := successfulOpenAIImageCount(imageResponse.Data); imageCount > 0 {
		usage.OutputTokens = model.ZeroNullInt64(imageCount)
		usage.ImageOutputTokens = model.ZeroNullInt64(imageCount)
		usage.TotalTokens = usage.OutputTokens
	}

	if meta.GetString(MetaResponseFormat) == "b64_json" {
		for _, data := range imageResponse.Data {
			if len(data.B64Json) > 0 {
				continue
			}

			_, data.B64Json, err = image.GetImageFromURL(c.Request.Context(), data.URL)
			if err != nil {
				log.Warnf("convert image url to b64_json failed, keep original url: %v", err)
				continue
			}

			data.URL = ""
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

func successfulOpenAIImageCount(data []*relaymodel.ImageData) int64 {
	var count int64
	for _, item := range data {
		if item == nil {
			continue
		}

		if item.URL != "" || item.B64Json != "" {
			count++
		}
	}

	return count
}

func ImagesStreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	scanner, cleanup := utils.NewStreamScanner(resp.Body, meta.OriginModel, meta.ActualModel)
	defer cleanup()

	usage := model.Usage{}

	for scanner.Scan() {
		line := scanner.Bytes()
		if !render.IsValidSSEData(line) {
			continue
		}

		data := render.ExtractSSEData(line)

		node, err := common.GetJSONNodeNoCopy(data)
		if err != nil {
			log.Error("error unmarshalling image stream response: " + err.Error())
			render.ResponsesData(c, data)

			continue
		}

		if streamUsage, err := getImageStreamUsage(&node); err != nil {
			log.Error("error unmarshalling image stream usage: " + err.Error())
		} else if streamUsage != nil {
			usage = streamUsage.ToModelUsage()
		}

		render.ResponsesData(c, data)
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading image stream: " + err.Error())
	}

	return adaptor.DoResponseResult{Usage: usage}, nil
}

func getImageStreamUsage(node *ast.Node) (*relaymodel.ImageUsage, error) {
	usageNode := node.Get("usage")
	if usageNode == nil || !usageNode.Exists() || usageNode.TypeSafe() == ast.V_NULL {
		return nil, nil
	}

	usageRaw, err := usageNode.Raw()
	if err != nil {
		return nil, err
	}

	var usage relaymodel.ImageUsage
	if err := sonic.UnmarshalString(usageRaw, &usage); err != nil {
		return nil, err
	}

	return &usage, nil
}
