package train12306

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
)

// extractPrices extracts price information from ticket data
func (s *Server) extractPrices(
	ypInfo, seatDiscountInfo string,
	ticketData TicketData,
) []Price {
	prices := make([]Price, 0, len(ypInfo)/PriceStrLength)
	discounts := make(map[string]int)

	// Parse discounts
	for i := range len(seatDiscountInfo) / DiscountStrLength {
		discountStr := seatDiscountInfo[i*DiscountStrLength : (i+1)*DiscountStrLength]
		if len(discountStr) >= 5 {
			discount, err := strconv.Atoi(discountStr[1:])
			if err == nil {
				discounts[string(discountStr[0])] = discount
			}
		}
	}

	// Parse prices
	for i := range len(ypInfo) / PriceStrLength {
		priceStr := ypInfo[i*PriceStrLength : (i+1)*PriceStrLength]
		if len(priceStr) < PriceStrLength {
			continue
		}

		var seatTypeCode string

		priceValue, err := strconv.Atoi(priceStr[6:10])
		if err != nil {
			continue
		}

		if priceValue >= 3000 {
			seatTypeCode = "W" // 无座
		} else if _, exists := SeatTypes[string(priceStr[0])]; exists {
			seatTypeCode = string(priceStr[0])
		} else {
			seatTypeCode = "H" // 其他
		}

		seatType := SeatTypes[seatTypeCode]

		price, err := strconv.ParseFloat(priceStr[1:6], 64)
		if err != nil {
			continue
		}

		price /= 10

		var discount *int
		if d, exists := discounts[seatTypeCode]; exists {
			discount = &d
		}

		// Get seat number based on seat type
		var num string
		switch seatType.Short {
		case "swz":
			num = ticketData.SwzNum
		case "tz":
			num = ticketData.TzNum
		case "zy":
			num = ticketData.ZyNum
		case "ze":
			num = ticketData.ZeNum
		case "gr":
			num = ticketData.GrNum
		case "srrb":
			num = ticketData.SrrbNum
		case "rw":
			num = ticketData.RwNum
		case "yw":
			num = ticketData.YwNum
		case "rz":
			num = ticketData.RzNum
		case "yz":
			num = ticketData.YzNum
		case "wz":
			num = ticketData.WzNum
		case "qt":
			num = ticketData.QtNum
		default:
			num = ""
		}

		prices = append(prices, Price{
			SeatName:     seatType.Name,
			Short:        seatType.Short,
			SeatTypeCode: seatTypeCode,
			Num:          num,
			Price:        price,
			Discount:     discount,
		})
	}

	return prices
}

// extractDWFlags extracts DW flags from the flag string
func (s *Server) extractDWFlags(dwFlagStr string) []string {
	dwFlagList := strings.Split(dwFlagStr, "#")

	var result []string

	if len(dwFlagList) > 0 && dwFlagList[0] == "5" {
		result = append(result, DWFlags[0]) // 智能动车组
	}

	if len(dwFlagList) > 1 && dwFlagList[1] == "1" {
		result = append(result, DWFlags[1]) // 复兴号
	}

	if len(dwFlagList) > 2 {
		if strings.HasPrefix(dwFlagList[2], "Q") {
			result = append(result, DWFlags[2]) // 静音车厢
		} else if strings.HasPrefix(dwFlagList[2], "R") {
			result = append(result, DWFlags[3]) // 温馨动卧
		}
	}

	if len(dwFlagList) > 5 && dwFlagList[5] == "D" {
		result = append(result, DWFlags[4]) // 动感号
	}

	if len(dwFlagList) > 6 && dwFlagList[6] != "z" {
		result = append(result, DWFlags[5]) // 支持选铺
	}

	if len(dwFlagList) > 7 && dwFlagList[7] != "z" {
		result = append(result, DWFlags[6]) // 老年优惠
	}

	return result
}

