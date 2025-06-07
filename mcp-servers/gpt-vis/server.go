package gptvis

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	DefaultVisRequestServer = "https://antv-studio.alipay.com/api/gpt-vis"
)

// Configuration templates for the chart server
var configTemplates = mcpservers.ConfigTemplates{
	"vis_request_server": {
		Name:        "图表生成服务器URL",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     DefaultVisRequestServer,
		Description: "用于生成图表的API服务器地址",
	},
}

// Server represents the MCP server for chart generation
type Server struct {
	*server.MCPServer
	visRequestServer string
	httpClient       *http.Client
}

// ChartTypeMapping maps tool names to chart types
var ChartTypeMapping = map[string]string{
	"generate_line_chart":       "line",
	"generate_column_chart":     "column",
	"generate_area_chart":       "area",
	"generate_pie_chart":        "pie",
	"generate_bar_chart":        "bar",
	"generate_histogram_chart":  "histogram",
	"generate_scatter_chart":    "scatter",
	"generate_word_cloud_chart": "word-cloud",
	"generate_radar_chart":      "radar",
	"generate_treemap_chart":    "treemap",
	"generate_dual_axes_chart":  "dual-axes",
	"generate_mind_map":         "mind-map",
	"generate_network_graph":    "network-graph",
	"generate_flow_diagram":     "flow-diagram",
	"generate_fishbone_diagram": "fishbone-diagram",
}

