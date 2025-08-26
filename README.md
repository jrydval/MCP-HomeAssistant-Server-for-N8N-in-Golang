# Home Assistant MCP Server

MCP (Model Context Protocol) server for Home Assistant integration. Allows AI assistants like Claude to control lights and switches in your Home Assistant installation.

<img width="1056" height="499" alt="image" src="https://github.com/user-attachments/assets/cc267d75-e439-4a8f-8900-b88196bc4dbf" />

<img width="1029" height="466" alt="image" src="https://github.com/user-attachments/assets/472c80d5-4b43-4525-b59a-507009c9ef80" />


## Features

- **Entity Control**: Turn lights and switches on/off
- **Area-based Control**: Control all lights/switches in a room at once
- **Entity Discovery**: List all available areas, devices, and entities
- **Entity Filtering**: Support for whitelist and blacklist filters
- **Multiple Formats**: Supports both individual and batch entity operations
- **Robust Configuration**: Environment variables and config file support
- **Logging**: Comprehensive logging for troubleshooting

## Installation

### Prerequisites
- Go 1.19+ installed
- Home Assistant with REST API access
- Long-lived access token from Home Assistant

### Build from Source

1. Clone/download this repository:
```bash
git clone <repository-url>
cd MCPServerHAS-SDK
```

2. Build the server:
```bash
# Simple build for current platform
go build -o ha-mcp-server main.go

# Or use the build script for multiple platforms
bash ./build.sh all
```

## Configuration

### Option 1: Environment Variables (Recommended)
```bash
export HA_TOKEN="your_home_assistant_long_lived_access_token"
export HA_URL="http://192.168.1.100:8123"

# Optional: Entity filtering
export HA_ENTITY_FILTER="light\\.*,switch\\.kitchen.*"
export HA_ENTITY_BLACKLIST="switch\\.dangerous.*"
```

### Option 2: Configuration File
```bash
cp config.json.example config.json
# Edit config.json with your credentials
```

Example config.json:
```json
{
  "ha_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "ha_url": "http://192.168.1.100:8123",
  "entity_filter": ["light\\..*", "switch\\.kitchen.*"],
  "entity_blacklist": ["switch\\.dangerous.*"]
}
```

## Usage

### Running the Server
```bash
# With environment variables
./ha-mcp-server

# With config file
CONFIG_FILE=config.json ./ha-mcp-server

# Monitor logs
tail -f ha-mcp.log
```

### MCP Tools Available

#### 1. get_entity_states
Get current states of all lights and switches.

#### 2. set_light_state / set_switch_state  
Control individual entities:
- `entity_id`: Entity ID (e.g., "light.living_room")
- `state`: "on" or "off"

#### 3. control_multiple_entities
Control multiple entities at once. Supports two modes:

**Area-based control:**
```json
{
  "area": "living room",
  "action": "on"
}
```

**Entity list control:**
```json
{
  "entities": ["light.lamp1", "light.lamp2"],
  "action": "off"
}
```

#### 4. get_areas
List all areas/rooms defined in Home Assistant.

## Integration Examples

### Claude Desktop Configuration
Add to your Claude desktop config file (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "home-assistant": {
      "command": "/path/to/ha-mcp-server",
      "env": {
        "HA_TOKEN": "your_token_here",
        "HA_URL": "http://192.168.1.100:8123"
      }
    }
  }
}
```

### n8n AI Agent
The server supports n8n AI agent format for batch operations:
```json
{
  "entities": ["light.living_room", "light.kitchen"],
  "action": "on"
}
```

## Entity Filtering

You can filter which entities are exposed:

### Whitelist (Entity Filter)
Only expose entities matching these regex patterns:
```bash
export HA_ENTITY_FILTER="light\\.living_room.*,switch\\.kitchen.*"
```

### Blacklist (Entity Blacklist)  
Hide entities matching these regex patterns:
```bash
export HA_ENTITY_BLACKLIST="switch\\.dangerous.*,light\\..*_backup"
```

## Troubleshooting

### Check Logs
```bash
tail -f ha-mcp.log
```

### Test Configuration
```bash
# Test environment variables
echo $HA_TOKEN
echo $HA_URL

# Test Home Assistant connection
curl -H "Authorization: Bearer $HA_TOKEN" $HA_URL/api/states
```

### Common Issues

1. **401 Unauthorized**: Check your HA_TOKEN
2. **Connection refused**: Verify HA_URL and network connectivity
3. **No entities found**: Check entity filters and Home Assistant setup
4. **Build failures**: Ensure Go 1.19+ is installed

### Debug Mode
Set environment variable for verbose logging:
```bash
export DEBUG=1
./ha-mcp-server
```

## Security

- Keep your Home Assistant token secure
- Consider using HTTPS for Home Assistant (recommended)
- Use entity filtering to limit exposed devices
- Run the server in a controlled environment

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test with your Home Assistant setup
5. Submit a pull request

## License

MIT

## Changelog

### v2.0.0
- Added area-based control functionality
- Enhanced entity filtering
- Improved batch operations
- Updated to use mark3labs/mcp-go SDK
- Better error handling and logging
- Support for multiple platforms

### v1.0.0
- Initial release
- Basic light and switch control
- MCP integration
