package openai

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
	"github.com/labring/aiproxy/core/relay/utils"
)

// ConvertResponseRequest converts a response creation request
func ConvertResponseRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2NodeReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	// Set the model
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

// ResponseHandler handles non-streaming response
func ResponseHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	responseBody, err := common.GetResponseBody(resp)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	// Parse the response
	var response relaymodel.Response

	err = sonic.Unmarshal(responseBody, &response)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	// Store the response ID if needed for later retrieval
	if response.Store && response.ID != "" {
		err = store.SaveStore(adaptor.StoreCache{
			ID:        response.ID,
			GroupID:   meta.Group.ID,
			TokenID:   meta.Token.ID,
			ChannelID: meta.Channel.ID,
			Model:     meta.ActualModel,
			ExpiresAt: time.Now().Add(time.Hour * 24 * 7), // Store for 7 days
		})
		if err != nil {
			log := common.GetLogger(c)
			log.Errorf("save response store failed: %v", err)
		}
	}

	// Write response
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(responseBody)))
	_, _ = c.Writer.Write(responseBody)

	// Calculate usage
	if response.Usage != nil {
		return response.Usage.ToModelUsage(), nil
	}

	return model.Usage{}, nil
}

// ResponseStreamHandler handles streaming response
func ResponseStreamHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)
	scanner, cleanup := utils.NewStreamScanner(resp.Body, meta.ActualModel)
	defer cleanup()

	var (
		usage      model.Usage
		responseID string
	)

	for scanner.Scan() {
		data := scanner.Bytes()
		if !render.IsValidSSEData(data) {
			continue
		}

		data = render.ExtractSSEData(data)
		if render.IsSSEDone(data) {
			break
		}

		// Parse the stream event
		var event relaymodel.ResponseStreamEvent

		err := sonic.Unmarshal(data, &event)
		if err != nil {
			log.Error("error unmarshalling response stream: " + err.Error())
			continue
		}

		// Store response ID if this is the first event with a response
		if event.Response != nil && responseID == "" {
			responseID = event.Response.ID
			if event.Response.Store && responseID != "" {
				err = store.SaveStore(adaptor.StoreCache{
					ID:        responseID,
					GroupID:   meta.Group.ID,
					TokenID:   meta.Token.ID,
					ChannelID: meta.Channel.ID,
					Model:     meta.ActualModel,
					ExpiresAt: time.Now().Add(time.Hour * 24 * 7),
				})
				if err != nil {
					log.Errorf("save response store failed: %v", err)
				}
			}
		}

		// Update usage if available
		if event.Response != nil && event.Response.Usage != nil {
			usage = event.Response.Usage.ToModelUsage()
		}

		// Forward the event
		render.ResponsesData(c, data)
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading response stream: " + err.Error())
	}

	return usage, nil
}

// GetResponseHandler handles GET /v1/responses/{response_id}
func GetResponseHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	c.Writer.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	c.Writer.Header().Set("Content-Length", resp.Header.Get("Content-Length"))
	_, _ = io.Copy(c.Writer, resp.Body)

	return model.Usage{}, nil
}

// DeleteResponseHandler handles DELETE /v1/responses/{response_id}
func DeleteResponseHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	c.Writer.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	c.Writer.Header().Set("Content-Length", resp.Header.Get("Content-Length"))
	c.Status(http.StatusNoContent)
	_, _ = io.Copy(c.Writer, resp.Body)

	return model.Usage{}, nil
}

// CancelResponseHandler handles POST /v1/responses/{response_id}/cancel
func CancelResponseHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	c.Writer.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	c.Writer.Header().Set("Content-Length", resp.Header.Get("Content-Length"))
	_, _ = io.Copy(c.Writer, resp.Body)

	return model.Usage{}, nil
}

// GetInputItemsHandler handles GET /v1/responses/{response_id}/input_items
func GetInputItemsHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	c.Writer.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	c.Writer.Header().Set("Content-Length", resp.Header.Get("Content-Length"))
	_, _ = io.Copy(c.Writer, resp.Body)

	return model.Usage{}, nil
}
