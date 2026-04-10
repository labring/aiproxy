package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/common/notify"
	"github.com/labring/aiproxy/core/common/trylock"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/monitor"
	"github.com/labring/aiproxy/core/relay/adaptors"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/render"
	"github.com/labring/aiproxy/core/relay/utils"
	log "github.com/sirupsen/logrus"
)

const channelTestRequestID = "channel-test"

var (
	modelConfigCache     map[string]model.ModelConfig = make(map[string]model.ModelConfig)
	modelConfigCacheOnce sync.Once
)

func guessModelConfig(modelName string) model.ModelConfig {
	modelConfigCacheOnce.Do(func() {
		for _, c := range adaptors.ChannelAdaptor {
			for _, m := range c.Metadata().Models {
				if _, ok := modelConfigCache[m.Model]; !ok {
					modelConfigCache[m.Model] = m
				}
			}
		}
	})

	if cachedConfig, ok := modelConfigCache[modelName]; ok {
		return cachedConfig
	}

	return model.ModelConfig{}
}

// testSingleModel tests a single model in the channel
// If saveToDB is true, the test result will be saved to database
func testSingleModel(
	mc *model.ModelCaches,
	channel *model.Channel,
	modelName string,
	saveToDB bool,
) (*model.ChannelTest, error) {
	modelConfig, ok := mc.ModelConfig.GetModelConfig(modelName)
	if !ok {
		return nil, errors.New(modelName + " model config not found")
	}

	if modelConfig.Type == mode.Unknown {
		newModelConfig := guessModelConfig(modelName)
		if newModelConfig.Type != mode.Unknown {
			modelConfig = newModelConfig
		}
	}

	if modelConfig.Type != mode.Unknown {
		a, ok := adaptors.GetAdaptor(channel.Type)
		if !ok {
			return nil, errors.New("adaptor not found")
		}

		if !a.SupportMode(modelConfig.Type) {
			return nil, fmt.Errorf("%s not supported by adaptor", modelConfig.Type)
		}
	}

	if modelConfig.ExcludeFromTests {
		return &model.ChannelTest{
			TestAt:      time.Now(),
			Model:       modelName,
			ActualModel: modelName,
			Success:     true,
			Code:        http.StatusOK,
			Mode:        modelConfig.Type,
			ChannelName: channel.Name,
			ChannelType: channel.Type,
			ChannelID:   channel.ID,
		}, nil
	}

	body, m, err := utils.BuildRequest(modelConfig)
	if err != nil {
		return nil, err
	}

	w := httptest.NewRecorder()
	newc, _ := gin.CreateTestContext(w)
	newc.Request = &http.Request{
		URL:    &url.URL{},
		Body:   io.NopCloser(body),
		Header: make(http.Header),
	}
	middleware.SetRequestID(newc, channelTestRequestID)

	testMeta := meta.NewMeta(
		channel,
		m,
		modelName,
		modelConfig,
		meta.WithRequestID(channelTestRequestID),
	)
	result := relayHandler(newc, testMeta, mc)
	success := result.Error == nil

	var (
		respStr string
		code    int
	)

	if success {
		switch testMeta.Mode {
		case mode.AudioSpeech,
			mode.ImagesGenerations:
			respStr = ""
		default:
			respStr = w.Body.String()
		}

		code = w.Code
	} else {
		respBody, _ := result.Error.MarshalJSON()
		respStr = conv.BytesToString(respBody)
		code = result.Error.StatusCode()
	}

	ct := &model.ChannelTest{
		TestAt:      testMeta.RequestAt,
		Model:       testMeta.OriginModel,
		ActualModel: testMeta.ActualModel,
		Mode:        testMeta.Mode,
		Took:        time.Since(testMeta.RequestAt).Seconds(),
		Success:     success,
		Response:    respStr,
		Code:        code,
		ChannelName: channel.Name,
		ChannelType: channel.Type,
		ChannelID:   channel.ID,
	}

	// Only save to database for saved channels (not preview tests)
	if saveToDB && channel.ID != 0 {
		return channel.UpdateModelTest(
			testMeta.RequestAt,
			testMeta.OriginModel,
			testMeta.ActualModel,
			testMeta.Mode,
			time.Since(testMeta.RequestAt).Seconds(),
			success,
			respStr,
			code,
		)
	}

	return ct, nil
}

