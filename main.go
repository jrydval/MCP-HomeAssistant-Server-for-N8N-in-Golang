package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/gorilla/websocket"
)

// Configuration structures
type Config struct {
	HAToken         string   `json:"ha_token"`
	HAURL           string   `json:"ha_url"`
	EntityFilter    []string `json:"entity_filter,omitempty"`
	EntityBlacklist []string `json:"entity_blacklist,omitempty"`
}

// WebSocket message structures for Home Assistant
type WSMessage struct {
	ID          int                    `json:"id,omitempty"`
	Type        string                 `json:"type"`
	AccessToken string                 `json:"access_token,omitempty"`
	Success     bool                   `json:"success,omitempty"`
	Result      interface{}           `json:"result,omitempty"`
	Error       map[string]interface{} `json:"error,omitempty"`
}

// WebSocket client for Home Assistant
func (h *HAService) getAreasViaWebSocket() ([]HAArea, error) {
	h.logger.Println("Attempting to get areas via WebSocket")
	
	// Parse WebSocket URL
	wsURL := strings.Replace(h.config.HAURL, "http", "ws", 1) + "/api/websocket"
	h.logger.Printf("Connecting to WebSocket: %s", wsURL)
	
	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		h.logger.Printf("WebSocket connection failed: %v", err)
		return nil, err
	}
	defer conn.Close()
	
	// Read initial auth required message
	_, message, err := conn.ReadMessage()
	if err != nil {
		h.logger.Printf("Failed to read initial message: %v", err)
		return nil, err
	}
	
	var authRequired WSMessage
	if err := json.Unmarshal(message, &authRequired); err != nil {
		h.logger.Printf("Failed to parse initial message: %v", err)
		return nil, err
	}
	
	h.logger.Printf("Received auth required message: %s", authRequired.Type)
	
	// Send authentication
	authMsg := WSMessage{
		Type:        "auth",
		AccessToken: h.config.HAToken,
	}
	
	if err := conn.WriteJSON(authMsg); err != nil {
		h.logger.Printf("Failed to send auth: %v", err)
		return nil, err
	}
	
	// Read auth response
	_, message, err = conn.ReadMessage()
	if err != nil {
		h.logger.Printf("Failed to read auth response: %v", err)
		return nil, err
	}
	
	var authResponse WSMessage
	if err := json.Unmarshal(message, &authResponse); err != nil {
		h.logger.Printf("Failed to parse auth response: %v", err)
		return nil, err
	}
	
	if authResponse.Type != "auth_ok" {
		h.logger.Printf("Authentication failed: %+v", authResponse)
		return nil, fmt.Errorf("authentication failed")
	}
	
	h.logger.Println("WebSocket authentication successful")
	
	// Request area registry
	areaRequest := WSMessage{
		ID:   1,
		Type: "config/area_registry/list",
	}
	
	if err := conn.WriteJSON(areaRequest); err != nil {
		h.logger.Printf("Failed to send area request: %v", err)
		return nil, err
	}
	
	// Read area registry response
	_, message, err = conn.ReadMessage()
	if err != nil {
		h.logger.Printf("Failed to read area response: %v", err)
		return nil, err
	}
	
	var areaResponse WSMessage
	if err := json.Unmarshal(message, &areaResponse); err != nil {
		h.logger.Printf("Failed to parse area response: %v", err)
		return nil, err
	}
	
	if !areaResponse.Success {
		h.logger.Printf("Area request failed: %+v", areaResponse.Error)
		return nil, fmt.Errorf("area request failed")
	}
	
	// Parse areas from result
	resultBytes, err := json.Marshal(areaResponse.Result)
	if err != nil {
		h.logger.Printf("Failed to marshal area result: %v", err)
		return nil, err
	}
	
	var areas []HAArea
	if err := json.Unmarshal(resultBytes, &areas); err != nil {
		h.logger.Printf("Failed to parse areas: %v", err)
		return nil, err
	}
	
	h.logger.Printf("Successfully retrieved %d areas via WebSocket", len(areas))
	return areas, nil
}

