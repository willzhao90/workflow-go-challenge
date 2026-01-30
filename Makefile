# Database connection parameters (matching docker-compose.yml)
DB_HOST := localhost
DB_PORT := 5876
DB_NAME := workflow_engine
DB_USER := workflow
DB_PASSWORD := workflow123

.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make db-connect  - Connect to PostgreSQL database via psql"
	@echo "  make db-migrate  - Run database migrations"

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
