package middleware

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/network"
	"github.com/labring/aiproxy/core/common/oncall"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/sirupsen/logrus"
)

const (
	XAiproxyGroup            = "X-Aiproxy-Group"
	XAiproxyGroupChannelMode = "X-Aiproxy-Group-Channel-Mode"

	GroupChannelModeOwn    = "own"
	GroupChannelModeGlobal = "global"
)

type APIResponse struct {
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
	Success bool   `json:"success"`
}

func SuccessResponse(c *gin.Context, data any) {
	c.JSON(http.StatusOK, &APIResponse{
		Success: true,
		Data:    data,
	})
}

func ErrorResponse(c *gin.Context, code int, message string) {
	c.JSON(code, &APIResponse{
		Success: false,
		Message: message,
	})
}

func AdminAuth(c *gin.Context) {
	if config.AdminKey == "" {
		ErrorResponse(c, http.StatusUnauthorized, "unauthorized, admin key is not set")
		c.Abort()
		return
	}

	accessToken := c.Request.Header.Get("Authorization")
	if accessToken == "" {
		accessToken = c.Query("key")
	}

	accessToken = strings.TrimPrefix(accessToken, "Bearer ")
	accessToken = strings.TrimPrefix(accessToken, "sk-")

	if accessToken != config.AdminKey {
		ErrorResponse(c, http.StatusUnauthorized, "unauthorized, no access token provided")
		c.Abort()
		return
	}

	c.Set(Token, &model.TokenCache{
		Key: config.AdminKey,
	})

	group := c.Param("group")
	if group != "" {
		log := common.GetLogger(c)
		log.Data["gid"] = group
	}

	c.Next()
}

func TokenAuth(c *gin.Context) {
	log := common.GetLogger(c)

	key := c.Request.Header.Get("Authorization")
	if key == "" {
		key = c.Request.Header.Get("X-Api-Key")
	}

	if key == "" {
		key = c.Request.Header.Get("X-Goog-Api-Key")
	}

	key = strings.TrimPrefix(
		strings.TrimPrefix(key, "Bearer "),
		"sk-",
	)

	var (
		token            model.TokenCache
		useInternalToken bool
	)

	if config.AdminKey != "" && config.AdminKey == key ||
		config.InternalToken != "" && config.InternalToken == key {
		token = model.TokenCache{
			Key: key,
		}
		useInternalToken = true
	} else {
		tokenCache, err := model.GetTokenByKeyForAuth(key)
		if err != nil {
			oncall.AlertDBError("TokenAuth", err)
			AbortLogWithMessage(c, http.StatusUnauthorized, err.Error())
			return
		}

		// Clear DB error state on successful token validation
		oncall.ClearDBError("TokenAuth")

		token = *tokenCache
	}

	SetLogTokenFields(log.Data, token, useInternalToken)

	if len(token.Subnets) > 0 {
		if ok, err := network.IsIPInSubnets(c.ClientIP(), token.Subnets); err != nil {
			AbortLogWithMessage(c, http.StatusInternalServerError, err.Error())
			return
		} else if !ok {
			AbortLogWithMessage(
				c,
				http.StatusForbidden,
				fmt.Sprintf(
					"token (%s[%d]) can only be used in the specified subnets: %v, current ip: %s",
					token.Name,
					token.ID,
					token.Subnets,
					c.ClientIP(),
				),
			)

			return
		}
	}

	modelCaches := model.LoadModelCaches()

	var (
		group            model.GroupCache
		groupChannelMode string
	)

	groupChannelModeHeader := c.Request.Header.Get(XAiproxyGroupChannelMode)
	if useInternalToken {
		groupID := c.Request.Header.Get(XAiproxyGroup)
		group = model.GroupCache{
			ID:     groupID,
			Status: model.GroupStatusInternal,
		}
		groupChannelMode = getInternalTokenGroupChannelMode(groupChannelModeHeader, groupID)
	} else {
		groupChannelMode = getTokenGroupChannelMode(token)

		groupCache, err := model.CacheGetGroup(token.Group)
		if err != nil {
			AbortLogWithMessage(
				c,
				http.StatusInternalServerError,
				fmt.Sprintf("failed to get group: %v", err),
			)

			return
		}

		group = *groupCache
	}

	if !useInternalToken && groupChannelMode != GroupChannelModeOwn {
		if err := model.ValidateTokenQuota(&token); err != nil {
			AbortLogWithMessage(c, http.StatusUnauthorized, err.Error())
			return
		}
	}

	c.Header("Group", group.ID)

	SetLogGroupFields(log.Data, group)

	if groupChannelMode != "" {
		log.Data["group_channel_mode"] = groupChannelMode
	}

	if group.Status != model.GroupStatusEnabled && group.Status != model.GroupStatusInternal {
		AbortLogWithMessage(c, http.StatusForbidden, "group is disabled")
		return
	}

	availableSets := model.ResolveTokenAvailableSets(
		group.GetAvailableSets(),
		token.GetConfiguredSets(),
	)
	groupChannelAvailableSets := resolveRequestGroupChannelAvailableSets(
		group,
		token,
		groupChannelMode,
	)
	availableModels := model.FilterModelsBySet(modelCaches.EnabledModelsBySet, availableSets)
	groupChannelAvailableModels := mergeGroupChannelModelsBySet(
		group,
		groupChannelAvailableSets,
		groupChannelMode,
		modelCaches.EnabledModelsBySet,
	)

	c.Set(Group, group)
	c.Set(Token, token)
	c.Set(AvailableSets, availableSets)
	c.Set(AvailableModels, availableModels)
	c.Set(GroupChannelAvailableSets, groupChannelAvailableSets)
	c.Set(GroupChannelAvailableModels, groupChannelAvailableModels)
	c.Set(ModelCaches, modelCaches)
	c.Set(GroupChannelMode, groupChannelMode)

	c.Next()
}

func parseGroupChannelMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case GroupChannelModeOwn, "group", "group-only", "group_only":
		return GroupChannelModeOwn
	case GroupChannelModeGlobal:
		return GroupChannelModeGlobal
	default:
		return GroupChannelModeGlobal
	}
}

func getTokenGroupChannelMode(
	token model.TokenCache,
) string {
	switch token.Scope {
	case model.ChannelScopeGlobal:
		return GroupChannelModeGlobal
	case model.ChannelScopeGroup:
		return GroupChannelModeOwn
	default:
		return GroupChannelModeGlobal
	}
}

func getInternalTokenGroupChannelMode(header, groupID string) string {
	if groupID == "" {
		return GroupChannelModeGlobal
	}

	return parseGroupChannelMode(header)
}

func resolveRequestGroupChannelAvailableSets(
	group model.GroupCache,
	token model.TokenCache,
	mode string,
) []string {
	if mode != GroupChannelModeOwn || group.ID == "" {
		return nil
	}

	if group.Status == model.GroupStatusInternal {
		return model.ResolveTokenGroupChannelAvailableSets(
			group.GetAvailableSets(),
			token.GetConfiguredGroupChannelSets(),
		)
	}

	cache, err := model.CacheGetGroupChannels(group.ID)
	if err != nil {
		return nil
	}

	availableSets := make([]string, 0)
	for _, channel := range cache.Channels {
		if channel.Status != model.ChannelStatusEnabled {
			continue
		}

		for _, set := range channel.GetSets() {
			if !slices.Contains(availableSets, set) {
				availableSets = append(availableSets, set)
			}
		}
	}

	return model.ResolveTokenGroupChannelAvailableSets(
		availableSets,
		token.GetConfiguredGroupChannelSets(),
	)
}

func mergeGroupChannelModelsBySet(
	group model.GroupCache,
	availableSets []string,
	mode string,
	global map[string][]string,
) map[string][]string {
	if mode == GroupChannelModeGlobal || group.ID == "" {
		return model.FilterModelsBySet(global, availableSets)
	}

	result := make(map[string][]string)

	cache, err := model.CacheGetGroupChannels(group.ID)
	if err != nil {
		return result
	}

	scopeModels := availableGroupScopeModels(group.ID, cache.Channels)
	if len(scopeModels) == 0 {
		return result
	}

	allowedSets := make(map[string]struct{}, len(availableSets))
	for _, set := range availableSets {
		allowedSets[set] = struct{}{}
	}

	for _, channel := range cache.Channels {
		if channel.Status != model.ChannelStatusEnabled {
			continue
		}

		supportedModels := groupChannelSupportedScopeModels(channel, scopeModels)
		if len(supportedModels) == 0 {
			continue
		}

		for _, set := range channel.GetSets() {
			if _, ok := allowedSets[set]; !ok {
				continue
			}

			for _, modelName := range supportedModels {
				result[set] = appendModelIfMissing(result[set], modelName)
			}
		}
	}

	for set := range result {
		slices.Sort(result[set])
	}

	return result
}

