package workflow

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	api "workflow-code-test/api/openapi"

	"github.com/gorilla/mux"
)

// HandleGetWorkflow retrieves a workflow by ID and returns it to the client
func (s *Service) HandleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	slog.Debug("Returning workflow definition for id", "id", id)

	// Set Content-Type header for all responses
	w.Header().Set("Content-Type", "application/json")

	// Use the GetWorkflow function to retrieve the workflow
	apiWorkflow, err := s.GetWorkflow(r.Context(), id)
	if err != nil {
		slog.Error("Failed to get workflow", "error", err, "id", id)

		// Check if workflow not found
		if err.Error() == fmt.Sprintf("workflow not found: %s", id) {
			writeErrorResponse(w, http.StatusNotFound, "Workflow not found")
			return
		}

		// Other errors
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve workflow")
		return
	}

	// Send response
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(apiWorkflow); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}

// HandleExecuteWorkflow executes a workflow with the provided input data
func (s *Service) HandleExecuteWorkflow(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	slog.Debug("Handling workflow execution for id", "id", id)

	// Set Content-Type header for all responses
	w.Header().Set("Content-Type", "application/json")

	// Parse request body
	var input api.WorkflowExecutionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		slog.Error("Failed to parse request body", "error", err)
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Execute workflow
	result, err := s.ExecuteWorkflow(r.Context(), id, input)
	if err != nil {
		slog.Error("Failed to execute workflow", "error", err, "id", id)

		// Check if workflow not found
		if err.Error() == fmt.Sprintf("workflow not found: workflow not found: %s", id) {
			writeErrorResponse(w, http.StatusNotFound, "Workflow not found")
			return
		}

		// Other errors
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to execute workflow")
		return
	}

	// Send response
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}
