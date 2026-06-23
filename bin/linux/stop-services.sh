#!/bin/bash

echo "Stopping MySQL Ops Platform services..."

# Stop backend
pkill -f "go run.*platform-backend/cmd/main.go" && echo "Backend stopped" || echo "Backend not running"

# Stop agent
pkill -f "go run.*agent/cmd/main.go" && echo "Agent stopped" || echo "Agent not running"

# Stop web console
pkill -f "npm run dev" && echo "Web console stopped" || echo "Web console not running"

echo "All services stopped"
