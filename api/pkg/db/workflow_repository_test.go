package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"workflow-code-test/api/pkg/db/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/aarondl/null/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetWorkflowByID(t *testing.T) {
	// Define test cases using table-driven tests (map format)
	tests := map[string]struct {
		// Input
		workflowID string

		// Mock setup
		setupMock func(mock sqlmock.Sqlmock)

		// Expected results
		expectedWorkflow *models.Workflow
		expectedError    error
		errorContains    string
	}{
		"success_with_workflow_nodes_and_edges": {
			workflowID: "test-workflow-123",
			setupMock: func(mock sqlmock.Sqlmock) {
				// Mock workflow query
				workflowRows := sqlmock.NewRows([]string{
					"id", "name", "description", "created_at", "updated_at",
				}).AddRow(
					"test-workflow-123",
					"Test Workflow",
					"This is a test workflow",
					time.Now(),
					time.Now(),
				)

				mock.ExpectQuery(`SELECT .* FROM "workflows" WHERE.*id = \$1`).
					WithArgs("test-workflow-123").
					WillReturnRows(workflowRows)

				// Mock workflow nodes query
				nodesRows := sqlmock.NewRows([]string{
					"id", "workflow_id", "node_id", "node_type", "config", "created_at", "updated_at",
				}).AddRow(
					"node1",
					"test-workflow-123",
					"node-1",
					"start",
					`{"key": "value"}`,
					time.Now(),
					time.Now(),
				).AddRow(
					"node2",
					"test-workflow-123",
					"node-2",
					"process",
					`{"key": "value2"}`,
					time.Now(),
					time.Now(),
				)

				mock.ExpectQuery(`SELECT .* FROM "workflow_nodes" WHERE.*workflow_id.*`).
					WithArgs("test-workflow-123").
					WillReturnRows(nodesRows)

				// Mock workflow edges query
				edgesRows := sqlmock.NewRows([]string{
					"id", "workflow_id", "source_node_id", "target_node_id", "condition", "created_at", "updated_at",
				}).AddRow(
					"edge1",
					"test-workflow-123",
					"node-1",
					"node-2",
					`{"condition": "always"}`,
					time.Now(),
					time.Now(),
				)

				mock.ExpectQuery(`SELECT .* FROM "workflow_edges" WHERE.*workflow_id.*`).
					WithArgs("test-workflow-123").
					WillReturnRows(edgesRows)
			},
			expectedWorkflow: &models.Workflow{
				ID:          "test-workflow-123",
				Name:        "Test Workflow",
				Description: null.StringFrom("This is a test workflow"),
			},
			expectedError: nil,
		},

		"workflow_not_found": {
			workflowID: "non-existent-workflow",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .* FROM "workflows" WHERE.*id = \$1`).
					WithArgs("non-existent-workflow").
					WillReturnError(sql.ErrNoRows)
			},
			expectedWorkflow: nil,
			expectedError:    nil,
			errorContains:    "workflow not found: non-existent-workflow",
		},

		"database_error": {
			workflowID: "test-workflow-456",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .* FROM "workflows" WHERE.*id = \$1`).
					WithArgs("test-workflow-456").
					WillReturnError(errors.New("database connection lost"))
			},
			expectedWorkflow: nil,
			expectedError:    nil,
			errorContains:    "failed to fetch workflow",
		},

		"empty_workflow_id": {
			workflowID: "",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .* FROM "workflows" WHERE.*id = \$1`).
					WithArgs("").
					WillReturnError(sql.ErrNoRows)
			},
			expectedWorkflow: nil,
			expectedError:    nil,
			errorContains:    "workflow not found: ",
		},

		"workflow_without_nodes_and_edges": {
			workflowID: "simple-workflow",
			setupMock: func(mock sqlmock.Sqlmock) {
				// Mock workflow query
				workflowRows := sqlmock.NewRows([]string{
					"id", "name", "description", "created_at", "updated_at",
				}).AddRow(
					"simple-workflow",
					"Simple Workflow",
					nil, // null description
					time.Now(),
					time.Now(),
				)

				mock.ExpectQuery(`SELECT .* FROM "workflows" WHERE.*id = \$1`).
					WithArgs("simple-workflow").
					WillReturnRows(workflowRows)

				// Mock empty workflow nodes query
				nodesRows := sqlmock.NewRows([]string{
					"id", "workflow_id", "node_id", "node_type", "config", "created_at", "updated_at",
				})

				mock.ExpectQuery(`SELECT .* FROM "workflow_nodes" WHERE.*workflow_id.*`).
					WithArgs("simple-workflow").
					WillReturnRows(nodesRows)

				// Mock empty workflow edges query
				edgesRows := sqlmock.NewRows([]string{
					"id", "workflow_id", "source_node_id", "target_node_id", "condition", "created_at", "updated_at",
				})

				mock.ExpectQuery(`SELECT .* FROM "workflow_edges" WHERE.*workflow_id.*`).
					WithArgs("simple-workflow").
					WillReturnRows(edgesRows)
			},
			expectedWorkflow: &models.Workflow{
				ID:          "simple-workflow",
				Name:        "Simple Workflow",
				Description: null.String{}, // null description
			},
			expectedError: nil,
		},
	}

	// Run test cases
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup mock database
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			// Setup expectations
			tc.setupMock(mock)

			// Create repository
			repo := NewWorkflowRepository(db)

			// Execute the function
			ctx := context.Background()
			workflow, err := repo.GetWorkflowByID(ctx, tc.workflowID)

			// Assert results
			if tc.errorContains != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
				assert.Nil(t, workflow)
			} else if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				assert.Nil(t, workflow)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, workflow)
				assert.Equal(t, tc.expectedWorkflow.ID, workflow.ID)
				assert.Equal(t, tc.expectedWorkflow.Name, workflow.Name)

				// Check if description is null or has a value
				if tc.expectedWorkflow.Description.IsZero() {
					assert.True(t, workflow.Description.IsZero())
				} else {
					assert.Equal(t, tc.expectedWorkflow.Description.String, workflow.Description.String)
				}
			}

			// Ensure all expectations were met
			err = mock.ExpectationsWereMet()
			assert.NoError(t, err)
		})
	}
}

// TestNewWorkflowRepository tests the constructor
func TestNewWorkflowRepository(t *testing.T) {
	tests := map[string]struct {
		db       *sql.DB
		expected *WorkflowRepository
	}{
		"creates_repository_with_valid_db": {
			db: &sql.DB{},
			expected: &WorkflowRepository{
				db: &sql.DB{},
			},
		},
		"creates_repository_with_nil_db": {
			db: nil,
			expected: &WorkflowRepository{
				db: nil,
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			repo := NewWorkflowRepository(tc.db)

			assert.NotNil(t, repo)
			assert.Equal(t, tc.db, repo.db)
		})
	}
}

// Benchmark test for GetWorkflowByID
func BenchmarkGetWorkflowByID(b *testing.B) {
	// Setup mock database
	db, mock, err := sqlmock.New()
	require.NoError(b, err)
	defer db.Close()

	// Create repository
	repo := NewWorkflowRepository(db)
	ctx := context.Background()

	// Setup mock expectations for benchmark
	for i := 0; i < b.N; i++ {
		workflowRows := sqlmock.NewRows([]string{
			"id", "name", "description", "created_at", "updated_at",
		}).AddRow(
			"benchmark-workflow",
			"Benchmark Workflow",
			"Benchmark description",
			time.Now(),
			time.Now(),
		)

		mock.ExpectQuery(`SELECT .* FROM "workflows" WHERE.*id = \$1`).
			WithArgs("benchmark-workflow").
			WillReturnRows(workflowRows)

		nodesRows := sqlmock.NewRows([]string{
			"id", "workflow_id", "node_id", "node_type", "config", "created_at", "updated_at",
		})
		mock.ExpectQuery(`SELECT .* FROM "workflow_nodes" WHERE.*workflow_id.*`).
			WithArgs("benchmark-workflow").
			WillReturnRows(nodesRows)

		edgesRows := sqlmock.NewRows([]string{
			"id", "workflow_id", "source_node_id", "target_node_id", "condition", "created_at", "updated_at",
		})
		mock.ExpectQuery(`SELECT .* FROM "workflow_edges" WHERE.*workflow_id.*`).
			WithArgs("benchmark-workflow").
			WillReturnRows(edgesRows)
	}

	// Reset timer after setup
	b.ResetTimer()

	// Run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = repo.GetWorkflowByID(ctx, "benchmark-workflow")
	}
}
