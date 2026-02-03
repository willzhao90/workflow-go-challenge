# Database connection parameters (matching docker-compose.yml)
DB_HOST := localhost
DB_PORT := 5876
DB_NAME := workflow_engine
DB_USER := workflow
DB_PASSWORD := workflow123

.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make db-connect     - Connect to PostgreSQL database via psql"
	@echo "  make db-migrate     - Run database migrations"
	@echo "  make db-generate    - Generate SQLBoiler models from database schema"
	@echo "  make api-generate   - Generate Go code from OpenAPI specification"
	@echo "  make generate-mocks - Generate mock files for testing"
	@echo "  make api-build      - Build the API server"
	@echo "  make api-run        - Run the API server locally"
	@echo "  make api-test       - Run API unit tests"
	@echo "  make api-lint       - Run golangci-lint checks"
	@echo "  make api-lint-fix   - Run golangci-lint with auto-fix"

.PHONY: db-connect
db-connect:
	@echo "Connecting to PostgreSQL database..."
	@echo "Host: $(DB_HOST):$(DB_PORT)"
	@echo "Database: $(DB_NAME)"
	@echo "User: $(DB_USER)"
	@echo ""
	@PGPASSWORD=$(DB_PASSWORD) psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME)

.PHONY: db-migrate
db-migrate:
	@cd api/db_migration && ./migrate.sh

# Generate SQLBoiler models from database schema
.PHONY: db-generate
db-generate:
	@echo "Generating SQLBoiler models from database schema..."
	@cd api/pkg/db && sqlboiler psql
	@echo "SQLBoiler models generated successfully!"

# API code generation
.PHONY: api-generate
api-generate:
	@echo "Generating Go code from OpenAPI specification..."
	@cd api/openapi && ./generate.sh

# Generate mocks for testing
.PHONY: generate-mocks
generate-mocks:
	@echo "Generating mocks for testing..."
	@cd api && mockgen -source=pkg/db/workflow_repository.go -destination=mocks/mock_workflow_db.go -package=mocks WorkFlowDB
	@echo "Mocks generated successfully!"

# Build the API
.PHONY: api-build
api-build:
	@echo "Building API server..."
	@cd api && go build -o ../bin/api-server .

# Run the API locally
.PHONY: api-run
api-run:
	@echo "Running API server..."
	@cd api && go run .

# Regenerate and build
.PHONY: api-rebuild
api-rebuild: api-generate api-build
	@echo "API regenerated and rebuilt successfully!"

# Development mode - regenerate, build and run
.PHONY: api-dev
api-dev: api-generate api-run

# Run tests
.PHONY: api-test
api-test:
	@echo "Running API unit tests..."
	@cd api && go test -v ./...

# Run specific test
.PHONY: api-test-workflow
api-test-workflow:
	@echo "Running workflow service tests..."
	@cd api && go test -v ./services/workflow/...

# Lint the code
.PHONY: api-lint
api-lint:
	@echo "Running golangci-lint..."
	@cd api && golangci-lint run ./...

# Lint with auto-fix
.PHONY: api-lint-fix
api-lint-fix:
	@echo "Running golangci-lint with auto-fix..."
	@cd api && golangci-lint run --fix ./...

# Install lint tool
.PHONY: api-lint-install
api-lint-install:
	@echo "Installing golangci-lint v2..."
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@echo "golangci-lint v2 installed successfully!"
