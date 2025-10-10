package model

import (
	"time"
)

// GroupUsageAlertItem 用量告警项
type GroupUsageAlertItem struct {
	GroupID         string
	YesterdayAmount float64
	TodayAmount     float64
	Ratio           float64
}

// GetGroupUsageAlert 获取用量异常的用户
// 检测条件：
// 1. 上一天总消耗金额大于阈值
// 2. 当天用量大于上一天的两倍
// 3. 当天用量/上一天用量的比率大于2
// 4. 不在白名单中
func GetGroupUsageAlert(threshold float64, whitelist []string) ([]GroupUsageAlertItem, error) {
	now := time.Now()

	// 计算当天的时间范围（0点到当前时间）
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayEnd := now

	// 计算上一天的时间范围（昨天0点到昨天23:59:59）
	yesterdayStart := todayStart.Add(-24 * time.Hour)
	yesterdayEnd := todayStart.Add(-time.Second)

	// 查询上一天的用量数据
	type YesterdayUsage struct {
		GroupID    string
		UsedAmount float64
	}

	var yesterdayUsages []YesterdayUsage
	err := LogDB.
		Model(&GroupSummary{}).
		Select("group_id, SUM(used_amount) as used_amount").
		Where("hour_timestamp BETWEEN ? AND ?", yesterdayStart.Unix(), yesterdayEnd.Unix()).
		Group("group_id").
		Having("SUM(used_amount) > ?", threshold).
		Find(&yesterdayUsages).Error
	if err != nil {
		return nil, err
	}

	if len(yesterdayUsages) == 0 {
		return nil, nil
	}

	// 提取 group_id 列表
	groupIDs := make([]string, len(yesterdayUsages))
	yesterdayUsageMap := make(map[string]float64)
	for i, usage := range yesterdayUsages {
		groupIDs[i] = usage.GroupID
		yesterdayUsageMap[usage.GroupID] = usage.UsedAmount
	}

	// 查询当天的用量数据（只查询有上一天记录的用户）
	type TodayUsage struct {
		GroupID    string
		UsedAmount float64
	}

	var todayUsages []TodayUsage
	err = LogDB.
		Model(&GroupSummary{}).
		Select("group_id, SUM(used_amount) as used_amount").
		Where("group_id IN ?", groupIDs).
		Where("hour_timestamp BETWEEN ? AND ?", todayStart.Unix(), todayEnd.Unix()).
		Group("group_id").
		Find(&todayUsages).Error
	if err != nil {
		return nil, err
	}

	// 构建当天用量映射
	todayUsageMap := make(map[string]float64)
	for _, usage := range todayUsages {
		todayUsageMap[usage.GroupID] = usage.UsedAmount
	}

	// 构建白名单映射，用于快速查找
	whitelistMap := make(map[string]bool)
	for _, groupID := range whitelist {
		whitelistMap[groupID] = true
	}

	// 筛选出符合条件的用户
	var alerts []GroupUsageAlertItem
	for groupID, yesterdayAmount := range yesterdayUsageMap {
		// 跳过白名单中的用户
		if whitelistMap[groupID] {
			continue
		}

		todayAmount, ok := todayUsageMap[groupID]
		if !ok || todayAmount == 0 {
			continue
		}

		ratio := todayAmount / yesterdayAmount

		// 检查是否满足告警条件：
		// 1. 当天用量大于昨天的两倍
		// 2. 比率大于2
		if todayAmount > yesterdayAmount*2 && ratio > 2 {
			alerts = append(alerts, GroupUsageAlertItem{
				GroupID:         groupID,
				YesterdayAmount: yesterdayAmount,
				TodayAmount:     todayAmount,
				Ratio:           ratio,
			})
		}
	}

	return alerts, nil
}