func availableGroupScopeModels(groupID string, channels []*model.GroupChannel) []string {
	if config.DisableModelConfig {
		return groupChannelAccessModels(channels)
	}

	scopeConfigs, err := model.CacheGetGroupScopeModelConfigs(groupID)
	if err != nil {
		return nil
	}

	return validGroupScopeModels(scopeConfigs)
}

func groupChannelAccessModels(channels []*model.GroupChannel) []string {
	modelCount := 0
	for _, channel := range channels {
		modelCount += len(model.GroupChannelAccessModels(channel))
	}

	models := make([]string, 0, modelCount)
	for _, channel := range channels {
		models = append(models, model.GroupChannelAccessModels(channel)...)
	}

	slices.Sort(models)

	return slices.CompactFunc(models, strings.EqualFold)
}

func validGroupScopeModels(scopeConfigs *model.GroupScopeModelConfigsCache) []string {
	models := make([]string, 0, len(scopeConfigs.Models))
	for _, modelName := range scopeConfigs.Models {
		if _, ok := scopeConfigs.Configs[modelName]; ok {
			models = append(models, modelName)
		}
	}

	return models
}

func groupChannelSupportedScopeModels(channel *model.GroupChannel, scopeModels []string) []string {
	accessModels := model.GroupChannelAccessModels(channel)
	if len(accessModels) == 0 {
		return nil
	}

	access := make(map[string]struct{}, len(accessModels))
	for _, modelName := range accessModels {
		access[strings.ToLower(modelName)] = struct{}{}
	}

	models := make([]string, 0, len(scopeModels))
	for _, modelName := range scopeModels {
		if _, ok := access[strings.ToLower(modelName)]; ok {
			models = append(models, modelName)
		}
	}

	return models
}

func appendModelIfMissing(models []string, modelName string) []string {
	if slices.ContainsFunc(models, func(item string) bool {
		return strings.EqualFold(item, modelName)
	}) {
		return models
	}

	return append(models, modelName)
}

func GetGroup(c *gin.Context) model.GroupCache {
	v, ok := c.MustGet(Group).(model.GroupCache)
	if !ok {
		panic(fmt.Sprintf("group cache type error: %T, %v", v, v))
	}

	return v
}

func GetToken(c *gin.Context) model.TokenCache {
	v, ok := c.MustGet(Token).(model.TokenCache)
	if !ok {
		panic(fmt.Sprintf("token cache type error: %T, %v", v, v))
	}

	return v
}

func GetAvailableSets(c *gin.Context) []string {
	v, ok := c.MustGet(AvailableSets).([]string)
	if !ok {
		panic(fmt.Sprintf("available sets type error: %T, %v", v, v))
	}

	return v
}

func GetAvailableModels(c *gin.Context) map[string][]string {
	v, ok := c.MustGet(AvailableModels).(map[string][]string)
	if !ok {
		panic(fmt.Sprintf("available models type error: %T, %v", v, v))
	}

	return v
}

func GetGroupChannelAvailableSets(c *gin.Context) []string {
	v, exists := c.Get(GroupChannelAvailableSets)
	if !exists {
		return GetAvailableSets(c)
	}

	sets, ok := v.([]string)
	if !ok {
		panic(fmt.Sprintf("group channel available sets type error: %T, %v", v, v))
	}

	return sets
}

func GetGroupChannelAvailableModels(c *gin.Context) map[string][]string {
	v, exists := c.Get(GroupChannelAvailableModels)
	if !exists {
		return GetAvailableModels(c)
	}

	models, ok := v.(map[string][]string)
	if !ok {
		panic(fmt.Sprintf("group channel available models type error: %T, %v", v, v))
	}

	return models
}