// TestChannel godoc
//
//	@Summary		Test channel model
//	@Description	Tests a single model in the channel
//	@Tags			channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id		path		int		true	"Channel ID"
//	@Param			model	path		string	true	"Model name"
//	@Success		200		{object}	middleware.APIResponse{data=model.ChannelTest}
//	@Router			/api/channel/{id}/{model} [get]
//
//nolint:goconst
func TestChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	modelName := strings.TrimPrefix(c.Param("model"), "/")
	if modelName == "" {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: "model is required",
		})

		return
	}

	channel, err := model.LoadChannelByID(id)
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: "channel not found",
		})

		return
	}

	if !slices.Contains(channel.Models, modelName) {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: "model not supported by channel",
		})

		return
	}

	ct, err := testSingleModel(model.LoadModelCaches(), channel, modelName, true)
	if err != nil {
		log.Errorf(
			"failed to test channel %s(%d) model %s: %s",
			channel.Name,
			channel.ID,
			modelName,
			err.Error(),
		)
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: fmt.Sprintf(
				"failed to test channel %s(%d) model %s: %s",
				channel.Name,
				channel.ID,
				modelName,
				err.Error(),
			),
		})

		return
	}

	if c.Query("success_body") != "true" && ct.Success {
		ct.Response = ""
	}

	c.JSON(http.StatusOK, middleware.APIResponse{
		Success: true,
		Data:    ct,
	})
}

type TestResult struct {
	Data    *model.ChannelTest `json:"data,omitempty"`
	Message string             `json:"message,omitempty"`
	Success bool               `json:"success"`
}

func processTestResult(
	mc *model.ModelCaches,
	channel *model.Channel,
	modelName string,
	saveToDB bool,
	returnSuccess, successResponseBody bool,
) *TestResult {
	ct, err := testSingleModel(mc, channel, modelName, saveToDB)

	e := &utils.UnsupportedModelTypeError{}
	if errors.As(err, &e) {
		log.Errorf("model %s not supported test: %s", modelName, err.Error())
		return nil
	}

	result := &TestResult{
		Success: err == nil,
	}
	if err != nil {
		result.Message = fmt.Sprintf(
			"failed to test channel %s(%d) model %s: %s",
			channel.Name,
			channel.ID,
			modelName,
			err.Error(),
		)

		return result
	}

	if !ct.Success {
		result.Data = ct
		return result
	}

	if !returnSuccess {
		return nil
	}

	if !successResponseBody {
		ct.Response = ""
	}

	result.Data = ct

	return result
}

// TestChannelModels godoc
//
//	@Summary		Test channel models
//	@Description	Tests all models in the channel
//	@Tags			channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id				path		int		true	"Channel ID"
//	@Param			return_success	query		bool	false	"Return success"
//	@Param			success_body	query		bool	false	"Success body"
//	@Param			stream			query		bool	false	"Stream"
//	@Success		200				{object}	middleware.APIResponse{data=[]TestResult}
//	@Router			/api/channel/{id}/test [get]
func TestChannelModels(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	channel, err := model.LoadChannelByID(id)
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: "channel not found",
		})

		return
	}

	returnSuccess := c.Query("return_success") == "true"
	successResponseBody := c.Query("success_body") == "true"
	isStream := c.Query("stream") == "true"

	results := make([]*TestResult, 0)
	resultsMutex := sync.Mutex{}
	hasError := atomic.Bool{}

	var wg sync.WaitGroup

	semaphore := make(chan struct{}, 5)

	models := slices.Clone(channel.Models)
	rand.Shuffle(len(models), func(i, j int) {
		models[i], models[j] = models[j], models[i]
	})

	mc := model.LoadModelCaches()

	for _, modelName := range models {
		wg.Add(1)

		semaphore <- struct{}{}

		go func(model string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			result := processTestResult(
				mc,
				channel,
				model,
				true,
				returnSuccess,
				successResponseBody,
			)
			if result == nil {
				return
			}

			if !result.Success || (result.Data != nil && !result.Data.Success) {
				hasError.Store(true)
			}

			resultsMutex.Lock()

			if isStream {
				err := render.OpenaiObjectData(c, result)
				if err != nil {
					log.Errorf("failed to render result: %s", err.Error())
				}
			} else {
				results = append(results, result)
			}

			resultsMutex.Unlock()
		}(modelName)
	}

	wg.Wait()

	if !hasError.Load() {
		err := model.ClearLastTestErrorAt(channel.ID)
		if err != nil {
			log.Errorf(
				"failed to clear last test error at for channel %s(%d): %s",
				channel.Name,
				channel.ID,
				err.Error(),
			)
		}
	}

	if !isStream {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: true,
			Data:    results,
		})
	}
}

