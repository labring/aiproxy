//go:build enterprise

package ppio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/env"
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

// ppioChannelWhere returns a WHERE clause that matches all PPIO-related channels.
// PPIO uses api.ppinfra.com for both OpenAI and Anthropic endpoints.
// Legacy channels may still reference the deprecated api.ppio.com domain, so we match both.
// Also matches ChannelTypePPIO for channels that may have an empty base_url.
func ppioChannelWhere() string {
	like := common.LikeOp()
	return "type = ? OR base_url " + like + " ? OR base_url " + like + " ?"
}

// ppioChannelArgs returns the args for ppioChannelWhere.
func ppioChannelArgs() []any {
	return []any{model.ChannelTypePPIO, "%ppio%", "%ppinfra%"}
}

// channelItem is the JSON shape returned by ListChannelsHandler.
type channelItem struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	Key     string `json:"key"`
}

// ListChannelsHandler handles GET /api/enterprise/ppio/channels
// Returns channels whose base_url contains "ppio" (any channel type).
func ListChannelsHandler(c *gin.Context) {
	var channels []model.Channel

	err := model.DB.Where(ppioChannelWhere(), ppioChannelArgs()...).
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
			baseURL = DefaultPPIOAPIBase
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

// GetConfigHandler handles GET /api/enterprise/ppio/config
// Returns the currently selected channel and config status.
func GetConfigHandler(c *gin.Context) {
	cfg := GetPPIOConfig()

	maskedKey := ""
	configured := cfg.APIKey != ""

	if configured {
		maskedKey = maskAPIKey(cfg.APIKey)
	}

	successResponse(c, gin.H{
		"channel_id":              cfg.ChannelID,
		"api_key":                 maskedKey,
		"api_base":                cfg.APIBase,
		"configured":              configured,
		"mgmt_token_configured":   cfg.MgmtToken != "",
		"auto_sync_enabled":       cfg.AutoSyncEnabled,
		"auto_sync_force_disabled": env.Bool("DISABLE_PPIO_AUTO_SYNC", false),
	})
}

// UpdateMgmtTokenHandler handles PUT /api/enterprise/ppio/mgmt-token
// Accepts a management console token and persists it.
func UpdateMgmtTokenHandler(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := SetPPIOMgmtToken(req.Token); err != nil {
		errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("failed to save mgmt token: %v", err))
		return
	}

	successResponse(c, gin.H{"message": "mgmt token saved"})
}

// UpdateAPIKeyHandler handles PUT /api/enterprise/ppio/api-key
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

	if err := SetPPIOAPIKeyDirect(req.APIKey, req.APIBase); err != nil {
		errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("failed to save API key: %v", err))
		return
	}

	successResponse(c, gin.H{"message": "API key saved"})
}

// UpdateConfigHandler handles PUT /api/enterprise/ppio/config
// Accepts a channel_id and reads key/base_url from that channel.
func UpdateConfigHandler(c *gin.Context) {
	var req struct {
		ChannelID int `json:"channel_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := SetPPIOConfigFromChannel(req.ChannelID); err != nil {
		errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("failed to save config: %v", err))
		return
	}

	successResponse(c, gin.H{"message": "config saved"})
}

// DiagnosticHandler handles GET /api/enterprise/ppio/sync/diagnostic
func DiagnosticHandler(c *gin.Context) {
	result, err := Diagnostic(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, result)
}

// PreviewHandler handles POST /api/enterprise/ppio/sync/preview
func PreviewHandler(c *gin.Context) {
	var opts SyncOptions
	if err := c.ShouldBindJSON(&opts); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	client, err := NewPPIOClient()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	cfg := GetPPIOConfig()

	allModels, err := client.FetchAllModelsMerged(c.Request.Context(), cfg.MgmtToken)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	diff, err := ComparePPIOModelsV2(allModels, opts)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, diff)
}

// ExecuteHandler handles POST /api/enterprise/ppio/sync/execute (SSE streaming)
func ExecuteHandler(c *gin.Context) {
	var opts SyncOptions
	if err := c.ShouldBindJSON(&opts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": err.Error(),
		})

		return
	}

	// Validate that changes are confirmed
	if !opts.ChangesConfirmed && !opts.DryRun {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "changes_not_confirmed",
			"message": "Please confirm the changes before executing sync",
		})

		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Create a channel for progress events
	eventChan := make(chan SyncProgressEvent, 10)
	done := make(chan struct{})

	// Start sync in goroutine. Uses context.Background() so the sync completes
	// even if the client disconnects; progress events are dropped (not blocked)
	// if the channel buffer is full or nobody is reading.
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

	// Stream events to client
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

			fmt.Fprintf(w, "data: %s\n\n", string(eventJSON))
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

// ModelCoverageHandler handles GET /api/enterprise/ppio/model-coverage.
// Returns the list of PPIO models that have a ModelConfig entry but are NOT
// assigned to any enabled channel.
func ModelCoverageHandler(c *gin.Context) {
	var localModels []model.ModelConfig

	if err := model.DB.Select("model", "config").
		Where("owner = ?", string(model.ModelOwnerPPIO)).Find(&localModels).Error; err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())

		return
	}

	var channels []model.Channel

	if err := model.DB.Select("models").
		Where(ppioChannelWhere()+" AND status = ?", append(ppioChannelArgs(), model.ChannelStatusEnabled)...).
		Find(&channels).Error; err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())

		return
	}

	// Build a set of all model names that appear in at least one enabled PPIO channel.
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

// HistoryHandler handles GET /api/enterprise/ppio/sync/history
func HistoryHandler(c *gin.Context) {
	histories := make([]SyncHistory, 0)

	err := model.DB.Order("synced_at DESC").Limit(10).Find(&histories).Error
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Parse result JSON for each history
	for i := range histories {
		var result SyncResult
		if err := sonic.Unmarshal([]byte(histories[i].Result), &result); err == nil {
			histories[i].ResultParsed = &result
		}
	}

	successResponse(c, histories)
}

// UpdateAutoSyncHandler handles PUT /api/enterprise/ppio/auto-sync.
func UpdateAutoSyncHandler(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := SetAutoSyncEnabled(req.Enabled); err != nil {
		errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("failed to save auto-sync setting: %v", err))
		return
	}

	successResponse(c, gin.H{"enabled": req.Enabled})
}
