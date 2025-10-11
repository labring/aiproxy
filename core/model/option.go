package model

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/common/notify"
	log "github.com/sirupsen/logrus"
)

type Option struct {
	Key   string `gorm:"size:64;primaryKey" json:"key"`
	Value string `gorm:"type:text"          json:"value"`
}

func GetAllOption() ([]*Option, error) {
	var options []*Option

	err := DB.Where("key IN (?)", optionKeys).Find(&options).Error
	return options, err
}

func GetOption(key string) (*Option, error) {
	if !slices.Contains(optionKeys, key) {
		return nil, ErrUnknownOptionKey
	}

	var option Option

	err := DB.Where("key = ?", key).First(&option).Error

	return &option, err
}

var (
	optionMap = make(map[string]string)
	// allowed option keys
	optionKeys []string
)

func InitOption2DB() error {
	err := initOptionMap()
	if err != nil {
		return err
	}

	err = loadOptionsFromDatabase(true)
	if err != nil {
		return err
	}

	return storeOptionMap()
}

func initOptionMap() error {
	optionMap["LogStorageHours"] = strconv.FormatInt(config.GetLogStorageHours(), 10)
	optionMap["RetryLogStorageHours"] = strconv.FormatInt(config.GetRetryLogStorageHours(), 10)
	optionMap["LogDetailStorageHours"] = strconv.FormatInt(config.GetLogDetailStorageHours(), 10)
	optionMap["CleanLogBatchSize"] = strconv.FormatInt(config.GetCleanLogBatchSize(), 10)
	optionMap["IPGroupsThreshold"] = strconv.FormatInt(config.GetIPGroupsThreshold(), 10)
	optionMap["IPGroupsBanThreshold"] = strconv.FormatInt(config.GetIPGroupsBanThreshold(), 10)
	optionMap["SaveAllLogDetail"] = strconv.FormatBool(config.GetSaveAllLogDetail())
	optionMap["LogDetailRequestBodyMaxSize"] = strconv.FormatInt(
		config.GetLogDetailRequestBodyMaxSize(),
		10,
	)
	optionMap["LogDetailResponseBodyMaxSize"] = strconv.FormatInt(
		config.GetLogDetailResponseBodyMaxSize(),
		10,
	)
	optionMap["DisableServe"] = strconv.FormatBool(config.GetDisableServe())
	optionMap["RetryTimes"] = strconv.FormatInt(config.GetRetryTimes(), 10)

	defaultChannelModelsJSON, err := sonic.Marshal(config.GetDefaultChannelModels())
	if err != nil {
		return err
	}

	optionMap["DefaultChannelModels"] = conv.BytesToString(defaultChannelModelsJSON)

	defaultChannelModelMappingJSON, err := sonic.Marshal(config.GetDefaultChannelModelMapping())
	if err != nil {
		return err
	}

	optionMap["DefaultChannelModelMapping"] = conv.BytesToString(defaultChannelModelMappingJSON)
	optionMap["GroupMaxTokenNum"] = strconv.FormatInt(config.GetGroupMaxTokenNum(), 10)

	groupConsumeLevelRatioJSON, err := sonic.Marshal(config.GetGroupConsumeLevelRatioStringKeyMap())
	if err != nil {
		return err
	}

	optionMap["GroupConsumeLevelRatio"] = conv.BytesToString(groupConsumeLevelRatioJSON)
	optionMap["NotifyNote"] = config.GetNotifyNote()
	optionMap["DefaultMCPHost"] = config.GetDefaultMCPHost()
	optionMap["PublicMCPHost"] = config.GetPublicMCPHost()
	optionMap["GroupMCPHost"] = config.GetGroupMCPHost()
	optionMap["DefaultWarnNotifyErrorRate"] = strconv.FormatFloat(
		config.GetDefaultWarnNotifyErrorRate(),
		'f',
		-1,
		64,
	)
	optionMap["UsageAlertThreshold"] = strconv.FormatInt(config.GetUsageAlertThreshold(), 10)

	usageAlertWhitelistJSON, err := sonic.Marshal(config.GetUsageAlertWhitelist())
	if err != nil {
		return err
	}

	optionMap["UsageAlertWhitelist"] = conv.BytesToString(usageAlertWhitelistJSON)
	optionMap["UsageAlertMinAvgThreshold"] = strconv.FormatInt(
		config.GetUsageAlertMinAvgThreshold(),
		10,
	)

	optionKeys = make([]string, 0, len(optionMap))
	for key := range optionMap {
		optionKeys = append(optionKeys, key)
	}

	return nil
}