// TestAllChannels godoc
//
//	@Summary		Test all channels
//	@Description	Tests all channels
//	@Tags			channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			test_disabled	query		bool	false	"Test disabled"
//	@Param			return_success	query		bool	false	"Return success"
//	@Param			success_body	query		bool	false	"Success body"
//	@Param			stream			query		bool	false	"Stream"
//	@Success		200				{object}	middleware.APIResponse{data=[]TestResult}
//
//	@Router			/api/channels/test [get]
func TestAllChannels(c *gin.Context) {
	testDisabled := c.Query("test_disabled") == "true"

	var (
		channels []*model.Channel
		err      error
	)

	if testDisabled {
		channels, err = model.LoadChannels()
	} else {
		channels, err = model.LoadEnabledChannels()
	}

	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	returnSuccess := c.Query("return_success") == "true"
	successResponseBody := c.Query("success_body") == "true"
	isStream := c.Query("stream") == "true"

	results := make([]*TestResult, 0)
	resultsMutex := sync.Mutex{}
	hasErrorMap := make(map[int]*atomic.Bool)

	var wg sync.WaitGroup

	semaphore := make(chan struct{}, 5)

	newChannels := slices.Clone(channels)
	rand.Shuffle(len(newChannels), func(i, j int) {
		newChannels[i], newChannels[j] = newChannels[j], newChannels[i]
	})

	mc := model.LoadModelCaches()

	for _, channel := range newChannels {
		channelHasError := &atomic.Bool{}
		hasErrorMap[channel.ID] = channelHasError

		models := slices.Clone(channel.Models)
		rand.Shuffle(len(models), func(i, j int) {
			models[i], models[j] = models[j], models[i]
		})

		for _, modelName := range models {
			wg.Add(1)

			semaphore <- struct{}{}

			go func(model string, ch *model.Channel, hasError *atomic.Bool) {
				defer wg.Done()
				defer func() { <-semaphore }()

				result := processTestResult(mc, ch, model, true, returnSuccess, successResponseBody)
				if result == nil {
					return
				}

				if !result.Success || (result.Data != nil && !result.Data.Success) {
					hasError.Store(true)
				}

				resultsMutex.Lock()

				if isStream {
					err := render.OpenaiObjectData(c, result)
					if err != nil {
						log.Errorf("failed to render result: %s", err.Error())
					}
				} else {
					results = append(results, result)
				}

				resultsMutex.Unlock()
			}(modelName, channel, channelHasError)
		}
	}

	wg.Wait()

	for id, hasError := range hasErrorMap {
		if !hasError.Load() {
			err := model.ClearLastTestErrorAt(id)
			if err != nil {
				log.Errorf("failed to clear last test error at for channel %d: %s", id, err.Error())
			}
		}
	}

	if !isStream {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: true,
			Data:    results,
		})
	}
}

func tryTestChannel(channelID int, modelName string) bool {
	return trylock.Lock(
		fmt.Sprintf("channel_test_lock:%d:%s", channelID, modelName),
		30*time.Second,
	)
}

func AutoTestBannedModels() {
	log := log.WithFields(log.Fields{
		"auto_test_banned_models": "true",
	})

	channels, err := monitor.GetAllBannedModelChannels(context.Background())
	if err != nil {
		log.Errorf("failed to get banned channels: %s", err.Error())
		return
	}

	if len(channels) == 0 {
		return
	}

	mc := model.LoadModelCaches()

	for modelName, ids := range channels {
		for _, id := range ids {
			if !tryTestChannel(int(id), modelName) {
				continue
			}

			channel, err := model.LoadChannelByID(int(id))
			if err != nil {
				log.Errorf("failed to get channel by model %s: %s", modelName, err.Error())
				continue
			}

			result, err := testSingleModel(mc, channel, modelName, true)
			if err != nil {
				notify.Error(
					fmt.Sprintf(
						"channel %s (type: %d, id: %d) model %s test failed",
						channel.Name,
						channel.Type,
						channel.ID,
						modelName,
					),
					err.Error(),
				)

				continue
			}

			if result.Success {
				notify.Info(
					fmt.Sprintf(
						"channel %s (type: %d, id: %d) model %s test success",
						channel.Name,
						channel.Type,
						channel.ID,
						modelName,
					),
					"unban it",
				)

				err = monitor.ClearChannelModelErrors(context.Background(), modelName, channel.ID)
				if err != nil {
					log.Errorf("clear channel errors failed: %+v", err)
				}
			} else {
				notify.Error(
					fmt.Sprintf(
						"channel %s (type: %d, id: %d) model %s test failed",
						channel.Name,
						channel.Type,
						channel.ID,
						modelName,
					),
					fmt.Sprintf("code: %d, response: %s", result.Code, result.Response),
				)
			}
		}
	}
}

// TestChannelRequest 用于测试未保存的渠道配置
// 尽可能接近 Channel 结构
type TestChannelRequest struct {
	Type         int               `json:"type"          binding:"required"`
	Key          string            `json:"key"           binding:"required"`
	BaseURL      string            `json:"base_url"`
	ProxyURL     string            `json:"proxy_url"`
	Name         string            `json:"name"`
	Models       []string          `json:"models"`
	ModelMapping map[string]string `json:"model_mapping"`
	Configs      map[string]any    `json:"configs"`
}