// WebSocket method to get device registry
func (h *HAService) getDevicesViaWebSocket() ([]HADevice, error) {
	h.logger.Println("Attempting to get devices via WebSocket")
	
	// Parse WebSocket URL
	wsURL := strings.Replace(h.config.HAURL, "http", "ws", 1) + "/api/websocket"
	
	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		h.logger.Printf("WebSocket connection failed: %v", err)
		return nil, err
	}
	defer conn.Close()
	
	// Read initial message and authenticate
	if err := h.authenticateWebSocket(conn); err != nil {
		return nil, err
	}
	
	// Request device registry
	deviceRequest := WSMessage{
		ID:   2,
		Type: "config/device_registry/list",
	}
	
	if err := conn.WriteJSON(deviceRequest); err != nil {
		h.logger.Printf("Failed to send device request: %v", err)
		return nil, err
	}
	
	// Read device registry response
	_, message, err := conn.ReadMessage()
	if err != nil {
		h.logger.Printf("Failed to read device response: %v", err)
		return nil, err
	}
	
	var deviceResponse WSMessage
	if err := json.Unmarshal(message, &deviceResponse); err != nil {
		h.logger.Printf("Failed to parse device response: %v", err)
		return nil, err
	}
	
	if !deviceResponse.Success {
		h.logger.Printf("Device request failed: %+v", deviceResponse.Error)
		return nil, fmt.Errorf("device request failed")
	}
	
	// Parse devices from result
	resultBytes, err := json.Marshal(deviceResponse.Result)
	if err != nil {
		h.logger.Printf("Failed to marshal device result: %v", err)
		return nil, err
	}
	
	var devices []HADevice
	if err := json.Unmarshal(resultBytes, &devices); err != nil {
		h.logger.Printf("Failed to parse devices: %v", err)
		return nil, err
	}
	
	h.logger.Printf("Successfully retrieved %d devices via WebSocket", len(devices))
	return devices, nil
}

// WebSocket method to get entity registry
func (h *HAService) getEntityRegistryViaWebSocket() ([]HAEntity, error) {
	h.logger.Println("Attempting to get entity registry via WebSocket")
	
	// Parse WebSocket URL
	wsURL := strings.Replace(h.config.HAURL, "http", "ws", 1) + "/api/websocket"
	
	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		h.logger.Printf("WebSocket connection failed: %v", err)
		return nil, err
	}
	defer conn.Close()
	
	// Read initial message and authenticate
	if err := h.authenticateWebSocket(conn); err != nil {
		return nil, err
	}
	
	// Request entity registry
	entityRequest := WSMessage{
		ID:   3,
		Type: "config/entity_registry/list",
	}
	
	if err := conn.WriteJSON(entityRequest); err != nil {
		h.logger.Printf("Failed to send entity request: %v", err)
		return nil, err
	}
	
	// Read entity registry response
	_, message, err := conn.ReadMessage()
	if err != nil {
		h.logger.Printf("Failed to read entity response: %v", err)
		return nil, err
	}
	
	var entityResponse WSMessage
	if err := json.Unmarshal(message, &entityResponse); err != nil {
		h.logger.Printf("Failed to parse entity response: %v", err)
		return nil, err
	}
	
	if !entityResponse.Success {
		h.logger.Printf("Entity request failed: %+v", entityResponse.Error)
		return nil, fmt.Errorf("entity request failed")
	}
	
	// Parse entities from result
	resultBytes, err := json.Marshal(entityResponse.Result)
	if err != nil {
		h.logger.Printf("Failed to marshal entity result: %v", err)
		return nil, err
	}
	
	var entities []HAEntity
	if err := json.Unmarshal(resultBytes, &entities); err != nil {
		h.logger.Printf("Failed to parse entities: %v", err)
		return nil, err
	}
	
	h.logger.Printf("Successfully retrieved %d entities via WebSocket", len(entities))
	return entities, nil
}

// Helper function to handle WebSocket authentication
func (h *HAService) authenticateWebSocket(conn *websocket.Conn) error {
	// Read initial auth required message
	_, message, err := conn.ReadMessage()
	if err != nil {
		h.logger.Printf("Failed to read initial message: %v", err)
		return err
	}
	
	var authRequired WSMessage
	if err := json.Unmarshal(message, &authRequired); err != nil {
		h.logger.Printf("Failed to parse initial message: %v", err)
		return err
	}
	
	// Send authentication
	authMsg := WSMessage{
		Type:        "auth",
		AccessToken: h.config.HAToken,
	}
	
	if err := conn.WriteJSON(authMsg); err != nil {
		h.logger.Printf("Failed to send auth: %v", err)
		return err
	}
	
	// Read auth response
	_, message, err = conn.ReadMessage()
	if err != nil {
		h.logger.Printf("Failed to read auth response: %v", err)
		return err
	}
	
	var authResponse WSMessage
	if err := json.Unmarshal(message, &authResponse); err != nil {
		h.logger.Printf("Failed to parse auth response: %v", err)
		return err
	}
	
	if authResponse.Type != "auth_ok" {
		h.logger.Printf("Authentication failed: %+v", authResponse)
		return fmt.Errorf("authentication failed")
	}
	
	return nil
}

