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
	case mode.ResponsesCompact:
		return containsMode(mode.ChatCompletions, mode.Responses, mode.ResponsesCompact)
	case mode.AlphaSearch:
		return containsMode(mode.ChatCompletions, mode.Responses, mode.AlphaSearch)
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

	channelHeader := c.Request.Header.Get("Aiproxy-Channel")
	adminChannelBypass := config.EnableAdminBypassChannelModelCheck &&
		group.Status == model.GroupStatusInternal && channelHeader != ""
	findModel := model.FindModelWithAllowList(
		GetActiveTokenModels(c),
		requestModel,
		GetActiveAvailableSets(c),
		GetActiveAvailableModels(c),
	)
