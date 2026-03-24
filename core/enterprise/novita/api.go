//go:build enterprise

package novita

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
)

type apiResponse struct {
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
	Success bool   `json:"success"`
}

func successResponse(c *gin.Context, data any) {
	c.JSON(http.StatusOK, &apiResponse{
		Success: true,
		Data:    data,
	})
}

func errorResponse(c *gin.Context, code int, message string) {
	c.JSON(code, &apiResponse{
		Success: false,
		Message: message,
	})
}

// maskAPIKey masks an API key, showing only first 6 and last 4 characters.
func maskAPIKey(key string) string {
	if len(key) <= 10 {
		return "****"
	}

	return key[:6] + "****" + key[len(key)-4:]
}

// channelItem is the JSON shape returned by ListChannelsHandler.
type channelItem struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	Key     string `json:"key"`
}

// novitaChannelWhere is the shared WHERE clause for finding Novita channels
// (by channel type OR base_url containing "novita").
func novitaChannelWhere() string {
	return "type = ? OR base_url " + common.LikeOp() + " ?"
}

// novitaChannelArgs returns the args for novitaChannelWhere.
func novitaChannelArgs() []any {
	return []any{model.ChannelTypeNovita, "%novita%"}
}

// ListChannelsHandler handles GET /api/enterprise/novita/channels.
func ListChannelsHandler(c *gin.Context) {
	var channels []model.Channel

	err := model.DB.Select("id, name, base_url, key").
		Where(novitaChannelWhere(), novitaChannelArgs()...).
		Order("id ASC").
		Find(&channels).Error
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]channelItem, 0, len(channels))
	for _, ch := range channels {
		baseURL := ch.BaseURL
		if baseURL == "" {
			baseURL = DefaultNovitaAPIBase
		}

		items = append(items, channelItem{
			ID:      ch.ID,
			Name:    ch.Name,
			BaseURL: baseURL,
			Key:     maskAPIKey(ch.Key),
		})
	}

	successResponse(c, items)
}

// GetConfigHandler handles GET /api/enterprise/novita/config.
func GetConfigHandler(c *gin.Context) {
	cfg := GetNovitaConfig()

	maskedKey := ""
	configured := cfg.APIKey != ""

	if configured {
		maskedKey = maskAPIKey(cfg.APIKey)
	}

	successResponse(c, gin.H{
		"channel_id":            cfg.ChannelID,
		"api_key":               maskedKey,
		"api_base":              cfg.APIBase,
		"configured":            configured,
		"mgmt_token_configured": cfg.MgmtToken != "",
		"exchange_rate":         cfg.ExchangeRate,
	})
}

// UpdateAPIKeyHandler handles PUT /api/enterprise/novita/api-key.
// Accepts an API key directly (bootstrap mode when no channels exist).
func UpdateAPIKeyHandler(c *gin.Context) {
	var req struct {
		APIKey  string `json:"api_key" binding:"required"`
		APIBase string `json:"api_base"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := SetNovitaAPIKeyDirect(req.APIKey, req.APIBase); err != nil {
		errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("failed to save API key: %v", err))
		return
	}

	successResponse(c, gin.H{"message": "API key saved"})
}

// UpdateConfigHandler handles PUT /api/enterprise/novita/config.
func UpdateConfigHandler(c *gin.Context) {
	var req struct {
		ChannelID int `json:"channel_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := SetNovitaConfigFromChannel(req.ChannelID); err != nil {
		errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("failed to save config: %v", err))
		return
	}

	successResponse(c, gin.H{"message": "config saved"})
}

// UpdateMgmtTokenHandler handles PUT /api/enterprise/novita/mgmt-token.
func UpdateMgmtTokenHandler(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := SetNovitaMgmtToken(req.Token); err != nil {
		errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("failed to save mgmt token: %v", err))
		return
	}

	successResponse(c, gin.H{"message": "mgmt token saved"})
}