// Helper functions for better area detection
func isCommonAreaWord(word string) bool {
	lowerWord := strings.ToLower(word)
	commonAreaWords := []string{
		"room", "bedroom", "bathroom", "kitchen", "office",
		"living", "dining", "family", "master", "guest",
		"hall", "hallway", "entrance", "foyer", "lobby",
		"garage", "basement", "attic", "closet", "storage",
		"porch", "patio", "deck", "balcony", "terrace",
	}
	
	for _, commonWord := range commonAreaWords {
		if lowerWord == commonWord {
			return true
		}
	}
	return false
}

func isDeviceName(name string) bool {
	lowerName := strings.ToLower(name)
	deviceNames := []string{
		"lolin", "nodemcu", "esp", "arduino", "sonoff", 
		"shelly", "zigbee", "zwave", "wifi", "bluetooth",
		"sensor", "switch", "light", "lamp", "bulb",
		"device", "module", "controller", "hub",
	}
	
	for _, deviceName := range deviceNames {
		if strings.Contains(lowerName, deviceName) {
			return true
		}
	}
	return false
}

// Home Assistant structures
type HAState struct {
	EntityID    string                 `json:"entity_id"`
	State       string                 `json:"state"`
	Attributes  map[string]interface{} `json:"attributes"`
	LastChanged string                 `json:"last_changed"`
	LastUpdated string                 `json:"last_updated"`
	Area        *HAArea                `json:"area,omitempty"`
}

type HAArea struct {
	AreaID   string   `json:"area_id"`
	Name     string   `json:"name"`
	Picture  string   `json:"picture,omitempty"`
	Aliases  []string `json:"aliases,omitempty"`
}

type HADevice struct {
	ID     string `json:"id"`
	AreaID string `json:"area_id,omitempty"`
	Name   string `json:"name"`
}

type HAEntity struct {
	EntityID string `json:"entity_id"`
	DeviceID string `json:"device_id,omitempty"`
	AreaID   string `json:"area_id,omitempty"`
}

// Home Assistant Service
type HAService struct {
	config       Config
	httpClient   *http.Client
	logger       *log.Logger
	mu           sync.Mutex
	executableDir string
}

func NewHAService() *HAService {
	// Get the directory where the executable is located
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not get executable path: %v\n", err)
		execPath = "."
	}
	executableDir := filepath.Dir(execPath)
	
	// Setup logging in the executable directory
	logFilePath := filepath.Join(executableDir, "ha-mcp.log")
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	var logger *log.Logger
	if err != nil {
		// Fallback to stderr if can't open log file
		fmt.Fprintf(os.Stderr, "Warning: Could not open log file %s: %v\n", logFilePath, err)
		logger = log.New(os.Stderr, "[HA-MCP] ", log.LstdFlags|log.Lshortfile)
	} else {
		logger = log.New(logFile, "[HA-MCP] ", log.LstdFlags|log.Lshortfile)
	}

	// HTTP client with connection pooling
	transport := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     30 * time.Second,
		DisableKeepAlives:   false,
	}

	service := &HAService{
		httpClient: &http.Client{
			Timeout:   8 * time.Second,
			Transport: transport,
		},
		logger:        logger,
		executableDir: executableDir,
	}

	service.logger.Printf("HA Service initialized, executable directory: %s", executableDir)
	service.logger.Printf("Log file: %s", logFilePath)
	return service
}

func (h *HAService) LoadConfig() error {
	h.logger.Println("Loading configuration...")
	
	// Try environment variables first
	token := os.Getenv("HA_TOKEN")
	url := os.Getenv("HA_URL")

	if token != "" && url != "" {
		h.config.HAToken = token
		h.config.HAURL = strings.TrimSuffix(url, "/")

		// Load entity filter from environment if available
		filterStr := os.Getenv("HA_ENTITY_FILTER")
		if filterStr != "" {
			h.config.EntityFilter = strings.Split(filterStr, ",")
		}

		// Load entity blacklist from environment if available
		blacklistStr := os.Getenv("HA_ENTITY_BLACKLIST")
		if blacklistStr != "" {
			h.config.EntityBlacklist = strings.Split(blacklistStr, ",")
		}
		
		h.logger.Printf("Configuration loaded from environment variables")
		return nil
	}

	// Fallback to config file in executable directory
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = filepath.Join(h.executableDir, "config.json")
	} else {
		// If CONFIG_FILE is relative path, make it relative to executable directory
		if !filepath.IsAbs(configFile) {
			configFile = filepath.Join(h.executableDir, configFile)
		}
	}

	h.logger.Printf("Looking for config file: %s", configFile)

	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %v", configFile, err)
	}

	if err := json.Unmarshal(data, &h.config); err != nil {
		return fmt.Errorf("failed to parse config file %s: %v", configFile, err)
	}

	h.config.HAURL = strings.TrimSuffix(h.config.HAURL, "/")
	h.logger.Printf("Configuration loaded from file: %s", configFile)
	return nil
}

