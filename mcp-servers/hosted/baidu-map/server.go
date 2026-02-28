package baidumap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Configuration templates for the Baidu Map server
var configTemplates = mcpservers.ConfigTemplates{
	"baidu_map_api_key": {
		Name:        "Baidu Map API Key",
		Required:    mcpservers.ConfigRequiredTypeInitOrReusingOnly,
		Example:     "your_baidu_map_api_key_here",
		Description: "Baidu Map API key for accessing map services",
	},
	"timeout": {
		Name:        "Request Timeout",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     "30",
		Description: "Request timeout in seconds (default: 30)",
		Validator: func(value string) error {
			timeout, err := strconv.Atoi(value)
			if err != nil {
				return errors.New("timeout must be a number")
			}

			if timeout < 1 || timeout > 120 {
				return errors.New("timeout must be between 1 and 120 seconds")
			}

			return nil
		},
	},
}

// Server represents the MCP server for Baidu Map
type Server struct {
	*server.MCPServer
	apiKey     string
	httpClient *http.Client
}

// NewServer creates a new MCP server for Baidu Map
func NewServer(config, reuse map[string]string) (mcpservers.Server, error) {
	// Get API key from config or environment
	apiKey := config["baidu_map_api_key"]
	if apiKey == "" {
		apiKey = reuse["baidu_map_api_key"]
	}

	if apiKey == "" {
		return nil, errors.New("baidu_map_api_key is required")
	}

	// Set up timeout
	timeout := 30 * time.Second
	if timeoutStr := config["timeout"]; timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = time.Duration(t) * time.Second
		}
	}

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: timeout,
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"mcp-server/baidu-map",
		"1.0.0",
	)

	baiduServer := &Server{
		MCPServer:  mcpServer,
		apiKey:     apiKey,
		httpClient: httpClient,
	}

	// Add all tools
	baiduServer.addTools()

	return baiduServer, nil
}

func ListTools(ctx context.Context) ([]mcp.Tool, error) {
	baiduServer := &Server{
		MCPServer: server.NewMCPServer(
			"mcp-server/baidu-map",
			"1.0.0",
		),
	}
	baiduServer.addTools()

	return mcpservers.ListServerTools(ctx, baiduServer)
}

// addTools adds all Baidu Map tools to the server
func (s *Server) addTools() {
	s.addGeocodeTools()
	s.addSearchTools()
	s.addRoutingTools()
	s.addWeatherTool()
	s.addIPLocationTool()
	s.addTrafficTool()
	s.addPOIExtractTool()
}

// addGeocodeTools adds geocoding and reverse geocoding tools
func (s *Server) addGeocodeTools() {
	// Geocode tool
	s.AddTool(
		mcp.Tool{
			Name:        "map_geocode",
			Description: "地理编码服务",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"address": map[string]any{
						"type":        "string",
						"description": "待解析的地址（最多支持84个字节。可以输入两种样式的值，分别是：1、标准的结构化地址信息，如北京市海淀区上地十街十号【推荐，地址结构越完整，解析精度越高】2、支持\"*路与*路交叉口\"描述方式，如北一环路和阜阳路的交叉路口第二种方式并不总是有返回结果，只有当地址库中存在该地址描述时才有返回。）",
					},
				},
				Required: []string{"address"},
			},
		},
		s.handleGeocode,
	)

	// Reverse geocode tool
	s.AddTool(
		mcp.Tool{
			Name:        "map_reverse_geocode",
			Description: "全球逆地理编码",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"latitude": map[string]any{
						"type":        "number",
						"description": "Latitude coordinate",
					},
					"longitude": map[string]any{
						"type":        "number",
						"description": "Longitude coordinate",
					},
				},
				Required: []string{"latitude", "longitude"},
			},
		},
		s.handleReverseGeocode,
	)
}