// formatTicketStatus formats ticket availability status
func (s *Server) formatTicketStatus(num string) string {
	// Check if it's a pure number
	if matched, _ := regexp.MatchString(`^\d+$`, num); matched {
		count, _ := strconv.Atoi(num)
		if count == 0 {
			return "无票"
		}

		return fmt.Sprintf("剩余%d张票", count)
	}

	// Handle special status strings
	switch num {
	case "有", "充足":
		return "有票"
	case "无", "--", "":
		return "无票"
	case "候补":
		return "无票需候补"
	default:
		return num + "票"
	}
}

// formatTicketsInfo formats ticket information for display
func (s *Server) formatTicketsInfo(ticketsInfo []TicketInfo) string {
	if len(ticketsInfo) == 0 {
		return "没有查询到相关车次信息"
	}

	var result strings.Builder
	result.WriteString("车次 | 出发站 -> 到达站 | 出发时间 -> 到达时间 | 历时\n")

	for _, ticketInfo := range ticketsInfo {
		var infoStr strings.Builder
		infoStr.WriteString(fmt.Sprintf(
			"%s(实际车次train_no: %s) %s(telecode: %s) -> %s(telecode: %s) %s -> %s 历时：%s",
			ticketInfo.StartTrainCode,
			ticketInfo.TrainNo,
			ticketInfo.FromStation,
			ticketInfo.FromStationTelecode,
			ticketInfo.ToStation,
			ticketInfo.ToStationTelecode,
			ticketInfo.StartTime,
			ticketInfo.ArriveTime,
			ticketInfo.Lishi,
		))

		for _, price := range ticketInfo.Prices {
			ticketStatus := s.formatTicketStatus(price.Num)
			infoStr.WriteString(
				fmt.Sprintf("\n- %s: %s %.1f元", price.SeatName, ticketStatus, price.Price),
			)
		}

		result.WriteString(infoStr.String() + "\n")
	}

	return result.String()
}

// filterTicketsInfo filters and sorts ticket information
func (s *Server) filterTicketsInfo(
	ticketsInfo []TicketInfo,
	trainFilterFlags, sortFlag string,
	sortReverse bool,
	limitedNum int,
) []TicketInfo {
	var result []TicketInfo

	// Apply train filters
	if trainFilterFlags == "" {
		result = ticketsInfo
	} else {
		for _, ticketInfo := range ticketsInfo {
			for _, filter := range trainFilterFlags {
				if s.matchesTrainFilter(ticketInfo, string(filter)) {
					result = append(result, ticketInfo)
					break
				}
			}
		}
	}

	// Apply sorting
	if sortFlag != "" {
		s.sortTicketsInfo(result, sortFlag, sortReverse)
	}

	// Apply limit
	if limitedNum > 0 && len(result) > limitedNum {
		result = result[:limitedNum]
	}

	return result
}

// matchesTrainFilter checks if a ticket matches the train filter
func (s *Server) matchesTrainFilter(ticketInfo TicketInfo, filter string) bool {
	switch filter {
	case "G":
		return strings.HasPrefix(ticketInfo.StartTrainCode, "G") ||
			strings.HasPrefix(ticketInfo.StartTrainCode, "C")
	case "D":
		return strings.HasPrefix(ticketInfo.StartTrainCode, "D")
	case "Z":
		return strings.HasPrefix(ticketInfo.StartTrainCode, "Z")
	case "T":
		return strings.HasPrefix(ticketInfo.StartTrainCode, "T")
	case "K":
		return strings.HasPrefix(ticketInfo.StartTrainCode, "K")
	case "O":
		return !s.matchesTrainFilter(ticketInfo, "G") &&
			!s.matchesTrainFilter(ticketInfo, "D") &&
			!s.matchesTrainFilter(ticketInfo, "Z") &&
			!s.matchesTrainFilter(ticketInfo, "T") &&
			!s.matchesTrainFilter(ticketInfo, "K")
	case "F":
		return s.containsFlag(ticketInfo.DwFlag, "复兴号")
	case "S":
		return s.containsFlag(ticketInfo.DwFlag, "智能动车组")
	}

	return false
}

