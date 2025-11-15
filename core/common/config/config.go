package config

import (
	"math"
	"slices"
	"strconv"
	"sync/atomic"

	"github.com/labring/aiproxy/core/common/env"
)

var (
	disableServe                 atomic.Bool
	logStorageHours              int64 // default 0 means no limit
	retryLogStorageHours         int64 // default 0 means no limit
	saveAllLogDetail             atomic.Bool
	logDetailRequestBodyMaxSize  int64 = 8 * 1024 // 8KB
	logDetailResponseBodyMaxSize int64 = 8 * 1024 // 8KB
	logDetailStorageHours        int64 = 3 * 24   // 3 days
	cleanLogBatchSize            int64 = 10000
	notifyNote                   atomic.Value
	ipGroupsThreshold            int64
	ipGroupsBanThreshold         int64
	retryTimes                   atomic.Int64
	defaultChannelModels         atomic.Value
	defaultChannelModelMapping   atomic.Value
	groupMaxTokenNum             atomic.Int64
	groupConsumeLevelRatio       atomic.Value
	usageAlertThreshold          atomic.Int64 // default 0 means disabled
	usageAlertWhitelist          atomic.Value
	usageAlertMinAvgThreshold    atomic.Int64 // 前三天平均用量最低阈值，default 0 means no limit

	defaultWarnNotifyErrorRate uint64 = math.Float64bits(0.5)

	defaultMCPHost atomic.Value
	publicMCPHost  atomic.Value
	groupMCPHost   atomic.Value

	// fuzzyTokenThreshold is the text length threshold for fuzzy token calculation.
	// If text length is below this threshold, precise token counting is used.
	// If text length is at or above this threshold, approximate counting (length/4) is used.
	// Set to 0 to always use precise counting (default behavior).
	fuzzyTokenThreshold atomic.Int64
)

func init() {
	defaultChannelModels.Store(make(map[int][]string))
	defaultChannelModelMapping.Store(make(map[int]map[string]string))
	groupConsumeLevelRatio.Store(make(map[float64]float64))
	usageAlertWhitelist.Store(make([]string, 0))
	notifyNote.Store("")
	defaultMCPHost.Store("")
	publicMCPHost.Store("")
	groupMCPHost.Store("")
}

func GetRetryTimes() int64 {
	return retryTimes.Load()
}

func SetRetryTimes(times int64) {
	times = env.Int64("RETRY_TIMES", times)
	retryTimes.Store(times)
}

func GetLogStorageHours() int64 {
	return atomic.LoadInt64(&logStorageHours)
}

func SetLogStorageHours(hours int64) {
	hours = env.Int64("LOG_STORAGE_HOURS", hours)
	atomic.StoreInt64(&logStorageHours, hours)
}

func GetRetryLogStorageHours() int64 {
	return atomic.LoadInt64(&retryLogStorageHours)
}

func SetRetryLogStorageHours(hours int64) {
	hours = env.Int64("RETRY_LOG_STORAGE_HOURS", hours)
	atomic.StoreInt64(&retryLogStorageHours, hours)
}

func GetLogDetailStorageHours() int64 {
	return atomic.LoadInt64(&logDetailStorageHours)
}

func SetLogDetailStorageHours(hours int64) {
	hours = env.Int64("LOG_DETAIL_STORAGE_HOURS", hours)
	atomic.StoreInt64(&logDetailStorageHours, hours)
}

func GetCleanLogBatchSize() int64 {
	return atomic.LoadInt64(&cleanLogBatchSize)
}

func SetCleanLogBatchSize(size int64) {
	size = env.Int64("CLEAN_LOG_BATCH_SIZE", size)
	atomic.StoreInt64(&cleanLogBatchSize, size)
}

func GetIPGroupsThreshold() int64 {
	return atomic.LoadInt64(&ipGroupsThreshold)
}

func SetIPGroupsThreshold(threshold int64) {
	threshold = env.Int64("IP_GROUPS_THRESHOLD", threshold)
	atomic.StoreInt64(&ipGroupsThreshold, threshold)
}

func GetIPGroupsBanThreshold() int64 {
	return atomic.LoadInt64(&ipGroupsBanThreshold)
}

func SetIPGroupsBanThreshold(threshold int64) {
	threshold = env.Int64("IP_GROUPS_BAN_THRESHOLD", threshold)
	atomic.StoreInt64(&ipGroupsBanThreshold, threshold)
}

func GetSaveAllLogDetail() bool {
	return saveAllLogDetail.Load()
}

func SetSaveAllLogDetail(enabled bool) {
	enabled = env.Bool("SAVE_ALL_LOG_DETAIL", enabled)
	saveAllLogDetail.Store(enabled)
}

func GetLogDetailRequestBodyMaxSize() int64 {
	return atomic.LoadInt64(&logDetailRequestBodyMaxSize)
}

func SetLogDetailRequestBodyMaxSize(size int64) {
	size = env.Int64("LOG_DETAIL_REQUEST_BODY_MAX_SIZE", size)
	atomic.StoreInt64(&logDetailRequestBodyMaxSize, size)
}

func GetLogDetailResponseBodyMaxSize() int64 {
	return atomic.LoadInt64(&logDetailResponseBodyMaxSize)
}

