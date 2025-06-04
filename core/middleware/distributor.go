package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/balance"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/consume"
	"github.com/labring/aiproxy/core/common/notify"
	"github.com/labring/aiproxy/core/common/reqlimit"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

func calculateGroupConsumeLevelRatio(usedAmount float64) float64 {
	v := config.GetGroupConsumeLevelRatio()
	if len(v) == 0 {
		return 1
	}
	var maxConsumeLevel float64 = -1
	var groupConsumeLevelRatio float64
	for consumeLevel, ratio := range v {
		if usedAmount < consumeLevel {
			continue
		}
		if consumeLevel > maxConsumeLevel {
			maxConsumeLevel = consumeLevel
			groupConsumeLevelRatio = ratio
		}
	}
	if groupConsumeLevelRatio <= 0 {
		groupConsumeLevelRatio = 1
	}
	return groupConsumeLevelRatio
}

func getGroupPMRatio(group model.GroupCache) (float64, float64) {
	groupRPMRatio := group.RPMRatio
	if groupRPMRatio <= 0 {
		groupRPMRatio = 1
	}
	groupTPMRatio := group.TPMRatio
	if groupTPMRatio <= 0 {
		groupTPMRatio = 1
	}
	return groupRPMRatio, groupTPMRatio
}

func GetGroupAdjustedModelConfig(group model.GroupCache, mc model.ModelConfig) model.ModelConfig {
	if groupModelConfig, ok := group.ModelConfigs[mc.Model]; ok {
		mc = mc.LoadFromGroupModelConfig(groupModelConfig)
	}
	rpmRatio, tpmRatio := getGroupPMRatio(group)
	groupConsumeLevelRatio := calculateGroupConsumeLevelRatio(group.UsedAmount)
	mc.RPM = int64(float64(mc.RPM) * rpmRatio * groupConsumeLevelRatio)
	mc.TPM = int64(float64(mc.TPM) * tpmRatio * groupConsumeLevelRatio)
	return mc
}

var (
	ErrRequestRateLimitExceeded = errors.New("request rate limit exceeded, please try again later")
	ErrRequestTpmLimitExceeded  = errors.New("request tpm limit exceeded, please try again later")
)

const (
	XRateLimitLimitRequests = "X-RateLimit-Limit-Requests"
	//nolint:gosec
	XRateLimitLimitTokens       = "X-RateLimit-Limit-Tokens"
	XRateLimitRemainingRequests = "X-RateLimit-Remaining-Requests"
	//nolint:gosec
	XRateLimitRemainingTokens = "X-RateLimit-Remaining-Tokens"
	XRateLimitResetRequests   = "X-RateLimit-Reset-Requests"
	//nolint:gosec
	XRateLimitResetTokens = "X-RateLimit-Reset-Tokens"
)

func setRpmHeaders(c *gin.Context, rpm, remainingRequests int64) {
	c.Header(XRateLimitLimitRequests, strconv.FormatInt(rpm, 10))
	c.Header(XRateLimitRemainingRequests, strconv.FormatInt(remainingRequests, 10))
	c.Header(XRateLimitResetRequests, "1m0s")
}

func setTpmHeaders(c *gin.Context, tpm, remainingRequests int64) {
	c.Header(XRateLimitLimitTokens, strconv.FormatInt(tpm, 10))
	c.Header(XRateLimitRemainingTokens, strconv.FormatInt(remainingRequests, 10))
	c.Header(XRateLimitResetTokens, "1m0s")
}

func UpdateGroupModelRequest(c *gin.Context, group model.GroupCache, rpm, rps int64) {
	if group.Status == model.GroupStatusInternal {
		return
	}

	log := GetLogger(c)
	log.Data["group_rpm"] = strconv.FormatInt(rpm, 10)
	log.Data["group_rps"] = strconv.FormatInt(rps, 10)
}

func UpdateGroupModelTokensRequest(c *gin.Context, group model.GroupCache, tpm, tps int64) {
	if group.Status == model.GroupStatusInternal {
		return
	}

	log := GetLogger(c)
	log.Data["group_tpm"] = strconv.FormatInt(tpm, 10)
	log.Data["group_tps"] = strconv.FormatInt(tps, 10)
}

func UpdateGroupModelTokennameRequest(c *gin.Context, rpm, rps int64) {
	c.Set(GroupModelTokenRPM, rpm)
	c.Set(GroupModelTokenRPS, rps)
	// log := GetLogger(c)
	// log.Data["rpm"] = strconv.FormatInt(rpm, 10)
	// log.Data["rps"] = strconv.FormatInt(rps, 10)
}