// addSearchTools adds place search and details tools
func (s *Server) addSearchTools() {
	// Place search tool
	s.AddTool(
		mcp.Tool{
			Name:        "map_search_places",
			Description: "地点检索服务（包括城市检索、圆形区域检索、多边形区域检索）",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "检索关键字",
					},
					"region": map[string]any{
						"type":        "string",
						"description": "检索行政区划区域",
					},
					"bounds": map[string]any{
						"type":        "string",
						"description": "检索多边形区域",
					},
					"location": map[string]any{
						"type":        "string",
						"description": "圆形区域检索中心点，不支持多个点",
					},
				},
				Required: []string{"query"},
			},
		},
		s.handlePlaceSearch,
	)

	// Place details tool
	s.AddTool(
		mcp.Tool{
			Name:        "map_place_details",
			Description: "地点详情检索服务",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"uid": map[string]any{
						"type":        "string",
						"description": "poi的uid",
					},
					"scope": map[string]any{
						"type":        "string",
						"description": "检索结果详细程度。取值为1 或空，则返回基本信息；取值为2，返回检索POI详细信息",
					},
				},
				Required: []string{"uid"},
			},
		},
		s.handlePlaceDetails,
	)
}

// addRoutingTools adds distance matrix and directions tools
func (s *Server) addRoutingTools() {
	// Distance matrix tool
	s.AddTool(
		mcp.Tool{
			Name:        "map_distance_matrix",
			Description: "计算多个出发地和目的地的距离和路线用时",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"origins": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "起点的纬度,经度。",
					},
					"destinations": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "终点的纬度,经度。",
					},
					"mode": map[string]any{
						"type":        "string",
						"description": "路线类型，可选值：driving（驾车）、walking（步行）、riding（骑行）、motorcycle（摩托车）",
						"enum":        []string{"driving", "walking", "riding", "motorcycle"},
						"default":     "driving",
					},
				},
				Required: []string{"origins", "destinations"},
			},
		},
		s.handleDistanceMatrix,
	)

	// Directions tool
	s.AddTool(
		mcp.Tool{
			Name:        "map_directions",
			Description: "路线规划服务， 计算出发地到目的地的距离、路线用时、路线方案",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"origin": map[string]any{
						"type":        "string",
						"description": "起点经纬度，格式为：纬度,经度；小数点后不超过6位，40.056878,116.30815",
					},
					"destination": map[string]any{
						"type":        "string",
						"description": "终点经纬度，格式为：纬度,经度；小数点后不超过6位，40.056878,116.30815",
					},
					"mode": map[string]any{
						"type":        "string",
						"description": "路线规划类型，可选值：driving（驾车）、walking（步行）、riding（骑行）、transit（公交）",
						"enum":        []string{"driving", "walking", "riding", "transit"},
						"default":     "driving",
					},
				},
				Required: []string{"origin", "destination"},
			},
		},
		s.handleDirections,
	)
}

// addWeatherTool adds weather tool
func (s *Server) addWeatherTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "map_weather",
			Description: "通过行政区划代码或者经纬度坐标获取实时天气信息和未来5天天气预报",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"districtId": map[string]any{
						"type":        "string",
						"description": "行政区划代码（适用于区、县级别）",
					},
					"location": map[string]any{
						"type":        "string",
						"description": "经纬度，经度在前纬度在后，逗号分隔，格式如116.404,39.915",
					},
				},
			},
		},
		s.handleWeather,
	)
}

// addIPLocationTool adds IP location tool
func (s *Server) addIPLocationTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "map_ip_location",
			Description: "通过IP地址获取位置信息",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"ip": map[string]any{
						"type":        "string",
						"description": "IP地址",
					},
				},
				Required: []string{"ip"},
			},
		},
		s.handleIPLocation,
	)
}

// addTrafficTool adds road traffic tool
func (s *Server) addTrafficTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "map_road_traffic",
			Description: "根据城市和道路名称查询具体道路的实时拥堵评价和拥堵路段、拥堵距离、拥堵趋势等信息",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"roadName": map[string]any{
						"type":        "string",
						"description": "道路名称",
					},
					"city": map[string]any{
						"type":        "string",
						"description": "城市名称",
					},
					"bounds": map[string]any{
						"type":        "string",
						"description": "矩形区域，左下角和右上角的经纬度坐标点，坐标对间使用;号分隔，格式为：纬度,经度;纬度,经度，如39.912078,116.464303;39.918276,116.475442",
					},
					"vertexes": map[string]any{
						"type":        "string",
						"description": "多边形边界点，经纬度顺序为：纬度,经度； 顶点顺序需按逆时针排列, 格式如vertexes=39.910528,116.472926;39.918276,116.475442;39.916671,116.459056;39.912078,116.464303",
					},
					"center": map[string]any{
						"type":        "string",
						"description": "中心点坐标，如39.912078,116.464303",
					},
					"radius": map[string]any{
						"type":        "number",
						"description": "查询半径，单位：米",
					},
				},
			},
		},
		s.handleRoadTraffic,
	)
}

