//go:build enterprise

package enterprise

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/labring/aiproxy/core/enterprise/analytics"
	"github.com/labring/aiproxy/core/enterprise/ppio"
	"github.com/labring/aiproxy/core/enterprise/quota"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

type MyTokenInfo struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Key          string    `json:"key"`
	Status       int       `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UsedAmount   float64   `json:"used_amount"`
	RequestCount int       `json:"request_count"`
}

type ModelAccessInfo struct {
	Model              string   `json:"model"`
	Type               int      `json:"type"`
	TypeName           string   `json:"type_name"`
	RPM                int64    `json:"rpm"`
	TPM                int64    `json:"tpm"`
	InputPrice         float64  `json:"input_price"`
	OutputPrice        float64  `json:"output_price"`
	PriceUnit          int64    `json:"price_unit"`
	SupportedEndpoints []string `json:"supported_endpoints"`
	MaxContext          int64    `json:"max_context,omitempty"`
	MaxOutput          int64    `json:"max_output,omitempty"`
}

var (
	// endpointsChatFamily: protocol-conversion endpoints always available for chat-type
	// models (OpenAI Chat, OpenAI Legacy, Anthropic Messages). These work via AI Proxy's
	// built-in protocol conversion regardless of what the upstream provider supports.
	// POST /v1/responses is NOT included — it requires explicit upstream support.
	endpointsChatFamily = []string{
		"POST /v1/chat/completions",
		"POST /v1/completions",
		"POST /v1/messages",
	}

	// endpointsResponsesOnly: for models that only support the Responses API upstream.
	endpointsResponsesOnly = []string{"POST /v1/responses"}

	endpointsEmbeddings  = []string{"POST /v1/embeddings"}
	endpointsModerations = []string{"POST /v1/moderations"}
	endpointsImages      = []string{"POST /v1/images/generations", "POST /v1/images/edits"}
	endpointsAudioSpeech = []string{"POST /v1/audio/speech"}
	endpointsAudioTransc = []string{"POST /v1/audio/transcriptions"}
	endpointsAudioTransl = []string{"POST /v1/audio/translations"}
	endpointsRerank      = []string{"POST /v1/rerank"}
	endpointsParsePdf    = []string{"POST /v1/parse/pdf"}
	endpointsVideo       = []string{"POST /v1/video/generations/jobs", "GET /v1/video/generations/jobs/{id}"}
	endpointsPPIONative  = []string{"POST /v3/{model}", "POST /v3/async/{model}", "GET /v3/async/task-result"}
	endpointsWebSearch   = []string{"POST /v1/web-search"}
)

// endpointSlugToPath maps provider endpoint slugs (from ModelConfig.Config["endpoints"])
// to the corresponding AI Proxy API paths. Used by both PPIO and Novita sync modules.
var endpointSlugToPath = map[string]string{
	"chat/completions":       "POST /v1/chat/completions",
	"completions":            "POST /v1/completions",
	"anthropic":              "POST /v1/messages",
	"responses":              "POST /v1/responses",
	"embeddings":             "POST /v1/embeddings",
	"moderations":            "POST /v1/moderations",
	"rerank":                 "POST /v1/rerank",
	"audio/speech":           "POST /v1/audio/speech",
	"audio/transcriptions":   "POST /v1/audio/transcriptions",
	"images/generations":     "POST /v1/images/generations",
	"video/generations/jobs": "POST /v1/video/generations/jobs",
}

// getModelSupportedEndpoints returns the supported endpoint paths for a model.
//
// Endpoint slugs from Config["endpoints"] are classified into three categories:
//   - Chat-base slugs (chat/completions, completions, anthropic) trigger protocol-
//     conversion expansion: all three chat-family paths are returned because AI Proxy
//     converts between them transparently.
//   - The "responses" slug adds POST /v1/responses independently — it requires
//     explicit upstream support and is NOT implied by chat-base slugs.
//   - Other slugs (embeddings, rerank, audio, etc.) map directly to their paths.
//
// Fallback chain when Config["endpoints"] is absent or all slugs are unknown:
//  1. Config["model_type"] string — accurate for synced models even if mc.Type is stale.
//  2. mc.Type — authoritative for non-synced models.
func getModelSupportedEndpoints(mc model.ModelConfig) []string {
	if slugs, ok := model.GetModelConfigStringSlice(mc.Config, "endpoints"); ok && len(slugs) > 0 {
		hasChatBase := false
		hasResponses := false
		var otherPaths []string

		for _, slug := range slugs {
			switch slug {
			case "chat/completions", "completions", "anthropic":
				hasChatBase = true
			case "responses":
				hasResponses = true
			default:
				if path, exists := endpointSlugToPath[slug]; exists {
					otherPaths = append(otherPaths, path)
				}
			}
		}

		var result []string
		if hasChatBase {
			result = append(result, endpointsChatFamily...)
		}
		if hasResponses {
			result = append(result, endpointsResponsesOnly...)
		}
		result = append(result, otherPaths...)

		if len(result) > 0 {
			return result
		}
	}
	// Fallback 1: use model_type string stored in Config — more reliable than
	// mc.Type for models that were synced before inferModeFromPPIO was added.
	if mt, _ := mc.Config[model.ModelConfigKey("model_type")].(string); mt != "" {
		if m, ok := ppio.ModelTypeToMode[mt]; ok {
			return getSupportedEndpoints(m)
		}
	}
	// Fallback 2: mc.Type (authoritative for non-synced models).
	return getSupportedEndpoints(mc.Type)
}

// modeToTypeName returns a human-friendly category name for a mode.
func modeToTypeName(m mode.Mode) string {
	switch m {
	case mode.ChatCompletions, mode.Completions, mode.Anthropic, mode.Gemini, mode.Responses:
		return "chat"
	case mode.Embeddings:
		return "embedding"
	case mode.ImagesGenerations, mode.ImagesEdits:
		return "image"
	case mode.AudioSpeech:
		return "tts"
	case mode.AudioTranscription:
		return "stt"
	case mode.AudioTranslation:
		return "audio_translation"
	case mode.VideoGenerationsJobs, mode.VideoGenerationsGetJobs, mode.VideoGenerationsContent:
		return "video"
	case mode.Rerank:
		return "rerank"
	case mode.Moderations:
		return "moderation"
	case mode.ParsePdf:
		return "parse_pdf"
	case mode.PPIONative:
		return "multimodal"
	case mode.WebSearch:
		return "web_search"
	default:
		return "other"
	}
}

func getSupportedEndpoints(modelType mode.Mode) []string {
	switch modelType {
	case mode.ChatCompletions, mode.Completions, mode.Anthropic, mode.Gemini:
		return endpointsChatFamily
	case mode.Responses:
		return endpointsResponsesOnly
	case mode.Embeddings:
		return endpointsEmbeddings
	case mode.Moderations:
		return endpointsModerations
	case mode.ImagesGenerations, mode.ImagesEdits:
		return endpointsImages
	case mode.AudioSpeech:
		return endpointsAudioSpeech
	case mode.AudioTranscription:
		return endpointsAudioTransc
	case mode.AudioTranslation:
		return endpointsAudioTransl
	case mode.Rerank:
		return endpointsRerank
	case mode.ParsePdf:
		return endpointsParsePdf
	case mode.VideoGenerationsJobs, mode.VideoGenerationsGetJobs, mode.VideoGenerationsContent:
		return endpointsVideo
	case mode.PPIONative:
		return endpointsPPIONative
	case mode.WebSearch:
		return endpointsWebSearch
	default:
		return nil
	}
}

type ModelGroupInfo struct {
	Owner  string            `json:"owner"`
	Models []ModelAccessInfo `json:"models"`
}

type MyAccessResponse struct {
	BaseURL       string            `json:"base_url"`
	OwnerBaseURLs map[string]string `json:"owner_base_urls,omitempty"`
	LocalOwner    string            `json:"local_owner,omitempty"`
	GroupID       string            `json:"group_id"`
	Tokens        []MyTokenInfo     `json:"tokens"`
	ModelGroups   []ModelGroupInfo  `json:"model_groups"`
}

var (
	ownerBaseURLsOnce sync.Once
	ownerBaseURLsMap  map[string]string
)

func loadOwnerBaseURLs() map[string]string {
	ownerBaseURLsOnce.Do(func() {
		raw := os.Getenv("ENTERPRISE_BASE_URLS")
		if raw == "" {
			return
		}
		var m map[string]string
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			return
		}
		ownerBaseURLsMap = m
	})
	return ownerBaseURLsMap
}

// GetMyAccess returns the user's access info including tokens and available models.
func GetMyAccess(c *gin.Context) {
	feishuUser := GetEnterpriseUser(c)
	if feishuUser == nil {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: not an enterprise user")
		return
	}

	groupID := feishuUser.GroupID

	// 1. Get all tokens for this group
	var tokens []model.Token
	if err := model.DB.Where("group_id = ?", groupID).Find(&tokens).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, "failed to load tokens: "+err.Error())
		return
	}

	tokenInfos := make([]MyTokenInfo, 0, len(tokens))
	for _, t := range tokens {
		tokenInfos = append(tokenInfos, MyTokenInfo{
			ID:           t.ID,
			Name:         string(t.Name),
			Key:          t.Key,
			Status:       t.Status,
			CreatedAt:    t.CreatedAt,
			UsedAmount:   t.UsedAmount,
			RequestCount: t.RequestCount,
		})
	}

	// 2. Determine base URL
	baseURL := os.Getenv("ENTERPRISE_BASE_URL")
	if baseURL == "" {
		scheme := "https"
		if c.Request.TLS == nil {
			if fwd := c.Request.Header.Get("X-Forwarded-Proto"); fwd != "" {
				scheme = fwd
			} else {
				scheme = "http"
			}
		}

		baseURL = fmt.Sprintf("%s://%s/v1", scheme, c.Request.Host)
	}

	// 3. Get available models via TokenCache.Range()
	modelCaches := model.LoadModelCaches()
	enabledConfigs := modelCaches.EnabledModelConfigsMap

	var availableModels []string

	// Find an enabled token to derive model list
	var enabledToken *model.Token
	for i := range tokens {
		if tokens[i].Status == model.TokenStatusEnabled {
			enabledToken = &tokens[i]
			break
		}
	}

	var groupSets []string
	if enabledToken != nil {
		// Build a TokenCache with group context so Range() can resolve available models
		tokenCache := enabledToken.ToTokenCache()

		group, err := model.CacheGetGroup(groupID)
		if err == nil {
			groupSets = group.GetAvailableSets()
			tokenCache.SetAvailableSets(groupSets)
			tokenCache.SetModelsBySet(modelCaches.EnabledModelsBySet)

			tokenCache.Range(func(modelName string) bool {
				availableModels = append(availableModels, modelName)
				return true
			})
		}
	}

	if len(groupSets) == 0 {
		groupSets = []string{model.ChannelDefaultSet}
	}

	// Load group-level overrides
	groupModelConfigs, _ := model.GetGroupModelConfigs(groupID)
	gmcMap := make(map[string]model.GroupModelConfig, len(groupModelConfigs))
	for _, gmc := range groupModelConfigs {
		gmcMap[gmc.Model] = gmc
	}

	modelProvider := make(map[string]string, len(availableModels))
	for _, set := range groupSets {
		chMap := modelCaches.EnabledModel2ChannelsBySet[set]
		for _, modelName := range availableModels {
			if _, exists := modelProvider[modelName]; exists {
				continue
			}

			if chs, ok := chMap[modelName]; ok && len(chs) > 0 {
				// Channels are already sorted by priority (highest first)
				modelProvider[modelName] = chs[0].Type.String()
			}
		}
	}

	ownerModels := make(map[string][]ModelAccessInfo)

	for _, modelName := range availableModels {
		mc, ok := enabledConfigs[modelName]
		if !ok {
			continue
		}

		rpm := mc.RPM
		tpm := mc.TPM
		inputPrice := float64(mc.Price.InputPrice)
		outputPrice := float64(mc.Price.OutputPrice)
		priceUnit := int64(mc.Price.InputPriceUnit)

		if priceUnit == 0 {
			priceUnit = model.PriceUnit
		}

		// Apply group-level overrides
		if gmc, exists := gmcMap[modelName]; exists {
			if gmc.OverrideLimit {
				rpm = gmc.RPM
				tpm = gmc.TPM
			}

			if gmc.OverridePrice {
				inputPrice = float64(gmc.Price.InputPrice)
				outputPrice = float64(gmc.Price.OutputPrice)

				if int64(gmc.Price.InputPriceUnit) != 0 {
					priceUnit = int64(gmc.Price.InputPriceUnit)
				}
			}
		}

		// Group by the enabled channel's provider, not the static ModelConfig.Owner.
		owner := modelProvider[modelName]
		if owner == "" {
			owner = string(mc.Owner)
		}

		if owner == "" {
			owner = "other"
		}

		maxCtx, _ := model.GetModelConfigInt(mc.Config, model.ModelConfigMaxContextTokensKey)
		maxOut, _ := model.GetModelConfigInt(mc.Config, model.ModelConfigMaxOutputTokensKey)

		ownerModels[owner] = append(ownerModels[owner], ModelAccessInfo{
			Model:              modelName,
			Type:               int(mc.Type),
			TypeName:           modeToTypeName(mc.Type),
			RPM:                rpm,
			TPM:                tpm,
			InputPrice:         inputPrice,
			OutputPrice:        outputPrice,
			PriceUnit:          priceUnit,
			SupportedEndpoints: getModelSupportedEndpoints(mc),
			MaxContext:          int64(maxCtx),
			MaxOutput:          int64(maxOut),
		})
	}

	// Sort owners and models
	owners := make([]string, 0, len(ownerModels))
	for owner := range ownerModels {
		owners = append(owners, owner)
	}

	sort.Strings(owners)

	modelGroups := make([]ModelGroupInfo, 0, len(owners))
	for _, owner := range owners {
		models := ownerModels[owner]
		sort.Slice(models, func(i, j int) bool {
			return models[i].Model < models[j].Model
		})

		modelGroups = append(modelGroups, ModelGroupInfo{
			Owner:  owner,
			Models: models,
		})
	}

	ownerURLs := loadOwnerBaseURLs()
	var localOwner string
	for owner, url := range ownerURLs {
		if url == baseURL {
			localOwner = owner
			break
		}
	}

	middleware.SuccessResponse(c, MyAccessResponse{
		BaseURL:       baseURL,
		OwnerBaseURLs: ownerURLs,
		LocalOwner:    localOwner,
		GroupID:       groupID,
		Tokens:        tokenInfos,
		ModelGroups:   modelGroups,
	})
}

type createTokenRequest struct {
	Name string `json:"name" binding:"required,max=32"`
}

// CreateMyToken creates a new API token for the current user's group.
func CreateMyToken(c *gin.Context) {
	feishuUser := GetEnterpriseUser(c)
	if feishuUser == nil {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: not an enterprise user")
		return
	}

	var req createTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	token := &model.Token{
		GroupID: feishuUser.GroupID,
		Name:    model.EmptyNullString(req.Name),
		Status:  model.TokenStatusEnabled,
	}

	if err := model.InsertToken(token, false, false); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, "failed to create token: "+err.Error())
		return
	}

	middleware.SuccessResponse(c, MyTokenInfo{
		ID:        token.ID,
		Name:      req.Name,
		Key:       token.Key,
		Status:    token.Status,
		CreatedAt: token.CreatedAt,
	})
}

// ModelUsage holds aggregated usage for a single model.
type ModelUsage struct {
	Model         string  `json:"model"`
	UsedAmount    float64 `json:"used_amount"`
	RequestCount  int64   `json:"request_count"`
	TotalTokens   int64   `json:"total_tokens"`
	SuccessRate   float64 `json:"success_rate"`
	AvgResponseMs float64 `json:"avg_response_ms"`
	AvgTtfbMs     float64 `json:"avg_ttfb_ms"`
	AvgCostPerReq float64 `json:"avg_cost_per_req"`
}

// MyUsageStats holds the aggregated usage stats for the current user.
type MyUsageStats struct {
	TotalAmount   float64          `json:"total_amount"`
	TotalTokens   int64            `json:"total_tokens"`
	TotalRequests int64            `json:"total_requests"`
	UniqueModels  int              `json:"unique_models"`
	AvgCostPerReq float64          `json:"avg_cost_per_req"`
	SuccessRate   float64          `json:"success_rate"`
	AvgResponseMs float64          `json:"avg_response_ms"`
	AvgTtfbMs     float64          `json:"avg_ttfb_ms"`
	TopModels     []ModelUsage     `json:"top_models"`
	Comparisons   *UsageComparisons `json:"comparisons,omitempty"`
}

// MetricComparison holds department and enterprise active-user averages for one metric.
type MetricComparison struct {
	DeptAvg       float64 `json:"dept_avg"`
	EnterpriseAvg float64 `json:"enterprise_avg"`
}

// UsageComparisons holds department and enterprise comparison data for all metrics.
type UsageComparisons struct {
	TotalAmount   MetricComparison `json:"total_amount"`
	TotalTokens   MetricComparison `json:"total_tokens"`
	TotalRequests MetricComparison `json:"total_requests"`
	UniqueModels  MetricComparison `json:"unique_models"`
	AvgCostPerReq MetricComparison `json:"avg_cost_per_req"`
	SuccessRate   MetricComparison `json:"success_rate"`
	AvgResponseMs MetricComparison `json:"avg_response_ms"`
	AvgTtfbMs     MetricComparison `json:"avg_ttfb_ms"`
}

// MyQuotaStatus holds the current period quota status for the user.
type MyQuotaStatus struct {
	PeriodQuota  float64 `json:"period_quota"`
	PeriodUsed   float64 `json:"period_used"`
	PeriodType   string  `json:"period_type"`  // "daily"/"weekly"/"monthly"
	PeriodStart  int64   `json:"period_start"` // unix seconds
	PolicyName   string  `json:"policy_name"`
	PolicyID     int     `json:"policy_id"`
	CurrentTier  int     `json:"current_tier"` // 1, 2, 3
	Tier1Ratio   float64 `json:"tier1_ratio"`
	Tier2Ratio   float64 `json:"tier2_ratio"`
	BlockAtTier3 bool    `json:"block_at_tier3"`
}

// MyStatsResponse is the response type for the /my-access/stats endpoint.
type MyStatsResponse struct {
	Usage *MyUsageStats  `json:"usage"`
	Quota *MyQuotaStatus `json:"quota"` // nil if no policy bound
}

// parseTimestampRange extracts start_timestamp/end_timestamp query params (unix seconds),
// defaulting to the last 7 days if not provided or invalid.
func parseTimestampRange(c *gin.Context) (startTs, endTs int64) {
	now := time.Now()
	startTs = now.Add(-7 * 24 * time.Hour).Unix()
	endTs = now.Unix()

	if v := c.Query("start_timestamp"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			startTs = parsed
		}
	}

	if v := c.Query("end_timestamp"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			endTs = parsed
		}
	}

	return startTs, endTs
}

// GetMyStats returns the current user's usage stats and quota status.
func GetMyStats(c *gin.Context) {
	feishuUser := GetEnterpriseUser(c)
	if feishuUser == nil {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: not an enterprise user")
		return
	}

	startTs, endTs := parseTimestampRange(c)

	// Query aggregated usage per model from group_summaries
	type modelAgg struct {
		Model        string  `gorm:"column:model"`
		UsedAmount   float64 `gorm:"column:used_amount"`
		RequestCount int64   `gorm:"column:request_count"`
		TotalTokens  int64   `gorm:"column:total_tokens"`
		SuccessCount int64   `gorm:"column:success_count"`
		TotalTimeMs  int64   `gorm:"column:total_time_ms"`
		TotalTtfbMs  int64   `gorm:"column:total_ttfb_ms"`
	}

	var aggResults []modelAgg

	if err := model.LogDB.
		Model(&model.GroupSummary{}).
		Select(
			"model",
			"SUM(used_amount) as used_amount",
			"SUM(request_count) as request_count",
			"SUM(total_tokens) as total_tokens",
			"SUM(status2xx_count) as success_count",
			"SUM(total_time_milliseconds) as total_time_ms",
			"SUM(total_ttfb_milliseconds) as total_ttfb_ms",
		).
		Where("group_id = ?", feishuUser.GroupID).
		Where("hour_timestamp >= ? AND hour_timestamp <= ?", startTs, endTs).
		Group("model").
		Find(&aggResults).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, "failed to query usage: "+err.Error())
		return
	}

	// Build usage stats
	usageStats := &MyUsageStats{
		TopModels: make([]ModelUsage, 0, len(aggResults)),
	}

	var totalSuccessCount, totalTimeMs, totalTtfbMs int64

	for _, r := range aggResults {
		usageStats.TotalAmount += r.UsedAmount
		usageStats.TotalTokens += r.TotalTokens
		usageStats.TotalRequests += r.RequestCount
		totalSuccessCount += r.SuccessCount
		totalTimeMs += r.TotalTimeMs
		totalTtfbMs += r.TotalTtfbMs
		mu := ModelUsage{
			Model:        r.Model,
			UsedAmount:   r.UsedAmount,
			RequestCount: r.RequestCount,
			TotalTokens:  r.TotalTokens,
		}
		if r.RequestCount > 0 {
			mu.SuccessRate = float64(r.SuccessCount) / float64(r.RequestCount) * 100
			mu.AvgResponseMs = float64(r.TotalTimeMs) / float64(r.RequestCount)
			mu.AvgCostPerReq = r.UsedAmount / float64(r.RequestCount)
		}
		if r.SuccessCount > 0 {
			mu.AvgTtfbMs = float64(r.TotalTtfbMs) / float64(r.SuccessCount)
		}
		usageStats.TopModels = append(usageStats.TopModels, mu)
	}

	usageStats.UniqueModels = len(aggResults)

	// Compute derived metrics (with zero-division protection)
	if usageStats.TotalRequests > 0 {
		usageStats.AvgCostPerReq = usageStats.TotalAmount / float64(usageStats.TotalRequests)
		usageStats.SuccessRate = float64(totalSuccessCount) / float64(usageStats.TotalRequests) * 100
		usageStats.AvgResponseMs = float64(totalTimeMs) / float64(usageStats.TotalRequests)
	}

	if totalSuccessCount > 0 {
		usageStats.AvgTtfbMs = float64(totalTtfbMs) / float64(totalSuccessCount)
	}

	// Sort by used amount desc, keep top 10
	sort.Slice(usageStats.TopModels, func(i, j int) bool {
		return usageStats.TopModels[i].UsedAmount > usageStats.TopModels[j].UsedAmount
	})

	if len(usageStats.TopModels) > 10 {
		usageStats.TopModels = usageStats.TopModels[:10]
	}

	usageStats.Comparisons = computeComparisons(feishuUser.DepartmentID, startTs, endTs)

	// Query quota status — use policy as the authoritative source for PeriodQuota/PeriodType
	// instead of the token's denormalized copy, which may lag behind due to async sync.
	// Period usage is computed from group_summaries for accuracy (avoids lazy period reset drift).
	var quotaStatus *MyQuotaStatus

	policy, _ := quota.GetPolicyForUser(c.Request.Context(), feishuUser.OpenID)
	if policy != nil && policy.PeriodQuota > 0 {
		periodType := quota.PolicyPeriodTypeToTokenPeriodType(policy.PeriodType)
		periodStart := currentPeriodStart(model.EmptyNullString(periodType))

		var periodUsed float64
		if err := model.LogDB.
			Model(&model.GroupSummary{}).
			Select("COALESCE(SUM(used_amount), 0)").
			Where("group_id = ?", feishuUser.GroupID).
			Where("hour_timestamp >= ?", periodStart.Unix()).
			Scan(&periodUsed).Error; err != nil {
			periodUsed = 0
		}

		usageRatio := periodUsed / policy.PeriodQuota

		blocked := policy.BlockAtTier3 && usageRatio >= 1.0
		currentTier := quota.ComputeTier(policy, usageRatio, blocked)

		quotaStatus = &MyQuotaStatus{
			PeriodQuota:  policy.PeriodQuota,
			PeriodUsed:   periodUsed,
			PeriodType:   periodType,
			PeriodStart:  periodStart.Unix(),
			PolicyName:   policy.Name,
			PolicyID:     policy.ID,
			CurrentTier:  currentTier,
			Tier1Ratio:   policy.Tier1Ratio,
			Tier2Ratio:   policy.Tier2Ratio,
			BlockAtTier3: policy.BlockAtTier3,
		}
	}

	middleware.SuccessResponse(c, MyStatsResponse{
		Usage: usageStats,
		Quota: quotaStatus,
	})
}

// groupAvgAgg holds the aggregated data for computing per-active-user averages.
type groupAvgAgg struct {
	UsedAmount   float64 `gorm:"column:used_amount"`
	TotalTokens  int64   `gorm:"column:total_tokens"`
	RequestCount int64   `gorm:"column:request_count"`
	SuccessCount int64   `gorm:"column:success_count"`
	TotalTimeMs  int64   `gorm:"column:total_time_ms"`
	TotalTtfbMs  int64   `gorm:"column:total_ttfb_ms"`
	ActiveUsers  int64   `gorm:"column:active_users"`
	UniqueModels int64   `gorm:"column:unique_models"`
}

// queryGroupAvg queries aggregated usage and active user count for a set of group IDs.
func queryGroupAvg(groupIDs []string, startTs, endTs int64) *groupAvgAgg {
	if len(groupIDs) == 0 {
		return nil
	}

	var result groupAvgAgg
	if err := model.LogDB.
		Model(&model.GroupSummary{}).
		Select(
			"SUM(used_amount) as used_amount",
			"SUM(total_tokens) as total_tokens",
			"SUM(request_count) as request_count",
			"SUM(status2xx_count) as success_count",
			"SUM(total_time_milliseconds) as total_time_ms",
			"SUM(total_ttfb_milliseconds) as total_ttfb_ms",
			"COUNT(DISTINCT group_id) as active_users",
			"COUNT(DISTINCT model) as unique_models",
		).
		Where("group_id IN ?", groupIDs).
		Where("hour_timestamp >= ? AND hour_timestamp <= ?", startTs, endTs).
		Find(&result).Error; err != nil {
		return nil
	}

	if result.ActiveUsers == 0 {
		return nil
	}

	return &result
}

// metricAvgs holds per-active-user averages for all 8 metrics.
type metricAvgs struct {
	TotalAmount   float64
	TotalTokens   float64
	TotalRequests float64
	UniqueModels  float64
	AvgCostPerReq float64
	SuccessRate   float64
	AvgResponseMs float64
	AvgTtfbMs     float64
}

// aggToMetrics computes per-active-user averages from aggregated data.
func aggToMetrics(agg *groupAvgAgg) metricAvgs {
	if agg == nil || agg.ActiveUsers == 0 {
		return metricAvgs{}
	}

	n := float64(agg.ActiveUsers)
	m := metricAvgs{
		TotalAmount:   agg.UsedAmount / n,
		TotalTokens:   float64(agg.TotalTokens) / n,
		TotalRequests: float64(agg.RequestCount) / n,
		UniqueModels:  float64(agg.UniqueModels) / n,
	}

	if agg.RequestCount > 0 {
		m.AvgCostPerReq = agg.UsedAmount / float64(agg.RequestCount)
		m.SuccessRate = float64(agg.SuccessCount) / float64(agg.RequestCount) * 100
		m.AvgResponseMs = float64(agg.TotalTimeMs) / float64(agg.RequestCount)
	}

	if agg.SuccessCount > 0 {
		m.AvgTtfbMs = float64(agg.TotalTtfbMs) / float64(agg.SuccessCount)
	}

	return m
}

// setDeptAvg writes metricAvgs into the DeptAvg field of each MetricComparison.
func setDeptAvg(comp *UsageComparisons, m metricAvgs) {
	comp.TotalAmount.DeptAvg = m.TotalAmount
	comp.TotalTokens.DeptAvg = m.TotalTokens
	comp.TotalRequests.DeptAvg = m.TotalRequests
	comp.UniqueModels.DeptAvg = m.UniqueModels
	comp.AvgCostPerReq.DeptAvg = m.AvgCostPerReq
	comp.SuccessRate.DeptAvg = m.SuccessRate
	comp.AvgResponseMs.DeptAvg = m.AvgResponseMs
	comp.AvgTtfbMs.DeptAvg = m.AvgTtfbMs
}

// setEnterpriseAvg writes metricAvgs into the EnterpriseAvg field of each MetricComparison.
func setEnterpriseAvg(comp *UsageComparisons, m metricAvgs) {
	comp.TotalAmount.EnterpriseAvg = m.TotalAmount
	comp.TotalTokens.EnterpriseAvg = m.TotalTokens
	comp.TotalRequests.EnterpriseAvg = m.TotalRequests
	comp.UniqueModels.EnterpriseAvg = m.UniqueModels
	comp.AvgCostPerReq.EnterpriseAvg = m.AvgCostPerReq
	comp.SuccessRate.EnterpriseAvg = m.SuccessRate
	comp.AvgResponseMs.EnterpriseAvg = m.AvgResponseMs
	comp.AvgTtfbMs.EnterpriseAvg = m.AvgTtfbMs
}

// computeComparisons calculates department and enterprise active-user averages.
func computeComparisons(departmentID string, startTs, endTs int64) *UsageComparisons {
	comp := &UsageComparisons{}

	// Department averages
	if departmentID != "" {
		deptGroupIDs, err := analytics.GetGroupIDsForDepartments([]string{departmentID})
		if err == nil && len(deptGroupIDs) > 0 {
			setDeptAvg(comp, aggToMetrics(queryGroupAvg(deptGroupIDs, startTs, endTs)))
		}
	}

	// Enterprise averages
	entGroupIDs, err := analytics.GetAllFeishuGroupIDs()
	if err == nil && len(entGroupIDs) > 0 {
		setEnterpriseAvg(comp, aggToMetrics(queryGroupAvg(entGroupIDs, startTs, endTs)))
	}

	return comp
}

// currentPeriodStart returns the start of the current quota period based on period type.
func currentPeriodStart(periodType model.EmptyNullString) time.Time {
	now := time.Now()

	switch periodType {
	case model.PeriodTypeDaily:
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case model.PeriodTypeWeekly:
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday = 7
		}
		monday := now.AddDate(0, 0, -(weekday - 1))
		return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, now.Location())
	default: // monthly or empty
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	}
}

// TokenPeriodStats holds per-token aggregated stats for a time period.
type TokenPeriodStats struct {
	TokenName    string  `json:"token_name"`
	UsedAmount   float64 `json:"used_amount"`
	RequestCount int64   `json:"request_count"`
	TotalTokens  int64   `json:"total_tokens"`
	SuccessRate  float64 `json:"success_rate"`
}

// GetMyTokenStats returns per-token usage stats for the current user's group within a time range.
func GetMyTokenStats(c *gin.Context) {
	feishuUser := GetEnterpriseUser(c)
	if feishuUser == nil {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: not an enterprise user")
		return
	}

	startTs, endTs := parseTimestampRange(c)

	type tokenAgg struct {
		TokenName    string  `gorm:"column:token_name"`
		UsedAmount   float64 `gorm:"column:used_amount"`
		RequestCount int64   `gorm:"column:request_count"`
		TotalTokens  int64   `gorm:"column:total_tokens"`
		SuccessCount int64   `gorm:"column:success_count"`
	}

	var results []tokenAgg

	if err := model.LogDB.
		Model(&model.GroupSummary{}).
		Select(
			"token_name",
			"SUM(used_amount) as used_amount",
			"SUM(request_count) as request_count",
			"SUM(total_tokens) as total_tokens",
			"SUM(status2xx_count) as success_count",
		).
		Where("group_id = ?", feishuUser.GroupID).
		Where("hour_timestamp >= ? AND hour_timestamp <= ?", startTs, endTs).
		Group("token_name").
		Find(&results).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, "failed to query token stats: "+err.Error())
		return
	}

	stats := make([]TokenPeriodStats, 0, len(results))
	for _, r := range results {
		var successRate float64
		if r.RequestCount > 0 {
			successRate = float64(r.SuccessCount) / float64(r.RequestCount) * 100
		}

		stats = append(stats, TokenPeriodStats{
			TokenName:    r.TokenName,
			UsedAmount:   r.UsedAmount,
			RequestCount: r.RequestCount,
			TotalTokens:  r.TotalTokens,
			SuccessRate:  successRate,
		})
	}

	middleware.SuccessResponse(c, stats)
}

// DisableMyToken disables (soft-deletes) a token belonging to the current user's group.
// Ownership is verified atomically in the UPDATE WHERE clause to prevent TOCTOU.
func DisableMyToken(c *gin.Context) {
	feishuUser := GetEnterpriseUser(c)
	if feishuUser == nil {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: not an enterprise user")
		return
	}

	idStr := c.Param("id")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid token id")
		return
	}

	if err := model.UpdateGroupTokenStatus(feishuUser.GroupID, id, model.TokenStatusDisabled); err != nil {
		middleware.ErrorResponse(c, http.StatusNotFound, "token not found or not owned by current user")
		return
	}

	middleware.SuccessResponse(c, gin.H{"message": "token disabled"})
}

// GetMyLogs returns paginated request logs for the current enterprise user's group.
func GetMyLogs(c *gin.Context) {
	feishuUser := GetEnterpriseUser(c)
	if feishuUser == nil {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: not an enterprise user")
		return
	}

	startTime, endTime := utils.ParseTimeRange(c, utils.NoSpanLimit)
	afterID, _ := strconv.Atoi(c.Query("after_id"))
	limit, _ := strconv.Atoi(c.Query("limit"))

	result, err := model.GetGroupUserLogs(
		feishuUser.GroupID,
		startTime,
		endTime,
		c.Query("model_name"),
		c.Query("request_id"),
		model.CodeType(c.Query("code_type")),
		afterID,
		limit,
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("failed to query logs: %v", err))
		return
	}

	middleware.SuccessResponse(c, result)
}

// GetMyLogDetail returns the request/response body for a specific log entry.
func GetMyLogDetail(c *gin.Context) {
	feishuUser := GetEnterpriseUser(c)
	if feishuUser == nil {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: not an enterprise user")
		return
	}

	logID, err := strconv.Atoi(c.Param("log_id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid log_id")
		return
	}

	detail, err := model.GetGroupLogDetail(logID, feishuUser.GroupID)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusNotFound, "log not found")
		return
	}

	middleware.SuccessResponse(c, detail)
}
