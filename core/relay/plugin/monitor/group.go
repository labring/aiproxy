package monitor

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/reqlimit"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/plugin"
	"github.com/labring/aiproxy/core/relay/plugin/noop"
)

const (
	GroupModelTokenRPM = "group_model_token_rpm"
	GroupModelTokenRPS = "group_model_token_rps"
	GroupModelTokenTPM = "group_model_token_tpm"
	GroupModelTokenTPS = "group_model_token_tps"
)

var _ plugin.Plugin = (*GroupMonitor)(nil)

type GroupMonitor struct {
	noop.Noop
}

func NewGroupMonitorPlugin() plugin.Plugin {
	return &GroupMonitor{}
}

func (m *GroupMonitor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
	do adaptor.DoResponse,
) (adaptor.DoResponseResult, adaptor.Error) {
	result, relayErr := do.DoResponse(meta, store, c, resp)

	if result.Usage.TotalTokens > 0 {
		count, overLimitCount, secondCount := reqlimit.PushGroupModelTokensRequest(
			context.Background(),
			meta.Group.ID,
			meta.OriginModel,
			meta.ModelConfig.TPM,
			int64(result.Usage.TotalTokens),
		)
		UpdateGroupModelTokensRequest(c, meta.Group, count+overLimitCount, secondCount)

		count, overLimitCount, secondCount = reqlimit.PushGroupModelTokennameTokensRequest(
			context.Background(),
			meta.Group.ID,
			meta.OriginModel,
			meta.Token.Name,
			int64(result.Usage.TotalTokens),
		)
		UpdateGroupModelTokennameTokensRequest(c, count+overLimitCount, secondCount)
	}

	return result, relayErr
}

func UpdateGroupModelRequest(c *gin.Context, group model.GroupCache, rpm, rps int64) {
	if group.Status == model.GroupStatusInternal {
		return
	}

	log := common.GetLogger(c)
	log.Data["group_rpm"] = strconv.FormatInt(rpm, 10)
	log.Data["group_rps"] = strconv.FormatInt(rps, 10)
}

func UpdateGroupModelTokensRequest(c *gin.Context, group model.GroupCache, tpm, tps int64) {
	if group.Status == model.GroupStatusInternal {
		return
	}

	log := common.GetLogger(c)
	log.Data["group_tpm"] = strconv.FormatInt(tpm, 10)
	log.Data["group_tps"] = strconv.FormatInt(tps, 10)
}

func UpdateGroupModelTokennameRequest(c *gin.Context, rpm, rps int64) {
	c.Set(GroupModelTokenRPM, rpm)
	c.Set(GroupModelTokenRPS, rps)
	// log := common.GetLogger(c)
	// log.Data["rpm"] = strconv.FormatInt(rpm, 10)
	// log.Data["rps"] = strconv.FormatInt(rps, 10)
}

func UpdateGroupModelTokennameTokensRequest(c *gin.Context, tpm, tps int64) {
	c.Set(GroupModelTokenTPM, tpm)
	c.Set(GroupModelTokenTPS, tps)
	// log := common.GetLogger(c)
	// log.Data["tpm"] = strconv.FormatInt(tpm, 10)
	// log.Data["tps"] = strconv.FormatInt(tps, 10)
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