// addPOIExtractTool adds POI extract tool
func (s *Server) addPOIExtractTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "map_poi_extract",
			Description: "POI智能标注",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"textContent": map[string]any{
						"type":        "string",
						"description": "描述POI的文本内容",
					},
				},
				Required: []string{"textContent"},
			},
		},
		s.handlePOIExtract,
	)
}

// Tool handlers

// handleGeocode handles geocoding requests
func (s *Server) handleGeocode(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	address, ok := args["address"].(string)
	if !ok || address == "" {
		return nil, errors.New("address is required")
	}

	apiURL := "https://api.map.baidu.com/geocoding/v3/"
	params := url.Values{}
	params.Add("address", address)
	params.Add("ak", s.apiKey)
	params.Add("output", "json")
	params.Add("from", "node_mcp")

	fullURL := apiURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var geocodeResp GeocodeResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&geocodeResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if geocodeResp.Status != 0 {
		return mcp.NewToolResultError(
			"Geocoding failed: " + getErrorMessage(geocodeResp.Response),
		), nil
	}

	result := map[string]any{
		"location":      geocodeResp.Result.Location,
		"precise":       geocodeResp.Result.Precise,
		"confidence":    geocodeResp.Result.Confidence,
		"comprehension": geocodeResp.Result.Comprehension,
		"level":         geocodeResp.Result.Level,
	}

	resultJSON, _ := sonic.MarshalIndent(result, "", "  ")

	return mcp.NewToolResultText(string(resultJSON)), nil
}

// handleReverseGeocode handles reverse geocoding requests
func (s *Server) handleReverseGeocode(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	latitude, ok := args["latitude"].(float64)
	if !ok {
		return nil, errors.New("latitude is required")
	}

	longitude, ok := args["longitude"].(float64)
	if !ok {
		return nil, errors.New("longitude is required")
	}

	apiURL := "https://api.map.baidu.com/reverse_geocoding/v3/"
	params := url.Values{}
	params.Add("location", fmt.Sprintf("%f,%f", latitude, longitude))
	params.Add("extensions_poi", "1")
	params.Add("ak", s.apiKey)
	params.Add("output", "json")
	params.Add("from", "node_mcp")

	fullURL := apiURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var reverseResp ReverseGeocodeResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&reverseResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if reverseResp.Status != 0 {
		return mcp.NewToolResultError(
			"Reverse geocoding failed: " + getErrorMessage(reverseResp.Response),
		), nil
	}

	var placeID *string
	if len(reverseResp.Result.POIs) > 0 {
		// Note: POIs is []interface{}, need to handle properly if needed
		placeID = nil
	}

	result := map[string]any{
		"place_id":              placeID,
		"location":              reverseResp.Result.Location,
		"formatted_address":     reverseResp.Result.FormattedAddress,
		"formatted_address_poi": reverseResp.Result.FormattedAddressPOI,
		"business":              reverseResp.Result.Business,
		"business_info":         reverseResp.Result.BusinessInfo,
		"addressComponent":      reverseResp.Result.AddressComponent,
		"edz":                   reverseResp.Result.EDZ,
		"pois":                  reverseResp.Result.POIs,
		"roads":                 reverseResp.Result.Roads,
		"poiRegions":            reverseResp.Result.POIRegions,
		"sematic_description":   reverseResp.Result.SematicDescription,
		"cityCode":              reverseResp.Result.CityCode,
	}

	resultJSON, _ := sonic.MarshalIndent(result, "", "  ")

	return mcp.NewToolResultText(string(resultJSON)), nil
}