func (h *HAService) makeHARequest(method, endpoint string, body interface{}) (*http.Response, error) {
	url := h.config.HAURL + endpoint
	
	// Debug logging
	h.logger.Printf("Making %s request to: %s", method, url)

	var req *http.Request
	var err error

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		req, err = http.NewRequest(method, url, strings.NewReader(string(jsonBody)))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, url, nil)
		if err != nil {
			return nil, err
		}
	}

	req.Header.Set("Authorization", "Bearer "+h.config.HAToken)
	
	// Debug logging
	h.logger.Printf("Request headers: %+v", req.Header)
	
	resp, err := h.httpClient.Do(req)
	if err != nil {
		h.logger.Printf("HTTP request failed: %v", err)
		return nil, err
	}
	
	// Debug logging
	h.logger.Printf("Response status: %d %s", resp.StatusCode, resp.Status)
	
	return resp, nil
}

func (h *HAService) isEntityBlacklisted(entityID string) bool {
	for _, pattern := range h.config.EntityBlacklist {
		// Try exact match first
		if pattern == entityID {
			return true
		}

		// Try regex match
		matched, err := regexp.MatchString(pattern, entityID)
		if err == nil && matched {
			return true
		}
	}
	return false
}

func (h *HAService) isEntityWhitelisted(entityID string) bool {
	for _, pattern := range h.config.EntityFilter {
		matched, err := regexp.MatchString(pattern, entityID)
		if err == nil && matched {
			return true
		}
	}
	return false
}

func (h *HAService) filterEntities(entities []HAState) []HAState {
	var filtered []HAState

	for _, entity := range entities {
		// Check if entity is blacklisted
		if h.isEntityBlacklisted(entity.EntityID) {
			continue
		}

		// If no whitelist filter is defined, include entity
		if len(h.config.EntityFilter) == 0 {
			filtered = append(filtered, entity)
			continue
		}

		// Check against whitelist filter
		if h.isEntityWhitelisted(entity.EntityID) {
			filtered = append(filtered, entity)
		}
	}

	return filtered
}

// Internal functions for area enrichment
func (h *HAService) getAreas() ([]HAArea, error) {
	h.logger.Println("Fetching areas from HA")
	
	// First try WebSocket API (most reliable)
	areas, err := h.getAreasViaWebSocket()
	if err == nil && len(areas) > 0 {
		h.logger.Printf("Successfully got %d areas via WebSocket", len(areas))
		return areas, nil
	}
	
	h.logger.Printf("WebSocket failed (%v), trying REST endpoints", err)
	
	// Fallback to REST endpoints
	endpoints := []string{
		"/api/config/area_registry",
		"/api/areas",
	}
	
	for _, endpoint := range endpoints {
		h.logger.Printf("Trying endpoint: %s", endpoint)
		resp, err := h.makeHARequest("GET", endpoint, nil)
		if err != nil {
			h.logger.Printf("Failed to get areas from %s: %v", endpoint, err)
			continue
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == 200 {
			h.logger.Printf("Success! Endpoint %s returned 200", endpoint)
			
			// Try to decode as JSON
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				h.logger.Printf("Failed to read response body from %s: %v", endpoint, err)
				continue
			}
			
			var areas []HAArea
			if err := json.Unmarshal(bodyBytes, &areas); err != nil {
				h.logger.Printf("Failed to decode areas from %s: %v", endpoint, err)
				continue
			}
			h.logger.Printf("Found %d areas from %s", len(areas), endpoint)
			return areas, nil
		} else {
			h.logger.Printf("Endpoint %s returned status %d", endpoint, resp.StatusCode)
		}
	}
	
	h.logger.Printf("All REST endpoints failed, falling back to states extraction")
	// As last resort, try to extract area info from states attributes
	return h.extractAreasFromStates()
}

