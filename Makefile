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
	@echo "  make api-generate   - Generate Go code from OpenAPI specification"
	@echo "  make api-build      - Build the API server"
	@echo "  make api-run        - Run the API server locally"

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

# API code generation
.PHONY: api-generate
api-generate:
	@echo "Generating Go code from OpenAPI specification..."
	@cd api/openapi && ./generate.sh

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
