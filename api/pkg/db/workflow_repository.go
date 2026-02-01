package db

import (
	"context"
	"database/sql"
	"fmt"

	"workflow-code-test/api/pkg/db/models"

	"github.com/aarondl/sqlboiler/v4/queries/qm"
)

type WorkFlowDB interface {
	GetWorkflowByID(ctx context.Context, workflowID string) (*models.Workflow, error)
}

// WorkflowRepository handles database operations for workflows
type WorkflowRepository struct {
	db *sql.DB
}

// NewWorkflowRepository creates a new workflow repository
func NewWorkflowRepository(db *sql.DB) *WorkflowRepository {
	return &WorkflowRepository{
		db: db,
	}
}

// GetWorkflowByID retrieves a workflow with all its nodes and edges
func (r *WorkflowRepository) GetWorkflowByID(ctx context.Context, workflowID string) (*models.Workflow, error) {
	// Fetch the workflow with related nodes and edges
	workflow, err := models.Workflows(
		qm.Where("id = ?", workflowID),
		qm.Load(models.WorkflowRels.WorkflowNodes),
		qm.Load(models.WorkflowRels.WorkflowEdges),
	).One(ctx, r.db)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("workflow not found: %s", workflowID)
		}
		return nil, fmt.Errorf("failed to fetch workflow: %w", err)
	}

	return workflow, nil
}