// containsFlag checks if the flag list contains a specific flag
func (s *Server) containsFlag(flags []string, flag string) bool {
	return slices.Contains(flags, flag)
}

// sortTicketsInfo sorts ticket information
func (s *Server) sortTicketsInfo(
	ticketsInfo []TicketInfo,
	sortFlag string,
	sortReverse bool,
) {
	switch sortFlag {
	case "startTime":
		sort.Slice(ticketsInfo, func(i, j int) bool {
			result := s.compareStartTime(ticketsInfo[i], ticketsInfo[j])
			if sortReverse {
				return result > 0
			}

			return result < 0
		})
	case "arriveTime":
		sort.Slice(ticketsInfo, func(i, j int) bool {
			result := s.compareArriveTime(ticketsInfo[i], ticketsInfo[j])
			if sortReverse {
				return result > 0
			}

			return result < 0
		})
	case "duration":
		sort.Slice(ticketsInfo, func(i, j int) bool {
			result := s.compareDuration(ticketsInfo[i], ticketsInfo[j])
			if sortReverse {
				return result > 0
			}

			return result < 0
		})
	}
}

// compareStartTime compares start times of two tickets
func (s *Server) compareStartTime(a, b TicketInfo) int {
	dateA, _ := time.Parse(time.DateOnly, a.StartDate)
	dateB, _ := time.Parse(time.DateOnly, b.StartDate)

	if dateA.Unix() != dateB.Unix() {
		return int(dateA.Unix() - dateB.Unix())
	}

	timePartsA := strings.Split(a.StartTime, ":")
	timePartsB := strings.Split(b.StartTime, ":")

	hourA, _ := strconv.Atoi(timePartsA[0])
	hourB, _ := strconv.Atoi(timePartsB[0])

	if hourA != hourB {
		return hourA - hourB
	}

	minuteA, _ := strconv.Atoi(timePartsA[1])
	minuteB, _ := strconv.Atoi(timePartsB[1])

	return minuteA - minuteB
}

// compareArriveTime compares arrive times of two tickets
func (s *Server) compareArriveTime(a, b TicketInfo) int {
	dateA, _ := time.Parse(time.DateOnly, a.ArriveDate)
	dateB, _ := time.Parse(time.DateOnly, b.ArriveDate)

	if dateA.Unix() != dateB.Unix() {
		return int(dateA.Unix() - dateB.Unix())
	}

	timePartsA := strings.Split(a.ArriveTime, ":")
	timePartsB := strings.Split(b.ArriveTime, ":")

	hourA, _ := strconv.Atoi(timePartsA[0])
	hourB, _ := strconv.Atoi(timePartsB[0])

	if hourA != hourB {
		return hourA - hourB
	}

	minuteA, _ := strconv.Atoi(timePartsA[1])
	minuteB, _ := strconv.Atoi(timePartsB[1])

	return minuteA - minuteB
}

// compareDuration compares durations of two tickets
func (s *Server) compareDuration(a, b TicketInfo) int {
	timePartsA := strings.Split(a.Lishi, ":")
	timePartsB := strings.Split(b.Lishi, ":")

	hourA, _ := strconv.Atoi(timePartsA[0])
	hourB, _ := strconv.Atoi(timePartsB[0])

	if hourA != hourB {
		return hourA - hourB
	}

	minuteA, _ := strconv.Atoi(timePartsA[1])
	minuteB, _ := strconv.Atoi(timePartsB[1])

	return minuteA - minuteB
}

// parseRouteStationsData parses route station data from API response
func (s *Server) parseRouteStationsData(rawData []any) []RouteStationData {
	result := make([]RouteStationData, 0, len(rawData))

	for _, item := range rawData {
		dataBytes, err := sonic.Marshal(item)
		if err != nil {
			continue
		}

		var routeStation RouteStationData
		if err := sonic.Unmarshal(dataBytes, &routeStation); err != nil {
			continue
		}

		result = append(result, routeStation)
	}

	return result
}

