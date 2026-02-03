package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"
	api "workflow-code-test/api/openapi"
	"workflow-code-test/api/pkg/cache"
)

const workflowCachePrefix = "workflow"

// GetWorkflow retrieves a workflow by ID from cache or database
func (s *Service) GetWorkflow(ctx context.Context, workflowID string) (*api.Workflow, error) {
	// Generate cache key
	cacheKey := fmt.Sprintf("%s:%s", workflowCachePrefix, workflowID)

	// Try to get from cache
	var apiWorkflow api.Workflow
	err := s.cache.Get(ctx, cacheKey, &apiWorkflow)
	if err == nil {
		// Found in cache, return it
		slog.Debug("Workflow found in cache", "id", workflowID)
		return &apiWorkflow, nil
	} else if _, ok := err.(cache.ErrCacheMiss); !ok {
		// Log non-cache-miss errors
		slog.Warn("Failed to get workflow from cache", "error", err, "id", workflowID)
	}

	// Get workflow from database using repository
	workflow, err := s.db.GetWorkflowByID(ctx, workflowID)
	if err != nil {
		return nil, err
	}

	// Convert DB model to API model using mapper
	apiWorkflowPtr, err := MapDBWorkflowToAPI(workflow)
	if err != nil {
		return nil, fmt.Errorf("failed to map workflow: %w", err)
	}

	// Store in cache (cache will handle JSON marshaling)
	// Cache for 5 minutes
	if err := s.cache.Set(ctx, cacheKey, apiWorkflowPtr, 5*time.Minute); err != nil {
		slog.Warn("Failed to cache workflow", "error", err, "id", workflowID)
		// Continue even if caching fails
	} else {
		slog.Debug("Workflow cached successfully", "id", workflowID)
	}

	return apiWorkflowPtr, nil
}
