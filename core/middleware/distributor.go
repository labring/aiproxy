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
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	monitorplugin "github.com/labring/aiproxy/core/relay/plugin/monitor"
)

func calculateGroupConsumeLevelRatio(usedAmount float64) float64 {
	v := config.GetGroupConsumeLevelRatio()
	if len(v) == 0 {
		return 1
	}

	var (
		maxConsumeLevel        float64 = -1
		groupConsumeLevelRatio float64
	)

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

func checkGroupModelRPMAndTPM(
	c *gin.Context,
	group model.GroupCache,
	mc model.ModelConfig,
	tokenName string,
) error {
	log := common.GetLogger(c)

	groupModelCount, groupModelOverLimitCount, groupModelSecondCount := reqlimit.PushGroupModelRequest(
		c.Request.Context(),
		group.ID,
		mc.Model,
		mc.RPM,
	)
	monitorplugin.UpdateGroupModelRequest(
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
	monitorplugin.UpdateGroupModelTokennameRequest(
		c,
		groupModelTokenCount+groupModelTokenOverLimitCount,
		groupModelTokenSecondCount,
	)

	if group.Status != model.GroupStatusInternal &&
		mc.RPM > 0 {
		log.Data["group_rpm_limit"] = strconv.FormatInt(mc.RPM, 10)
		if groupModelCount > mc.RPM {
			setRpmHeaders(c, mc.RPM, 0)
			return ErrRequestRateLimitExceeded
		}

		setRpmHeaders(c, mc.RPM, mc.RPM-groupModelCount)
	}

	groupModelCountTPM, groupModelCountTPS := reqlimit.GetGroupModelTokensRequest(
		c.Request.Context(),
		group.ID,
		mc.Model,
	)
	monitorplugin.UpdateGroupModelTokensRequest(c, group, groupModelCountTPM, groupModelCountTPS)

	groupModelTokenCountTPM, groupModelTokenCountTPS := reqlimit.GetGroupModelTokennameTokensRequest(
		c.Request.Context(),
		group.ID,
		mc.Model,
		tokenName,
	)
	monitorplugin.UpdateGroupModelTokennameTokensRequest(
		c,
		groupModelTokenCountTPM,
		groupModelTokenCountTPS,
	)

	if group.Status != model.GroupStatusInternal &&
		mc.TPM > 0 {
		log.Data["group_tpm_limit"] = strconv.FormatInt(mc.TPM, 10)
		if groupModelCountTPM >= mc.TPM {
			setTpmHeaders(c, mc.TPM, 0)
			return ErrRequestTpmLimitExceeded
		}

		setTpmHeaders(c, mc.TPM, mc.TPM-groupModelCountTPM)
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
		log := common.GetLogger(c)

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
			)

			return false
		}

		notify.ErrorThrottle(
			"getGroupBalanceError",
			time.Minute*3,
			fmt.Sprintf("Get group `%s` balance error", group.ID),
			err.Error(),
		)
		AbortWithMessage(
			c,
			http.StatusInternalServerError,
			fmt.Sprintf("get group `%s` balance error", group.ID),
		)

		return false
	}

	if group.Status != model.GroupStatusInternal &&
		group.BalanceAlertEnabled &&
		!gbc.CheckBalance(group.BalanceAlertThreshold) {
		notify.ErrorThrottle(
			"groupBalanceAlert:"+group.ID,
			time.Minute*30,
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
			relaymodel.WithType(GroupBalanceNotEnough),
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
	case mode.ChatCompletions, mode.Completions, mode.Anthropic,
		mode.Responses, mode.ResponsesGet, mode.ResponsesDelete, mode.ResponsesCancel, mode.ResponsesInputItems:
		return modelMode == mode.ChatCompletions ||
			modelMode == mode.Completions ||
			modelMode == mode.Anthropic ||
			modelMode == mode.Responses ||
			modelMode == mode.ResponsesGet ||
			modelMode == mode.ResponsesDelete ||
			modelMode == mode.ResponsesCancel ||
			modelMode == mode.ResponsesInputItems
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

	log := common.GetLogger(c)

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
		)

		return
	}

	if requestModel == "" {
		AbortLogWithMessage(c, http.StatusBadRequest, "no model provided")
		return
	}

	findModel := token.FindModel(requestModel)

	if findModel == "" {
		AbortLogWithMessage(
			c,
			http.StatusNotFound,
			fmt.Sprintf(
				"The model `%s` does not exist or you do not have access to it.",
				requestModel,
			),
		)

		return
	}

	SetLogModelFields(log.Data, findModel)

	mc, ok := GetModelCaches(c).ModelConfig.GetModelConfig(findModel)
	if !ok {
		AbortLogWithMessage(
			c,
			http.StatusNotFound,
			fmt.Sprintf(
				"The model `%s` does not exist or you do not have access to it.",
				findModel,
			),
		)

		return
	}

	mc = GetGroupAdjustedModelConfig(group, mc)

	c.Set(RequestModel, findModel)
	c.Set(ModelConfig, mc)

	if !CheckRelayMode(mode, mc.Type) {
		AbortLogWithMessage(
			c,
			http.StatusNotFound,
			fmt.Sprintf(
				"The model `%s` does not exist on this endpoint.",
				findModel,
			),
		)

		return
	}

	user, err := getRequestUser(c, mode)
	if err != nil {
		AbortLogWithMessage(
			c,
			http.StatusInternalServerError,
			err.Error(),
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
		)

		return
	}

	c.Set(RequestMetadata, metadata)

	if err := checkGroupModelRPMAndTPM(c, group, mc, token.Name); err != nil {
		errMsg := err.Error()

		consume.Summary(
			http.StatusTooManyRequests,
			time.Time{},
			NewMetaByContext(c, nil, mode),
			model.Usage{},
			model.Price{},
			true,
		)
		AbortLogWithMessage(c, http.StatusTooManyRequests, errMsg)

		return
	}

	c.Next()
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

func GetResponseID(c *gin.Context) string {
	return c.GetString(ResponseID)
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
	responseID := GetResponseID(c)

	opts = append(
		opts,
		meta.WithRequestAt(requestAt),
		meta.WithRequestID(requestID),
		meta.WithGroup(group),
		meta.WithToken(token),
		meta.WithEndpoint(c.Request.URL.Path),
		meta.WithJobID(jobID),
		meta.WithGenerationID(generationID),
		meta.WithResponseID(responseID),
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
func getRequestModel(c *gin.Context, m mode.Mode, group string, tokenID int) (string, error) {
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

		store, err := model.CacheGetStore(group, tokenID, jobID)
		if err != nil {
			return "", fmt.Errorf("get request model failed: %w", err)
		}

		c.Set(JobID, store.ID)
		c.Set(ChannelID, store.ChannelID)

		return store.Model, nil
	case m == mode.VideoGenerationsContent:
		generationID := c.Param("id")

		store, err := model.CacheGetStore(group, tokenID, generationID)
		if err != nil {
			return "", fmt.Errorf("get request model failed: %w", err)
		}

		c.Set(GenerationID, store.ID)
		c.Set(ChannelID, store.ChannelID)

		return store.Model, nil
	case m == mode.ResponsesGet || m == mode.ResponsesDelete ||
		m == mode.ResponsesCancel || m == mode.ResponsesInputItems:
		responseID := c.Param("response_id")

		store, err := model.CacheGetStore(group, tokenID, responseID)
		if err != nil {
			return "", fmt.Errorf("get request model failed: %w", err)
		}

		c.Set(ResponseID, store.ID)
		c.Set(ChannelID, store.ChannelID)

		return store.Model, nil
	case m == mode.Responses:
		body, err := common.GetRequestBodyReusable(c.Request)
		if err != nil {
			return "", fmt.Errorf("get request model failed: %w", err)
		}

		responseID, err := GetPreviousResponseIDFromJSON(body)
		if err != nil {
			return "", fmt.Errorf("get request previous response id failed: %w", err)
		}

		modelName, err := GetModelFromJSON(body)
		if err != nil {
			return "", err
		}

		if responseID != "" {
			store, err := model.CacheGetStore(group, tokenID, responseID)
			if err != nil {
				return "", fmt.Errorf("get request model failed: %w", err)
			}

			c.Set(ResponseID, store.ID)
			c.Set(ChannelID, store.ChannelID)
		}

		return modelName, nil
	default:
		body, err := common.GetRequestBodyReusable(c.Request)
		if err != nil {
			return "", fmt.Errorf("get request model failed: %w", err)
		}

		return GetModelFromJSON(body)
	}
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

func GetPreviousResponseIDFromJSON(body []byte) (string, error) {
	node, err := sonic.GetWithOptions(body, ast.SearchOptions{}, "previous_response_id")
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
		body, err := common.GetRequestBodyReusable(c.Request)
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
		body, err := common.GetRequestBodyReusable(c.Request)
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