// parseRouteStationsInfo converts RouteStationData to RouteStationInfo
func (s *Server) parseRouteStationsInfo(
	routeStationsData []RouteStationData,
) []RouteStationInfo {
	result := make([]RouteStationInfo, 0, len(routeStationsData))

	for i, routeStationData := range routeStationsData {
		stationNo, _ := strconv.Atoi(routeStationData.StationNo)

		var arriveTime string
		if i == 0 {
			arriveTime = routeStationData.StartTime
		} else {
			arriveTime = routeStationData.ArriveTime
		}

		result = append(result, RouteStationInfo{
			ArriveTime:   arriveTime,
			StationName:  routeStationData.StationName,
			StopoverTime: routeStationData.StopoverTime,
			StationNo:    stationNo,
		})
	}

	return result
}

// parseInterlineData parses interline data from API response
func (s *Server) parseInterlineData(rawData []any) []InterlineData {
	result := make([]InterlineData, 0, len(rawData))

	for _, item := range rawData {
		dataBytes, err := sonic.Marshal(item)
		if err != nil {
			continue
		}

		var interlineData InterlineData
		if err := sonic.Unmarshal(dataBytes, &interlineData); err != nil {
			continue
		}

		result = append(result, interlineData)
	}

	return result
}

// parseInterlinesInfo converts InterlineData to InterlineInfo
func (s *Server) parseInterlinesInfo(interlineData []InterlineData) []InterlineInfo {
	result := make([]InterlineInfo, 0, len(interlineData))

	for _, ticket := range interlineData {
		interlineTickets := s.parseInterlinesTicketInfo(ticket.FullList)
		lishi := s.extractLishi(ticket.AllLishi)

		var startTrainCode string
		if len(interlineTickets) > 0 {
			startTrainCode = interlineTickets[0].StartTrainCode
		}

		result = append(result, InterlineInfo{
			Lishi:             lishi,
			StartTime:         ticket.StartTime,
			StartDate:         ticket.TrainDate,
			MiddleDate:        ticket.MiddleDate,
			ArriveDate:        ticket.ArriveDate,
			ArriveTime:        ticket.ArriveTime,
			FromStationCode:   ticket.FromStationCode,
			FromStationName:   ticket.FromStationName,
			MiddleStationCode: ticket.MiddleStationCode,
			MiddleStationName: ticket.MiddleStationName,
			EndStationCode:    ticket.EndStationCode,
			EndStationName:    ticket.EndStationName,
			StartTrainCode:    startTrainCode,
			FirstTrainNo:      ticket.FirstTrainNo,
			SecondTrainNo:     ticket.SecondTrainNo,
			TrainCount:        ticket.TrainCount,
			TicketList:        interlineTickets,
			SameStation:       ticket.SameStation == "0",
			SameTrain:         ticket.SameTrain == "Y",
			WaitTime:          ticket.WaitTime,
		})
	}

	return result
}