func UpdateGroupModelTokennameTokensRequest(c *gin.Context, tpm, tps int64) {
	c.Set(GroupModelTokenTPM, tpm)
	c.Set(GroupModelTokenTPS, tps)
	// log := GetLogger(c)
	// log.Data["tpm"] = strconv.FormatInt(tpm, 10)
	// log.Data["tps"] = strconv.FormatInt(tps, 10)
}

func checkGroupModelRPMAndTPM(
	c *gin.Context,
	group model.GroupCache,
	mc model.ModelConfig,
	tokenName string,
) error {
	log := GetLogger(c)

	adjustedModelConfig := GetGroupAdjustedModelConfig(group, mc)

	groupModelCount, groupModelOverLimitCount, groupModelSecondCount := reqlimit.PushGroupModelRequest(
		c.Request.Context(),
		group.ID,
		mc.Model,
		adjustedModelConfig.RPM,
	)
	UpdateGroupModelRequest(
		c,
		group,
		groupModelCount+groupModelOverLimitCount,
		groupModelSecondCount,
	)

	groupModelTokenCount, groupModelTokenOverLimitCount, groupModelTokenSecondCount := reqlimit.PushGroupModelTokennameRequest(
		c.Request.Context(),
		group.ID,
		mc.Model,
		tokenName,
	)
	UpdateGroupModelTokennameRequest(
		c,
		groupModelTokenCount+groupModelTokenOverLimitCount,
		groupModelTokenSecondCount,
	)

	if group.Status != model.GroupStatusInternal &&
		adjustedModelConfig.RPM > 0 {
		log.Data["group_rpm_limit"] = strconv.FormatInt(adjustedModelConfig.RPM, 10)
		if groupModelCount > adjustedModelConfig.RPM {
			setRpmHeaders(c, adjustedModelConfig.RPM, 0)
			return ErrRequestRateLimitExceeded
		}
		setRpmHeaders(c, adjustedModelConfig.RPM, adjustedModelConfig.RPM-groupModelCount)
	}

	groupModelCountTPM, groupModelCountTPS := reqlimit.GetGroupModelTokensRequest(
		c.Request.Context(),
		group.ID,
		mc.Model,
	)
	UpdateGroupModelTokensRequest(c, group, groupModelCountTPM, groupModelCountTPS)

	groupModelTokenCountTPM, groupModelTokenCountTPS := reqlimit.GetGroupModelTokennameTokensRequest(
		c.Request.Context(),
		group.ID,
		mc.Model,
		tokenName,
	)
	UpdateGroupModelTokennameTokensRequest(c, groupModelTokenCountTPM, groupModelTokenCountTPS)

	if group.Status != model.GroupStatusInternal &&
		adjustedModelConfig.TPM > 0 {
		log.Data["group_tpm_limit"] = strconv.FormatInt(adjustedModelConfig.TPM, 10)
		if groupModelCountTPM >= adjustedModelConfig.TPM {
			setTpmHeaders(c, adjustedModelConfig.TPM, 0)
			return ErrRequestTpmLimitExceeded
		}
		setTpmHeaders(c, adjustedModelConfig.TPM, adjustedModelConfig.TPM-groupModelCountTPM)
	}

	return nil
}

type GroupBalanceConsumer struct {
	Group        string
	balance      float64
	CheckBalance func(amount float64) bool
	Consumer     balance.PostGroupConsumer
}

func GetGroupBalanceConsumerFromContext(c *gin.Context) *GroupBalanceConsumer {
	gbcI, ok := c.Get(GroupBalance)
	if ok {
		groupBalanceConsumer, ok := gbcI.(*GroupBalanceConsumer)
		if !ok {
			panic("internal error: group balance consumer unavailable")
		}
		return groupBalanceConsumer
	}
	return nil
}