// handlePlaceSearch handles place search requests
func (s *Server) handlePlaceSearch(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, errors.New("query is required")
	}

	apiURL := "https://api.map.baidu.com/place/v2/search"
	params := url.Values{}
	params.Add("query", query)
	params.Add("ak", s.apiKey)
	params.Add("output", "json")
	params.Add("from", "node_mcp")

	if region, ok := args["region"].(string); ok && region != "" {
		params.Add("region", region)
	}

	if bounds, ok := args["bounds"].(string); ok && bounds != "" {
		params.Add("bounds", bounds)
	}

	if location, ok := args["location"].(string); ok && location != "" {
		params.Add("location", location)
	}

	fullURL := apiURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var searchResp PlacesSearchResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if searchResp.Status != 0 {
		return mcp.NewToolResultError(
			"Place search failed: " + getErrorMessage(searchResp.Response),
		), nil
	}

	// Handle different response structures
	places := searchResp.Results
	if len(places) == 0 && len(searchResp.Result) > 0 {
		places = searchResp.Result
	}

	results := make([]map[string]any, 0, len(places))
	for _, place := range places {
		results = append(results, map[string]any{
			"name":      place.Name,
			"location":  place.Location,
			"address":   place.Address,
			"province":  place.Province,
			"city":      place.City,
			"area":      place.Area,
			"street_id": place.StreetID,
			"telephone": place.Telephone,
			"detail":    place.Detail,
			"uid":       place.UID,
		})
	}

	result := map[string]any{
		"result_type": searchResp.ResultType,
		"query_type":  searchResp.QueryType,
		"results":     results,
	}

	resultJSON, _ := sonic.MarshalIndent(result, "", "  ")

	return mcp.NewToolResultText(string(resultJSON)), nil
}

// handlePlaceDetails handles place details requests
func (s *Server) handlePlaceDetails(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	uid, ok := args["uid"].(string)
	if !ok || uid == "" {
		return nil, errors.New("uid is required")
	}

	apiURL := "https://api.map.baidu.com/place/v2/detail"
	params := url.Values{}
	params.Add("uid", uid)
	params.Add("ak", s.apiKey)
	params.Add("output", "json")
	params.Add("from", "node_mcp")

	if scope, ok := args["scope"].(string); ok && scope != "" {
		params.Add("scope", scope)
	}

	fullURL := apiURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var detailResp PlaceDetailsResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&detailResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if detailResp.Status != 0 {
		return mcp.NewToolResultError(
			"Place details request failed: " + getErrorMessage(detailResp.Response),
		), nil
	}

	result := map[string]any{
		"uid":       detailResp.Result.UID,
		"name":      detailResp.Result.Name,
		"location":  detailResp.Result.Location,
		"address":   detailResp.Result.Address,
		"province":  detailResp.Result.Province,
		"city":      detailResp.Result.City,
		"area":      detailResp.Result.Area,
		"street_id": detailResp.Result.StreetID,
		"detail":    detailResp.Result.Detail,
	}

	if detailResp.Result.DetailInfo != nil {
		result["detail_info"] = detailResp.Result.DetailInfo
	}

	resultJSON, _ := sonic.MarshalIndent(result, "", "  ")

	return mcp.NewToolResultText(string(resultJSON)), nil
}

// handleDistanceMatrix handles distance matrix requests
func (s *Server) handleDistanceMatrix(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	originsRaw, ok := args["origins"].([]any)
	if !ok {
		return nil, errors.New("origins is required")
	}

	destinationsRaw, ok := args["destinations"].([]any)
	if !ok {
		return nil, errors.New("destinations is required")
	}

	mode := "driving"
	if m, ok := args["mode"].(string); ok && m != "" {
		mode = m
	}

	// Convert origins and destinations to strings
	var origins, destinations []string
	for _, o := range originsRaw {
		if s, ok := o.(string); ok {
			origins = append(origins, s)
		}
	}

	for _, d := range destinationsRaw {
		if s, ok := d.(string); ok {
			destinations = append(destinations, s)
		}
	}

	apiURL := "https://api.map.baidu.com/routematrix/v2/" + mode
	params := url.Values{}
	params.Add("origins", strings.Join(origins, "|"))
	params.Add("destinations", strings.Join(destinations, "|"))
	params.Add("ak", s.apiKey)
	params.Add("output", "json")
	params.Add("from", "node_mcp")

	fullURL := apiURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var matrixResp DistanceMatrixResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&matrixResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if matrixResp.Status != 0 {
		return mcp.NewToolResultError(
			"Distance matrix request failed: " + getErrorMessage(matrixResp.Response),
		), nil
	}

	results := make([]map[string]any, 0, len(matrixResp.Result))
	for _, row := range matrixResp.Result {
		results = append(results, map[string]any{
			"elements": map[string]any{
				"duration": row.Duration,
				"distance": row.Distance,
			},
		})
	}

	result := map[string]any{
		"results": results,
	}

	resultJSON, _ := sonic.MarshalIndent(result, "", "  ")

	return mcp.NewToolResultText(string(resultJSON)), nil
}

