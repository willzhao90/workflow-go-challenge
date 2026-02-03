# API Development Guide

## Project Structure

```
api/
├── openapi/          # OpenAPI spec & generated code
├── db_migration/     # Database migration files
├── pkg/db/           # Database layer
├── services/         # Business logic and API handlers
├── mocks/            # Generated mocks
└── builder/          # API server setup
```

## Tech Stack

- **API Framework**: Echo v4
- **Database**: PostgreSQL  
- **ORM**: SQLBoiler (type-safe, generated models)
- **API Spec**: OpenAPI 3.0 (spec-first development)
- **Code Generation**: oapi-codegen
- **Mocking**: mockgen
- **Migration**: migrate

## Maintenance

### Update Database Schema

1. Create migration file:
```bash
touch api/db_migration/sql/003_your_change.sql
```

2. Run migration:
```bash
make db-migrate
```

### Update Database Models (SQLBoiler)

SQLBoiler generates type-safe models in Go from your database schema:

```bash
make db-generate  # Uses api/pkg/db/sqlboiler.toml config
```

Generated models appear in `api/pkg/db/models/`

### Generate Mocks

```bash
make generate-mocks
```

### Spec-First API Development

1. Edit `api/openapi/openapi.yaml`
2. Generate code:
```bash
make api-generate
```
3. Implement the generated interface in services

## Development Workflow

```bash
# Start database (using docker-compose)
docker-compose up -d postgres

# Run migrations
make db-migrate

# Generate API code from spec
make api-generate

# Generate mocks for testing
make generate-mocks

# Run API server
make api-run

# Run tests
make api-test

# Run linter
make api-lint

# Fix linting issues
make api-lint-fix
```

## Key Packages

### `pkg/db`
- PostgreSQL connection management
- SQLBoiler models & type-safe queries
- Repository pattern implementation
- Config: `sqlboiler.toml`

### `services/workflow`
- Business logic
- Workflow execution engine
- DTO mapping

## Available Make Commands

```bash
make help            # Show all available commands
make db-connect      # Connect to PostgreSQL via psql
make db-migrate      # Run database migrations
make db-generate     # Generate SQLBoiler models from database
make api-generate    # Generate Go code from OpenAPI spec
make generate-mocks  # Generate mock files
make api-build       # Build the API server
make api-run         # Run the API server
make api-dev         # Regenerate and run (development mode)
make api-test        # Run all tests
make api-lint        # Run golangci-lint
make api-lint-fix    # Auto-fix lint issues
```

## Practice Preference

1. Always update OpenAPI spec first, then run `make api-generate`
2. Use `make db-migrate` for all database changes
3. Use SQLBoiler generated methods for DB operations
4. Write unit tests with mocks (`make generate-mocks`)
5. Run `make api-lint` before committing

## TODO

1. Unified Error type library with status code
2. Better logs (suggest https://github.com/uber-go/zap)
3. Retry and rate limiter on executing integration node
4. Both integration node and condition node in workflows could have more input variable to make it more generic
5. Add concurrency if have situation that multiple workflow steps run in parallel
6. For real Email or other types of notifications, can decouple them after a queue
7. Add tracing and metrics