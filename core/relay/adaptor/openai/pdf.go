package openai

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

// ConvertParsePdfRequest converts a multipart form request for PDF parsing,
// mirroring ConvertSTTRequest: form values are forwarded with the model name
// rewritten and files are copied through, so response_format and any other
// field keep reaching the upstream AIProxy-compatible endpoint.
func ConvertParsePdfRequest(
	meta *meta.Meta,
	request *http.Request,
) (adaptor.ConvertResult, error) {
	if err := common.ParseMultipartFormWithLimit(request); err != nil {
		return adaptor.ConvertResult{}, convertRequestError(
			meta,
			fmt.Sprintf("parse multipart form: %s", err),
		)
	}

	multipartBody := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(multipartBody)

	// The request model may come from the query string instead of the form,
	// so always write the model field instead of rewriting it in place.
	if err := multipartWriter.WriteField("model", meta.ActualModel); err != nil {
		return adaptor.ConvertResult{}, fmt.Errorf("write model field: %w", err)
	}

	for key, values := range request.MultipartForm.Value {
		if len(values) == 0 || key == "model" {
			continue
		}

		if err := multipartWriter.WriteField(key, values[0]); err != nil {
			return adaptor.ConvertResult{}, fmt.Errorf("write field %s: %w", key, err)
		}
	}

	if err := processFormFiles(multipartWriter, request.MultipartForm.File); err != nil {
		return adaptor.ConvertResult{}, fmt.Errorf("process form files: %w", err)
	}

	multipartWriter.Close()

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type": {multipartWriter.FormDataContentType()},
		},
		Body: multipartBody,
	}, nil
}

type parsePdfUsageResponse struct {
	Pages     int64    `json:"pages"`
	Markdowns []string `json:"markdowns"`
}

// ParsePdfHandler relays the upstream parse response body unchanged and bills
// by page count, matching the native doc2x adaptor usage semantics.
func ParsePdfHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	responseBody, err := common.GetResponseBody(resp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	// The upstream may answer with either format; usage extraction is best
	// effort so unexpected payloads still pass through.
	var usageResponse parsePdfUsageResponse

	_ = sonic.Unmarshal(responseBody, &usageResponse)

	pages := usageResponse.Pages
	if pages == 0 {
		pages = int64(len(usageResponse.Markdowns))
	}

	usage := model.Usage{
		InputTokens: model.ZeroNullInt64(pages),
		TotalTokens: model.ZeroNullInt64(pages),
	}

	if c.Request.Context().Err() != nil {
		return adaptor.DoResponseResult{Usage: usage}, nil
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(responseBody)))

	if _, err := c.Writer.Write(responseBody); err != nil {
		common.GetLogger(c).Warnf("write response body failed: %v", err)
	}

	return adaptor.DoResponseResult{Usage: usage}, nil
}
