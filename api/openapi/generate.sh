#!/bin/bash

# Generate Go code from OpenAPI specification
# This script generates Go structs and server interfaces from the OpenAPI YAML file

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
API_DIR="$(dirname "$SCRIPT_DIR")"

echo "Generating Go code from OpenAPI specification..."

# Check if oapi-codegen is installed
if ! command -v oapi-codegen &> /dev/null; then
    echo "oapi-codegen is not installed. Installing..."
    go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
fi

# Generate the code
cd "$SCRIPT_DIR"
oapi-codegen \
    -package api \
    -generate types,chi-server,spec \
    -o api.gen.go \
    openapi.yaml

echo "✅ Successfully generated api.gen.go from openapi.yaml"

# Format the generated code
if command -v gofmt &> /dev/null; then
    echo "Formatting generated code..."
    gofmt -w api.gen.go
fi

echo "✅ Code generation complete!"