// Fallback method to extract areas from entity states attributes
func (h *HAService) extractAreasFromStates() ([]HAArea, error) {
	h.logger.Println("Extracting areas from entity states")
	
	resp, err := h.makeHARequest("GET", "/api/states", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HA API returned status %d for states", resp.StatusCode)
	}

	var states []HAState
	if err := json.NewDecoder(resp.Body).Decode(&states); err != nil {
		return nil, err
	}

	// Extract unique areas from entity attributes
	areasMap := make(map[string]*HAArea)
	for _, state := range states {
		// Skip non-light/switch entities for area extraction
		if !strings.HasPrefix(state.EntityID, "light.") && !strings.HasPrefix(state.EntityID, "switch.") {
			continue
		}
		
		// Check for explicit area attribute first
		if areaName, hasArea := state.Attributes["area"]; hasArea {
			if areaStr, ok := areaName.(string); ok && areaStr != "" {
				areaID := strings.ReplaceAll(strings.ToLower(areaStr), " ", "_")
				if _, exists := areasMap[areaID]; !exists {
					areasMap[areaID] = &HAArea{
						AreaID: areaID,
						Name:   areaStr,
					}
				}
			}
		}
		
		// Try to extract area from friendly name patterns
		if friendlyName, hasFriendly := state.Attributes["friendly_name"]; hasFriendly {
			if nameStr, ok := friendlyName.(string); ok {
				// Look for common area patterns in friendly names
				// Examples: "Workshop Light", "Kitchen Switch", "Living Room Lamp"
				parts := strings.Split(nameStr, " ")
				if len(parts) >= 2 {
					var possibleArea string
					
					// Check for two-word areas like "Living Room", "Master Bedroom"
					if len(parts) >= 3 && isCommonAreaWord(parts[1]) {
						possibleArea = parts[0] + " " + parts[1]
					} else {
						// Single word area
						possibleArea = parts[0]
					}
					
					// Only consider meaningful area names (avoid device names)
					if len(possibleArea) > 3 && !isDeviceName(possibleArea) {
						areaID := strings.ReplaceAll(strings.ToLower(possibleArea), " ", "_")
						if _, exists := areasMap[areaID]; !exists {
							areasMap[areaID] = &HAArea{
								AreaID: areaID,
								Name:   possibleArea,
							}
						}
					}
				}
			}
		}
	}

	// Convert map to slice
	var areas []HAArea
	for _, area := range areasMap {
		areas = append(areas, *area)
	}

	h.logger.Printf("Extracted %d areas from entity states", len(areas))
	return areas, nil
}

func (h *HAService) getDevices() ([]HADevice, error) {
	h.logger.Println("Fetching devices from HA")
	
	// First try WebSocket API
	devicesWS, err := h.getDevicesViaWebSocket()
	if err == nil && len(devicesWS) >= 0 { // Accept empty result as valid
		h.logger.Printf("Successfully got %d devices via WebSocket", len(devicesWS))
		return devicesWS, nil
	}
	
	h.logger.Printf("WebSocket failed (%v), trying REST endpoint", err)
	
	resp, err := h.makeHARequest("GET", "/api/config/device_registry", nil)
	if err != nil {
		h.logger.Printf("Failed to get devices: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		h.logger.Printf("HA API returned status %d for devices, skipping device registry", resp.StatusCode)
		return []HADevice{}, nil // Return empty slice instead of error
	}

	var devices []HADevice
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		return nil, err
	}

	h.logger.Printf("Found %d devices", len(devices))
	return devices, nil
}

