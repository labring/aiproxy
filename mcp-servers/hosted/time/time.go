package time

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Result represents the result of a time query
type Result struct {
	Timezone string `json:"timezone"`
	DateTime string `json:"datetime"`
	IsDST    bool   `json:"is_dst"`
}

// ConversionResult represents the result of a time conversion
type ConversionResult struct {
	Source         Result `json:"source"`
	Target         Result `json:"target"`
	TimeDifference string `json:"time_difference"`
}

// Configuration templates for the time server
var configTemplates = map[string]mcpservers.ConfigTemplate{
	"local_timezone": {
		Name:        "Local Timezone",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     "America/New_York",
		Description: "Override for local timezone (IANA timezone name)",
		Validator: func(value string) error {
			_, err := time.LoadLocation(value)
			return err
		},
	},
}

// getLocalTimezone returns the local timezone
func getLocalTimezone(override string) (*time.Location, error) {
	if override != "" {
		return time.LoadLocation(override)
	}

	// Get local timezone
	return time.Local, nil
}

// getTimezone returns a timezone by name
func getTimezone(timezoneName string) (*time.Location, error) {
	loc, err := time.LoadLocation(timezoneName)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone: %w", err)
	}

	return loc, nil
}

// getCurrentTime gets current time in specified timezone
func getCurrentTime(timezoneName string) (*Result, error) {
	timezone, err := getTimezone(timezoneName)
	if err != nil {
		return nil, err
	}

	currentTime := time.Now().In(timezone)

	// Check if DST is in effect
	_, offset := currentTime.Zone()
	stdTime := time.Date(currentTime.Year(), time.January, 1, 12, 0, 0, 0, timezone)
	_, stdOffset := stdTime.Zone()
	isDST := offset != stdOffset

	return &Result{
		Timezone: timezoneName,
		DateTime: currentTime.Format(time.RFC3339),
		IsDST:    isDST,
	}, nil
}

// convertTime converts time between timezones
func convertTime(sourceTimezone, timeStr, targetTimezone string) (*ConversionResult, error) {
	sourceTZ, err := getTimezone(sourceTimezone)
	if err != nil {
		return nil, err
	}

	targetTZ, err := getTimezone(targetTimezone)
	if err != nil {
		return nil, err
	}

	// Parse time string (expecting HH:MM format)
	parsedTime, err := time.Parse("15:04", timeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid time format. Expected HH:MM [24-hour format]: %w", err)
	}

	// Create source time using current date
	now := time.Now().In(sourceTZ)
	sourceTime := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		parsedTime.Hour(),
		parsedTime.Minute(),
		0,
		0,
		sourceTZ,
	)

	// Convert to target timezone
	targetTime := sourceTime.In(targetTZ)

	// Calculate time difference
	_, sourceOffset := sourceTime.Zone()
	_, targetOffset := targetTime.Zone()
	hoursDifference := float64(targetOffset-sourceOffset) / 3600.0

	// Format time difference
	var timeDiffStr string
	if hoursDifference == float64(int(hoursDifference)) {
		timeDiffStr = fmt.Sprintf("%+.1fh", hoursDifference)
	} else {
		timeDiffStr = strings.TrimRight(
			strings.TrimRight(fmt.Sprintf("%+.2f", hoursDifference), "0"),
			".",
		) + "h"
	}

	// Check DST for source
	_, sourceStdOffset := time.Date(sourceTime.Year(), time.January, 1, 12, 0, 0, 0, sourceTZ).
		Zone()
	sourceIsDST := sourceOffset != sourceStdOffset

	// Check DST for target
	_, targetStdOffset := time.Date(targetTime.Year(), time.January, 1, 12, 0, 0, 0, targetTZ).
		Zone()
	targetIsDST := targetOffset != targetStdOffset

	return &ConversionResult{
		Source: Result{
			Timezone: sourceTimezone,
			DateTime: sourceTime.Format(time.RFC3339),
			IsDST:    sourceIsDST,
		},
		Target: Result{
			Timezone: targetTimezone,
			DateTime: targetTime.Format(time.RFC3339),
			IsDST:    targetIsDST,
		},
		TimeDifference: timeDiffStr,
	}, nil
}

// NewServer creates a new MCP server for time functionality
func NewServer(config, _ map[string]string) (mcpservers.Server, error) {
	// Get local timezone
	localTZ, err := getLocalTimezone(config["local_timezone"])
	if err != nil {
		return nil, fmt.Errorf("failed to get local timezone: %w", err)
	}

	localTZName := localTZ.String()

	// Create MCP server
	mcpServer := server.NewMCPServer("mcp-time", "1.0.0")

	// Add get_current_time tool
	getCurrentTimeTool := mcp.Tool{
		Name:        "get_current_time",
		Description: "Get current time in a specific timezone",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"timezone": map[string]any{
					"type": "string",
					"description": fmt.Sprintf(
						"IANA timezone name (e.g., 'America/New_York', 'Europe/London'). Use '%s' as local timezone if no timezone provided by the user.",
						localTZName,
					),
				},
			},
			Required: []string{"timezone"},
		},
	}

	mcpServer.AddTool(
		getCurrentTimeTool,
		func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := request.GetArguments()

			timezone, ok := args["timezone"].(string)
			if !ok || timezone == "" {
				return nil, errors.New("missing required argument: timezone")
			}

			result, err := getCurrentTime(timezone)
			if err != nil {
				return nil, fmt.Errorf("error processing mcp-server-time query: %w", err)
			}

			resultJSON, err := sonic.MarshalIndent(result, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("failed to marshal result: %w", err)
			}

			return mcp.NewToolResultText(string(resultJSON)), nil
		},
	)

	// Add convert_time tool
	convertTimeTool := mcp.Tool{
		Name:        "convert_time",
		Description: "Convert time between timezones",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"source_timezone": map[string]any{
					"type": "string",
					"description": fmt.Sprintf(
						"Source IANA timezone name (e.g., 'America/New_York', 'Europe/London'). Use '%s' as local timezone if no source timezone provided by the user.",
						localTZName,
					),
				},
				"time": map[string]any{
					"type":        "string",
					"description": "Time to convert in 24-hour format (HH:MM)",
				},
				"target_timezone": map[string]any{
					"type": "string",
					"description": fmt.Sprintf(
						"Target IANA timezone name (e.g., 'Asia/Tokyo', 'America/San_Francisco'). Use '%s' as local timezone if no target timezone provided by the user.",
						localTZName,
					),
				},
			},
			Required: []string{"source_timezone", "time", "target_timezone"},
		},
	}

	mcpServer.AddTool(
		convertTimeTool,
		func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := request.GetArguments()

			sourceTimezone, ok := args["source_timezone"].(string)
			if !ok || sourceTimezone == "" {
				return nil, errors.New("missing required argument: source_timezone")
			}

			timeStr, ok := args["time"].(string)
			if !ok || timeStr == "" {
				return nil, errors.New("missing required argument: time")
			}

			targetTimezone, ok := args["target_timezone"].(string)
			if !ok || targetTimezone == "" {
				return nil, errors.New("missing required argument: target_timezone")
			}

			result, err := convertTime(sourceTimezone, timeStr, targetTimezone)
			if err != nil {
				return nil, fmt.Errorf("error processing mcp-server-time query: %w", err)
			}

			resultJSON, err := sonic.MarshalIndent(result, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("failed to marshal result: %w", err)
			}

			return mcp.NewToolResultText(string(resultJSON)), nil
		},
	)

	return mcpServer, nil
}
