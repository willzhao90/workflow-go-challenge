package workflow

import (
	"encoding/json"
	"log/slog"
	"net/http"

	api "workflow-code-test/api/openapi"
)

// writeErrorResponse is a helper function to write error responses
func writeErrorResponse(w http.ResponseWriter, statusCode int, errorMessage string) {
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(api.Error{
		Error: errorMessage,
	}); err != nil {
		slog.Error("Failed to encode error response", "error", err, "message", errorMessage)
	}
}
