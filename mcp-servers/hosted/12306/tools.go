package train12306

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/mark3labs/mcp-go/mcp"
)

// addGetCurrentDateTool adds the get current date tool
func (s *Server) addGetCurrentDateTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "get-current-date",
			Description: "获取当前日期，以上海时区（Asia/Shanghai, UTC+8）为准，返回格式为 \"yyyy-MM-dd\"。主要用于解析用户提到的相对日期（如\"明天\"、\"下周三\"），为其他需要日期的接口提供准确的日期输入。",
			InputSchema: mcp.ToolInputSchema{
				Type:       "object",
				Properties: map[string]any{},
				Required:   []string{},
			},
		},
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			location, err := time.LoadLocation(TimeZone)
			if err != nil {
				return nil, fmt.Errorf("failed to load timezone: %w", err)
			}

			now := time.Now().In(location)
			formattedDate := now.Format(time.DateOnly)

			return mcp.NewToolResultText(formattedDate), nil
		},
	)
}

// addGetStationsCodeInCityTool adds the get stations code in city tool
func (s *Server) addGetStationsCodeInCityTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "get-stations-code-in-city",
			Description: "通过中文城市名查询该城市 **所有** 火车站的名称及其对应的 `station_code`，结果是一个包含多个车站信息的列表。",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"city": map[string]any{
						"type":        "string",
						"description": "中文城市名称，例如：\"北京\", \"上海\"",
					},
				},
				Required: []string{"city"},
			},
		},
		func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := request.GetArguments()

			city, ok := args["city"].(string)
			if !ok || city == "" {
				return nil, errors.New("city is required")
			}

			stations, exists := s.cityStations[city]
			if !exists {
				return mcp.NewToolResultText("Error: City not found."), nil
			}

			result, err := sonic.Marshal(stations)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(result)), nil
		},
	)
}

// addGetStationCodeOfCitysTool adds the get station code of cities tool
func (s *Server) addGetStationCodeOfCitysTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "get-station-code-of-citys",
			Description: "通过中文城市名查询代表该城市的 `station_code`。此接口主要用于在用户提供**城市名**作为出发地或到达地时，为接口准备 `station_code` 参数。",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"citys": map[string]any{
						"type":        "string",
						"description": "要查询的城市，比如\"北京\"。若要查询多个城市，请用|分割，比如\"北京|上海\"。",
					},
				},
				Required: []string{"citys"},
			},
		},
		func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := request.GetArguments()

			citys, ok := args["citys"].(string)
			if !ok || citys == "" {
				return nil, errors.New("citys is required")
			}

			result := make(map[string]any)
			for city := range strings.SplitSeq(citys, "|") {
				if station, exists := s.cityStationCodes[city]; exists {
					result[city] = station
				} else {
					result[city] = map[string]string{"error": "未检索到城市。"}
				}
			}

			response, err := sonic.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(response)), nil
		},
	)
}

// addGetStationCodeByNamesTool adds the get station code by names tool
func (s *Server) addGetStationCodeByNamesTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "get-station-code-by-names",
			Description: "通过具体的中文车站名查询其 `station_code` 和车站名。此接口主要用于在用户提供**具体车站名**作为出发地或到达地时，为接口准备 `station_code` 参数。",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"stationNames": map[string]any{
						"type":        "string",
						"description": "具体的中文车站名称，例如：\"北京南\", \"上海虹桥\"。若要查询多个站点，请用|分割，比如\"北京南|上海虹桥\"。",
					},
				},
				Required: []string{"stationNames"},
			},
		},
		func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := request.GetArguments()

			stationNames, ok := args["stationNames"].(string)
			if !ok || stationNames == "" {
				return nil, errors.New("stationNames is required")
			}

			result := make(map[string]any)
			for stationName := range strings.SplitSeq(stationNames, "|") {
				cleanName := strings.TrimSuffix(stationName, "站")

				if station, exists := s.nameStations[cleanName]; exists {
					result[stationName] = station
				} else {
					result[stationName] = map[string]string{"error": "未检索到车站。"}
				}
			}

			response, err := sonic.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(response)), nil
		},
	)
}

// addGetStationByTelecodeTool adds the get station by telecode tool
func (s *Server) addGetStationByTelecodeTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "get-station-by-telecode",
			Description: "通过车站的 `station_telecode` 查询车站的详细信息，包括名称、拼音、所属城市等。此接口主要用于在已知 `telecode` 的情况下获取更完整的车站数据，或用于特殊查询及调试目的。一般用户对话流程中较少直接触发。",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"stationTelecode": map[string]any{
						"type":        "string",
						"description": "车站的 `station_telecode` (3位字母编码)",
					},
				},
				Required: []string{"stationTelecode"},
			},
		},
		func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := request.GetArguments()

			stationTelecode, ok := args["stationTelecode"].(string)
			if !ok || stationTelecode == "" {
				return nil, errors.New("stationTelecode is required")
			}

			station, exists := s.stations[stationTelecode]
			if !exists {
				return mcp.NewToolResultText("Error: Station not found."), nil
			}

			result, err := sonic.Marshal(station)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(result)), nil
		},
	)
}

// addGetTicketsTool adds the get tickets tool
func (s *Server) addGetTicketsTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "get-tickets",
			Description: "查询12306余票信息。",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式为 \"yyyy-MM-dd\"。如果用户提供的是相对日期（如\"明天\"），请务必先调用 `get-current-date` 接口获取当前日期，并计算出目标日期。",
					},
					"fromStation": map[string]any{
						"type":        "string",
						"description": "出发地的 `station_code` 。必须是通过 `get-station-code-by-names` 或 `get-station-code-of-citys` 接口查询得到的编码，严禁直接使用中文地名。",
					},
					"toStation": map[string]any{
						"type":        "string",
						"description": "到达地的 `station_code` 。必须是通过 `get-station-code-by-names` 或 `get-station-code-of-citys` 接口查询得到的编码，严禁直接使用中文地名。",
					},
					"trainFilterFlags": map[string]any{
						"type":        "string",
						"description": "车次筛选条件，默认为空，即不筛选。支持多个标志同时筛选。例如用户说\"高铁票\"，则应使用 \"G\"。可选标志：[G(高铁/城际),D(动车),Z(直达特快),T(特快),K(快速),O(其他),F(复兴号),S(智能动车组)]",
						"default":     "",
					},
					"sortFlag": map[string]any{
						"type":        "string",
						"description": "排序方式，默认为空，即不排序。仅支持单一标识。可选标志：[startTime(出发时间从早到晚), arriveTime(抵达时间从早到晚), duration(历时从短到长)]",
						"default":     "",
					},
					"sortReverse": map[string]any{
						"type":        "boolean",
						"description": "是否逆向排序结果，默认为false。仅在设置了sortFlag时生效。",
						"default":     false,
					},
					"limitedNum": map[string]any{
						"type":        "integer",
						"description": "返回的余票数量限制，默认为0，即不限制。",
						"default":     0,
						"minimum":     0,
					},
				},
				Required: []string{"date", "fromStation", "toStation"},
			},
		},
		s.handleGetTickets,
	)
}
