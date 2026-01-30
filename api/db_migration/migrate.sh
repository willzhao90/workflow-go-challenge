#!/bin/bash

# Database initialization script for workflow API
# This script runs all SQL files in the sql directory in order

# Database connection parameters
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5876}
DB_NAME=${DB_NAME:-workflow_engine}
DB_USER=${DB_USER:-workflow}
DB_PASSWORD=${DB_PASSWORD:-workflow123}

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Get the directory where this script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
SQL_DIR="$SCRIPT_DIR/sql"

echo -e "${YELLOW}Workflow Database Migration${NC}"
echo "================================="

# Function to run SQL file
run_sql_file() {
    local file=$1
    local filename=$(basename "$file")
    
    echo -e "${YELLOW}Running: $filename${NC}"
    PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f "$file"
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ $filename completed successfully${NC}"
        return 0
    else
        echo -e "${RED}✗ Error running $filename${NC}"
        return 1
    fi
}

# Check if database exists
echo -e "${YELLOW}Checking database connection...${NC}"
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c "SELECT 1" > /dev/null 2>&1

if [ $? -ne 0 ]; then
    echo -e "${RED}Cannot connect to database. Please ensure PostgreSQL is running and database exists.${NC}"
    echo "Connection string: postgres://$DB_USER:****@$DB_HOST:$DB_PORT/$DB_NAME"
    exit 1
fi

echo -e "${GREEN}✓ Database connection successful${NC}"

# Check if sql directory exists
if [ ! -d "$SQL_DIR" ]; then
    echo -e "${RED}SQL directory not found: $SQL_DIR${NC}"
    exit 1
fi

# Find all SQL files and sort them
SQL_FILES=($(find "$SQL_DIR" -name "*.sql" -type f | sort))

if [ ${#SQL_FILES[@]} -eq 0 ]; then
    echo -e "${YELLOW}No SQL files found in $SQL_DIR${NC}"
    exit 0
fi

echo ""
echo -e "${YELLOW}Found ${#SQL_FILES[@]} SQL file(s) to execute:${NC}"
for file in "${SQL_FILES[@]}"; do
    echo "  - $(basename "$file")"
done

echo ""
read -p "Do you want to continue? (y/n): " -n 1 -r
echo ""

if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted."
    exit 0
fi

echo ""
echo -e "${YELLOW}Running migrations...${NC}"
echo ""

# Run all SQL files in order
SUCCESS_COUNT=0
FAIL_COUNT=0

for file in "${SQL_FILES[@]}"; do
    if run_sql_file "$file"; then
        ((SUCCESS_COUNT++))
    else
        ((FAIL_COUNT++))
        # Ask if user wants to continue after error
        echo ""
        read -p "Error occurred. Continue with remaining files? (y/n): " -n 1 -r
        echo ""
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            break
        fi
    fi
    echo ""
done

# Summary
echo -e "${YELLOW}Migration Summary:${NC}"
echo -e "  Successful: ${GREEN}$SUCCESS_COUNT${NC}"
if [ $FAIL_COUNT -gt 0 ]; then
    echo -e "  Failed: ${RED}$FAIL_COUNT${NC}"
else
    echo -e "  Failed: $FAIL_COUNT"
fi

if [ $FAIL_COUNT -eq 0 ]; then
    echo ""
    echo -e "${GREEN}✓ All migrations completed successfully!${NC}"
    
    # Optional: Show table counts
    echo ""
    echo -e "${YELLOW}Verifying database state...${NC}"
    
    # Check if tables exist and show counts
    if PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c "\dt workflows" 2>/dev/null | grep -q workflows; then
        WORKFLOW_COUNT=$(PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -t -c "SELECT COUNT(*) FROM workflows" 2>/dev/null)
        NODE_COUNT=$(PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -t -c "SELECT COUNT(*) FROM workflow_nodes" 2>/dev/null)
        EDGE_COUNT=$(PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -t -c "SELECT COUNT(*) FROM workflow_edges" 2>/dev/null)
        
        echo "Table counts:"
        echo "  - Workflows: $WORKFLOW_COUNT"
        echo "  - Nodes: $NODE_COUNT"
        echo "  - Edges: $EDGE_COUNT"
    fi
else
    echo ""
    echo -e "${RED}Some migrations failed. Please check the errors above.${NC}"
    exit 1
fi