func (h *HAService) getEntityRegistry() ([]HAEntity, error) {
	h.logger.Println("Fetching entity registry from HA")
	
	// First try WebSocket API
	entitiesWS, err := h.getEntityRegistryViaWebSocket()
	if err == nil && len(entitiesWS) >= 0 { // Accept empty result as valid
		h.logger.Printf("Successfully got %d entities via WebSocket", len(entitiesWS))
		return entitiesWS, nil
	}
	
	h.logger.Printf("WebSocket failed (%v), trying REST endpoint", err)
	
	resp, err := h.makeHARequest("GET", "/api/config/entity_registry", nil)
	if err != nil {
		h.logger.Printf("Failed to get entity registry: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		h.logger.Printf("HA API returned status %d for entity registry, falling back to states-based area matching", resp.StatusCode)
		return h.extractEntityAreaFromStates()
	}

	var entities []HAEntity
	if err := json.NewDecoder(resp.Body).Decode(&entities); err != nil {
		return nil, err
	}

	h.logger.Printf("Found %d entities in registry", len(entities))
	return entities, nil
}

// Fallback method to create entity-area mappings from states
func (h *HAService) extractEntityAreaFromStates() ([]HAEntity, error) {
	h.logger.Println("Extracting entity-area mappings from states")
	
	resp, err := h.makeHARequest("GET", "/api/states", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HA API returned status %d for states", resp.StatusCode)
	}

	var states []HAState
	if err := json.NewDecoder(resp.Body).Decode(&states); err != nil {
		return nil, err
	}

	// Create entity mappings based on friendly names and patterns
	var entities []HAEntity
	for _, state := range states {
		// Skip non-light/switch entities
		if !strings.HasPrefix(state.EntityID, "light.") && !strings.HasPrefix(state.EntityID, "switch.") {
			continue
		}
		
		entity := HAEntity{
			EntityID: state.EntityID,
		}
		
		// Try to extract area from friendly name patterns
		if friendlyName, hasFriendly := state.Attributes["friendly_name"]; hasFriendly {
			if nameStr, ok := friendlyName.(string); ok {
				parts := strings.Split(nameStr, " ")
				if len(parts) >= 2 {
					var possibleArea string
					
					// Check for two-word areas like "Living Room", "Master Bedroom"
					if len(parts) >= 3 && isCommonAreaWord(parts[1]) {
						possibleArea = parts[0] + " " + parts[1]
					} else {
						// Single word area
						possibleArea = parts[0]
					}
					
					// Only consider meaningful area names (avoid device names)
					if len(possibleArea) > 3 && !isDeviceName(possibleArea) {
						entity.AreaID = strings.ReplaceAll(strings.ToLower(possibleArea), " ", "_")
					}
				}
			}
		}
		
		entities = append(entities, entity)
	}

	h.logger.Printf("Extracted %d entity-area mappings from states", len(entities))
	return entities, nil
}

// Cache for area enrichment data
type AreaEnrichmentCache struct {
	areas      map[string]*HAArea
	devices    map[string]string // device_id -> area_id
	entities   map[string]string // entity_id -> area_id
	lastUpdate time.Time
	mu         sync.RWMutex
}

var areaCache = &AreaEnrichmentCache{
	areas:    make(map[string]*HAArea),
	devices:  make(map[string]string),
	entities: make(map[string]string),
}

func (h *HAService) updateAreaCache() error {
	areaCache.mu.Lock()
	defer areaCache.mu.Unlock()

	// Update cache every 5 minutes
	if time.Since(areaCache.lastUpdate) < 5*time.Minute {
		return nil
	}

	h.logger.Println("Updating area cache")

	// Get areas (with fallbacks)
	areas, err := h.getAreas()
	if err != nil {
		h.logger.Printf("Warning: Could not update areas cache: %v", err)
		// Don't return error, continue with empty areas
		areas = []HAArea{}
	}

	// Clear and rebuild areas map
	areaCache.areas = make(map[string]*HAArea)
	for i := range areas {
		areaCache.areas[areas[i].AreaID] = &areas[i]
	}

	// Get devices (with fallbacks)
	devices, err := h.getDevices()
	if err != nil {
		h.logger.Printf("Warning: Could not update devices cache: %v", err)
		// Don't return error, continue with empty devices
		devices = []HADevice{}
	}

	// Clear and rebuild devices map
	areaCache.devices = make(map[string]string)
	for _, device := range devices {
		if device.AreaID != "" {
			areaCache.devices[device.ID] = device.AreaID
		}
	}

	// Get entity registry (with fallbacks)
	entities, err := h.getEntityRegistry()
	if err != nil {
		h.logger.Printf("Warning: Could not update entity registry cache: %v", err)
		// Don't return error, continue with empty entities
		entities = []HAEntity{}
	}

	// Clear and rebuild entities map
	areaCache.entities = make(map[string]string)
	for _, entity := range entities {
		// Direct area assignment
		if entity.AreaID != "" {
			areaCache.entities[entity.EntityID] = entity.AreaID
		} else if entity.DeviceID != "" {
			// Area through device
			if deviceAreaID, exists := areaCache.devices[entity.DeviceID]; exists {
				areaCache.entities[entity.EntityID] = deviceAreaID
			}
		}
	}

	areaCache.lastUpdate = time.Now()
	h.logger.Printf("Area cache updated: %d areas, %d devices, %d entities", len(areaCache.areas), len(areaCache.devices), len(areaCache.entities))
	return nil
}

func (h *HAService) enrichWithArea(states []HAState) []HAState {
	// Update cache if needed - never fail, just log warnings
	h.updateAreaCache()

	areaCache.mu.RLock()
	defer areaCache.mu.RUnlock()

	// If we have no area information, just return original states
	if len(areaCache.areas) == 0 && len(areaCache.entities) == 0 {
		h.logger.Println("No area information available, returning states without area info")
		return states
	}

	// Enrich states with area information
	enrichedCount := 0
	for i := range states {
		if areaID, exists := areaCache.entities[states[i].EntityID]; exists {
			if area, areaExists := areaCache.areas[areaID]; areaExists {
				states[i].Area = area
				enrichedCount++
			}
		}
	}

	h.logger.Printf("Enriched %d out of %d entities with area information", enrichedCount, len(states))
	return states
}

func (h *HAService) getAllStates() ([]HAState, error) {
	h.logger.Println("Fetching all states from HA")
	
	resp, err := h.makeHARequest("GET", "/api/states", nil)
	if err != nil {
		h.logger.Printf("Failed to get states: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		h.logger.Printf("HA API returned status %d", resp.StatusCode)
		return nil, fmt.Errorf("HA API returned status %d", resp.StatusCode)
	}

	var states []HAState
	if err := json.NewDecoder(resp.Body).Decode(&states); err != nil {
		return nil, err
	}

	// Filter for lights and switches only
	var filtered []HAState
	for _, state := range states {
		if strings.HasPrefix(state.EntityID, "light.") || strings.HasPrefix(state.EntityID, "switch.") {
			filtered = append(filtered, state)
		}
	}

	result := h.filterEntities(filtered)
	
	// Enrich with area information
	result = h.enrichWithArea(result)
	
	h.logger.Printf("Returning %d filtered entities with area info", len(result))
	return result, nil
}

func (h *HAService) getEntityState(entityID string) (*HAState, error) {
	h.logger.Printf("Getting state for entity: %s", entityID)
	
	resp, err := h.makeHARequest("GET", "/api/states/"+entityID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("entity %s not found", entityID)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HA API returned status %d", resp.StatusCode)
	}

	var state HAState
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, err
	}

	// Enrich with area information
	states := []HAState{state}
	states = h.enrichWithArea(states)
	
	return &states[0], nil
}

func (h *HAService) controlEntity(entityID, action string) error {
	h.logger.Printf("Controlling entity %s: %s", entityID, action)

	var domain, service string

	if strings.HasPrefix(entityID, "light.") {
		domain = "light"
	} else if strings.HasPrefix(entityID, "switch.") {
		domain = "switch"
	} else {
		return fmt.Errorf("unsupported entity type for %s", entityID)
	}

	switch action {
	case "on", "turn_on":
		service = "turn_on"
	case "off", "turn_off":
		service = "turn_off"
	default:
		return fmt.Errorf("unsupported action: %s", action)
	}

	serviceCall := map[string]interface{}{
		"entity_id": entityID,
	}

	startTime := time.Now()
	resp, err := h.makeHARequest("POST", fmt.Sprintf("/api/services/%s/%s", domain, service), serviceCall)
	duration := time.Since(startTime)

	if err != nil {
		h.logger.Printf("HA API request failed for %s after %v: %v", entityID, duration, err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		h.logger.Printf("HA API returned status %d for %s after %v", resp.StatusCode, entityID, duration)
		return fmt.Errorf("HA API returned status %d", resp.StatusCode)
	}

	h.logger.Printf("Successfully controlled %s (%s) in %v", entityID, action, duration)
	return nil
}

// Global HA service instance
var haService *HAService

// MCP Tool Handlers using mark3labs/mcp-go

// get_all_states handler
func getAllStatesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	states, err := haService.getAllStates()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get states: %v", err)), nil
	}

	// Convert states to JSON for the response
	statesJSON, err := json.Marshal(states)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize states: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found %d lights and switches:\n%s", len(states), string(statesJSON))), nil
}