func GetActiveAvailableSets(c *gin.Context) []string {
	if GetGroupChannelMode(c) == GroupChannelModeOwn {
		return GetGroupChannelAvailableSets(c)
	}

	return GetAvailableSets(c)
}

func GetActiveAvailableModels(c *gin.Context) map[string][]string {
	if GetGroupChannelMode(c) == GroupChannelModeOwn {
		return GetGroupChannelAvailableModels(c)
	}

	return GetAvailableModels(c)
}

func GetActiveTokenModels(c *gin.Context) []string {
	token := GetToken(c)
	if GetGroupChannelMode(c) == GroupChannelModeOwn {
		return []string(token.GroupChannelModels)
	}

	return []string(token.Models)
}

func GetModelCaches(c *gin.Context) *model.ModelCaches {
	v, ok := c.MustGet(ModelCaches).(*model.ModelCaches)
	if !ok {
		panic(fmt.Sprintf("model caches type error: %T, %v", v, v))
	}

	return v
}

func GetGroupChannelMode(c *gin.Context) string {
	mode, _ := c.Get(GroupChannelMode)

	modeString, _ := mode.(string)
	if modeString == "" {
		return GroupChannelModeGlobal
	}

	return modeString
}

func SetLogFieldsFromMeta(m *meta.Meta, fields logrus.Fields) {
	SetLogServiceTier(fields, m.RequestServiceTier)
	SetLogPromptCacheKey(fields, m.PromptCacheKey)
	SetLogRequestUser(fields, m.User)

	SetLogRequestIDField(fields, m.RequestID)

	SetLogModeField(fields, m.Mode)
	SetLogModelFields(fields, m.OriginModel)
	SetLogActualModelFields(fields, m.ActualModel)

	SetLogGroupFields(fields, m.Group)
	SetLogTokenFields(fields, m.Token, false)
	SetLogChannelFields(fields, m.Channel)
}

func SetLogServiceTier(fields logrus.Fields, serviceTier string) {
	if serviceTier == "" {
		return
	}

	fields["service_tier"] = serviceTier
}

func SetLogPromptCacheKey(fields logrus.Fields, promptCacheKey string) {
	if promptCacheKey == "" {
		return
	}

	fields["prompt_cache_key"] = promptCacheKey
}

func SetLogRequestUser(fields logrus.Fields, user string) {
	if user == "" {
		return
	}

	fields["user"] = user
}

func SetLogModeField(fields logrus.Fields, mode mode.Mode) {
	fields["mode"] = mode.String()
}

func SetLogActualModelFields(fields logrus.Fields, actualModel string) {
	fields["actmodel"] = actualModel
}

func SetLogModelFields(fields logrus.Fields, model string) {
	fields["model"] = model
}

func SetLogChannelFields(fields logrus.Fields, channel meta.ChannelMeta) {
	if channel.ID > 0 {
		fields["chid"] = channel.ID
	}

	if channel.Scope != "" {
		fields["chscope"] = string(channel.Scope)
	}

	if channel.Scope == model.ChannelScopeGroup && channel.GroupID != "" {
		fields["chgroup"] = channel.GroupID
	}

	if channel.Name != "" {
		fields["chname"] = channel.Name
	}

	if channel.Type > 0 {
		fields["chtype"] = int(channel.Type)
		fields["chtype_name"] = channel.Type.String()
	}
}

func SetLogRequestIDField(fields logrus.Fields, requestID string) {
	fields["reqid"] = requestID
}

func SetLogGroupFields(fields logrus.Fields, group model.GroupCache) {
	if group.ID != "" {
		fields["gid"] = group.ID
	}
}

func SetLogTokenFields(fields logrus.Fields, token model.TokenCache, internal bool) {
	if token.ID > 0 {
		fields["kid"] = token.ID
	}

	if token.Name != "" {
		fields["kname"] = token.Name
	}

	if token.Key != "" {
		fields["key"] = maskTokenKey(token.Key)
	}

	if internal {
		fields["internal"] = "true"
	}
}

func maskTokenKey(key string) string {
	if len(key) <= 8 {
		return "*****"
	}
	return key[:4] + "*****" + key[len(key)-4:]
}