// handleDirections handles directions requests
func (s *Server) handleDirections(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	origin, ok := args["origin"].(string)
	if !ok || origin == "" {
		return nil, errors.New("origin is required")
	}

	destination, ok := args["destination"].(string)
	if !ok || destination == "" {
		return nil, errors.New("destination is required")
	}

	mode := "driving"
	if m, ok := args["mode"].(string); ok && m != "" {
		mode = m
	}

	apiURL := "https://api.map.baidu.com/directionlite/v1/" + mode
	params := url.Values{}
	params.Add("origin", origin)
	params.Add("destination", destination)
	params.Add("ak", s.apiKey)
	params.Add("from", "node_mcp")

	fullURL := apiURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var directionsResp DirectionsResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&directionsResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if directionsResp.Status != 0 {
		return mcp.NewToolResultError(
			"Directions request failed: " + getErrorMessage(directionsResp.Response),
		), nil
	}

	routes := make([]map[string]any, 0, len(directionsResp.Result.Routes))
	for _, route := range directionsResp.Result.Routes {
		steps := make([]map[string]any, 0, len(route.Steps))
		for _, step := range route.Steps {
			steps = append(steps, map[string]any{
				"instructions": step.Instruction,
			})
		}

		routes = append(routes, map[string]any{
			"distance": route.Distance,
			"duration": route.Duration,
			"steps":    steps,
		})
	}

	result := map[string]any{
		"routes": routes,
	}

	resultJSON, _ := sonic.MarshalIndent(result, "", "  ")

	return mcp.NewToolResultText(string(resultJSON)), nil
}

// handleWeather handles weather requests
func (s *Server) handleWeather(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	apiURL := "https://api.map.baidu.com/weather/v1/"
	params := url.Values{}
	params.Add("data_type", "all")
	params.Add("coordtype", "bd09ll")
	params.Add("ak", s.apiKey)
	params.Add("from", "node_mcp")

	if location, ok := args["location"].(string); ok && location != "" {
		params.Add("location", location)
	}

	if districtID, ok := args["districtId"].(string); ok && districtID != "" {
		params.Add("district_id", districtID)
	}

	fullURL := apiURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var weatherResp WeatherResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&weatherResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if weatherResp.Status != 0 {
		return mcp.NewToolResultError(
			"Weather search failed: " + getErrorMessage(weatherResp.Response),
		), nil
	}

	result := map[string]any{
		"location":       weatherResp.Result.Location,
		"now":            weatherResp.Result.Now,
		"forecasts":      weatherResp.Result.Forecasts,
		"forecast_hours": weatherResp.Result.ForecastHours,
		"indexes":        weatherResp.Result.Indexes,
		"alerts":         weatherResp.Result.Alerts,
	}

	resultJSON, _ := sonic.MarshalIndent(result, "", "  ")

	return mcp.NewToolResultText(string(resultJSON)), nil
}

// handleIPLocation handles IP location requests
func (s *Server) handleIPLocation(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	ip, ok := args["ip"].(string)
	if !ok || ip == "" {
		return nil, errors.New("ip is required")
	}

	apiURL := "https://api.map.baidu.com/location/ip"
	params := url.Values{}
	params.Add("ip", ip)
	params.Add("coor", "bd09ll")
	params.Add("ak", s.apiKey)
	params.Add("from", "node_mcp")

	fullURL := apiURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var ipResp IPLocationResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&ipResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if ipResp.Status != 0 {
		return mcp.NewToolResultError(
			"IP address search failed: " + getErrorMessage(ipResp.Response),
		), nil
	}

	result := map[string]any{
		"formatted_address": ipResp.Address,
		"address_detail":    ipResp.Content.AddressDetail,
		"point":             ipResp.Content.Point,
	}

	resultJSON, _ := sonic.MarshalIndent(result, "", "  ")

	return mcp.NewToolResultText(string(resultJSON)), nil
}

