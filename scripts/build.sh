#!/bin/bash

# Build script for simulator

set -e

echo "Building simulator..."

# Build for current platform
go build -o simulator ./cmd/simulator

echo "Build complete: ./simulator"

# Optional: Build for multiple platforms
if [ "$1" == "all" ]; then
    echo "Building for all platforms..."
    
    # Linux
    GOOS=linux GOARCH=amd64 go build -o simulator-linux-amd64 ./cmd/simulator
    
    # Windows
    GOOS=windows GOARCH=amd64 go build -o simulator-windows-amd64.exe ./cmd/simulator
    
    # macOS
    GOOS=darwin GOARCH=amd64 go build -o simulator-darwin-amd64 ./cmd/simulator
    GOOS=darwin GOARCH=arm64 go build -o simulator-darwin-arm64 ./cmd/simulator
    
    echo "Multi-platform build complete"
fi