// UpdateExchangeRateHandler handles PUT /api/enterprise/novita/exchange-rate.
func UpdateExchangeRateHandler(c *gin.Context) {
	var req struct {
		Rate float64 `json:"rate" binding:"required,gt=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := SetNovitaExchangeRate(req.Rate); err != nil {
		errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("failed to save exchange rate: %v", err))
		return
	}

	successResponse(c, gin.H{"message": "exchange rate saved"})
}

// DiagnosticHandler handles GET /api/enterprise/novita/sync/diagnostic.
func DiagnosticHandler(c *gin.Context) {
	result, err := Diagnostic(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, result)
}

// PreviewHandler handles POST /api/enterprise/novita/sync/preview.
func PreviewHandler(c *gin.Context) {
	var opts SyncOptions
	if err := c.ShouldBindJSON(&opts); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	client, err := NewNovitaClient()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	cfg := GetNovitaConfig()

	allModels, fetchErr := client.FetchAllModelsMerged(c.Request.Context(), cfg.MgmtToken)
	if fetchErr != nil {
		errorResponse(c, http.StatusInternalServerError, fetchErr.Error())
		return
	}

	diff, err := CompareNovitaModelsV2(allModels, opts, cfg.ExchangeRate)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, diff)
}

// ExecuteHandler handles POST /api/enterprise/novita/sync/execute (SSE streaming).
func ExecuteHandler(c *gin.Context) {
	var opts SyncOptions
	if err := c.ShouldBindJSON(&opts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": err.Error(),
		})

		return
	}

	if !opts.ChangesConfirmed && !opts.DryRun {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "changes_not_confirmed",
			"message": "Please confirm the changes before executing sync",
		})

		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	eventChan := make(chan SyncProgressEvent, 10)
	done := make(chan struct{})

	// Uses context.Background() so the sync completes even if the client disconnects;
	// progress events are dropped (not blocked) if nobody is reading.
	go func() {
		defer close(done)

		safeSend := func(event SyncProgressEvent) {
			select {
			case eventChan <- event:
			default:
			}
		}

		result, err := ExecuteSync(context.Background(), opts, func(event SyncProgressEvent) {
			safeSend(event)
		})
		if err != nil {
			if errors.Is(err, ErrSyncInProgress) {
				safeSend(SyncProgressEvent{Type: "error", Message: "同步操作正在进行中，请稍后再试"})
			} else {
				safeSend(SyncProgressEvent{Type: "error", Message: fmt.Sprintf("同步失败: %v", err)})
			}
		} else if !result.Success && len(result.Errors) > 0 {
			safeSend(SyncProgressEvent{
				Type:    "success",
				Step:    "complete",
				Message: fmt.Sprintf("同步完成（部分失败：%d 个错误）", len(result.Errors)),
				Data:    result,
			})
		}
	}()

	c.Stream(func(w io.Writer) bool {
		select {
		case event, ok := <-eventChan:
			if !ok {
				return false
			}

			eventJSON, err := sonic.Marshal(event)
			if err != nil {
				return false
			}

			fmt.Fprintf(w, "data: %s\n\n", eventJSON)
			c.Writer.Flush()

			if event.Type == "success" || event.Type == "error" {
				return false
			}

			return true

		case <-done:
			return false

		case <-c.Request.Context().Done():
			return false
		}
	})
}

// HistoryHandler handles GET /api/enterprise/novita/sync/history.
func HistoryHandler(c *gin.Context) {
	histories := make([]SyncHistory, 0)

	err := model.DB.Order("synced_at DESC").Limit(10).Find(&histories).Error
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	for i := range histories {
		var result SyncResult
		if err := sonic.Unmarshal([]byte(histories[i].Result), &result); err == nil {
			histories[i].ResultParsed = &result
		}
	}

	successResponse(c, histories)
}

// ModelCoverageHandler handles GET /api/enterprise/novita/model-coverage.
func ModelCoverageHandler(c *gin.Context) {
	var localModels []model.ModelConfig

	if err := model.DB.Select("model", "config").
		Where("owner = ?", string(model.ModelOwnerNovita)).Find(&localModels).Error; err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())

		return
	}

	var channels []model.Channel

	if err := model.DB.Select("models").
		Where("("+novitaChannelWhere()+") AND status = ?", append(novitaChannelArgs(), model.ChannelStatusEnabled)...).
		Find(&channels).Error; err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())

		return
	}

	inChannel := make(map[string]struct{})

	for _, ch := range channels {
		for _, m := range ch.Models {
			inChannel[m] = struct{}{}
		}
	}

	uncovered := make([]ModelCoverageItem, 0)

	for _, mc := range localModels {
		if _, ok := inChannel[mc.Model]; ok {
			continue
		}

		item := ModelCoverageItem{Model: mc.Model}

		if eps, ok := model.GetModelConfigStringSlice(mc.Config, "endpoints"); ok {
			item.Endpoints = eps
		}

		if mt, ok := mc.Config[model.ModelConfigKey("model_type")].(string); ok {
			item.ModelType = mt
		}

		uncovered = append(uncovered, item)
	}

	successResponse(c, ModelCoverageResult{
		Total:     len(localModels),
		Covered:   len(localModels) - len(uncovered),
		Uncovered: uncovered,
	})
}
