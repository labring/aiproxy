//go:build enterprise

package enterprise

import (
	"fmt"
	"net/http"
	"os"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
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
	// endpointsChatFamilyBase: protocol-conversion endpoints always available for chat-type
	// models (OpenAI Chat, OpenAI Legacy, Anthropic Messages). These work via AI Proxy's
	// built-in protocol conversion regardless of what the upstream provider supports.
	endpointsChatFamilyBase = []string{
		"POST /v1/chat/completions",
		"POST /v1/completions",
		"POST /v1/messages",
	}

	// endpointsChatFamily: full chat family including the Responses API. Only applicable
	// when the upstream provider explicitly supports POST /v1/responses.
	endpointsChatFamily = []string{
		"POST /v1/chat/completions",
		"POST /v1/completions",
		"POST /v1/messages",
		"POST /v1/responses",
	}
	endpointsEmbeddings    = []string{"POST /v1/embeddings"}
	endpointsModerations   = []string{"POST /v1/moderations"}
	endpointsImages        = []string{"POST /v1/images/generations", "POST /v1/images/edits"}
	endpointsAudioSpeech   = []string{"POST /v1/audio/speech"}
	endpointsAudioTransc   = []string{"POST /v1/audio/transcriptions"}
	endpointsAudioTransl   = []string{"POST /v1/audio/translations"}
	endpointsRerank        = []string{"POST /v1/rerank"}
	endpointsParsePdf      = []string{"POST /v1/parse/pdf"}
	endpointsVideo         = []string{"POST /v1/video/generations/jobs", "GET /v1/video/generations/jobs/{id}"}
)

// ppioSlugToPath maps PPIO endpoint slugs (from ModelConfig.Config["endpoints"])
// to the corresponding AI Proxy API paths.
var ppioSlugToPath = map[string]string{
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
// For models with PPIO endpoint data in Config["endpoints"], it derives the list
// from that per-model data. If every resolved path belongs to the chat-family,
// the chat-family list is returned so that users see all equivalent endpoints
// (AI Proxy's protocol conversion makes chat/completions, completions, and messages
// mutually reachable). POST /v1/responses is only included when the model explicitly
// declares the "responses" slug, because that endpoint requires upstream support.
//
// Fallback chain when Config["endpoints"] is absent or all slugs are unknown:
//  1. Config["model_type"] string — accurate for PPIO models even if mc.Type is stale
//     (mc.Type was hardcoded to 1 for all PPIO models before inferModeFromPPIO).
//  2. mc.Type — authoritative for non-PPIO models.
func getModelSupportedEndpoints(mc model.ModelConfig) []string {
	if slugs, ok := model.GetModelConfigStringSlice(mc.Config, "endpoints"); ok && len(slugs) > 0 {
		paths := make([]string, 0, len(slugs))
		allChat := true
		hasResponses := false
		for _, slug := range slugs {
			path, exists := ppioSlugToPath[slug]
			if !exists {
				continue
			}
			paths = append(paths, path)
			if slug == "responses" {
				hasResponses = true
			}
			if allChat && !slices.Contains(endpointsChatFamily, path) {
				allChat = false
			}
		}
		if len(paths) > 0 {
			if allChat {
				// Protocol conversion covers chat/completions↔completions↔messages.
				// Only add /v1/responses when the upstream explicitly supports it.
				if hasResponses {
					return endpointsChatFamily
				}
				return endpointsChatFamilyBase
			}
			return paths
		}
	}
	// Fallback 1: use PPIO model_type string stored in Config — more reliable than
	// mc.Type for PPIO models that were synced before inferModeFromPPIO was added.
	if mt, _ := mc.Config[model.ModelConfigKey("model_type")].(string); mt != "" {
		if m, ok := ppio.ModelTypeToMode[mt]; ok {
			return getSupportedEndpoints(m)
		}
	}
	// Fallback 2: mc.Type (authoritative for non-PPIO models).
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
	default:
		return "other"
	}
}

func getSupportedEndpoints(modelType mode.Mode) []string {
	switch modelType {
	case mode.ChatCompletions, mode.Completions, mode.Anthropic, mode.Gemini, mode.Responses:
		return endpointsChatFamily
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
	default:
		return nil
	}
}

type ModelGroupInfo struct {
	Owner  string            `json:"owner"`
	Models []ModelAccessInfo `json:"models"`
}

