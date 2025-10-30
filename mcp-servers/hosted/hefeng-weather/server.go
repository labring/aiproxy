package hefengweather

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Configuration templates for the weather server
var configTemplates = mcpservers.ConfigTemplates{
	"hefeng_api_key": {
		Name:        "和风天气API密钥",
		Required:    mcpservers.ConfigRequiredTypeInitOrReusingOnly,
		Example:     "your_hefeng_api_key_here",
		Description: "和风天气API密钥，用于获取天气数据",
	},
	"hefeng_api_base": {
		Name:        "和风天气API基础URL",
		Required:    mcpservers.ConfigRequiredTypeInitOrReusingOnly,
		Example:     "https://devapi.qweather.com/v7",
		Description: "和风天气API基础URL，用于获取天气数据",
		Validator: func(value string) error {
			_, err := url.Parse(value)
			return err
		},
	},
}

// WeatherServer represents the MCP server for weather functionality
type WeatherServer struct {
	*server.MCPServer
	apiKey     string
	httpClient *http.Client
	apiBase    string
}

// WeatherArguments represents the arguments for weather requests
type WeatherArguments struct {
	Location string `json:"location"`
	Days     string `json:"days"`
}

// NowResponse represents the current weather response
type NowResponse struct {
	Code string `json:"code"`
	Now  struct {
		ObsTime   string `json:"obsTime"`
		Temp      string `json:"temp"`
		FeelsLike string `json:"feelsLike"`
		Text      string `json:"text"`
		WindDir   string `json:"windDir"`
		WindScale string `json:"windScale"`
	} `json:"now"`
}

// DailyResponse represents the daily weather forecast response
type DailyResponse struct {
	Code  string `json:"code"`
	Daily []struct {
		FxDate         string `json:"fxDate"`
		TempMax        string `json:"tempMax"`
		TempMin        string `json:"tempMin"`
		TextDay        string `json:"textDay"`
		TextNight      string `json:"textNight"`
		WindDirDay     string `json:"windDirDay"`
		WindScaleDay   string `json:"windScaleDay"`
		WindDirNight   string `json:"windDirNight"`
		WindScaleNight string `json:"windScaleNight"`
	} `json:"daily"`
}

// HourlyResponse represents the hourly weather forecast response
type HourlyResponse struct {
	Code   string `json:"code"`
	Hourly []struct {
		FxTime    string `json:"fxTime"`
		Temp      string `json:"temp"`
		Text      string `json:"text"`
		WindDir   string `json:"windDir"`
		WindScale string `json:"windScale"`
		Humidity  string `json:"humidity"`
	} `json:"hourly"`
}

// NewServer creates a new MCP server for weather functionality
func NewServer(config, reuse map[string]string) (mcpservers.Server, error) {
	// Get API key from config or environment
	apiKey := config["hefeng_api_key"]
	if apiKey == "" {
		apiKey = reuse["hefeng_api_key"]
	}

	if apiKey == "" {
		return nil, errors.New("api key is required")
	}

	apiBase := config["hefeng_api_base"]
	if apiBase == "" {
		apiBase = reuse["hefeng_api_base"]
	}

	if apiBase == "" {
		return nil, errors.New("api base is required")
	}

	if !strings.HasSuffix(apiBase, "/v7") {
		apiBase += "/v7"
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"hefeng-weather",
		"1.0.0",
	)

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	weatherServer := &WeatherServer{
		MCPServer:  mcpServer,
		apiKey:     apiKey,
		apiBase:    apiBase,
		httpClient: httpClient,
	}

	// Add weather tool
	weatherServer.addWeatherTool()

	return weatherServer, nil
}

func ListTools(ctx context.Context) ([]mcp.Tool, error) {
	weatherServer := &WeatherServer{
		MCPServer: server.NewMCPServer("hefeng-weather", "1.0.0"),
	}
	weatherServer.addWeatherTool()

	return mcpservers.ListServerTools(ctx, weatherServer)
}

// validateDays validates the days parameter
func validateDays(days string) error {
	validDays := []string{"now", "24h", "72h", "168h", "3d", "7d", "10d", "15d", "30d"}
	if slices.Contains(validDays, days) {
		return nil
	}

	return fmt.Errorf("无效的预报天数: %s，有效值为: %s", days, strings.Join(validDays, ", "))
}

// makeHeFengRequest makes a request to the HeFeng API
func (s *WeatherServer) makeHeFengRequest(ctx context.Context, endpoint string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP错误: %d, %s", resp.StatusCode, string(body))
	}

	return sonic.ConfigDefault.NewDecoder(resp.Body).Decode(v)
}

// addWeatherTool adds the weather tool to the server
func (s *WeatherServer) addWeatherTool() {
	weatherTool := mcp.Tool{
		Name:        "get-weather",
		Description: "获取中国国内的天气预报",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"location": map[string]any{
					"type":        "string",
					"description": "逗号分隔的经纬度信息 (e.g., 116.40,39.90)",
				},
				"days": map[string]any{
					"type": "string",
					"enum": []string{
						"now", "24h", "72h", "168h", "3d", "7d", "10d", "15d", "30d",
					},
					"description": "预报天数，now为实时天气，24h为24小时预报，72h为72小时预报，168h为168小时预报，3d为3天预报，以此类推",
					"default":     "now",
				},
			},
			Required: []string{"location"},
		},
	}

	s.AddTool(weatherTool, s.handleWeather)
}

