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
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
	"github.com/labring/aiproxy/core/relay/utils"
	"github.com/sirupsen/logrus"
)

var responseStreamInitialBufferTimeout = 2 * time.Second

// ConvertResponseRequest converts a response creation request
func ConvertResponseRequest(
	meta *meta.Meta,
	req *http.Request,
	callback ...func(node *ast.Node) error,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2NodeReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	for _, callback := range callback {
		if callback == nil {
			continue
		}

		if err := callback(&node); err != nil {
			return adaptor.ConvertResult{}, err
		}
	}

	if err := normalizeResponsesInputSystemRole(&node); err != nil {
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

func normalizeResponsesInputSystemRole(node *ast.Node) error {
	inputNode := node.Get("input")
	if !inputNode.Exists() || inputNode.TypeSafe() != ast.V_ARRAY {
		return nil
	}

	inputItems, err := inputNode.ArrayUseNode()
	if err != nil {
		return err
	}

	for index, inputItem := range inputItems {
		if inputItem.TypeSafe() != ast.V_OBJECT {
			continue
		}

		roleNode := inputItem.Get("role")
		if !roleNode.Exists() || roleNode.TypeSafe() != ast.V_STRING {
			continue
		}

		role, err := roleNode.String()
		if err != nil {
			return err
		}

		if role != relaymodel.RoleSystem {
			continue
		}

		_, err = inputItem.Set("role", ast.NewString(relaymodel.RoleDeveloper))
		if err != nil {
			return err
		}

		_, err = inputNode.SetByIndex(index, inputItem)
		if err != nil {
			return err
		}
	}

	return nil
}

// ResponseHandler handles non-streaming response
func ResponseHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if !adaptor.IsSuccessfulResponseStatus(mode.Responses, resp.StatusCode) {
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

	// Parse the response
	var response relaymodel.Response

	err = sonic.Unmarshal(responseBody, &response)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	// Store the response ID if needed for later retrieval
	if response.Store && response.ID != "" {
		err = store.SaveStore(adaptor.StoreCache{
			ID:        model.ResponseStoreID(response.ID),
			GroupID:   meta.Group.ID,
			TokenID:   meta.Token.ID,
			ChannelID: meta.Channel.ID,
			Model:     meta.OriginModel,
			ExpiresAt: time.Now().Add(time.Hour * 24 * 7), // Store for 7 days
		}, meta.Channel.Scope)
		if err != nil {
			log := common.GetLogger(c)
			log.Errorf("save response store failed: %v", err)
		}
	}

	responseBody, err = rewriteTopLevelModel(responseBody, responseModelName(meta))
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"rewrite_response_model_failed",
			http.StatusInternalServerError,
		)
	}

	// Write response
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(responseBody)))
	_, _ = c.Writer.Write(responseBody)

	// Calculate usage
	usage := response.ToModelUsage()

	return adaptor.DoResponseResult{
		Usage:      usage,
		UpstreamID: response.ID,
		AsyncUsage: responseNeedsAsyncUsage(&response),
	}, nil
}