func GetGroupBalanceConsumer(
	c *gin.Context,
	group model.GroupCache,
) (*GroupBalanceConsumer, error) {
	gbc := GetGroupBalanceConsumerFromContext(c)
	if gbc != nil {
		return gbc, nil
	}

	if group.Status == model.GroupStatusInternal {
		gbc = &GroupBalanceConsumer{
			Group: group.ID,
			CheckBalance: func(_ float64) bool {
				return true
			},
			Consumer: nil,
		}
	} else {
		log := GetLogger(c)
		groupBalance, consumer, err := balance.GetGroupRemainBalance(c.Request.Context(), group)
		if err != nil {
			return nil, err
		}
		log.Data["balance"] = strconv.FormatFloat(groupBalance, 'f', -1, 64)

		gbc = &GroupBalanceConsumer{
			Group:   group.ID,
			balance: groupBalance,
			CheckBalance: func(amount float64) bool {
				return groupBalance >= amount
			},
			Consumer: consumer,
		}
	}

	c.Set(GroupBalance, gbc)
	return gbc, nil
}

const (
	GroupBalanceNotEnough = "group_balance_not_enough"
)

func checkGroupBalance(c *gin.Context, group model.GroupCache) bool {
	gbc, err := GetGroupBalanceConsumer(c, group)
	if err != nil {
		if errors.Is(err, balance.ErrNoRealNameUsedAmountLimit) {
			AbortLogWithMessage(
				c,
				http.StatusForbidden,
				err.Error(),
				"no_real_name_used_amount_limit",
			)
			return false
		}
		notify.ErrorThrottle(
			"getGroupBalanceError",
			time.Minute,
			fmt.Sprintf("Get group `%s` balance error", group.ID),
			err.Error(),
		)
		AbortWithMessage(
			c,
			http.StatusInternalServerError,
			fmt.Sprintf("get group `%s` balance error", group.ID),
			"get_group_balance_error",
		)
		return false
	}

	if group.Status != model.GroupStatusInternal &&
		group.BalanceAlertEnabled &&
		!gbc.CheckBalance(group.BalanceAlertThreshold) {
		notify.ErrorThrottle(
			"groupBalanceAlert:"+group.ID,
			time.Minute*15,
			fmt.Sprintf("Group `%s` balance below threshold", group.ID),
			fmt.Sprintf(
				"Group `%s` balance has fallen below the threshold\nCurrent balance: %.2f",
				group.ID,
				gbc.balance,
			),
		)
	}

	if !gbc.CheckBalance(0) {
		AbortLogWithMessage(
			c,
			http.StatusForbidden,
			fmt.Sprintf("group `%s` balance not enough", group.ID),
			GroupBalanceNotEnough,
		)
		return false
	}
	return true
}

func NewDistribute(mode mode.Mode) gin.HandlerFunc {
	return func(c *gin.Context) {
		distribute(c, mode)
	}
}

func CheckRelayMode(requestMode, modelMode mode.Mode) bool {
	if modelMode == mode.Unknown {
		return true
	}
	switch requestMode {
	case mode.ChatCompletions, mode.Completions, mode.Anthropic:
		return modelMode == mode.ChatCompletions ||
			modelMode == mode.Completions ||
			modelMode == mode.Anthropic
	case mode.ImagesGenerations, mode.ImagesEdits:
		return modelMode == mode.ImagesGenerations ||
			modelMode == mode.ImagesEdits
	case mode.VideoGenerationsJobs, mode.VideoGenerationsGetJobs, mode.VideoGenerationsContent:
		return modelMode == mode.VideoGenerationsJobs ||
			modelMode == mode.VideoGenerationsGetJobs ||
			modelMode == mode.VideoGenerationsContent
	default:
		return requestMode == modelMode
	}
}