type MyAccessResponse struct {
	BaseURL     string           `json:"base_url"`
	GroupID     string           `json:"group_id"`
	Tokens      []MyTokenInfo    `json:"tokens"`
	ModelGroups []ModelGroupInfo `json:"model_groups"`
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

	if enabledToken != nil {
		// Build a TokenCache with group context so Range() can resolve available models
		tokenCache := enabledToken.ToTokenCache()

		group, err := model.CacheGetGroup(groupID)
		if err == nil {
			tokenCache.SetAvailableSets(group.GetAvailableSets())
			tokenCache.SetModelsBySet(modelCaches.EnabledModelsBySet)

			tokenCache.Range(func(modelName string) bool {
				availableModels = append(availableModels, modelName)
				return true
			})
		}
	}

	// Load group-level overrides
	groupModelConfigs, _ := model.GetGroupModelConfigs(groupID)
	gmcMap := make(map[string]model.GroupModelConfig, len(groupModelConfigs))
	for _, gmc := range groupModelConfigs {
		gmcMap[gmc.Model] = gmc
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

		owner := string(mc.Owner)
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

	middleware.SuccessResponse(c, MyAccessResponse{
		BaseURL:     baseURL,
		GroupID:     groupID,
		Tokens:      tokenInfos,
		ModelGroups: modelGroups,
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
	Model        string  `json:"model"`
	UsedAmount   float64 `json:"used_amount"`
	RequestCount int64   `json:"request_count"`
	TotalTokens  int64   `json:"total_tokens"`
}

// MyUsageStats holds the aggregated usage stats for the current user.
type MyUsageStats struct {
	TotalAmount   float64      `json:"total_amount"`
	TotalTokens   int64        `json:"total_tokens"`
	TotalRequests int64        `json:"total_requests"`
	UniqueModels  int          `json:"unique_models"`
	TopModels     []ModelUsage `json:"top_models"`
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

// GetMyStats returns the current user's usage stats and quota status.
func GetMyStats(c *gin.Context) {
	feishuUser := GetEnterpriseUser(c)
	if feishuUser == nil {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: not an enterprise user")
		return
	}

	// Parse time range from query params (unix seconds)
	now := time.Now()
	startTs := now.Add(-7 * 24 * time.Hour).Unix()
	endTs := now.Unix()

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

	// Query aggregated usage per model from group_summaries
	type modelAgg struct {
		Model        string  `gorm:"column:model"`
		UsedAmount   float64 `gorm:"column:used_amount"`
		RequestCount int64   `gorm:"column:request_count"`
		TotalTokens  int64   `gorm:"column:total_tokens"`
	}

	var aggResults []modelAgg

	if err := model.LogDB.
		Model(&model.GroupSummary{}).
		Select(
			"model",
			"SUM(used_amount) as used_amount",
			"SUM(request_count) as request_count",
			"SUM(total_tokens) as total_tokens",
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

	for _, r := range aggResults {
		usageStats.TotalAmount += r.UsedAmount
		usageStats.TotalTokens += r.TotalTokens
		usageStats.TotalRequests += r.RequestCount
		usageStats.TopModels = append(usageStats.TopModels, ModelUsage{
			Model:        r.Model,
			UsedAmount:   r.UsedAmount,
			RequestCount: r.RequestCount,
			TotalTokens:  r.TotalTokens,
		})
	}

	usageStats.UniqueModels = len(aggResults)

	// Sort by used amount desc, keep top 5
	sort.Slice(usageStats.TopModels, func(i, j int) bool {
		return usageStats.TopModels[i].UsedAmount > usageStats.TopModels[j].UsedAmount
	})

	if len(usageStats.TopModels) > 5 {
		usageStats.TopModels = usageStats.TopModels[:5]
	}

	// Query quota status
	var quotaStatus *MyQuotaStatus

	if feishuUser.TokenID > 0 {
		var token model.Token
		if err := model.DB.Where("id = ?", feishuUser.TokenID).First(&token).Error; err == nil && token.PeriodQuota > 0 {
			policy, _ := quota.GetPolicyForUser(c.Request.Context(), feishuUser.OpenID)
			if policy != nil {
				periodUsed := token.UsedAmount - token.PeriodLastUpdateAmount
				if periodUsed < 0 {
					periodUsed = 0
				}

				usageRatio := periodUsed / token.PeriodQuota

				currentTier := 1
				if usageRatio >= policy.Tier2Ratio {
					currentTier = 3
				} else if usageRatio >= policy.Tier1Ratio {
					currentTier = 2
				}

				quotaStatus = &MyQuotaStatus{
					PeriodQuota:  token.PeriodQuota,
					PeriodUsed:   periodUsed,
					PeriodType:   string(token.PeriodType),
					PeriodStart:  token.PeriodLastUpdateTime.Unix(),
					PolicyName:   policy.Name,
					PolicyID:     policy.ID,
					CurrentTier:  currentTier,
					Tier1Ratio:   policy.Tier1Ratio,
					Tier2Ratio:   policy.Tier2Ratio,
					BlockAtTier3: policy.BlockAtTier3,
				}
			}
		}
	}

	middleware.SuccessResponse(c, MyStatsResponse{
		Usage: usageStats,
		Quota: quotaStatus,
	})
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