// parseInterlinesTicketInfo converts InterlineTicketData to TicketInfo
func (s *Server) parseInterlinesTicketInfo(
	interlineTicketsData []InterlineTicketData,
) []TicketInfo {
	result := make([]TicketInfo, 0, len(interlineTicketsData))

	for _, interlineTicketData := range interlineTicketsData {
		prices := s.extractInterlinePrices(
			interlineTicketData.YpInfo,
			interlineTicketData.SeatDiscountInfo,
			interlineTicketData,
		)

		// Parse start time
		startTime, err := time.Parse("15:04", interlineTicketData.StartTime)
		if err != nil {
			continue
		}

		// Parse duration
		lishiParts := strings.Split(interlineTicketData.Lishi, ":")
		if len(lishiParts) != 2 {
			continue
		}

		durationHours, _ := strconv.Atoi(lishiParts[0])
		durationMinutes, _ := strconv.Atoi(lishiParts[1])

		// Parse start date
		startDate, err := time.Parse("20060102", interlineTicketData.StartTrainDate)
		if err != nil {
			continue
		}

		// Calculate start and arrive datetime
		startDateTime := time.Date(startDate.Year(), startDate.Month(), startDate.Day(),
			startTime.Hour(), startTime.Minute(), 0, 0, time.UTC)
		arriveDateTime := startDateTime.Add(
			time.Duration(durationHours)*time.Hour + time.Duration(durationMinutes)*time.Minute,
		)

		result = append(result, TicketInfo{
			TrainNo:             interlineTicketData.TrainNo,
			StartTrainCode:      interlineTicketData.StationTrainCode,
			StartDate:           startDateTime.Format(time.DateOnly),
			ArriveDate:          arriveDateTime.Format(time.DateOnly),
			StartTime:           interlineTicketData.StartTime,
			ArriveTime:          interlineTicketData.ArriveTime,
			Lishi:               interlineTicketData.Lishi,
			FromStation:         interlineTicketData.FromStationName,
			ToStation:           interlineTicketData.ToStationName,
			FromStationTelecode: interlineTicketData.FromStationTelecode,
			ToStationTelecode:   interlineTicketData.ToStationTelecode,
			Prices:              prices,
			DwFlag:              s.extractDWFlags(interlineTicketData.DwFlag),
		})
	}

	return result
}

// extractInterlinePrices extracts price information from interline ticket data
func (s *Server) extractInterlinePrices(
	ypInfo, seatDiscountInfo string,
	ticketData InterlineTicketData,
) []Price {
	prices := make([]Price, 0, len(ypInfo)/PriceStrLength)
	discounts := make(map[string]int)

	// Parse discounts
	for i := range len(seatDiscountInfo) / DiscountStrLength {
		discountStr := seatDiscountInfo[i*DiscountStrLength : (i+1)*DiscountStrLength]
		if len(discountStr) >= 5 {
			discount, err := strconv.Atoi(discountStr[1:])
			if err == nil {
				discounts[string(discountStr[0])] = discount
			}
		}
	}

	// Parse prices
	for i := range len(ypInfo) / PriceStrLength {
		priceStr := ypInfo[i*PriceStrLength : (i+1)*PriceStrLength]
		if len(priceStr) < PriceStrLength {
			continue
		}

		var seatTypeCode string

		priceValue, err := strconv.Atoi(priceStr[6:10])
		if err != nil {
			continue
		}

		if priceValue >= 3000 {
			seatTypeCode = "W" // 无座
		} else if _, exists := SeatTypes[string(priceStr[0])]; exists {
			seatTypeCode = string(priceStr[0])
		} else {
			seatTypeCode = "H" // 其他
		}

		seatType := SeatTypes[seatTypeCode]

		price, err := strconv.ParseFloat(priceStr[1:6], 64)
		if err != nil {
			continue
		}

		price /= 10

		var discount *int
		if d, exists := discounts[seatTypeCode]; exists {
			discount = &d
		}

		// Get seat number based on seat type
		var num string
		switch seatType.Short {
		case "swz":
			num = ticketData.SwzNum
		case "tz":
			num = ticketData.TzNum
		case "zy":
			num = ticketData.ZyNum
		case "ze":
			num = ticketData.ZeNum
		case "gr":
			num = ticketData.GrNum
		case "srrb":
			num = ticketData.SrrbNum
		case "rw":
			num = ticketData.RwNum
		case "yw":
			num = ticketData.YwNum
		case "rz":
			num = ticketData.RzNum
		case "yz":
			num = ticketData.YzNum
		case "wz":
			num = ticketData.WzNum
		case "qt":
			num = ticketData.QtNum
		default:
			num = ""
		}

		prices = append(prices, Price{
			SeatName:     seatType.Name,
			Short:        seatType.Short,
			SeatTypeCode: seatTypeCode,
			Num:          num,
			Price:        price,
			Discount:     discount,
		})
	}

	return prices
}