func distribute(c *gin.Context, mode mode.Mode) {
	c.Set(Mode, mode)

	if config.GetDisableServe() {
		AbortLogWithMessage(c, http.StatusServiceUnavailable, "service is under maintenance")
		return
	}

	log := GetLogger(c)

	group := GetGroup(c)
	token := GetToken(c)

	if !checkGroupBalance(c, group) {
		return
	}

	requestModel, err := getRequestModel(c, mode, group.ID, token.ID)
	if err != nil {
		AbortLogWithMessage(
			c,
			http.StatusInternalServerError,
			err.Error(),
			"get_request_model_error",
		)
		return
	}
	if requestModel == "" {
		AbortLogWithMessage(c, http.StatusBadRequest, "no model provided", "no_model_provided")
		return
	}

	c.Set(RequestModel, requestModel)

	SetLogModelFields(log.Data, requestModel)

	mc, ok := GetModelCaches(c).ModelConfig.GetModelConfig(requestModel)
	if !ok || !CheckRelayMode(mode, mc.Type) {
		AbortLogWithMessage(
			c,
			http.StatusNotFound,
			fmt.Sprintf(
				"The model `%s` does not exist or you do not have access to it.",
				requestModel,
			),
			"model_not_found",
		)
		return
	}
	c.Set(ModelConfig, mc)

	if !token.ContainsModel(requestModel) {
		AbortLogWithMessage(
			c,
			http.StatusNotFound,
			fmt.Sprintf(
				"The model `%s` does not exist or you do not have access to it.",
				requestModel,
			),
			"model_not_found",
		)
		return
	}

	user, err := getRequestUser(c, mode)
	if err != nil {
		AbortLogWithMessage(
			c,
			http.StatusInternalServerError,
			err.Error(),
			"get_request_user_error",
		)
		return
	}
	c.Set(RequestUser, user)

	metadata, err := getRequestMetadata(c, mode)
	if err != nil {
		AbortLogWithMessage(
			c,
			http.StatusInternalServerError,
			err.Error(),
			"get_request_metadata_error",
		)
		return
	}
	c.Set(RequestMetadata, metadata)

	if err := checkGroupModelRPMAndTPM(c, group, mc, token.Name); err != nil {
		errMsg := err.Error()
		consume.AsyncConsume(
			nil,
			http.StatusTooManyRequests,
			time.Time{},
			NewMetaByContext(c, nil, mode),
			model.Usage{},
			model.Price{},
			errMsg,
			c.ClientIP(),
			0,
			nil,
			true,
			user,
			metadata,
			model.RequestRate{},
			GetGroupModelTokenRequestRate(c),
		)
		AbortLogWithMessage(c, http.StatusTooManyRequests, errMsg, "request_rate_limit_exceeded")
		return
	}

	c.Next()
}

func GetGroupModelTokenRequestRate(c *gin.Context) model.RequestRate {
	return model.RequestRate{
		RPM: GetGroupModelTokenRPM(c),
		RPS: GetGroupModelTokenRPS(c),
		TPM: GetGroupModelTokenTPM(c),
		TPS: GetGroupModelTokenTPS(c),
	}
}

func GetGroupModelTokenRPM(c *gin.Context) int64 {
	return c.GetInt64(GroupModelTokenRPM)
}

func GetGroupModelTokenRPS(c *gin.Context) int64 {
	return c.GetInt64(GroupModelTokenRPS)
}

func GetGroupModelTokenTPM(c *gin.Context) int64 {
	return c.GetInt64(GroupModelTokenTPM)
}

func GetGroupModelTokenTPS(c *gin.Context) int64 {
	return c.GetInt64(GroupModelTokenTPS)
}

func GetRequestModel(c *gin.Context) string {
	return c.GetString(RequestModel)
}

func GetRequestUser(c *gin.Context) string {
	return c.GetString(RequestUser)
}

func GetChannelID(c *gin.Context) int {
	return c.GetInt(ChannelID)
}

func GetJobID(c *gin.Context) string {
	return c.GetString(JobID)
}

func GetGenerationID(c *gin.Context) string {
	return c.GetString(GenerationID)
}

func GetRequestMetadata(c *gin.Context) map[string]string {
	return c.GetStringMapString(RequestMetadata)
}

func GetModelConfig(c *gin.Context) model.ModelConfig {
	v, ok := c.MustGet(ModelConfig).(model.ModelConfig)
	if !ok {
		panic(fmt.Sprintf("model config type error: %T, %v", v, v))
	}
	return v
}

func NewMetaByContext(c *gin.Context,
	channel *model.Channel,
	mode mode.Mode,
	opts ...meta.Option,
) *meta.Meta {
	requestID := GetRequestID(c)
	group := GetGroup(c)
	token := GetToken(c)
	modelName := GetRequestModel(c)
	modelConfig := GetModelConfig(c)
	requestAt := GetRequestAt(c)
	jobID := GetJobID(c)
	generationID := GetGenerationID(c)

	opts = append(
		opts,
		meta.WithRequestAt(requestAt),
		meta.WithRequestID(requestID),
		meta.WithGroup(group),
		meta.WithToken(token),
		meta.WithEndpoint(c.Request.URL.Path),
		meta.WithJobID(jobID),
		meta.WithGenerationID(generationID),
	)

	return meta.NewMeta(
		channel,
		mode,
		modelName,
		modelConfig,
		opts...,
	)
}

