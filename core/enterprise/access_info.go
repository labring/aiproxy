//go:build enterprise

package enterprise

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
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
	RPM                int64    `json:"rpm"`
	TPM                int64    `json:"tpm"`
	InputPrice         float64  `json:"input_price"`
	OutputPrice        float64  `json:"output_price"`
	PriceUnit          int64    `json:"price_unit"`
	SupportedEndpoints []string `json:"supported_endpoints"`
}

var (
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

		ownerModels[owner] = append(ownerModels[owner], ModelAccessInfo{
			Model:              modelName,
			Type:               int(mc.Type),
			RPM:                rpm,
			TPM:                tpm,
			InputPrice:         inputPrice,
			OutputPrice:        outputPrice,
			PriceUnit:          priceUnit,
			SupportedEndpoints: getSupportedEndpoints(mc.Type),
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

// DisableMyToken disables (soft-deletes) a token belonging to the current user's group.
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

	// Verify token belongs to user's group
	var token model.Token
	if err := model.DB.Where("id = ? AND group_id = ?", id, feishuUser.GroupID).First(&token).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusNotFound, "token not found")
		return
	}

	if err := model.UpdateTokenStatus(id, model.TokenStatusDisabled); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, "failed to disable token: "+err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{"message": "token disabled"})
}
