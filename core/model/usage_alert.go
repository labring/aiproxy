package model

import (
	"time"
)

// GroupUsageAlertItem 用量告警项
type GroupUsageAlertItem struct {
	GroupID           string
	ThreeDayAvgAmount float64 // 前三天的平均用量
	TodayAmount       float64
	Ratio             float64
}

// calculateSpikeThreshold 根据前三天平均用量计算动态告警倍率
// 用量越大，倍率越低，避免误报；用量越小，倍率越高，捕获异常
func calculateSpikeThreshold(avgAmount float64) float64 {
	switch {
	case avgAmount < 100:
		return 5.0
	case avgAmount < 300:
		return 4.0
	case avgAmount < 1000:
		return 3.0
	case avgAmount < 2000:
		return 2.5
	case avgAmount < 5000:
		return 2.0
	default:
		return 1.5
	}
}

// GetGroupUsageAlert 获取用量突升异常的用户
// 新的检测逻辑：
// 1. 基准阈值：当天用量必须 >= threshold（如 100）才开始检测
// 2. 动态倍率：根据前三天平均用量分段计算告警倍率
//   - 前三天平均用量 < 100：倍率 5.0（小用户突增 5 倍很异常）
//   - 前三天平均用量 [100, 300)：倍率 4.0
//   - 前三天平均用量 [300, 1000)：倍率 3.0
//   - 前三天平均用量 [1000, 2000)：倍率 2.5
//   - 前三天平均用量 [2000, 5000)：倍率 2.0
//   - 前三天平均用量 >= 5000：倍率 1.5（大用户增长 1.5 倍就值得关注）
//
// 3. 前三天平均用量必须 >= minAvgThreshold（如 3）才进行检测
// 4. 不在白名单中
func GetGroupUsageAlert(
	threshold, minAvgThreshold float64,
	whitelist []string,
) ([]GroupUsageAlertItem, error) {
	now := time.Now()

	// 计算当天的时间范围（0点到当前时间）
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayEnd := now

	// 计算前三天的时间范围（从3天前的0点到昨天的23:59:59）
	threeDaysAgoStart := todayStart.Add(-72 * time.Hour) // 3天前的0点
	yesterdayEnd := todayStart.Add(-time.Second)         // 昨天的23:59:59

	// 查询当天用量达到阈值的用户（第一步筛选）
	type TodayUsage struct {
		GroupID    string
		UsedAmount float64
	}

	var todayUsages []TodayUsage

	err := LogDB.
		Model(&GroupSummary{}).
		Select("group_id, SUM(used_amount) as used_amount").
		Where("hour_timestamp BETWEEN ? AND ?", todayStart.Unix(), todayEnd.Unix()).
		Group("group_id").
		Having("SUM(used_amount) >= ?", threshold).
		Find(&todayUsages).Error
	if err != nil {
		return nil, err
	}

	if len(todayUsages) == 0 {
		return nil, nil
	}

	// 提取 group_id 列表
	groupIDs := make([]string, len(todayUsages))

	todayUsageMap := make(map[string]float64)
	for i, usage := range todayUsages {
		groupIDs[i] = usage.GroupID
		todayUsageMap[usage.GroupID] = usage.UsedAmount
	}

	// 查询这些用户的前三天用量
	type ThreeDayUsage struct {
		GroupID    string
		UsedAmount float64
	}

	var threeDayUsages []ThreeDayUsage

	err = LogDB.
		Model(&GroupSummary{}).
		Select("group_id, SUM(used_amount) as used_amount").
		Where("group_id IN ?", groupIDs).
		Where("hour_timestamp BETWEEN ? AND ?", threeDaysAgoStart.Unix(), yesterdayEnd.Unix()).
		Group("group_id").
		Find(&threeDayUsages).Error
	if err != nil {
		return nil, err
	}

	// 构建前三天平均用量映射
	threeDayAvgUsageMap := make(map[string]float64)
	for _, usage := range threeDayUsages {
		// 计算平均值：总用量除以3
		threeDayAvgUsageMap[usage.GroupID] = usage.UsedAmount / 3.0
	}

	// 构建白名单映射，用于快速查找
	whitelistMap := make(map[string]bool)
	for _, groupID := range whitelist {
		whitelistMap[groupID] = true
	}

	// 筛选出符合条件的用户
	var alerts []GroupUsageAlertItem
	for groupID, todayAmount := range todayUsageMap {
		// 跳过白名单中的用户
		if whitelistMap[groupID] {
			continue
		}

		// 获取前三天平均用量（如果没有前三天的数据，默认为 0）
		threeDayAvgAmount := threeDayAvgUsageMap[groupID]

		// 过滤掉前三天平均用量低于阈值的用户
		if threeDayAvgAmount <= minAvgThreshold || threeDayAvgAmount == 0 {
			continue
		}

		// 计算实际比率
		ratio := todayAmount / threeDayAvgAmount

		// 根据前三天平均用量计算动态告警倍率
		requiredRatio := calculateSpikeThreshold(threeDayAvgAmount)

		// 检查是否满足告警条件
		if ratio >= requiredRatio {
			alerts = append(alerts, GroupUsageAlertItem{
				GroupID:           groupID,
				ThreeDayAvgAmount: threeDayAvgAmount,
				TodayAmount:       todayAmount,
				Ratio:             ratio,
			})
		}
	}

	return alerts, nil
}