// https://platform.openai.com/docs/api-reference/chat
func getRequestModel(c *gin.Context, m mode.Mode, groupID string, tokenID int) (string, error) {
	path := c.Request.URL.Path
	switch {
	case m == mode.ParsePdf:
		query := c.Request.URL.Query()
		model := query.Get("model")
		if model != "" {
			return model, nil
		}

		fallthrough
	case m == mode.AudioTranscription,
		m == mode.AudioTranslation,
		m == mode.ImagesEdits:
		return c.Request.FormValue("model"), nil

	case strings.HasPrefix(path, "/v1/engines") && strings.HasSuffix(path, "/embeddings"):
		// /engines/:model/embeddings
		return c.Param("model"), nil

	case m == mode.VideoGenerationsGetJobs:
		jobID := c.Param("id")
		store, err := model.CacheGetStore(jobID)
		if err != nil {
			return "", fmt.Errorf("get request model failed: %w", err)
		}
		if err := validateStoreGroupAndToken(store, groupID, tokenID); err != nil {
			return "", fmt.Errorf("validate store group and token failed: %w", err)
		}
		c.Set(JobID, store.ID)
		c.Set(ChannelID, store.ChannelID)
		return store.Model, nil
	case m == mode.VideoGenerationsContent:
		generationID := c.Param("id")
		store, err := model.CacheGetStore(generationID)
		if err != nil {
			return "", fmt.Errorf("get request model failed: %w", err)
		}
		if err := validateStoreGroupAndToken(store, groupID, tokenID); err != nil {
			return "", fmt.Errorf("validate store group and token failed: %w", err)
		}
		c.Set(GenerationID, store.ID)
		c.Set(ChannelID, store.ChannelID)
		return store.Model, nil
	default:
		body, err := common.GetRequestBody(c.Request)
		if err != nil {
			return "", fmt.Errorf("get request model failed: %w", err)
		}
		return GetModelFromJSON(body)
	}
}

func validateStoreGroupAndToken(store *model.StoreCache, groupID string, tokenID int) error {
	if store.GroupID != groupID {
		return fmt.Errorf("store group id mismatch: %s != %s", store.GroupID, groupID)
	}
	if store.TokenID != tokenID {
		return fmt.Errorf("store token id mismatch: %d != %d", store.TokenID, tokenID)
	}
	return nil
}

func GetModelFromJSON(body []byte) (string, error) {
	node, err := sonic.GetWithOptions(body, ast.SearchOptions{}, "model")
	if err != nil {
		if errors.Is(err, ast.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("get request model failed: %w", err)
	}
	return node.String()
}

// https://platform.openai.com/docs/api-reference/chat
func getRequestUser(c *gin.Context, m mode.Mode) (string, error) {
	switch m {
	case mode.ChatCompletions,
		mode.Completions,
		mode.Embeddings,
		mode.ImagesGenerations,
		mode.AudioSpeech,
		mode.Rerank,
		mode.Anthropic:
		body, err := common.GetRequestBody(c.Request)
		if err != nil {
			return "", fmt.Errorf("get request model failed: %w", err)
		}
		return GetRequestUserFromJSON(body)
	default:
		return "", nil
	}
}

func GetRequestUserFromJSON(body []byte) (string, error) {
	node, err := sonic.GetWithOptions(body, ast.SearchOptions{}, "user")
	if err != nil {
		if errors.Is(err, ast.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("get request user failed: %w", err)
	}
	if node.Exists() {
		return node.String()
	}
	return "", nil
}

func getRequestMetadata(c *gin.Context, m mode.Mode) (map[string]string, error) {
	switch m {
	case mode.ChatCompletions,
		mode.Completions,
		mode.Embeddings,
		mode.ImagesGenerations,
		mode.AudioSpeech,
		mode.Rerank,
		mode.Anthropic:
		body, err := common.GetRequestBody(c.Request)
		if err != nil {
			return nil, fmt.Errorf("get request metadata failed: %w", err)
		}
		return GetRequestMetadataFromJSON(body)
	default:
		return nil, nil
	}
}

type RequestWithMetadata struct {
	Metadata map[string]string `json:"metadata,omitempty"`
}

func GetRequestMetadataFromJSON(body []byte) (map[string]string, error) {
	var requestWithMetadata RequestWithMetadata
	if err := sonic.Unmarshal(body, &requestWithMetadata); err != nil {
		return nil, fmt.Errorf("get request metadata failed: %w", err)
	}
	return requestWithMetadata.Metadata, nil
}