// ResponseStreamHandler handles streaming response
func ResponseStreamHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if !adaptor.IsSuccessfulResponseStatus(mode.Responses, resp.StatusCode) {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	done := make(chan struct{})
	defer close(done)

	events := scanResponseStreamEvents(resp.Body, done, meta.OriginModel, meta.ActualModel)

	var (
		errorState    responsesStreamErrorState
		pendingEvents [][]byte
		wroteStream   bool
		bufferTimer   *time.Timer
	)
	defer func() {
		stopResponseStreamBufferTimer(bufferTimer)
	}()

readLoop:
	for {
		var item responseStreamEventItem

		if bufferTimer == nil {
			next, ok := <-events
			if !ok {
				break
			}

			item = next
		} else {
			select {
			case next, ok := <-events:
				if !ok {
					break readLoop
				}

				item = next
			case <-bufferTimer.C:
				bufferTimer = nil

				flushDelayedResponseStreamEvents(c, &pendingEvents, &wroteStream)
				continue
			}
		}

		if item.scanErr != nil {
			log.Error("error reading response stream: " + item.scanErr.Error())
			continue
		}

		if item.parseErr != nil {
			log.Error("error unmarshalling response stream: " + item.parseErr.Error())
			continue
		}

		event := item.event
		data := item.data
		data = rewriteResponseStreamEventModel(data, &event, responseModelName(meta), log)

		if err := errorState.errorBeforeEvent(&event); err != nil {
			return errorState.result(), err
		}

		// Store response ID if this is the first event with a response
		if event.Response != nil && errorState.responseID == "" {
			errorState.responseID = event.Response.ID
			if event.Response.Store && errorState.responseID != "" {
				saveErr := store.SaveStore(adaptor.StoreCache{
					ID:        model.ResponseStoreID(errorState.responseID),
					GroupID:   meta.Group.ID,
					TokenID:   meta.Token.ID,
					ChannelID: meta.Channel.ID,
					Model:     meta.OriginModel,
					ExpiresAt: time.Now().Add(time.Hour * 24 * 7),
				}, meta.Channel.Scope)
				if saveErr != nil {
					log.Errorf("save response store failed: %v", saveErr)
				}
			}
		}

		// Update usage if available
		errorState.update(&event)

		if event.Type == relaymodel.EventResponseFailed || event.Type == relaymodel.EventError {
			if wroteStream {
				log.Error(
					"response stream failed after data was sent: " + responseStreamErrorMessage(
						&event,
					),
				)
			} else {
				err, handled := errorState.handleFailure(&event)
				if handled && err == nil {
					continue
				}

				if handled {
					return errorState.result(), err
				}
			}
		}

		if responseStreamEventCanDelay(event.Type) && !wroteStream {
			pendingEvents = append(pendingEvents, append([]byte(nil), data...))

			if bufferTimer == nil {
				bufferTimer = time.NewTimer(responseStreamInitialBufferTimeout)
			}

			continue
		}

		stopResponseStreamBufferTimer(bufferTimer)
		bufferTimer = nil

		flushDelayedResponseStreamEvents(c, &pendingEvents, &wroteStream)

		// Forward the event
		render.ResponsesData(c, data)

		wroteStream = true
	}

	if errorState.pendingFailure != nil && !wroteStream {
		return errorState.result(), responseStreamError(errorState.pendingFailure)
	}

	flushDelayedResponseStreamEvents(c, &pendingEvents, &wroteStream)

	return errorState.result(), nil
}

type responseStreamEventItem struct {
	data     []byte
	event    relaymodel.ResponseStreamEvent
	parseErr error
	scanErr  error
}

func scanResponseStreamEvents(
	body io.Reader,
	done <-chan struct{},
	modelNames ...string,
) <-chan responseStreamEventItem {
	events := make(chan responseStreamEventItem, 16)

	go func() {
		defer close(events)

		scanner, cleanup := utils.NewStreamScanner(body, modelNames...)
		defer cleanup()

		for scanner.Scan() {
			data := scanner.Bytes()
			if !render.IsValidSSEData(data) {
				continue
			}

			data = append([]byte(nil), render.ExtractSSEData(data)...)

			var event relaymodel.ResponseStreamEvent

			parseErr := sonic.Unmarshal(data, &event)

			item := responseStreamEventItem{
				data:     data,
				event:    event,
				parseErr: parseErr,
			}

			select {
			case events <- item:
			case <-done:
				return
			}
		}

		if err := scanner.Err(); err != nil {
			select {
			case events <- responseStreamEventItem{scanErr: err}:
			case <-done:
			}
		}
	}()

	return events
}