func storeOptionMap() error {
	for key, value := range optionMap {
		err := saveOption(key, value)
		if err != nil {
			return err
		}
	}

	return nil
}

func loadOptionsFromDatabase(isInit bool) error {
	options, err := GetAllOption()
	if err != nil {
		return err
	}

	for _, option := range options {
		err := updateOption(option.Key, option.Value, isInit)
		if err != nil {
			if !errors.Is(err, ErrUnknownOptionKey) {
				return fmt.Errorf(
					"failed to update option: %s, value: %s, error: %w",
					option.Key,
					option.Value,
					err,
				)
			}

			if isInit {
				log.Warnf("unknown option: %s, value: %s", option.Key, option.Value)
			}

			continue
		}

		if isInit {
			delete(optionMap, option.Key)
		}
	}

	return nil
}

func SyncOptions(ctx context.Context, wg *sync.WaitGroup, frequency time.Duration) {
	defer wg.Done()

	ticker := time.NewTicker(frequency)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := loadOptionsFromDatabase(false); err != nil {
				notify.ErrorThrottle(
					"syncOptions",
					time.Minute*5,
					"failed to sync options",
					err.Error(),
				)
			}
		}
	}
}

func saveOption(key, value string) error {
	option := Option{
		Key:   key,
		Value: value,
	}
	result := DB.Save(&option)

	return HandleUpdateResult(result, "option:"+key)
}

func UpdateOption(key, value string) error {
	err := updateOption(key, value, false)
	if err != nil {
		return err
	}

	return saveOption(key, value)
}