func SetLogDetailResponseBodyMaxSize(size int64) {
	size = env.Int64("LOG_DETAIL_RESPONSE_BODY_MAX_SIZE", size)
	atomic.StoreInt64(&logDetailResponseBodyMaxSize, size)
}

func GetDisableServe() bool {
	return disableServe.Load()
}

func SetDisableServe(disabled bool) {
	disabled = env.Bool("DISABLE_SERVE", disabled)
	disableServe.Store(disabled)
}

func GetDefaultChannelModels() map[int][]string {
	d, _ := defaultChannelModels.Load().(map[int][]string)
	return d
}

func SetDefaultChannelModels(models map[int][]string) {
	models = env.JSON("DEFAULT_CHANNEL_MODELS", models)
	for key, ms := range models {
		slices.Sort(ms)
		models[key] = slices.Compact(ms)
	}

	defaultChannelModels.Store(models)
}

func GetDefaultChannelModelMapping() map[int]map[string]string {
	d, _ := defaultChannelModelMapping.Load().(map[int]map[string]string)
	return d
}

func SetDefaultChannelModelMapping(mapping map[int]map[string]string) {
	mapping = env.JSON("DEFAULT_CHANNEL_MODEL_MAPPING", mapping)
	defaultChannelModelMapping.Store(mapping)
}

func GetGroupConsumeLevelRatio() map[float64]float64 {
	r, _ := groupConsumeLevelRatio.Load().(map[float64]float64)
	return r
}

func GetGroupConsumeLevelRatioStringKeyMap() map[string]float64 {
	ratio := GetGroupConsumeLevelRatio()

	stringMap := make(map[string]float64)
	for k, v := range ratio {
		stringMap[strconv.FormatFloat(k, 'f', -1, 64)] = v
	}

	return stringMap
}

func SetGroupConsumeLevelRatio(ratio map[float64]float64) {
	ratio = env.JSON("GROUP_CONSUME_LEVEL_RATIO", ratio)
	groupConsumeLevelRatio.Store(ratio)
}

// GetGroupMaxTokenNum returns max number of tokens per group, 0 means unlimited
func GetGroupMaxTokenNum() int64 {
	return groupMaxTokenNum.Load()
}

func SetGroupMaxTokenNum(num int64) {
	num = env.Int64("GROUP_MAX_TOKEN_NUM", num)
	groupMaxTokenNum.Store(num)
}

func GetNotifyNote() string {
	n, _ := notifyNote.Load().(string)
	return n
}

func SetNotifyNote(note string) {
	note = env.String("NOTIFY_NOTE", note)
	notifyNote.Store(note)
}

func GetDefaultMCPHost() string {
	h, _ := defaultMCPHost.Load().(string)
	return h
}

func SetDefaultMCPHost(host string) {
	host = env.String("DEFAULT_MCP_HOST", host)
	defaultMCPHost.Store(host)
}

func GetPublicMCPHost() string {
	h, _ := publicMCPHost.Load().(string)
	return h
}

func SetPublicMCPHost(host string) {
	host = env.String("PUBLIC_MCP_HOST", host)
	publicMCPHost.Store(host)
}

func GetGroupMCPHost() string {
	h, _ := groupMCPHost.Load().(string)
	return h
}

func SetGroupMCPHost(host string) {
	host = env.String("GROUP_MCP_HOST", host)
	groupMCPHost.Store(host)
}

func GetDefaultWarnNotifyErrorRate() float64 {
	return math.Float64frombits(atomic.LoadUint64(&defaultWarnNotifyErrorRate))
}

func SetDefaultWarnNotifyErrorRate(rate float64) {
	rate = env.Float64("DEFAULT_WARN_NOTIFY_ERROR_RATE", rate)
	atomic.StoreUint64(&defaultWarnNotifyErrorRate, math.Float64bits(rate))
}

func GetUsageAlertThreshold() int64 {
	return usageAlertThreshold.Load()
}

func SetUsageAlertThreshold(threshold int64) {
	threshold = env.Int64("USAGE_ALERT_THRESHOLD", threshold)
	usageAlertThreshold.Store(threshold)
}

func GetUsageAlertWhitelist() []string {
	w, _ := usageAlertWhitelist.Load().([]string)
	return w
}

func SetUsageAlertWhitelist(whitelist []string) {
	whitelist = env.JSON("USAGE_ALERT_WHITELIST", whitelist)
	usageAlertWhitelist.Store(whitelist)
}

func GetUsageAlertMinAvgThreshold() int64 {
	return usageAlertMinAvgThreshold.Load()
}

func SetUsageAlertMinAvgThreshold(threshold int64) {
	threshold = env.Int64("USAGE_ALERT_MIN_AVG_THRESHOLD", threshold)
	usageAlertMinAvgThreshold.Store(threshold)
}

func GetFuzzyTokenThreshold() int64 {
	return fuzzyTokenThreshold.Load()
}

func SetFuzzyTokenThreshold(threshold int64) {
	threshold = env.Int64("FUZZY_TOKEN_THRESHOLD", threshold)
	fuzzyTokenThreshold.Store(threshold)
}
