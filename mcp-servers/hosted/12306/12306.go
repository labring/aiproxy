package train12306

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/server"
)

var configTemplates = mcpservers.ConfigTemplates{
	"user-agent": {
		Name:        "User Agent",
		Required:    mcpservers.ConfigRequiredTypeInitOptional,
		Example:     DefaultUserAgent,
		Description: "Custom User-Agent string to use for requests",
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

			if timeout < 5 || timeout > 120 {
				return errors.New("timeout must be between 5 and 120 seconds")
			}

			return nil
		},
	},
}

// Server represents the MCP server for 12306 train ticket queries
type Server struct {
	*server.MCPServer
	client           *http.Client
	userAgent        string
	stations         map[string]StationData
	cityStations     map[string][]StationInfo
	cityStationCodes map[string]StationInfo
	nameStations     map[string]StationInfo
	lcQueryPath      string
}

type StationInfo struct {
	StationCode string `json:"station_code"`
	StationName string `json:"station_name"`
}

// NewServer creates a new 12306 MCP server
func NewServer(config, _ map[string]string) (mcpservers.Server, error) {
	userAgent := config["user-agent"]
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}

	timeout := 30 * time.Second
	if timeoutStr := config["timeout"]; timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = time.Duration(t) * time.Second
		}
	}

	client := &http.Client{
		Timeout: timeout,
	}

	// Create MCP server
	mcpServer := server.NewMCPServer("12306-mcp", VERSION)

	trainServer := &Server{
		MCPServer: mcpServer,
		client:    client,
		userAgent: userAgent,
	}

	// Initialize stations and other data
	if err := trainServer.initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize server: %w", err)
	}

	// Add tools
	trainServer.addTools()

	return trainServer, nil
}

// initialize loads station data and other required information
func (s *Server) initialize(ctx context.Context) error {
	// Load stations
	stations, err := s.getStations(ctx)
	if err != nil {
		return fmt.Errorf("failed to load stations: %w", err)
	}

	s.stations = stations

	// Build station lookup maps
	s.buildStationMaps()

	// Get LC query path
	lcQueryPath, err := s.getLCQueryPath(ctx)
	if err != nil {
		return fmt.Errorf("failed to get LC query path: %w", err)
	}

	s.lcQueryPath = lcQueryPath

	return nil
}

// buildStationMaps builds various station lookup maps
func (s *Server) buildStationMaps() {
	s.cityStations = make(map[string][]StationInfo)
	s.cityStationCodes = make(map[string]StationInfo)
	s.nameStations = make(map[string]StationInfo)

	// Build city stations map
	for _, station := range s.stations {
		city := station.City
		stationInfo := StationInfo{
			StationCode: station.StationCode,
			StationName: station.StationName,
		}

		if s.cityStations[city] == nil {
			s.cityStations[city] = []StationInfo{}
		}

		s.cityStations[city] = append(s.cityStations[city], stationInfo)

		// Build name stations map
		s.nameStations[station.StationName] = stationInfo
	}

	// Build city codes map (representative station for each city)
	for city, stations := range s.cityStations {
		for _, station := range stations {
			if station.StationName == city {
				s.cityStationCodes[city] = station
				break
			}
		}
		// If no station matches city name, use the first one
		if _, exists := s.cityStationCodes[city]; !exists && len(stations) > 0 {
			s.cityStationCodes[city] = stations[0]
		}
	}
}

// addTools adds all the tools to the server
func (s *Server) addTools() {
	s.addGetCurrentDateTool()
	s.addGetStationsCodeInCityTool()
	s.addGetStationCodeOfCitysTool()
	s.addGetStationCodeByNamesTool()
	s.addGetStationByTelecodeTool()
	s.addGetTicketsTool()
	s.addGetInterlineTicketsTool()
	s.addGetTrainRouteStationsTool()
}