// get_entity_state handler
func getEntityStateHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	entityID, err := request.RequireString("entity_id")
	if err != nil {
		return mcp.NewToolResultError("entity_id parameter is required"), nil
	}

	state, err := haService.getEntityState(entityID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get entity state: %v", err)), nil
	}

	stateJSON, err := json.Marshal(state)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize state: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Entity %s is %s:\n%s", entityID, state.State, string(stateJSON))), nil
}

// control_entity handler
func controlEntityHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	entityID, err := request.RequireString("entity_id")
	if err != nil {
		return mcp.NewToolResultError("entity_id parameter is required"), nil
	}

	action, err := request.RequireString("action")
	if err != nil {
		return mcp.NewToolResultError("action parameter is required"), nil
	}

	err = haService.controlEntity(entityID, action)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to control entity: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully turned %s %s", entityID, action)), nil
}

// control_multiple_entities handler (simplified version)
func controlMultipleEntitiesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()
	
	// Get entities from parameter
	entitiesInterface, ok := arguments["entities"]
	if !ok {
		return mcp.NewToolResultError("entities parameter is required"), nil
	}
	
	entitiesSlice, entitiesOk := entitiesInterface.([]interface{})
	if !entitiesOk {
		return mcp.NewToolResultError("entities must be an array"), nil
	}

	haService.logger.Printf("Processing %d entities in batch", len(entitiesSlice))
	
	results := make([]map[string]interface{}, 0, len(entitiesSlice))
	var errors []string

	// Sequential processing for STDIO stability
	for i, entityInterface := range entitiesSlice {
		// Handle object format: [{"entity_id": "light.entity1", "action": "on"}, ...]
		entityMap, ok := entityInterface.(map[string]interface{})
		if !ok {
			errorMsg := fmt.Sprintf("Entity %d: must be an object with entity_id and action", i)
			results = append(results, map[string]interface{}{
				"index":   i,
				"success": false,
				"error":   errorMsg,
			})
			errors = append(errors, errorMsg)
			continue
		}

		entityID, entityOk := entityMap["entity_id"].(string)
		if !entityOk {
			errorMsg := fmt.Sprintf("Entity %d: entity_id is required and must be a string", i)
			results = append(results, map[string]interface{}{
				"index":     i,
				"entity_id": "",
				"success":   false,
				"error":     errorMsg,
			})
			errors = append(errors, errorMsg)
			continue
		}

		action, actionOk := entityMap["action"].(string)
		if !actionOk {
			errorMsg := fmt.Sprintf("Entity %s: action is required and must be a string", entityID)
			results = append(results, map[string]interface{}{
				"index":     i,
				"entity_id": entityID,
				"success":   false,
				"error":     errorMsg,
			})
			errors = append(errors, errorMsg)
			continue
		}

		err := haService.controlEntity(entityID, action)
		if err != nil {
			errorMsg := fmt.Sprintf("Entity %s: %v", entityID, err)
			results = append(results, map[string]interface{}{
				"index":     i,
				"entity_id": entityID,
				"action":    action,
				"success":   false,
				"error":     err.Error(),
			})
			errors = append(errors, errorMsg)
		} else {
			results = append(results, map[string]interface{}{
				"index":     i,
				"entity_id": entityID,
				"action":    action,
				"success":   true,
			})
		}

		// Small pause between requests
		if i < len(entitiesSlice)-1 {
			time.Sleep(50 * time.Millisecond)
		}
	}

	successCount := 0
	for _, result := range results {
		if result["success"].(bool) {
			successCount++
		}
	}

	haService.logger.Printf("Batch completed: %d successful, %d failed", successCount, len(entitiesSlice)-successCount)

	// Create response
	response := map[string]interface{}{
		"results": results,
	}

	if len(errors) > 0 {
		response["errors"] = errors
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize response: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Processed %d entities: %d successful, %d failed\n%s",
		len(entitiesSlice), successCount, len(entitiesSlice)-successCount, string(responseJSON))), nil
}