func stopResponseStreamBufferTimer(timer *time.Timer) {
	if timer == nil {
		return
	}

	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func flushDelayedResponseStreamEvents(
	c *gin.Context,
	pendingEvents *[][]byte,
	wroteStream *bool,
) {
	if len(*pendingEvents) == 0 {
		return
	}

	for _, pendingEvent := range *pendingEvents {
		render.ResponsesData(c, pendingEvent)
	}

	*pendingEvents = nil
	*wroteStream = true
}

func rewriteResponseStreamEventModel(
	data []byte,
	event *relaymodel.ResponseStreamEvent,
	originModel string,
	log *logrus.Entry,
) []byte {
	if originModel == "" || event == nil || event.Response == nil ||
		event.Response.Model == originModel {
		return data
	}

	rewrittenData, err := rewriteNestedModel(data, originModel, "response", "model")
	if err != nil {
		log.Error("error rewriting response stream event model: " + err.Error())
		return data
	}

	return rewrittenData
}

func writeResponseObjectWithOriginModel(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (relaymodel.Response, adaptor.Error) {
	responseBody, err := common.GetResponseBody(resp)
	if err != nil {
		return relaymodel.Response{}, relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	var response relaymodel.Response

	err = sonic.Unmarshal(responseBody, &response)
	if err != nil {
		return relaymodel.Response{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	responseBody, err = rewriteTopLevelModel(responseBody, responseModelName(meta))
	if err != nil {
		return response, relaymodel.WrapperOpenAIError(
			err,
			"rewrite_response_model_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(responseBody)))
	_, _ = c.Writer.Write(responseBody)

	return response, nil
}

func rewriteTopLevelModel(data []byte, modelName string) ([]byte, error) {
	node, err := common.GetJSONNodeNoCopy(data)
	if err != nil {
		return nil, err
	}

	return rewriteTopLevelModelNode(data, &node, modelName)
}

func rewriteNestedModel(data []byte, modelName string, path ...string) ([]byte, error) {
	if modelName == "" {
		return data, nil
	}

	node, err := common.GetJSONNodeNoCopy(data)
	if err != nil {
		return nil, err
	}

	if len(path) == 0 {
		return rewriteTopLevelModelNode(data, &node, modelName)
	}

	parent := &node
	for _, key := range path[:len(path)-1] {
		next := parent.Get(key)
		if !next.Exists() {
			return data, nil
		}

		parent = next
	}

	modelKey := path[len(path)-1]

	modelNode := parent.Get(modelKey)
	if !modelNode.Exists() {
		return data, nil
	}

	if currentModel, err := modelNode.String(); err == nil && currentModel == modelName {
		return data, nil
	}

	_, err = parent.Set(modelKey, ast.NewString(modelName))
	if err != nil {
		return nil, err
	}

	return node.MarshalJSON()
}

func rewriteTopLevelModelNode(data []byte, node *ast.Node, modelName string) ([]byte, error) {
	if modelName == "" || node == nil {
		return data, nil
	}

	modelNode := node.Get("model")
	if !modelNode.Exists() {
		return data, nil
	}

	if currentModel, err := modelNode.String(); err == nil && currentModel == modelName {
		return data, nil
	}

	_, err := node.Set("model", ast.NewString(modelName))
	if err != nil {
		return nil, err
	}

	return node.MarshalJSON()
}

func responseNeedsAsyncUsage(response *relaymodel.Response) bool {
	if response == nil || response.ID == "" || response.Usage != nil {
		return false
	}

	if usage := response.ToModelUsage(); usage.TotalTokens > 0 || usage.WebSearchCount > 0 {
		return false
	}

	switch response.Status {
	case relaymodel.ResponseStatusInProgress, relaymodel.ResponseStatusQueued:
		return true
	default:
		return false
	}
}

// GetResponseHandler handles GET /v1/responses/{response_id}
func GetResponseHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	response, err := writeResponseObjectWithOriginModel(meta, c, resp)
	if err != nil {
		return adaptor.DoResponseResult{}, err
	}

	return adaptor.DoResponseResult{
		UpstreamID: response.ID,
	}, nil
}

// DeleteResponseHandler handles DELETE /v1/responses/{response_id}
func DeleteResponseHandler(
	_ *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if !adaptor.IsSuccessfulResponseStatus(mode.ResponsesDelete, resp.StatusCode) {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	c.Writer.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	c.Writer.Header().Set("Content-Length", resp.Header.Get("Content-Length"))
	c.Status(http.StatusNoContent)
	_, _ = io.Copy(c.Writer, resp.Body)

	return adaptor.DoResponseResult{}, nil
}

// CancelResponseHandler handles POST /v1/responses/{response_id}/cancel
func CancelResponseHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	response, err := writeResponseObjectWithOriginModel(meta, c, resp)
	if err != nil {
		return adaptor.DoResponseResult{}, err
	}

	return adaptor.DoResponseResult{
		Usage:      response.ToModelUsage(),
		UpstreamID: response.ID,
		AsyncUsage: responseNeedsAsyncUsage(&response),
	}, nil
}

// GetInputItemsHandler handles GET /v1/responses/{response_id}/input_items
func GetInputItemsHandler(
	_ *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	c.Writer.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	c.Writer.Header().Set("Content-Length", resp.Header.Get("Content-Length"))
	_, _ = io.Copy(c.Writer, resp.Body)

	return adaptor.DoResponseResult{}, nil
}