// extractLishi extracts duration in hh:mm format from Chinese format
func (s *Server) extractLishi(allLishi string) string {
	re := regexp.MustCompile(`(?:(\d+)小时)?(\d+)分钟`)

	matches := re.FindStringSubmatch(allLishi)
	if len(matches) < 3 {
		return "00:00"
	}

	hours := "00"
	if matches[1] != "" {
		h, _ := strconv.Atoi(matches[1])
		hours = fmt.Sprintf("%02d", h)
	}

	minutes := "00"
	if matches[2] != "" {
		m, _ := strconv.Atoi(matches[2])
		minutes = fmt.Sprintf("%02d", m)
	}

	return fmt.Sprintf("%s:%s", hours, minutes)
}

// filterInterlineInfo filters and sorts interline information
func (s *Server) filterInterlineInfo(
	interlinesInfo []InterlineInfo,
	trainFilterFlags, sortFlag string,
	sortReverse bool,
	limitedNum int,
) []InterlineInfo {
	result := make([]InterlineInfo, 0, len(interlinesInfo))

	// Apply train filters
	if trainFilterFlags == "" {
		result = interlinesInfo
	} else {
		for _, interlineInfo := range interlinesInfo {
			for _, filter := range trainFilterFlags {
				if s.matchesInterlineTrainFilter(interlineInfo, string(filter)) {
					result = append(result, interlineInfo)
					break
				}
			}
		}
	}

	// Apply sorting
	if sortFlag != "" {
		s.sortInterlineInfo(result, sortFlag, sortReverse)
	}

	// Apply limit
	if limitedNum > 0 && len(result) > limitedNum {
		result = result[:limitedNum]
	}

	return result
}

// matchesInterlineTrainFilter checks if an interline ticket matches the train filter
func (s *Server) matchesInterlineTrainFilter(
	interlineInfo InterlineInfo,
	filter string,
) bool {
	switch filter {
	case "G":
		return strings.HasPrefix(interlineInfo.StartTrainCode, "G") ||
			strings.HasPrefix(interlineInfo.StartTrainCode, "C")
	case "D":
		return strings.HasPrefix(interlineInfo.StartTrainCode, "D")
	case "Z":
		return strings.HasPrefix(interlineInfo.StartTrainCode, "Z")
	case "T":
		return strings.HasPrefix(interlineInfo.StartTrainCode, "T")
	case "K":
		return strings.HasPrefix(interlineInfo.StartTrainCode, "K")
	case "O":
		return !s.matchesInterlineTrainFilter(interlineInfo, "G") &&
			!s.matchesInterlineTrainFilter(interlineInfo, "D") &&
			!s.matchesInterlineTrainFilter(interlineInfo, "Z") &&
			!s.matchesInterlineTrainFilter(interlineInfo, "T") &&
			!s.matchesInterlineTrainFilter(interlineInfo, "K")
	case "F":
		return len(interlineInfo.TicketList) > 0 &&
			s.containsFlag(interlineInfo.TicketList[0].DwFlag, "复兴号")
	case "S":
		return len(interlineInfo.TicketList) > 0 &&
			s.containsFlag(interlineInfo.TicketList[0].DwFlag, "智能动车组")
	}

	return false
}

// sortInterlineInfo sorts interline information
func (s *Server) sortInterlineInfo(
	interlinesInfo []InterlineInfo,
	sortFlag string,
	sortReverse bool,
) {
	switch sortFlag {
	case "startTime":
		sort.Slice(interlinesInfo, func(i, j int) bool {
			result := s.compareInterlineStartTime(interlinesInfo[i], interlinesInfo[j])
			if sortReverse {
				return result > 0
			}

			return result < 0
		})
	case "arriveTime":
		sort.Slice(interlinesInfo, func(i, j int) bool {
			result := s.compareInterlineArriveTime(interlinesInfo[i], interlinesInfo[j])
			if sortReverse {
				return result > 0
			}

			return result < 0
		})
	case "duration":
		sort.Slice(interlinesInfo, func(i, j int) bool {
			result := s.compareInterlineDuration(interlinesInfo[i], interlinesInfo[j])
			if sortReverse {
				return result > 0
			}

			return result < 0
		})
	}
}