func UpdateOptions(options map[string]string) error {
	errs := make([]error, 0)
	for key, value := range options {
		err := UpdateOption(key, value)
		if err != nil && !errors.Is(err, ErrUnknownOptionKey) {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

var ErrUnknownOptionKey = errors.New("unknown option key")

func toBool(value string) bool {
	result, _ := strconv.ParseBool(value)
	return result
}

//nolint:gocyclo
func updateOption(key, value string, isInit bool) (err error) {
	switch key {
	case "LogStorageHours":
		logStorageHours, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}

		config.SetLogStorageHours(logStorageHours)
	case "RetryLogStorageHours":
		retryLogStorageHours, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}

		config.SetRetryLogStorageHours(retryLogStorageHours)
	case "LogDetailStorageHours":
		logDetailStorageHours, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}

		config.SetLogDetailStorageHours(logDetailStorageHours)
	case "IPGroupsThreshold":
		ipGroupsThreshold, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}

		config.SetIPGroupsThreshold(ipGroupsThreshold)
	case "IPGroupsBanThreshold":
		ipGroupsBanThreshold, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}

		config.SetIPGroupsBanThreshold(ipGroupsBanThreshold)
	case "SaveAllLogDetail":
		config.SetSaveAllLogDetail(toBool(value))
	case "LogDetailRequestBodyMaxSize":
		logDetailRequestBodyMaxSize, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}

		config.SetLogDetailRequestBodyMaxSize(logDetailRequestBodyMaxSize)
	case "LogDetailResponseBodyMaxSize":
		logDetailResponseBodyMaxSize, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}

		config.SetLogDetailResponseBodyMaxSize(logDetailResponseBodyMaxSize)
	case "CleanLogBatchSize":
		cleanLogBatchSize, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}

		config.SetCleanLogBatchSize(cleanLogBatchSize)
	case "DisableServe":
		config.SetDisableServe(toBool(value))
	case "GroupMaxTokenNum":
		groupMaxTokenNum, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return err
		}

		if groupMaxTokenNum < 0 {
			return errors.New("group max token num must be greater than 0")
		}

		config.SetGroupMaxTokenNum(groupMaxTokenNum)
	case "DefaultChannelModels":
		var newModels map[int][]string

		err := sonic.Unmarshal(conv.StringToBytes(value), &newModels)
		if err != nil {
			return err
		}
		// check model config exist
		allModelsMap := make(map[string]struct{})
		for _, models := range newModels {
			for _, model := range models {
				allModelsMap[model] = struct{}{}
			}
		}

		allModels := make([]string, 0, len(allModelsMap))
		for model := range allModelsMap {
			allModels = append(allModels, model)
		}

		foundModels, missingModels, err := GetModelConfigWithModels(allModels)
		if err != nil {
			return err
		}

		if !isInit && len(missingModels) > 0 {
			sort.Strings(missingModels)
			return fmt.Errorf("model config not found: %v", missingModels)
		}

		if len(missingModels) > 0 {
			sort.Strings(missingModels)
			log.Errorf("model config not found: %v", missingModels)
		}

		allowedNewModels := make(map[int][]string)
		for t, ms := range newModels {
			for _, m := range ms {
				if slices.Contains(foundModels, m) {
					allowedNewModels[t] = append(allowedNewModels[t], m)
				}
			}
		}

		config.SetDefaultChannelModels(allowedNewModels)
	case "DefaultChannelModelMapping":
		var newMapping map[int]map[string]string

		err := sonic.Unmarshal(conv.StringToBytes(value), &newMapping)
		if err != nil {
			return err
		}

		config.SetDefaultChannelModelMapping(newMapping)
	case "RetryTimes":
		retryTimes, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return err
		}

		if retryTimes < 0 {
			return errors.New("retry times must be greater than 0")
		}

		config.SetRetryTimes(retryTimes)
	case "GroupConsumeLevelRatio":
		var newGroupRpmRatio map[string]float64

		err := sonic.Unmarshal(conv.StringToBytes(value), &newGroupRpmRatio)
		if err != nil {
			return err
		}

		newGroupRpmRatioMap := make(map[float64]float64)
		for k, v := range newGroupRpmRatio {
			consumeLevel, err := strconv.ParseFloat(k, 64)
			if err != nil {
				return err
			}

			if consumeLevel < 0 {
				return errors.New("consume level must be greater than 0")
			}

			if v < 0 {
				return errors.New("rpm ratio must be greater than 0")
			}

			newGroupRpmRatioMap[consumeLevel] = v
		}

		config.SetGroupConsumeLevelRatio(newGroupRpmRatioMap)
	case "NotifyNote":
		config.SetNotifyNote(value)
	case "DefaultMCPHost":
		config.SetDefaultMCPHost(value)
	case "PublicMCPHost":
		config.SetPublicMCPHost(value)
	case "GroupMCPHost":
		config.SetGroupMCPHost(value)
	case "DefaultWarnNotifyErrorRate":
		rate, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}

		config.SetDefaultWarnNotifyErrorRate(rate)
	case "UsageAlertThreshold":
		threshold, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}

		config.SetUsageAlertThreshold(threshold)
	case "UsageAlertWhitelist":
		var whitelist []string

		err := sonic.Unmarshal(conv.StringToBytes(value), &whitelist)
		if err != nil {
			return err
		}

		config.SetUsageAlertWhitelist(whitelist)
	case "UsageAlertMinAvgThreshold":
		threshold, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}

		config.SetUsageAlertMinAvgThreshold(threshold)
	default:
		return ErrUnknownOptionKey
	}

	return err
}
