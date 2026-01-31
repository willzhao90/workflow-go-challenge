package workflow

import (
	"encoding/json"
	"fmt"
	"time"

	api "workflow-code-test/api/openapi"
	"workflow-code-test/api/pkg/db/models"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// MapDBWorkflowToAPI converts a database workflow model to API workflow model
func MapDBWorkflowToAPI(dbWorkflow *models.Workflow) (*api.Workflow, error) {
	if dbWorkflow == nil {
		return nil, nil
	}

	// Convert string ID to UUID type (required by generated API struct)
	parsedUUID, err := uuid.Parse(dbWorkflow.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid workflow ID format: %v", err)
	}

	apiWorkflow := &api.Workflow{
		Id: openapi_types.UUID(parsedUUID),
	}

	// Map name if present
	if dbWorkflow.Name != "" {
		apiWorkflow.Name = &dbWorkflow.Name
	}

	// Map description if present
	if dbWorkflow.Description.Valid {
		apiWorkflow.Description = &dbWorkflow.Description.String
	}

	// Map nodes if loaded
	if dbWorkflow.R != nil && dbWorkflow.R.WorkflowNodes != nil {
		nodes, err := mapDBNodesToAPI(dbWorkflow.R.WorkflowNodes)
		if err != nil {
			return nil, err
		}
		apiWorkflow.Nodes = &nodes
	}

	// Map edges if loaded
	if dbWorkflow.R != nil && dbWorkflow.R.WorkflowEdges != nil {
		edges, err := mapDBEdgesToAPI(dbWorkflow.R.WorkflowEdges)
		if err != nil {
			return nil, err
		}
		apiWorkflow.Edges = &edges
	}

	return apiWorkflow, nil
}

// mapDBNodesToAPI converts database nodes to API nodes
func mapDBNodesToAPI(dbNodes models.WorkflowNodeSlice) ([]api.WorkflowNode, error) {
	apiNodes := make([]api.WorkflowNode, 0, len(dbNodes))

	for _, dbNode := range dbNodes {
		apiNode := api.WorkflowNode{
			Id:   dbNode.NodeID,
			Type: api.WorkflowNodeType(dbNode.Type),
		}

		// Parse position JSON
		if dbNode.Position != nil {
			var position api.Position
			if err := json.Unmarshal(dbNode.Position, &position); err == nil {
				apiNode.Position = &position
			}
		}

		// Parse data JSON
		if dbNode.Data.Valid && dbNode.Data.JSON != nil {
			var nodeData api.NodeData

			// First unmarshal to a map to extract fields
			var dataMap map[string]interface{}
			if err := json.Unmarshal(dbNode.Data.JSON, &dataMap); err == nil {
				// Map label
				if label, ok := dataMap["label"].(string); ok {
					nodeData.Label = &label
				}

				// Map description
				if desc, ok := dataMap["description"].(string); ok {
					nodeData.Description = &desc
				}

				// Map metadata
				if metadata, ok := dataMap["metadata"].(map[string]interface{}); ok {
					nodeData.Metadata = &metadata
				}

				apiNode.Data = &nodeData
			}
		}

		apiNodes = append(apiNodes, apiNode)
	}

	return apiNodes, nil
}

// mapDBEdgesToAPI converts database edges to API edges
func mapDBEdgesToAPI(dbEdges models.WorkflowEdgeSlice) ([]api.WorkflowEdge, error) {
	apiEdges := make([]api.WorkflowEdge, 0, len(dbEdges))

	for _, dbEdge := range dbEdges {
		apiEdge := api.WorkflowEdge{
			Id:     dbEdge.EdgeID,
			Source: dbEdge.Source,
			Target: dbEdge.Target,
		}

		// Map optional fields
		if dbEdge.Type.Valid {
			apiEdge.Type = &dbEdge.Type.String
		}

		if dbEdge.SourceHandle.Valid {
			apiEdge.SourceHandle = &dbEdge.SourceHandle.String
		}

		if dbEdge.Animated.Valid {
			apiEdge.Animated = &dbEdge.Animated.Bool
		}

		if dbEdge.Label.Valid {
			apiEdge.Label = &dbEdge.Label.String
		}

		// Parse style JSON
		if dbEdge.Style.Valid && dbEdge.Style.JSON != nil {
			var style map[string]interface{}
			if err := json.Unmarshal(dbEdge.Style.JSON, &style); err == nil {
				apiEdge.Style = &style
			}
		}

		// Parse label style JSON
		if dbEdge.LabelStyle.Valid && dbEdge.LabelStyle.JSON != nil {
			var labelStyle map[string]interface{}
			if err := json.Unmarshal(dbEdge.LabelStyle.JSON, &labelStyle); err == nil {
				apiEdge.LabelStyle = &labelStyle
			}
		}

		apiEdges = append(apiEdges, apiEdge)
	}

	return apiEdges, nil
}

// CreateExecutionResult creates a workflow execution result
func CreateExecutionResult(status api.WorkflowExecutionResultStatus, steps []api.ExecutionStep) *api.WorkflowExecutionResult {
	now := time.Now()
	return &api.WorkflowExecutionResult{
		ExecutedAt: now,
		Status:     status,
		Steps:      steps,
	}
}

// CreateExecutionStep creates a single execution step
func CreateExecutionStep(nodeId string, nodeType string, status api.ExecutionStepStatus) api.ExecutionStep {
	output := make(map[string]interface{})
	return api.ExecutionStep{
		NodeId: nodeId,
		Type:   nodeType,
		Status: status,
		Output: &output,
	}
}
