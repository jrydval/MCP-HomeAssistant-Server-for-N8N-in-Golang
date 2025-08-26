#!/bin/bash

# Test script for Home Assistant MCP Server with Official SDK

echo "=== Home Assistant MCP Server - Test Suite ==="
echo ""

# Check if server binary exists
if [ ! -f "./ha-mcp-server" ]; then
    echo "❌ Server binary not found. Please run './build.sh' first."
    exit 1
fi

# Function to run command with timeout (cross-platform)
run_with_timeout() {
    local timeout_duration=$1
    local command="$2"
    
    if command -v gtimeout >/dev/null 2>&1; then
        # Use gtimeout on macOS (if installed via brew install coreutils)
        gtimeout "$timeout_duration" bash -c "$command"
    elif command -v timeout >/dev/null 2>&1; then
        # Use timeout on Linux
        timeout "$timeout_duration" bash -c "$command"
    else
        # Fallback: run without timeout but with background process
        echo "⚠️ Warning: timeout command not found, running without timeout limit"
        bash -c "$command" &
        local pid=$!
        sleep 3  # Wait 3 seconds then kill
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null
            wait "$pid" 2>/dev/null
        fi
    fi
}

echo "🧪 Running MCP Server tests..."
echo ""

# Test 1: Initialize request
echo "🔧 Test 1: Initialize Request"
echo "Request:"
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}'
echo ""
echo "Response:"
run_with_timeout 5s 'echo '\''{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}'\'' | ./ha-mcp-server'
echo ""
echo "---"
echo ""

# Test 2: List tools
echo "🛠️  Test 2: List Tools"
echo "Request:"
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
echo ""
echo "Response:"
run_with_timeout 5s 'echo '\''{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'\'' | ./ha-mcp-server'
echo ""
echo "---"
echo ""

# Test 3: Invalid method (should return error)
echo "❌ Test 3: Invalid Method (should return error)"
echo "Request:"
echo '{"jsonrpc":"2.0","id":3,"method":"invalid/method","params":{}}'
echo ""
echo "Response:"
run_with_timeout 5s 'echo '\''{"jsonrpc":"2.0","id":3,"method":"invalid/method","params":{}}'\'' | ./ha-mcp-server'
echo ""
echo "---"
echo ""

# Test 4: Call tool without HA connection (will fail gracefully)
echo "🏠 Test 4: Call get_all_states (will fail without HA connection)"
echo "Request:"
echo '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_all_states","arguments":{}}}'
echo ""
echo "Response:"
run_with_timeout 5s 'echo '\''{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_all_states","arguments":{}}}'\'' | ./ha-mcp-server'
echo ""
echo "---"
echo ""

# Test 4b: Call tool without HA connection (will fail gracefully)
echo "🏠 Test 4: Call get_areas (will fail without HA connection)"
echo "Request:"
echo '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_areas","arguments":{}}}'
echo ""
echo "Response:"
run_with_timeout 5s 'echo '\''{"jsonrpc":"2.0","id":44,"method":"tools/call","params":{"name":"get_areas","arguments":{}}}'\'' | ./ha-mcp-server'
echo ""
echo "---"
echo ""


# Test 5: Call tool with invalid tool name
echo "❌ Test 5: Invalid Tool Name"
echo "Request:"
echo '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"invalid_tool","arguments":{}}}'
echo ""
echo "Response:"
run_with_timeout 5s 'echo '\''{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"invalid_tool","arguments":{}}}'\'' | ./ha-mcp-server'
echo ""
echo "---"
echo ""

echo "✅ Basic tests completed!"
echo ""
echo "💡 Tips for further testing:"
echo "1. Set HA_TOKEN and HA_URL environment variables to test with real Home Assistant"
echo "2. Use 'tail -f ha-mcp.log' to monitor detailed logs"
echo "3. For interactive testing, run: ./ha-mcp-server"
echo "   Then paste JSON requests manually"
echo ""
echo "🔗 Example with real Home Assistant:"
echo "export HA_TOKEN='your_token'"
echo "export HA_URL='http://192.168.1.100:8123'"
echo "export HA_ENTITY_BLACKLIST='light.camera_villa_floodlight_timed'"
echo "echo '{\"jsonrpc\":\"2.0\",\"id\":6,\"method\":\"tools/call\",\"params\":{\"name\":\"get_all_states\",\"arguments\":{}}}' | ./ha-mcp-server"