// handleRoadTraffic handles road traffic requests
func (s *Server) handleRoadTraffic(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	baseURL := "https://api.map.baidu.com"
	params := url.Values{}
	params.Add("ak", s.apiKey)
	params.Add("from", "node_mcp")

	var apiURL string

	// Determine API endpoint based on parameters
	if roadName, ok := args["roadName"].(string); ok && roadName != "" {
		if city, ok := args["city"].(string); ok && city != "" {
			apiURL = baseURL + "/traffic/v1/road"

			params.Add("road_name", roadName)
			params.Add("city", city)
		}
	} else if bounds, ok := args["bounds"].(string); ok && bounds != "" {
		apiURL = baseURL + "/traffic/v1/bound"

		params.Add("bounds", bounds)
	} else if vertexes, ok := args["vertexes"].(string); ok && vertexes != "" {
		apiURL = baseURL + "/traffic/v1/polygon"

		params.Add("vertexes", vertexes)
	} else if center, ok := args["center"].(string); ok && center != "" {
		if radius, ok := args["radius"].(float64); ok {
			apiURL = baseURL + "/traffic/v1/around"

			params.Add("center", center)
			params.Add("radius", strconv.FormatFloat(radius, 'f', -1, 64))
		}
	}

	if apiURL == "" {
		return nil, errors.New("insufficient parameters for road traffic query")
	}

	fullURL := apiURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var trafficResp RoadTrafficResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&trafficResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if trafficResp.Status != 0 {
		return mcp.NewToolResultError(
			"Road traffic search failed: " + getErrorMessage(trafficResp.Response),
		), nil
	}

	result := map[string]any{
		"description":  trafficResp.Description,
		"evaluation":   trafficResp.Evaluation,
		"road_traffic": trafficResp.RoadTraffic,
	}

	resultJSON, _ := sonic.MarshalIndent(result, "", "  ")

	return mcp.NewToolResultText(string(resultJSON)), nil
}

// handlePOIExtract handles POI extraction requests
func (s *Server) handlePOIExtract(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	textContent, ok := args["textContent"].(string)
	if !ok || textContent == "" {
		return nil, errors.New("textContent is required")
	}

	// Submit POI extraction request
	submitURL := "https://api.map.baidu.com/api_mark/v1/submit"
	params := url.Values{}
	params.Add("text_content", textContent)
	params.Add("id", "75274677") // Device ID
	params.Add("msg_type", "text")
	params.Add("ak", s.apiKey)
	params.Add("from", "node_mcp")

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		submitURL,
		strings.NewReader(params.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("submit request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	submitResp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("submit request failed: %w", err)
	}
	defer submitResp.Body.Close()

	var submitData MarkSubmitResponse
	if err := sonic.ConfigDefault.NewDecoder(submitResp.Body).Decode(&submitData); err != nil {
		return nil, fmt.Errorf("failed to parse submit response: %w", err)
	}

	if submitData.Status != 0 {
		return mcp.NewToolResultError(
			"Mark submit failed: " + getErrorMessage(submitData.Response),
		), nil
	}

	// Poll for results
	resultURL := "https://api.map.baidu.com/api_mark/v1/result"
	mapID := submitData.Result.MapID

	resultParams := url.Values{}
	resultParams.Add("map_id", mapID)
	resultParams.Add("ak", s.apiKey)
	resultParams.Add("from", "node_mcp")

	// Poll every 1 second for up to 20 seconds
	maxTime := 20 * time.Second
	intervalTime := 1 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < maxTime {
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			resultURL,
			strings.NewReader(resultParams.Encode()),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resultResp, err := s.httpClient.Do(req)
		if err != nil {
			time.Sleep(intervalTime)
			continue
		}
		defer resultResp.Body.Close()

		var resultData MarkResultResponse
		if err := sonic.ConfigDefault.NewDecoder(resultResp.Body).Decode(&resultData); err != nil {
			time.Sleep(intervalTime)
			continue
		}

		if len(resultData.Result.Data) > 0 {
			result := map[string]any{
				"jumpUrl": resultData.Result.Data[0].Link.JumpURL,
				"title":   resultData.Result.Data[0].Link.Title,
				"desc":    resultData.Result.Data[0].Link.Desc,
				"image":   resultData.Result.Data[0].Link.Image,
				"poi":     resultData.Result.Data[0].Link.POI,
			}

			resultJSON, _ := sonic.MarshalIndent(result, "", "  ")

			return mcp.NewToolResultText(string(resultJSON)), nil
		}

		time.Sleep(intervalTime)
	}

	return mcp.NewToolResultError(
		"POI result is null",
	), nil
}

// getErrorMessage extracts error message from response
func getErrorMessage(resp Response) string {
	if resp.Message != "" {
		return resp.Message
	}

	if resp.Msg != "" {
		return resp.Msg
	}

	return strconv.Itoa(resp.Status)
}