type Response struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"errorMessage"`
	ErrorCode    string `json:"errorCode"`
	TraceID      string `json:"traceId"`
	ResultObj    string `json:"resultObj"`
}

// NewServer creates a new MCP server for chart functionality
func NewServer(config, _ map[string]string) (mcpservers.Server, error) {
	// Get VIS request server URL from config or environment
	visRequestServer := config["vis_request_server"]
	if visRequestServer == "" {
		visRequestServer = DefaultVisRequestServer
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"gpt-vis",
		"0.4.0",
	)

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	chartServer := &Server{
		MCPServer:        mcpServer,
		visRequestServer: visRequestServer,
		httpClient:       httpClient,
	}

	// Add all chart tools
	chartServer.addAllChartTools()

	return chartServer, nil
}

// generateChartURL generates a chart URL using the provided configuration
func (s *Server) generateChartURL(
	ctx context.Context,
	chartType string,
	options map[string]any,
) (string, error) {
	cloneOptions := maps.Clone(options)
	cloneOptions["type"] = chartType
	cloneOptions["source"] = "mcp-server-chart"

	// Marshal request data
	jsonData, err := sonic.Marshal(cloneOptions)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request data: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		s.visRequestServer,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Make request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var chartResponse Response
	if err := sonic.Unmarshal(body, &chartResponse); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if !chartResponse.Success {
		return "", fmt.Errorf(
			"failed to generate chart: %s, errorCode: %s, traceId: %s",
			chartResponse.ErrorMessage,
			chartResponse.ErrorCode,
			chartResponse.TraceID,
		)
	}

	return chartResponse.ResultObj, nil
}

// validateChartData performs basic validation on chart data
func validateChartData(chartType string, args map[string]any) error {
	// Check if data field exists for most chart types
	if chartType != "mind-map" && chartType != "fishbone-diagram" {
		data, ok := args["data"]
		if !ok {
			return errors.New("data field is required")
		}

		// Check if data is array for most chart types
		if chartType != "network-graph" && chartType != "flow-diagram" {
			if dataArray, ok := data.([]any); !ok || len(dataArray) == 0 {
				return errors.New("data must be a non-empty array")
			}
		}
	}

	return nil
}

// addAllChartTools adds all chart generation tools
func (s *Server) addAllChartTools() {
	// Define all chart tools
	tools := []mcp.Tool{
		s.createLineChartTool(),
		s.createColumnChartTool(),
		s.createAreaChartTool(),
		s.createPieChartTool(),
		s.createBarChartTool(),
		s.createHistogramChartTool(),
		s.createScatterChartTool(),
		s.createWordCloudChartTool(),
		s.createRadarChartTool(),
		s.createTreemapChartTool(),
		s.createDualAxesChartTool(),
		s.createMindMapTool(),
		s.createNetworkGraphTool(),
		s.createFlowDiagramTool(),
		s.createFishboneDiagramTool(),
	}

	// Add all tools with the same handler
	for _, tool := range tools {
		s.AddTool(tool, s.handleChartGeneration)
	}
}

// handleChartGeneration handles chart generation requests
func (s *Server) handleChartGeneration(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	toolName := request.Params.Name
	args := request.GetArguments()

	// Get chart type from tool name
	chartType, exists := ChartTypeMapping[toolName]
	if !exists {
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}

	// Validate chart data
	if err := validateChartData(chartType, args); err != nil {
		return nil, fmt.Errorf("invalid parameters: %s", err.Error())
	}

	// Generate chart URL
	url, err := s.generateChartURL(ctx, chartType, args)
	if err != nil {
		return nil, fmt.Errorf("failed to generate chart: %w", err)
	}

	return mcp.NewToolResultText(url), nil
}

// Chart tool creation methods
func (s *Server) createLineChartTool() mcp.Tool {
	return mcp.Tool{
		Name:        "generate_line_chart",
		Description: "Generate a line chart to show trends over time, such as, the ratio of Apple computer sales to Apple's profits changed from 2000 to 2016.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"data": map[string]any{
					"type":        "array",
					"description": "Data for line chart, such as, [{ time: '2015', value: 23 }].",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"time":  map[string]any{"type": "string"},
							"value": map[string]any{"type": "number"},
							"group": map[string]any{"type": "string"},
						},
						"required": []string{"time", "value"},
					},
				},
				"stack": map[string]any{
					"type":        "boolean",
					"default":     false,
					"description": "Whether stacking is enabled. When enabled, line charts require a 'group' field in the data.",
				},
				"theme": map[string]any{
					"type":        "string",
					"enum":        []string{"default", "academy"},
					"default":     "default",
					"description": "Set the theme for the chart, optional, default is 'default'.",
				},
				"width": map[string]any{
					"type":        "number",
					"default":     600,
					"description": "Set the width of chart, default is 600.",
				},
				"height": map[string]any{
					"type":        "number",
					"default":     400,
					"description": "Set the height of chart, default is 400.",
				},
				"title": map[string]any{
					"type":        "string",
					"default":     "",
					"description": "Set the title of chart.",
				},
				"axisXTitle": map[string]any{
					"type":        "string",
					"default":     "",
					"description": "Set the x-axis title of chart.",
				},
				"axisYTitle": map[string]any{
					"type":        "string",
					"default":     "",
					"description": "Set the y-axis title of chart.",
				},
			},
			Required: []string{"data"},
		},
	}
}

func (s *Server) createColumnChartTool() mcp.Tool {
	return mcp.Tool{
		Name:        "generate_column_chart",
		Description: "Generate a column chart, which are best for comparing categorical data, such as, when values are close, column charts are preferable because our eyes are better at judging height than other visual elements like area or angles.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"data": map[string]any{
					"type":        "array",
					"description": "Data for column chart, such as, [{ category: '北京', value: 825, group: '油车' }].",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"category": map[string]any{"type": "string"},
							"value":    map[string]any{"type": "number"},
							"group":    map[string]any{"type": "string"},
						},
						"required": []string{"category", "value"},
					},
				},
				"group": map[string]any{
					"type":        "boolean",
					"default":     true,
					"description": "Whether grouping is enabled. When enabled, column charts require a 'group' field in the data. When `group` is true, `stack` should be false.",
				},
				"stack": map[string]any{
					"type":        "boolean",
					"default":     false,
					"description": "Whether stacking is enabled. When enabled, column charts require a 'group' field in the data. When `stack` is true, `group` should be false.",
				},
				"theme":      getThemeProperty(),
				"width":      getWidthProperty(),
				"height":     getHeightProperty(),
				"title":      getTitleProperty(),
				"axisXTitle": getAxisXTitleProperty(),
				"axisYTitle": getAxisYTitleProperty(),
			},
			Required: []string{"data"},
		},
	}
}

func (s *Server) createAreaChartTool() mcp.Tool {
	return mcp.Tool{
		Name:        "generate_area_chart",
		Description: "Generate a area chart to show data trends under continuous independent variables and observe the overall data trend, such as, displacement = velocity (average or instantaneous) × time: s = v × t. If the x-axis is time (t) and the y-axis is velocity (v) at each moment, an area chart allows you to observe the trend of velocity over time and infer the distance traveled by the area's size.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"data": map[string]any{
					"type":        "array",
					"description": "Data for area chart, such as, [{ time: '2018', value: 99.9 }].",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"time":  map[string]any{"type": "string"},
							"value": map[string]any{"type": "number"},
							"group": map[string]any{"type": "string"},
						},
						"required": []string{"time", "value"},
					},
				},
				"stack": map[string]any{
					"type":        "boolean",
					"default":     false,
					"description": "Whether stacking is enabled. When enabled, area charts require a 'group' field in the data.",
				},
				"theme":      getThemeProperty(),
				"width":      getWidthProperty(),
				"height":     getHeightProperty(),
				"title":      getTitleProperty(),
				"axisXTitle": getAxisXTitleProperty(),
				"axisYTitle": getAxisYTitleProperty(),
			},
			Required: []string{"data"},
		},
	}
}

func (s *Server) createPieChartTool() mcp.Tool {
	return mcp.Tool{
		Name:        "generate_pie_chart",
		Description: "Generate a pie chart to show the proportion of parts, such as, market share and budget allocation.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"data": map[string]any{
					"type":        "array",
					"description": "Data for pie chart, such as, [{ category: '分类一', value: 27 }].",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"category": map[string]any{"type": "string"},
							"value":    map[string]any{"type": "number"},
						},
						"required": []string{"category", "value"},
					},
				},
				"innerRadius": map[string]any{
					"type":        "number",
					"default":     0,
					"description": "Set the innerRadius of pie chart, the value between 0 and 1. Set the pie chart as a donut chart. Set the value to 0.6 or number in [0 ,1] to enable it.",
				},
				"theme":  getThemeProperty(),
				"width":  getWidthProperty(),
				"height": getHeightProperty(),
				"title":  getTitleProperty(),
			},
			Required: []string{"data"},
		},
	}
}

func (s *Server) createBarChartTool() mcp.Tool {
	return mcp.Tool{
		Name:        "generate_bar_chart",
		Description: "Generate a bar chart to show data for numerical comparisons among different categories, such as, comparing categorical data and for horizontal comparisons.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"data": map[string]any{
					"type":        "array",
					"description": "Data for bar chart, such as, [{ category: '分类一', value: 10 }].",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"category": map[string]any{"type": "string"},
							"value":    map[string]any{"type": "number"},
							"group":    map[string]any{"type": "string"},
						},
						"required": []string{"category", "value"},
					},
				},
				"group": map[string]any{
					"type":        "boolean",
					"default":     false,
					"description": "Whether grouping is enabled. When enabled, bar charts require a 'group' field in the data. When `group` is true, `stack` should be false.",
				},
				"stack": map[string]any{
					"type":        "boolean",
					"default":     true,
					"description": "Whether stacking is enabled. When enabled, bar charts require a 'group' field in the data. When `stack` is true, `group` should be false.",
				},
				"theme":      getThemeProperty(),
				"width":      getWidthProperty(),
				"height":     getHeightProperty(),
				"title":      getTitleProperty(),
				"axisXTitle": getAxisXTitleProperty(),
				"axisYTitle": getAxisYTitleProperty(),
			},
			Required: []string{"data"},
		},
	}
}

func (s *Server) createHistogramChartTool() mcp.Tool {
	return mcp.Tool{
		Name:        "generate_histogram_chart",
		Description: "Generate a histogram chart to show the frequency of data points within a certain range. It can observe data distribution, such as, normal and skewed distributions, and identify data concentration areas and extreme points.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"data": map[string]any{
					"type":        "array",
					"description": "Data for histogram chart, such as, [78, 88, 60, 100, 95].",
					"items":       map[string]any{"type": "number"},
				},
				"binNumber": map[string]any{
					"type":        "number",
					"description": "Number of intervals to define the number of intervals in a histogram.",
				},
				"theme":      getThemeProperty(),
				"width":      getWidthProperty(),
				"height":     getHeightProperty(),
				"title":      getTitleProperty(),
				"axisXTitle": getAxisXTitleProperty(),
				"axisYTitle": getAxisYTitleProperty(),
			},
			Required: []string{"data"},
		},
	}
}

func (s *Server) createScatterChartTool() mcp.Tool {
	return mcp.Tool{
		Name:        "generate_scatter_chart",
		Description: "Generate a scatter chart to show the relationship between two variables, helps discover their relationship or trends, such as, the strength of correlation, data distribution patterns.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"data": map[string]any{
					"type":        "array",
					"description": "Data for scatter chart, such as, [{ x: 10, y: 15 }].",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"x": map[string]any{"type": "number"},
							"y": map[string]any{"type": "number"},
						},
						"required": []string{"x", "y"},
					},
				},
				"theme":      getThemeProperty(),
				"width":      getWidthProperty(),
				"height":     getHeightProperty(),
				"title":      getTitleProperty(),
				"axisXTitle": getAxisXTitleProperty(),
				"axisYTitle": getAxisYTitleProperty(),
			},
			Required: []string{"data"},
		},
	}
}

func (s *Server) createWordCloudChartTool() mcp.Tool {
	return mcp.Tool{
		Name:        "generate_word_cloud_chart",
		Description: "Generate a word cloud chart to show word frequency or weight through text size variation, such as, analyzing common words in social media, reviews, or feedback.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"data": map[string]any{
					"type":        "array",
					"description": "Data for word cloud chart, such as, [{ value: 4.272, text: '形成' }].",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"text":  map[string]any{"type": "string"},
							"value": map[string]any{"type": "number"},
						},
						"required": []string{"text", "value"},
					},
				},
				"theme":  getThemeProperty(),
				"width":  getWidthProperty(),
				"height": getHeightProperty(),
				"title":  getTitleProperty(),
			},
			Required: []string{"data"},
		},
	}
}

func (s *Server) createRadarChartTool() mcp.Tool {
	return mcp.Tool{
		Name:        "generate_radar_chart",
		Description: "Generate a radar chart to display multidimensional data (four dimensions or more), such as, evaluate Huawei and Apple phones in terms of five dimensions: ease of use, functionality, camera, benchmark scores, and battery life.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"data": map[string]any{
					"type":        "array",
					"description": "Data for radar chart, such as, [{ name: 'Design', value: 70 }].",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name":  map[string]any{"type": "string"},
							"value": map[string]any{"type": "number"},
							"group": map[string]any{"type": "string"},
						},
						"required": []string{"name", "value"},
					},
				},
				"theme":  getThemeProperty(),
				"width":  getWidthProperty(),
				"height": getHeightProperty(),
				"title":  getTitleProperty(),
			},
			Required: []string{"data"},
		},
	}
}

func (s *Server) createTreemapChartTool() mcp.Tool {
	return mcp.Tool{
		Name:        "generate_treemap_chart",
		Description: "Generate a treemap chart to display hierarchical data and can intuitively show comparisons between items at the same level, such as, show disk space usage with treemap.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"data": map[string]any{
					"type":        "array",
					"description": "Data for treemap chart, such as, [{ name: 'Design', value: 70, children: [{ name: 'Tech', value: 20 }] }].",
				},
				"theme":  getThemeProperty(),
				"width":  getWidthProperty(),
				"height": getHeightProperty(),
				"title":  getTitleProperty(),
			},
			Required: []string{"data"},
		},
	}
}

func (s *Server) createDualAxesChartTool() mcp.Tool {
	return mcp.Tool{
		Name:        "generate_dual_axes_chart",
		Description: "Generate a dual axes chart which is a combination chart that integrates two different chart types, typically combining a bar chart with a line chart to display both the trend and comparison of data, such as, the trend of sales and profit over time.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"categories": map[string]any{
					"type":        "array",
					"description": "Categories for dual axes chart, such as, ['2015', '2016', '2017'].",
					"items":       map[string]any{"type": "string"},
				},
				"series": map[string]any{
					"type":        "array",
					"description": "Series for dual axes chart, such as, [{ type: 'column', data: [91.9, 99.1, 101.6, 114.4, 121],axisYTitle: '销售额' }, { type: 'line', data: [0.055, 0.06, 0.062, 0.07, 0.075], 'axisYTitle': '利润率' }].",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"type": map[string]any{
								"type": "string",
								"enum": []string{"column", "line"},
							},
							"data": map[string]any{
								"type":  "array",
								"items": map[string]any{"type": "number"},
							},
							"axisYTitle": map[string]any{"type": "string"},
						},
						"required": []string{"type", "data"},
					},
				},
				"theme":      getThemeProperty(),
				"width":      getWidthProperty(),
				"height":     getHeightProperty(),
				"title":      getTitleProperty(),
				"axisXTitle": getAxisXTitleProperty(),
			},
			Required: []string{"categories", "series"},
		},
	}
}

func (s *Server) createMindMapTool() mcp.Tool {
	return mcp.Tool{
		Name:        "generate_mind_map",
		Description: "Generate a mind map chart to organizes and presents information in a hierarchical structure with branches radiating from a central topic, such as, a diagram showing the relationship between a main topic and its subtopics.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"data": map[string]any{
					"type":        "object",
					"description": "Data for mind map chart, such as, { name: 'main topic', children: [{ name: 'topic 1', children: [{ name:'subtopic 1-1' }] }.",
				},
				"theme":  getThemeProperty(),
				"width":  getWidthProperty(),
				"height": getHeightProperty(),
			},
			Required: []string{"data"},
		},
	}
}

func (s *Server) createNetworkGraphTool() mcp.Tool {
	return mcp.Tool{
		Name:        "generate_network_graph",
		Description: "Generate a network graph chart to show relationships (edges) between entities (nodes), such as, relationships between people in social networks.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"data": map[string]any{
					"type":        "object",
					"description": "Data for network graph chart, such as, { nodes: [{ name: 'node1' }, { name: 'node2' }], edges: [{ source: 'node1', target: 'node2', name: 'edge1' }] }",
					"properties": map[string]any{
						"nodes": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"name": map[string]any{"type": "string"},
								},
								"required": []string{"name"},
							},
						},
						"edges": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"source": map[string]any{"type": "string"},
									"target": map[string]any{"type": "string"},
									"name":   map[string]any{"type": "string"},
								},
								"required": []string{"source", "target"},
							},
						},
					},
					"required": []string{"nodes", "edges"},
				},
				"theme":  getThemeProperty(),
				"width":  getWidthProperty(),
				"height": getHeightProperty(),
			},
			Required: []string{"data"},
		},
	}
}

func (s *Server) createFlowDiagramTool() mcp.Tool {
	return mcp.Tool{
		Name:        "generate_flow_diagram",
		Description: "Generate a flow diagram chart to show the steps and decision points of a process or system, such as, scenarios requiring linear process presentation.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"data": map[string]any{
					"type":        "object",
					"description": "Data for flow diagram chart, such as, { nodes: [{ name: 'node1' }, { name: 'node2' }], edges: [{ source: 'node1', target: 'node2', name: 'edge1' }] }.",
					"properties": map[string]any{
						"nodes": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"name": map[string]any{"type": "string"},
								},
								"required": []string{"name"},
							},
						},
						"edges": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"source": map[string]any{"type": "string"},
									"target": map[string]any{"type": "string"},
									"name":   map[string]any{"type": "string"},
								},
								"required": []string{"source", "target"},
							},
						},
					},
					"required": []string{"nodes", "edges"},
				},
				"theme":  getThemeProperty(),
				"width":  getWidthProperty(),
				"height": getHeightProperty(),
			},
			Required: []string{"data"},
		},
	}
}

func (s *Server) createFishboneDiagramTool() mcp.Tool {
	return mcp.Tool{
		Name:        "generate_fishbone_diagram",
		Description: "Generate a fishbone diagram chart to uses a fish skeleton, like structure to display the causes or effects of a core problem, with the problem as the fish head and the causes/effects as the fish bones. It suits problems that can be split into multiple related factors.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"data": map[string]any{
					"type":        "object",
					"description": "Data for fishbone diagram chart, such as, { name: 'main topic', children: [{ name: 'topic 1', children: [{ name: 'subtopic 1-1' }] }.",
				},
				"theme":  getThemeProperty(),
				"width":  getWidthProperty(),
				"height": getHeightProperty(),
			},
			Required: []string{"data"},
		},
	}
}

// Helper functions for common properties
func getThemeProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"enum":        []string{"default", "academy"},
		"default":     "default",
		"description": "Set the theme for the chart, optional, default is 'default'.",
	}
}

func getWidthProperty() map[string]any {
	return map[string]any{
		"type":        "number",
		"default":     600,
		"description": "Set the width of chart, default is 600.",
	}
}

func getHeightProperty() map[string]any {
	return map[string]any{
		"type":        "number",
		"default":     400,
		"description": "Set the height of chart, default is 400.",
	}
}

func getTitleProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"default":     "",
		"description": "Set the title of chart.",
	}
}

func getAxisXTitleProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"default":     "",
		"description": "Set the x-axis title of chart.",
	}
}

func getAxisYTitleProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"default":     "",
		"description": "Set the y-axis title of chart.",
	}
}