// handleWeather handles the weather tool
func (s *WeatherServer) handleWeather(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	location, ok := args["location"].(string)
	if !ok || location == "" {
		return nil, errors.New("location参数是必需的")
	}

	days := "now"
	if d, ok := args["days"].(string); ok && d != "" {
		days = d
	}

	// Validate days parameter
	if err := validateDays(days); err != nil {
		return nil, err
	}

	// URL encode the location parameter
	encodedLocation := url.QueryEscape(location)

	switch days {
	case "now":
		return s.handleCurrentWeather(ctx, encodedLocation, location)
	case "24h", "72h", "168h":
		return s.handleHourlyWeather(ctx, encodedLocation, location, days)
	default:
		return s.handleDailyWeather(ctx, encodedLocation, location, days)
	}
}

// handleCurrentWeather handles current weather requests
func (s *WeatherServer) handleCurrentWeather(
	ctx context.Context,
	encodedLocation, location string,
) (*mcp.CallToolResult, error) {
	weatherURL := fmt.Sprintf(
		"%s/weather/now?location=%s&key=%s",
		s.apiBase,
		encodedLocation,
		s.apiKey,
	)

	var weatherData NowResponse

	err := s.makeHeFengRequest(ctx, weatherURL, &weatherData)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("获取天气数据失败: %v", err)), nil
	}

	// Check API response code
	if weatherData.Code != "200" {
		return mcp.NewToolResultError("API返回错误代码: " + weatherData.Code), nil
	}

	now := weatherData.Now
	weatherText := fmt.Sprintf(`地点: %s
观测时间: %s
天气: %s
温度: %s°C
体感温度: %s°C
风向: %s
风力: %s级`,
		location,
		now.ObsTime,
		now.Text,
		now.Temp,
		now.FeelsLike,
		now.WindDir,
		now.WindScale)

	return mcp.NewToolResultText(weatherText), nil
}

// handleHourlyWeather handles hourly weather forecast requests
func (s *WeatherServer) handleHourlyWeather(
	ctx context.Context,
	encodedLocation, location, days string,
) (*mcp.CallToolResult, error) {
	weatherURL := fmt.Sprintf(
		"%s/weather/%s?location=%s&key=%s",
		s.apiBase,
		days,
		encodedLocation,
		s.apiKey,
	)

	var weatherData HourlyResponse

	err := s.makeHeFengRequest(ctx, weatherURL, &weatherData)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("获取逐小时天气预报数据失败: %v", err)), nil
	}

	// Check API response code
	if weatherData.Code != "200" {
		return mcp.NewToolResultError("API返回错误代码: " + weatherData.Code), nil
	}

	if len(weatherData.Hourly) == 0 {
		return mcp.NewToolResultError(fmt.Sprintf("无法获取 %s 的逐小时天气预报数据", location)), nil
	}

	var hoursText strings.Builder
	for _, hour := range weatherData.Hourly {
		hoursText.WriteString(fmt.Sprintf(`时间: %s
天气: %s
温度: %s°C
湿度: %s%%
风向: %s %s级
------------------------
`,
			hour.FxTime,
			hour.Text,
			hour.Temp,
			hour.Humidity,
			hour.WindDir,
			hour.WindScale))
	}

	result := fmt.Sprintf("地点: %s\n%s小时预报:\n%s", location, days, hoursText.String())

	return mcp.NewToolResultText(result), nil
}

// handleDailyWeather handles daily weather forecast requests
func (s *WeatherServer) handleDailyWeather(
	ctx context.Context,
	encodedLocation, location, days string,
) (*mcp.CallToolResult, error) {
	weatherURL := fmt.Sprintf(
		"%s/weather/%s?location=%s&key=%s",
		s.apiBase,
		days,
		encodedLocation,
		s.apiKey,
	)

	var weatherData DailyResponse

	err := s.makeHeFengRequest(ctx, weatherURL, &weatherData)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("获取天气预报数据失败: %v", err)), nil
	}

	// Check API response code
	if weatherData.Code != "200" {
		return mcp.NewToolResultError("API返回错误代码: " + weatherData.Code), nil
	}

	if len(weatherData.Daily) == 0 {
		return mcp.NewToolResultError(fmt.Sprintf("无法获取 %s 的天气预报数据", location)), nil
	}

	var forecastText strings.Builder
	for _, day := range weatherData.Daily {
		forecastText.WriteString(fmt.Sprintf(`日期: %s
白天天气: %s
夜间天气: %s
最高温度: %s°C
最低温度: %s°C
白天风向: %s %s级
夜间风向: %s %s级
------------------------
`,
			day.FxDate,
			day.TextDay,
			day.TextNight,
			day.TempMax,
			day.TempMin,
			day.WindDirDay,
			day.WindScaleDay,
			day.WindDirNight,
			day.WindScaleNight))
	}

	// Parse days number for display
	daysNum := days
	if parsedDays, err := strconv.Atoi(strings.TrimSuffix(days, "d")); err == nil {
		daysNum = strconv.Itoa(parsedDays)
	}

	result := fmt.Sprintf("地点: %s\n%s天预报:\n%s", location, daysNum, forecastText.String())

	return mcp.NewToolResultText(result), nil
}