// TestSingleModelRequest 测试单个模型的请求
type TestSingleModelRequest struct {
	Type         int               `json:"type"          binding:"required"`
	Key          string            `json:"key"           binding:"required"`
	BaseURL      string            `json:"base_url"`
	ProxyURL     string            `json:"proxy_url"`
	Name         string            `json:"name"`
	Model        string            `json:"model"         binding:"required"`
	ModelMapping map[string]string `json:"model_mapping"`
	Configs      map[string]any    `json:"configs"`
}

// createTempChannel 创建临时 Channel 对象
func createTempChannel(req *TestChannelRequest) *model.Channel {
	return &model.Channel{
		Type:         model.ChannelType(req.Type),
		Key:          req.Key,
		BaseURL:      req.BaseURL,
		ProxyURL:     req.ProxyURL,
		Name:         req.Name,
		Models:       req.Models,
		ModelMapping: req.ModelMapping,
		Configs:      model.ChannelConfigs(req.Configs),
	}
}

// TestChannelPreview godoc
//
//	@Summary		Test channel preview (single model)
//	@Description	Test a single model in channel without saving to database
//	@Tags			channel
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request	body		TestSingleModelRequest	true	"Channel test request"
//	@Success		200		{object}	middleware.APIResponse{data=model.ChannelTest}
//	@Router			/api/channel/test [post]
func TestChannelPreview(c *gin.Context) {
	var req TestSingleModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	// 创建临时 Channel 对象（不保存到数据库）
	channel := &model.Channel{
		Type:         model.ChannelType(req.Type),
		Key:          req.Key,
		BaseURL:      req.BaseURL,
		ProxyURL:     req.ProxyURL,
		Name:         req.Name,
		Models:       []string{req.Model},
		ModelMapping: req.ModelMapping,
		Configs:      model.ChannelConfigs(req.Configs),
	}

	// 获取模型缓存
	mc := model.LoadModelCaches()

	// 测试单个模型 (不保存到数据库)
	ct, err := testSingleModel(mc, channel, req.Model, false)
	if err != nil {
		log.Errorf("failed to test channel preview: %s", err.Error())
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	// 不返回响应体中的敏感信息
	if ct.Success {
		ct.Response = ""
	}

	c.JSON(http.StatusOK, middleware.APIResponse{
		Success: true,
		Data:    ct,
	})
}

// TestChannelPreviewAll godoc
//
//	@Summary		Test channel preview (all models)
//	@Description	Test all models in channel without saving to database
//	@Tags			channel
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			return_success	query		bool				false	"Return success"
//	@Param			success_body	query		bool				false	"Success body"
//	@Param			stream			query		bool				false	"Stream mode (SSE)"
//	@Param			request			body		TestChannelRequest	true	"Channel test request"
//	@Success		200				{object}	middleware.APIResponse{data=[]TestResult}
//	@Router			/api/channel/test-all [post]
func TestChannelPreviewAll(c *gin.Context) {
	var req TestChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	// 检查是否有模型可测试
	if len(req.Models) == 0 {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: "no models to test",
		})

		return
	}

	// 创建临时 Channel 对象（不保存到数据库）
	channel := createTempChannel(&req)

	// 获取模型缓存
	mc := model.LoadModelCaches()

	returnSuccess := c.Query("return_success") == "true"
	successResponseBody := c.Query("success_body") == "true"
	isStream := c.Query("stream") == "true"

	results := make([]*TestResult, 0)
	resultsMutex := sync.Mutex{}
	hasError := atomic.Bool{}

	var wg sync.WaitGroup

	semaphore := make(chan struct{}, 5)

	// 随机打乱模型顺序
	models := slices.Clone(req.Models)
	rand.Shuffle(len(models), func(i, j int) {
		models[i], models[j] = models[j], models[i]
	})

	for _, modelName := range models {
		wg.Add(1)

		semaphore <- struct{}{}

		go func(model string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			result := processTestResult(
				mc,
				channel,
				model,
				false,
				returnSuccess,
				successResponseBody,
			)
			if result == nil {
				return
			}

			if !result.Success || (result.Data != nil && !result.Data.Success) {
				hasError.Store(true)
			}

			resultsMutex.Lock()
			if isStream {
				err := render.OpenaiObjectData(c, result)
				if err != nil {
					log.Errorf("failed to render result: %s", err.Error())
				}
			} else {
				results = append(results, result)
			}
			resultsMutex.Unlock()
		}(modelName)
	}

	wg.Wait()

	if !isStream {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: true,
			Data:    results,
		})
	}
}
