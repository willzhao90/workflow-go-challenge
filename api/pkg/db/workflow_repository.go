package db

import (
	"context"
	"database/sql"
	"fmt"

	"workflow-code-test/api/pkg/db/models"

	"github.com/aarondl/sqlboiler/v4/queries/qm"
)

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

// GetAllWorkflows retrieves all workflows with basic information
func (r *WorkflowRepository) GetAllWorkflows(ctx context.Context) (models.WorkflowSlice, error) {
	workflows, err := models.Workflows(
		qm.OrderBy("created_at DESC"),
	).All(ctx, r.db)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch workflows: %w", err)
	}

	return workflows, nil
}

// GetWorkflowWithNodesAndEdges retrieves a workflow with eagerly loaded nodes and edges
func (r *WorkflowRepository) GetWorkflowWithNodesAndEdges(ctx context.Context, workflowID string) (*models.Workflow, error) {
	workflow, err := models.Workflows(
		qm.Where("id = ?", workflowID),
		qm.Load(qm.Rels(
			models.WorkflowRels.WorkflowNodes,
			models.WorkflowRels.WorkflowEdges,
		)),
	).One(ctx, r.db)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("workflow not found: %s", workflowID)
		}
		return nil, fmt.Errorf("failed to fetch workflow with relations: %w", err)
	}

	return workflow, nil
}

// WorkflowExists checks if a workflow exists
func (r *WorkflowRepository) WorkflowExists(ctx context.Context, workflowID string) (bool, error) {
	exists, err := models.WorkflowExists(ctx, r.db, workflowID)
	if err != nil {
		return false, fmt.Errorf("failed to check workflow existence: %w", err)
	}

	return exists, nil
}

// GetWorkflowNodes retrieves all nodes for a specific workflow
func (r *WorkflowRepository) GetWorkflowNodes(ctx context.Context, workflowID string) (models.WorkflowNodeSlice, error) {
	nodes, err := models.WorkflowNodes(
		qm.Where("workflow_id = ?", workflowID),
		qm.OrderBy("created_at ASC"),
	).All(ctx, r.db)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch workflow nodes: %w", err)
	}

	return nodes, nil
}

// GetWorkflowEdges retrieves all edges for a specific workflow
func (r *WorkflowRepository) GetWorkflowEdges(ctx context.Context, workflowID string) (models.WorkflowEdgeSlice, error) {
	edges, err := models.WorkflowEdges(
		qm.Where("workflow_id = ?", workflowID),
		qm.OrderBy("created_at ASC"),
	).All(ctx, r.db)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch workflow edges: %w", err)
	}

	return edges, nil
}