// compareInterlineStartTime compares start times of two interline tickets
func (s *Server) compareInterlineStartTime(a, b InterlineInfo) int {
	dateA, _ := time.Parse(time.DateOnly, a.StartDate)
	dateB, _ := time.Parse(time.DateOnly, b.StartDate)

	if dateA.Unix() != dateB.Unix() {
		return int(dateA.Unix() - dateB.Unix())
	}

	timePartsA := strings.Split(a.StartTime, ":")
	timePartsB := strings.Split(b.StartTime, ":")

	hourA, _ := strconv.Atoi(timePartsA[0])
	hourB, _ := strconv.Atoi(timePartsB[0])

	if hourA != hourB {
		return hourA - hourB
	}

	minuteA, _ := strconv.Atoi(timePartsA[1])
	minuteB, _ := strconv.Atoi(timePartsB[1])

	return minuteA - minuteB
}

// compareInterlineArriveTime compares arrive times of two interline tickets
func (s *Server) compareInterlineArriveTime(a, b InterlineInfo) int {
	dateA, _ := time.Parse(time.DateOnly, a.ArriveDate)
	dateB, _ := time.Parse(time.DateOnly, b.ArriveDate)

	if dateA.Unix() != dateB.Unix() {
		return int(dateA.Unix() - dateB.Unix())
	}

	timePartsA := strings.Split(a.ArriveTime, ":")
	timePartsB := strings.Split(b.ArriveTime, ":")

	hourA, _ := strconv.Atoi(timePartsA[0])
	hourB, _ := strconv.Atoi(timePartsB[0])

	if hourA != hourB {
		return hourA - hourB
	}

	minuteA, _ := strconv.Atoi(timePartsA[1])
	minuteB, _ := strconv.Atoi(timePartsB[1])

	return minuteA - minuteB
}

// compareInterlineDuration compares durations of two interline tickets
func (s *Server) compareInterlineDuration(a, b InterlineInfo) int {
	timePartsA := strings.Split(a.Lishi, ":")
	timePartsB := strings.Split(b.Lishi, ":")

	hourA, _ := strconv.Atoi(timePartsA[0])
	hourB, _ := strconv.Atoi(timePartsB[0])

	if hourA != hourB {
		return hourA - hourB
	}

	minuteA, _ := strconv.Atoi(timePartsA[1])
	minuteB, _ := strconv.Atoi(timePartsB[1])

	return minuteA - minuteB
}

// formatInterlinesInfo formats interline information for display
func (s *Server) formatInterlinesInfo(interlinesInfo []InterlineInfo) string {
	var result strings.Builder
	result.WriteString("出发时间 -> 到达时间 | 出发车站 -> 中转车站 -> 到达车站 | 换乘标志 |换乘等待时间| 总历时\n\n")

	for _, interlineInfo := range interlinesInfo {
		result.WriteString(fmt.Sprintf(
			"%s %s -> %s %s | ",
			interlineInfo.StartDate,
			interlineInfo.StartTime,
			interlineInfo.ArriveDate,
			interlineInfo.ArriveTime,
		))
		result.WriteString(fmt.Sprintf(
			"%s -> %s -> %s | ",
			interlineInfo.FromStationName,
			interlineInfo.MiddleStationName,
			interlineInfo.EndStationName,
		))

		switch {
		case interlineInfo.SameStation:
			result.WriteString("同站换乘")
		case interlineInfo.SameTrain:
			result.WriteString("同车换乘")
		default:
			result.WriteString("换站换乘")
		}

		result.WriteString(
			fmt.Sprintf(" | %s | %s\n\n", interlineInfo.WaitTime, interlineInfo.Lishi),
		)
		result.WriteString("\t" + strings.ReplaceAll(
			s.formatTicketsInfo(interlineInfo.TicketList),
			"\n",
			"\n\t",
		))
		result.WriteString("\n")
	}

	return result.String()
}