func main() {
	// Initialize HA Service
	haService = NewHAService()

	haService.logger.Println("Starting Home Assistant MCP Server")

	if err := haService.LoadConfig(); err != nil {
		haService.logger.Printf("Error loading configuration: %v", err)
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please set HA_TOKEN and HA_URL environment variables or create a config.json file\n")
		os.Exit(1)
	}

	haService.logger.Printf("Configuration loaded - HA URL: %s", haService.config.HAURL)
	haService.logger.Printf("Entity filters: %v", haService.config.EntityFilter)
	haService.logger.Printf("Entity blacklist: %v", haService.config.EntityBlacklist)

	// Create MCP server with mark3labs/mcp-go
	s := server.NewMCPServer(
		"home-assistant-mcp",
		"2.0.0",
		server.WithToolCapabilities(false),
	)

	// Register only the requested 4 tools:

	// 1. get_all_states
	getAllStatesTool := mcp.NewTool("get_all_states",
		mcp.WithDescription("Get the state of all lights and switches"),
	)
	s.AddTool(getAllStatesTool, getAllStatesHandler)

	// 2. get_entity_state
	getEntityStateTool := mcp.NewTool("get_entity_state",
		mcp.WithDescription("Get the state of a specific light or switch"),
		mcp.WithString("entity_id",
			mcp.Required(),
			mcp.Description("The entity ID (e.g., light.living_room, switch.kitchen)"),
		),
	)
	s.AddTool(getEntityStateTool, getEntityStateHandler)

	// 3. control_entity
	controlEntityTool := mcp.NewTool("control_entity",
		mcp.WithDescription("Turn a light or switch on or off"),
		mcp.WithString("entity_id",
			mcp.Required(),
			mcp.Description("The entity ID (e.g., light.living_room, switch.kitchen)"),
		),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Action to perform: 'on', 'off', 'turn_on', or 'turn_off'"),
			mcp.Enum("on", "off", "turn_on", "turn_off"),
		),
	)
	s.AddTool(controlEntityTool, controlEntityHandler)

	// 4. control_multiple_entities
	controlMultipleEntitiesTool := mcp.NewTool("control_multiple_entities",
		mcp.WithDescription("Control multiple lights or switches at once. Requires an array of objects with entity_id and action properties."),
		mcp.WithArray("entities",
			mcp.Required(),
			mcp.Description("Array of entities to control. Format: [{'entity_id': 'light.entity1', 'action': 'on'}, {'entity_id': 'switch.entity2', 'action': 'off'}]"),
		),
	)
	s.AddTool(controlMultipleEntitiesTool, controlMultipleEntitiesHandler)

	haService.logger.Println("MCP Server configured with 4 tools, starting STDIO transport...")

	// Start the STDIO server
	if err := server.ServeStdio(s); err != nil {
		haService.logger.Printf("Server failed: %v", err)
		log.Fatalf("Server failed: %v", err)
	}

	haService.logger.Println("MCP Server stopped")
}
