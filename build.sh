#!/bin/bash

# Build script for Home Assistant MCP Server with Official SDK

echo "=== Home Assistant MCP Server - Official SDK Build ==="
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Error: Go is not installed or not in PATH"
    exit 1
fi

echo "🔧 Building Home Assistant MCP Server..."

# Initialize go modules if needed
if [ ! -f "go.sum" ]; then
    echo "📦 Initializing Go modules..."
    go mod tidy
fi

# Build for current platform
echo "🏗️  Building for current platform..."
go build -o ha-mcp-server main.go

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Build successful: ha-mcp-server"
    echo ""
    echo "📁 Files created:"
    ls -la ha-mcp-server
    echo ""
    echo "🚀 Usage:"
    echo "  # With environment variables:"
    echo "  export HA_TOKEN='your_token'"
    echo "  export HA_URL='http://192.168.1.100:8123'"
    echo "  ./ha-mcp-server"
    echo ""
    echo "  # With config file:"
    echo "  cp config.json.example config.json"
    echo "  # Edit config.json with your credentials"
    echo "  ./ha-mcp-server"
    echo ""
    echo "  # Monitor logs:"
    echo "  tail -f ha-mcp.log"
else
    echo "❌ Build failed"
    exit 1
fi

# Build for multiple platforms (optional)
if [ "$1" = "all" ]; then
    echo ""
    echo "🌍 Building for multiple platforms..."
    
    # Linux AMD64
    echo "🐧 Building for Linux AMD64..."
    GOOS=linux GOARCH=amd64 go build -o ha-mcp-server-linux-amd64 main.go
    
    # Linux ARM64
    echo "🐧 Building for Linux ARM64..."
    GOOS=linux GOARCH=arm64 go build -o ha-mcp-server-linux-arm64 main.go
    
    # Windows AMD64
    echo "🪟 Building for Windows AMD64..."
    GOOS=windows GOARCH=amd64 go build -o ha-mcp-server-windows-amd64.exe main.go
    
    # macOS AMD64
    echo "🍎 Building for macOS AMD64..."
    GOOS=darwin GOARCH=amd64 go build -o ha-mcp-server-macos-amd64 main.go
    
    # macOS ARM64 (Apple Silicon)
    echo "🍎 Building for macOS ARM64..."
    GOOS=darwin GOARCH=arm64 go build -o ha-mcp-server-macos-arm64 main.go
    
    echo ""
    echo "✅ All builds completed!"
    echo "📦 Generated binaries:"
    ls -la ha-mcp-server*
fi

echo ""
echo "📖 For more information, see README.md"
