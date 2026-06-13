#!/bin/bash

# 测试脚本：在Linux主机上手动测试环境检测功能

set -e

HOST="10.1.81.32"  # 使用一台正常运行的主机
AGENT_PORT="9090"
AGENT_TOKEN="dev-agent-token-CHANGE-ME-at-least-16"

echo "=== Testing Agent on $HOST:$AGENT_PORT ==="

echo -e "\n1. Health Check"
curl -s http://$HOST:$AGENT_PORT/health | python3 -m json.tool 2>/dev/null || curl -s http://$HOST:$AGENT_PORT/health

echo -e "\n\n2. Check existing tools API"
curl -s http://$HOST:$AGENT_PORT/agent/tasks/check-tools \
  -H "Authorization: Bearer $AGENT_TOKEN" | python3 -m json.tool 2>/dev/null || curl -s http://$HOST:$AGENT_PORT/agent/tasks/check-tools -H "Authorization: Bearer $AGENT_TOKEN"

echo -e "\n\n3. Environment Check API (new feature)"
curl -s http://$HOST:$AGENT_PORT/agent/tasks/check-environment \
  -H "Authorization: Bearer $AGENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"check_tools":true,"check_resources":true,"check_network":true}' | python3 -m json.tool 2>/dev/null || curl -s http://$HOST:$AGENT_PORT/agent/tasks/check-environment \
  -H "Authorization: Bearer $AGENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"check_tools":true,"check_resources":true,"check_network":true}'

echo -e "\n\n=== Test Complete ==="
