package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/balance"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/notify"
	"github.com/labring/aiproxy/core/common/reqlimit"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	monitorplugin "github.com/labring/aiproxy/core/relay/plugin/monitor"
	"gorm.io/gorm"
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

	return ApplyGroupModelRatios(group, mc)
}

func GetGroupScopeAdjustedModelConfig(
	group model.GroupCache,
	mc model.ModelConfig,
) model.ModelConfig {
	return ApplyGroupModelRatios(group, mc)
}

func ApplyGroupModelRatios(group model.GroupCache, mc model.ModelConfig) model.ModelConfig {
	rpmRatio, tpmRatio := getGroupPMRatio(group)
	groupConsumeLevelRatio := calculateGroupConsumeLevelRatio(group.UsedAmount)
	mc.RPM = int64(float64(mc.RPM) * rpmRatio * groupConsumeLevelRatio)
	mc.TPM = int64(float64(mc.TPM) * tpmRatio * groupConsumeLevelRatio)

	return mc
}

func ResolveModelConfig(
	group model.GroupCache,
	groupChannelMode string,
	modelCaches *model.ModelCaches,
	modelName string,
) (model.ModelConfig, bool) {
	if groupChannelMode == GroupChannelModeOwn {
		mc, ok := model.ResolveGroupScopeModelConfig(group.ID, modelName)
		if !ok {
			return model.ModelConfig{}, false
		}

		return GetGroupScopeAdjustedModelConfig(group, mc), true
	}

	mc, ok := modelCaches.ModelConfig.GetModelConfig(modelName)
	if !ok {
		return model.ModelConfig{}, false
	}

	return GetGroupAdjustedModelConfig(group, mc), true
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

func CheckGroupModelRPMAndTPM(
	c *gin.Context,
	group model.GroupCache,
	mc model.ModelConfig,
	tokenName string,
	channelScope model.ChannelScope,
	channelID int,
) error {
	if channelScope == model.ChannelScopeGroup {
		return checkGroupChannelModelRPMAndTPM(c, group, mc, channelID)
	}

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

func checkGroupChannelModelRPMAndTPM(
	c *gin.Context,
	group model.GroupCache,
	mc model.ModelConfig,
	channelID int,
) error {
	log := common.GetLogger(c)
	channelKey := strconv.Itoa(channelID)

	groupModelCount, groupModelOverLimitCount, groupModelSecondCount := reqlimit.PushGroupChannelModelRequest(
		c.Request.Context(),
		group.ID,
		channelKey,
		mc.Model,
	)

	if group.Status != model.GroupStatusInternal && mc.RPM > 0 {
		totalRequests := groupModelCount + groupModelOverLimitCount

		log.Data["group_channel_rpm_limit"] = strconv.FormatInt(mc.RPM, 10)
		if groupModelCount > mc.RPM {
			setRpmHeaders(c, mc.RPM, 0)
			return ErrRequestRateLimitExceeded
		}

		setRpmHeaders(c, mc.RPM, mc.RPM-groupModelCount)

		log.Data["group_channel_rpm"] = strconv.FormatInt(totalRequests, 10)
		log.Data["group_channel_rps"] = strconv.FormatInt(groupModelSecondCount, 10)
	}

	groupModelCountTPM, groupModelCountTPS := reqlimit.GetGroupChannelModelTokensRequest(
		c.Request.Context(),
		group.ID,
		channelKey,
		mc.Model,
	)

	if group.Status != model.GroupStatusInternal && mc.TPM > 0 {
		log.Data["group_channel_tpm_limit"] = strconv.FormatInt(mc.TPM, 10)
		if groupModelCountTPM >= mc.TPM {
			setTpmHeaders(c, mc.TPM, 0)
			return ErrRequestTpmLimitExceeded
		}

		setTpmHeaders(c, mc.TPM, mc.TPM-groupModelCountTPM)
		log.Data["group_channel_tpm"] = strconv.FormatInt(groupModelCountTPM, 10)
		log.Data["group_channel_tps"] = strconv.FormatInt(groupModelCountTPS, 10)
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
	GroupMinimumBalance   = 0.3
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

	if !gbc.CheckBalance(GroupMinimumBalance) {
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

	containsMode := func(modes ...mode.Mode) bool {
		return slices.Contains(modes, modelMode)
	}

	switch requestMode {
	case mode.GeminiVideo:
		return modelMode == mode.GeminiVideo
	case mode.GeminiFiles:
		return containsMode(mode.Gemini, mode.GeminiFiles, mode.GeminiVideo)
	case mode.GeminiVideoOperations:
		return containsMode(mode.GeminiVideo, mode.GeminiVideoOperations)
	case mode.AliVideo:
		return modelMode == mode.AliVideo
	case mode.AliVideoTasks:
		return containsMode(mode.AliVideo, mode.AliVideoTasks)
	case mode.DoubaoVideo:
		return modelMode == mode.DoubaoVideo
	case mode.DoubaoVideoTasks, mode.DoubaoVideoTasksDelete:
		return containsMode(mode.DoubaoVideo, mode.DoubaoVideoTasks, mode.DoubaoVideoTasksDelete)
	case mode.AudioSpeech:
		return containsMode(mode.AudioSpeech, mode.GeminiTTS)
	case mode.ChatCompletions, mode.Anthropic, mode.Gemini:
		return containsMode(
			mode.ChatCompletions,
			mode.Completions,
			mode.Anthropic,
			mode.Gemini,
			mode.GeminiTTS,
			mode.GeminiImage,
			mode.Responses,
		)
	case mode.Completions:
		return containsMode(
			mode.ChatCompletions,
			mode.Completions,
			mode.Anthropic,
			mode.Gemini,
			mode.GeminiTTS,
			mode.GeminiImage,
		)
	case mode.Responses:
		return containsMode(
			mode.ChatCompletions,
			mode.Anthropic,
			mode.Gemini,
			mode.GeminiTTS,
			mode.GeminiImage,
			mode.Responses,
		)
	case mode.ResponsesGet, mode.ResponsesDelete, mode.ResponsesCancel, mode.ResponsesInputItems:
		return containsMode(
			mode.ChatCompletions,
			mode.Anthropic,
			mode.Gemini,
			mode.GeminiTTS,
			mode.GeminiImage,
			mode.Responses,
			mode.ResponsesGet,
			mode.ResponsesDelete,
			mode.ResponsesCancel,
			mode.ResponsesInputItems,
		)
	case mode.ImagesGenerations:
		return containsMode(mode.ImagesGenerations, mode.ImagesEdits, mode.GeminiImage)
	case mode.ImagesEdits:
		return containsMode(mode.ImagesGenerations, mode.ImagesEdits)
	case mode.VideoGenerationsJobs, mode.VideoGenerationsGetJobs, mode.VideoGenerationsContent:
		return containsMode(
			mode.VideoGenerationsJobs,
			mode.VideoGenerationsGetJobs,
			mode.VideoGenerationsContent,
			mode.GeminiVideo,
			mode.AliVideo,
			mode.DoubaoVideo,
		)
	case mode.Videos,
		mode.VideosGet,
		mode.VideosContent,
		mode.VideosRemix,
		mode.VideosEdits,
		mode.VideosExtensions:
		return containsMode(
			mode.VideoGenerationsJobs,
			mode.VideoGenerationsGetJobs,
			mode.VideoGenerationsContent,
			mode.Videos,
			mode.VideosGet,
			mode.VideosContent,
			mode.VideosDelete,
			mode.VideosRemix,
			mode.VideosEdits,
			mode.VideosExtensions,
			mode.GeminiVideo,
			mode.AliVideo,
			mode.DoubaoVideo,
		)
	case mode.VideosDelete:
		return containsMode(
			mode.VideoGenerationsJobs,
			mode.VideoGenerationsGetJobs,
			mode.VideoGenerationsContent,
			mode.Videos,
			mode.VideosGet,
			mode.VideosContent,
			mode.VideosDelete,
			mode.VideosRemix,
			mode.VideosEdits,
			mode.VideosExtensions,
		)
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

	findModel := model.FindModelWithAllowList(
		GetActiveTokenModels(c),
		requestModel,
		GetActiveAvailableSets(c),
		GetActiveAvailableModels(c),
	)

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

	mc, ok := ResolveModelConfig(group, GetGroupChannelMode(c), GetModelCaches(c), findModel)
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

	c.Set(RequestModel, findModel)
	c.Set(ModelConfig, mc)

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
	SetLogRequestUser(log.Data, user)

	promptCacheKey, err := getPromptCacheKey(c, mode)
	if err != nil {
		AbortLogWithMessage(
			c,
			http.StatusInternalServerError,
			err.Error(),
		)

		return
	}

	c.Set(PromptCacheKey, promptCacheKey)
	SetLogPromptCacheKey(log.Data, promptCacheKey)

	requestServiceTier, err := getRequestServiceTier(c, mode)
	if err != nil {
		AbortLogWithMessage(
			c,
			http.StatusInternalServerError,
			err.Error(),
		)

		return
	}

	c.Set(RequestServiceTier, requestServiceTier)
	SetLogServiceTier(log.Data, requestServiceTier)

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

	clearRequestBodyNode(c)
	c.Next()
}

func GetRequestModel(c *gin.Context) string {
	return c.GetString(RequestModel)
}

func GetRequestUser(c *gin.Context) string {
	return c.GetString(RequestUser)
}

func GetPromptCacheKey(c *gin.Context) string {
	return c.GetString(PromptCacheKey)
}

func GetChannelID(c *gin.Context) int {
	return c.GetInt(ChannelID)
}

func GetChannelScope(c *gin.Context) model.ChannelScope {
	scope, _ := c.Get(ChannelScope)
	if typedScope, ok := scope.(model.ChannelScope); ok {
		return typedScope
	}

	if stringScope, ok := scope.(string); ok {
		return model.ChannelScope(stringScope)
	}

	return ""
}

func setStoreChannel(c *gin.Context, store *model.StoreCache, scope model.ChannelScope) {
	if store == nil {
		return
	}

	c.Set(ChannelID, store.ChannelID)
	c.Set(ChannelScope, scope)
}

func getStoredRequestStore(
	group string,
	tokenID int,
	storeID string,
	scope model.ChannelScope,
) (*model.StoreCache, error) {
	return model.CacheGetStoreByScope(group, tokenID, storeID, scope)
}

func getStoredRequestStoreScope(c *gin.Context) model.ChannelScope {
	if GetGroupChannelMode(c) == GroupChannelModeOwn {
		return model.ChannelScopeGroup
	}

	return model.ChannelScopeGlobal
}

func GetJobID(c *gin.Context) string {
	return c.GetString(JobID)
}

func GetGenerationID(c *gin.Context) string {
	return c.GetString(GenerationID)
}

func GetOperationID(c *gin.Context) string {
	return c.GetString(OperationID)
}

func GetResponseID(c *gin.Context) string {
	return c.GetString(ResponseID)
}

func GetVideoID(c *gin.Context) string {
	return c.GetString(VideoID)
}

func GetFileID(c *gin.Context) string {
	return c.GetString(FileID)
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
	operationID := GetOperationID(c)
	responseID := GetResponseID(c)
	videoID := GetVideoID(c)
	fileID := GetFileID(c)
	promptCacheKey := GetPromptCacheKey(c)
	user := GetRequestUser(c)
	requestServiceTier := GetRequestServiceTier(c)

	opts = append(
		opts,
		meta.WithRequestAt(requestAt),
		meta.WithRequestID(requestID),
		meta.WithGroup(group),
		meta.WithToken(token),
		meta.WithEndpoint(c.Request.URL.Path),
		meta.WithJobID(jobID),
		meta.WithGenerationID(generationID),
		meta.WithOperationID(operationID),
		meta.WithResponseID(responseID),
		meta.WithVideoID(videoID),
		meta.WithFileID(fileID),
		meta.WithPromptCacheKey(promptCacheKey),
		meta.WithUser(user),
		meta.WithRequestServiceTier(requestServiceTier),
	)

	return meta.NewMeta(
		channel,
		mode,
		modelName,
		modelConfig,
		opts...,
	)
}

func getRequestBodyNode(c *gin.Context) (*ast.Node, error) {
	if cached, ok := c.Get(requestBodyNode); ok {
		node, ok := cached.(*ast.Node)
		if !ok {
			return nil, fmt.Errorf("request body node type error: %T", cached)
		}

		return node, nil
	}

	node, err := common.UnmarshalRequest2NodeReusable(c.Request)
	if err != nil {
		return nil, err
	}

	c.Set(requestBodyNode, &node)

	return &node, nil
}

func clearRequestBodyNode(c *gin.Context) {
	if c == nil {
		return
	}

	delete(c.Keys, requestBodyNode)
}

func getStringFieldFromNode(node *ast.Node, key, errMessage string) (string, error) {
	field := node.Get(key)
	if field == nil || !field.Exists() || field.TypeSafe() == ast.V_NULL {
		return "", nil
	}

	value, err := field.String()
	if err != nil {
		return "", fmt.Errorf("%s: %w", errMessage, err)
	}

	return value, nil
}

func getMetadataFromNode(node *ast.Node) (map[string]string, error) {
	field := node.Get("metadata")
	if field == nil || !field.Exists() || field.TypeSafe() == ast.V_NULL {
		return nil, nil
	}

	raw, err := field.Raw()
	if err != nil {
		return nil, fmt.Errorf("get request metadata failed: %w", err)
	}

	var metadata map[string]string
	if err := sonic.UnmarshalString(raw, &metadata); err != nil {
		return nil, fmt.Errorf("get request metadata failed: %w", err)
	}

	return metadata, nil
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
	case m == mode.VideoGenerationsJobs &&
		strings.HasPrefix(c.Request.Header.Get("Content-Type"), "multipart/form-data"):
		return getLimitedMultipartFormValue(c.Request, "model")
	case isVideosCreateMode(m):
		return getVideosCreateRequestModel(c, group, tokenID)

	case strings.HasPrefix(path, "/v1/engines") && strings.HasSuffix(path, "/embeddings"):
		// /engines/:model/embeddings
		return c.Param("model"), nil

	case m == mode.VideoGenerationsGetJobs:
		jobID := c.Param("id")

		storeScope := getStoredRequestStoreScope(c)

		store, err := getStoredRequestStore(
			group,
			tokenID,
			model.VideoJobStoreID(jobID),
			storeScope,
		)
		if err != nil {
			return "", fmt.Errorf("get request model failed: %w", err)
		}

		c.Set(JobID, jobID)
		setStoreChannel(c, store, storeScope)

		return store.Model, nil
	case m == mode.VideoGenerationsContent:
		generationID := c.Param("id")

		storeScope := getStoredRequestStoreScope(c)

		store, err := getStoredRequestStore(
			group,
			tokenID,
			model.VideoGenerationStoreID(generationID),
			storeScope,
		)
		if err != nil {
			return "", fmt.Errorf("get request model failed: %w", err)
		}

		c.Set(GenerationID, generationID)
		setStoreChannel(c, store, storeScope)

		return store.Model, nil
	case isVideosStoredMode(m):
		return getStoredVideoRequestModel(c, group, tokenID)
	case isStoredResponseMode(m):
		return getStoredResponseRequestModel(c, group, tokenID)
	case m == mode.Responses:
		node, err := getRequestBodyNode(c)
		if err != nil {
			return "", fmt.Errorf("get request model failed: %w", err)
		}

		responseID, err := getStringFieldFromNode(
			node,
			"previous_response_id",
			"get request previous response id failed",
		)
		if err != nil {
			return "", err
		}

		modelName, err := getStringFieldFromNode(node, "model", "get request model failed")
		if err != nil {
			return "", err
		}

		if responseID != "" {
			storeScope := getStoredRequestStoreScope(c)

			store, err := getStoredRequestStore(
				group,
				tokenID,
				model.ResponseStoreID(responseID),
				storeScope,
			)
			if err != nil {
				return "", fmt.Errorf("get request model failed: %w", err)
			}

			c.Set(ResponseID, responseID)
			setStoreChannel(c, store, storeScope)
		}

		return modelName, nil
	case m == mode.Gemini || m == mode.GeminiVideo || m == mode.GeminiVideoOperations:
		return getGeminiRequestModel(c, group, tokenID)
	case m == mode.GeminiFiles:
		return getGeminiFileRequestModel(c, group, tokenID)
	case isProviderVideoMode(m):
		return getProviderVideoRequestModel(c, m, group, tokenID)
	default:
		node, err := getRequestBodyNode(c)
		if err != nil {
			return "", fmt.Errorf("get request model failed: %w", err)
		}

		return getStringFieldFromNode(node, "model", "get request model failed")
	}
}

func getGeminiRequestModel(c *gin.Context, group string, tokenID int) (string, error) {
	modelName, operationID := getGeminiPathModelAndOperationID(c)

	if operationID != "" {
		storeScope := getStoredRequestStoreScope(c)

		store, err := getStoredRequestStore(
			group,
			tokenID,
			model.VideoJobStoreID(operationID),
			storeScope,
		)
		if err != nil {
			return "", fmt.Errorf("get request model failed: %w", err)
		}

		c.Set(OperationID, operationID)
		setStoreChannel(c, store, storeScope)

		return store.Model, nil
	}

	modelName, _, _ = strings.Cut(modelName, ":")

	return modelName, nil
}

func isStoredResponseMode(m mode.Mode) bool {
	return m == mode.ResponsesGet ||
		m == mode.ResponsesDelete ||
		m == mode.ResponsesCancel ||
		m == mode.ResponsesInputItems
}

func getStoredResponseRequestModel(c *gin.Context, group string, tokenID int) (string, error) {
	responseID := c.Param("response_id")

	storeScope := getStoredRequestStoreScope(c)

	store, err := getStoredRequestStore(
		group,
		tokenID,
		model.ResponseStoreID(responseID),
		storeScope,
	)
	if err != nil {
		return "", fmt.Errorf("get request model failed: %w", err)
	}

	c.Set(ResponseID, responseID)
	setStoreChannel(c, store, storeScope)

	return store.Model, nil
}

func isProviderVideoMode(m mode.Mode) bool {
	return m == mode.AliVideo ||
		m == mode.AliVideoTasks ||
		m == mode.DoubaoVideo ||
		m == mode.DoubaoVideoTasks ||
		m == mode.DoubaoVideoTasksDelete
}

func getProviderVideoRequestModel(
	c *gin.Context,
	m mode.Mode,
	group string,
	tokenID int,
) (string, error) {
	if m == mode.AliVideoTasks || m == mode.DoubaoVideoTasks || m == mode.DoubaoVideoTasksDelete {
		return getNativeVideoTaskRequestModel(c, group, tokenID)
	}

	node, err := getRequestBodyNode(c)
	if err != nil {
		return "", fmt.Errorf("get request model failed: %w", err)
	}

	return getStringFieldFromNode(node, "model", "get request model failed")
}

func getGeminiFileRequestModel(c *gin.Context, group string, tokenID int) (string, error) {
	fileID := strings.TrimPrefix(c.Param("model"), "/")

	fileID = strings.TrimSuffix(fileID, ":download")
	if fileID == "" {
		return "", errors.New("get request model failed: file id is empty")
	}

	storeScope := getStoredRequestStoreScope(c)

	store, err := getStoredRequestStore(
		group,
		tokenID,
		model.GeminiFileStoreID(fileID),
		storeScope,
	)
	if err != nil {
		return "", fmt.Errorf("get request model failed: %w", err)
	}

	c.Set(FileID, fileID)
	setStoreChannel(c, store, storeScope)

	return store.Model, nil
}

func getGeminiPathModel(c *gin.Context) string {
	modelName, operationID := getGeminiPathModelAndOperationID(c)
	if operationID == "" {
		return modelName
	}

	if modelName == "" {
		return "operations/" + operationID
	}

	return "models/" + modelName + "/operations/" + operationID
}

func getGeminiPathModelAndOperationID(c *gin.Context) (string, string) {
	modelName := strings.TrimPrefix(c.Param("model"), "/")
	if operationID := strings.TrimPrefix(c.Param("operation_id"), "/"); operationID != "" {
		return modelName, operationID
	}

	if operationID, ok := strings.CutPrefix(modelName, "operations/"); ok {
		return "", operationID
	}

	if before, after, ok := strings.Cut(modelName, "/operations/"); ok {
		return strings.TrimPrefix(strings.TrimPrefix(before, "models/"), "/"), after
	}

	return modelName, ""
}

func isVideosCreateMode(m mode.Mode) bool {
	return m == mode.Videos ||
		m == mode.VideosRemix ||
		m == mode.VideosEdits ||
		m == mode.VideosExtensions
}

func isVideosStoredMode(m mode.Mode) bool {
	return m == mode.VideosGet || m == mode.VideosContent || m == mode.VideosDelete
}

func getVideosCreateRequestModel(c *gin.Context, group string, tokenID int) (string, error) {
	videoID := c.Param("video_id")
	if videoID != "" {
		storeScope := getStoredRequestStoreScope(c)

		store, err := getStoredRequestStore(
			group,
			tokenID,
			model.VideoGenerationStoreID(videoID),
			storeScope,
		)
		if err != nil {
			return "", fmt.Errorf("get request model failed: %w", err)
		}

		c.Set(VideoID, videoID)
		setStoreChannel(c, store, storeScope)
	}

	if strings.HasPrefix(c.Request.Header.Get("Content-Type"), "multipart/form-data") {
		requestModel, err := getLimitedMultipartFormValue(c.Request, "model")
		if err != nil {
			return requestModel, err
		}

		referenceModel, err := getVideoCreateRequestModelFromReference(
			c,
			group,
			tokenID,
			func() (string, error) {
				return getLimitedMultipartFormValue(c.Request, "video")
			},
		)
		if err != nil || requestModel != "" {
			return requestModel, err
		}

		return referenceModel, nil
	}

	node, err := getRequestBodyNode(c)
	if err != nil {
		return "", fmt.Errorf("get request model failed: %w", err)
	}

	requestModel, err := getStringFieldFromNode(node, "model", "get request model failed")
	if err != nil {
		return requestModel, err
	}

	referenceModel, err := getVideoCreateRequestModelFromReference(
		c,
		group,
		tokenID,
		func() (string, error) {
			return getStringFieldFromNode(node, "video", "get request video failed")
		},
	)
	if err != nil || requestModel != "" {
		return requestModel, err
	}

	return referenceModel, nil
}

func getVideoCreateRequestModelFromReference(
	c *gin.Context,
	group string,
	tokenID int,
	videoIDFromRequest func() (string, error),
) (string, error) {
	m := GetMode(c)
	if m != mode.VideosEdits && m != mode.VideosExtensions {
		return "", nil
	}

	videoID, err := videoIDFromRequest()
	if err != nil {
		return "", err
	}

	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return "", nil
	}

	storeScope := getStoredRequestStoreScope(c)

	store, err := getStoredRequestStore(
		group,
		tokenID,
		model.VideoGenerationStoreID(videoID),
		storeScope,
	)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}

		return "", fmt.Errorf("get request model failed: %w", err)
	}

	c.Set(VideoID, videoID)
	setStoreChannel(c, store, storeScope)

	return store.Model, nil
}

func getLimitedMultipartFormValue(req *http.Request, key string) (string, error) {
	if err := common.ParseMultipartFormWithLimit(req); err != nil {
		return "", fmt.Errorf("parse multipart form: %w", err)
	}

	if req.MultipartForm == nil || req.MultipartForm.Value == nil {
		return "", nil
	}

	values := req.MultipartForm.Value[key]
	if len(values) == 0 {
		return "", nil
	}

	return values[0], nil
}

func getStoredVideoRequestModel(c *gin.Context, group string, tokenID int) (string, error) {
	videoID := c.Param("video_id")

	storeScope := getStoredRequestStoreScope(c)

	store, err := getStoredRequestStore(
		group,
		tokenID,
		model.VideoGenerationStoreID(videoID),
		storeScope,
	)
	if err != nil {
		return "", fmt.Errorf("get request model failed: %w", err)
	}

	c.Set(VideoID, videoID)
	c.Set(GenerationID, videoID)
	setStoreChannel(c, store, storeScope)

	return store.Model, nil
}

func getNativeVideoTaskRequestModel(c *gin.Context, group string, tokenID int) (string, error) {
	taskID := strings.TrimSpace(c.Param("task_id"))
	if taskID == "" {
		taskID = strings.TrimSpace(c.Param("id"))
	}

	if taskID == "" {
		return "", errors.New("get request model failed: task id is empty")
	}

	storeScope := getStoredRequestStoreScope(c)

	store, err := getStoredRequestStore(
		group,
		tokenID,
		model.VideoGenerationStoreID(taskID),
		storeScope,
	)
	if err != nil {
		return "", fmt.Errorf("get request model failed: %w", err)
	}

	c.Set(VideoID, taskID)
	c.Set(GenerationID, taskID)
	setStoreChannel(c, store, storeScope)

	return store.Model, nil
}

func GetModelFromJSON(body []byte) (string, error) {
	node, err := common.GetJSONNodeNoCopy(body)
	if err != nil {
		return "", fmt.Errorf("get request model failed: %w", err)
	}

	return getStringFieldFromNode(&node, "model", "get request model failed")
}

func GetPreviousResponseIDFromJSON(body []byte) (string, error) {
	node, err := common.GetJSONNodeNoCopy(body)
	if err != nil {
		return "", fmt.Errorf("get request model failed: %w", err)
	}

	return getStringFieldFromNode(&node, "previous_response_id", "get request model failed")
}

func getPromptCacheKey(c *gin.Context, m mode.Mode) (string, error) {
	switch m {
	case mode.Responses, mode.ChatCompletions:
	default:
		return "", nil
	}

	node, err := getRequestBodyNode(c)
	if err != nil {
		return "", fmt.Errorf("get request prompt_cache_key failed: %w", err)
	}

	return getStringFieldFromNode(node, "prompt_cache_key", "get request prompt_cache_key failed")
}

func GetPromptCacheKeyFromJSON(body []byte) (string, error) {
	node, err := common.GetJSONNodeNoCopy(body)
	if err != nil {
		return "", fmt.Errorf("get request prompt_cache_key failed: %w", err)
	}

	return getStringFieldFromNode(&node, "prompt_cache_key", "get request prompt_cache_key failed")
}

func getRequestServiceTier(c *gin.Context, m mode.Mode) (string, error) {
	switch m {
	case mode.ChatCompletions, mode.Completions, mode.Responses, mode.Anthropic, mode.Gemini:
	default:
		return "", nil
	}

	node, err := getRequestBodyNode(c)
	if err != nil {
		return "", fmt.Errorf("get request service_tier failed: %w", err)
	}

	return getRequestServiceTierFromNode(node, m)
}

func getRequestServiceTierFromNode(node *ast.Node, m mode.Mode) (string, error) {
	switch m {
	case mode.Gemini:
		return getStringFieldFromNode(node, "serviceTier", "get request serviceTier failed")
	case mode.ChatCompletions, mode.Completions, mode.Responses, mode.Anthropic:
		return getStringFieldFromNode(node, "service_tier", "get request service_tier failed")
	default:
		return "", nil
	}
}

func GetRequestServiceTier(c *gin.Context) string {
	return c.GetString(RequestServiceTier)
}

// https://platform.openai.com/docs/api-reference/chat
func getRequestUser(c *gin.Context, m mode.Mode) (string, error) {
	switch m {
	case mode.ChatCompletions,
		mode.Responses,
		mode.Completions,
		mode.Embeddings,
		mode.ImagesGenerations,
		mode.AudioSpeech,
		mode.Rerank,
		mode.Anthropic,
		mode.Gemini:
		node, err := getRequestBodyNode(c)
		if err != nil {
			return "", fmt.Errorf("get request model failed: %w", err)
		}

		return getRequestUserFromNode(node, m)
	default:
		return "", nil
	}
}

func GetRequestUserFromJSON(body []byte, m mode.Mode) (string, error) {
	node, err := common.GetJSONNodeNoCopy(body)
	if err != nil {
		return "", fmt.Errorf("get request user failed: %w", err)
	}

	return getRequestUserFromNode(&node, m)
}

func getRequestUserFromNode(node *ast.Node, m mode.Mode) (string, error) {
	if m == mode.Anthropic {
		userIDNode := node.GetByPath("metadata", "user_id")
		if userIDNode != nil && userIDNode.Valid() && userIDNode.TypeSafe() != ast.V_NULL {
			userID, err := userIDNode.String()
			if err != nil {
				return "", fmt.Errorf("get request user failed: %w", err)
			}

			if userID != "" {
				return userID, nil
			}
		}
	}

	return getStringFieldFromNode(node, "user", "get request user failed")
}

func getRequestMetadata(c *gin.Context, m mode.Mode) (map[string]string, error) {
	switch m {
	case mode.ChatCompletions,
		mode.Completions,
		mode.Embeddings,
		mode.ImagesGenerations,
		mode.AudioSpeech,
		mode.Rerank,
		mode.Anthropic,
		mode.Gemini:
		node, err := getRequestBodyNode(c)
		if err != nil {
			return nil, fmt.Errorf("get request metadata failed: %w", err)
		}

		return getMetadataFromNode(node)
	default:
		return nil, nil
	}
}

func GetRequestMetadataFromJSON(body []byte) (map[string]string, error) {
	node, err := common.GetJSONNodeNoCopy(body)
	if err != nil {
		return nil, fmt.Errorf("get request metadata failed: %w", err)
	}

	return getMetadataFromNode(&node)
}
