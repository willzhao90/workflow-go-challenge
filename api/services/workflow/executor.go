package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	api "workflow-code-test/api/openapi"
)

// ExecuteWorkflow handles the actual workflow execution
func (s *Service) ExecuteWorkflow(ctx context.Context, workflowID string, input api.WorkflowExecutionInput) (*api.WorkflowExecutionResult, error) {
	// Initialize results
	result := &api.WorkflowExecutionResult{
		ExecutedAt: time.Now(),
		Status:     api.WorkflowExecutionResultStatusCompleted,
		Steps:      []api.ExecutionStep{},
	}

	// Get workflow definition from database
	workflow, err := s.db.GetWorkflowByID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("workflow not found: %w", err)
	}

	// Map workflow to API model
	apiWorkflow, err := MapDBWorkflowToAPI(workflow)
	if err != nil {
		return nil, fmt.Errorf("failed to map workflow: %w", err)
	}

	// Execute workflow steps
	steps, err := s.executeWorkflowSteps(ctx, *apiWorkflow, input)
	if err != nil {
		result.Status = api.WorkflowExecutionResultStatusFailed
		slog.Error("Workflow execution failed", "error", err, "workflowID", workflowID)
	}

	result.Steps = steps

	return result, nil
}

// executeWorkflowSteps executes all steps in the workflow
func (s *Service) executeWorkflowSteps(ctx context.Context, workflow api.Workflow, input api.WorkflowExecutionInput) ([]api.ExecutionStep, error) {
	steps := []api.ExecutionStep{}

	// Extract values from input for use in execution
	var executeVars map[string]interface{}
	if input.FormData != nil {
		executeVars = *input.FormData
	}

	// Initialize executeVars if nil
	if executeVars == nil {
		executeVars = make(map[string]interface{})
	}

	// Build a map of nodes by ID for quick lookup
	nodeMap := make(map[string]api.WorkflowNode)
	if workflow.Nodes != nil {
		for _, node := range *workflow.Nodes {
			nodeMap[node.Id] = node
		}
	}

	// Build adjacency list from edges
	adjacencyList := make(map[string][]api.WorkflowEdge)
	if workflow.Edges != nil {
		for _, edge := range *workflow.Edges {
			adjacencyList[edge.Source] = append(adjacencyList[edge.Source], edge)
		}
	}

	// Find start node
	var startNodeId string
	for _, node := range *workflow.Nodes {
		if node.Type == api.WorkflowNodeTypeStart {
			startNodeId = node.Id
			break
		}
	}

	if startNodeId == "" {
		return nil, fmt.Errorf("no start node found in workflow")
	}

	// Track visited nodes to avoid cycles
	visited := make(map[string]bool)

	// Execute nodes using BFS traversal from start node
	queue := []string{startNodeId}

	for len(queue) > 0 {
		currentNodeId := queue[0]
		queue = queue[1:]

		// Skip if already visited
		if visited[currentNodeId] {
			continue
		}
		visited[currentNodeId] = true

		// Get the node
		node, exists := nodeMap[currentNodeId]
		if !exists {
			slog.Warn("Node not found in nodeMap", "nodeId", currentNodeId)
			continue
		}
		output := make(map[string]interface{})

		// Get label and description from node data
		var label, description string
		if node.Data != nil {
			if node.Data.Label != nil {
				label = *node.Data.Label
			}
			if node.Data.Description != nil {
				description = *node.Data.Description
			}
		}

		step := api.ExecutionStep{
			NodeId:      node.Id,
			Type:        string(node.Type),
			Status:      api.ExecutionStepStatusCompleted,
			Label:       &label,
			Description: &description,
			Output:      &output,
		}

		switch node.Type {
		case api.WorkflowNodeTypeStart:
			output["message"] = "Workflow started successfully"

		case api.WorkflowNodeTypeForm:
			// Process form fields based on metadata
			if err := s.processFormNode(node, executeVars, output); err != nil {
				step.Status = api.ExecutionStepStatusFailed
				errorMsg := err.Error()
				step.Error = &errorMsg
				output["message"] = "Failed to process form data"
			} else {
				output["message"] = "Form data processed successfully"
			}

		case api.WorkflowNodeTypeIntegration:
			// Process integration node based on metadata
			if err := s.processIntegrationNode(ctx, node, executeVars, output); err != nil {
				step.Status = api.ExecutionStepStatusFailed
				errorMsg := err.Error()
				step.Error = &errorMsg
				output["message"] = "Failed to process integration"
			} else {
				// Update executeVars with output values for subsequent steps
				for k, v := range output {
					executeVars[k] = v
				}

				// Replace placeholders in description with actual values
				if node.Data != nil && node.Data.Description != nil {
					updatedDesc := *node.Data.Description
					for key, value := range executeVars {
						placeholder := fmt.Sprintf("{{%s}}", key)
						updatedDesc = strings.ReplaceAll(updatedDesc, placeholder, fmt.Sprintf("%v", value))
					}
					description = updatedDesc
					step.Description = &description
				}
			}

		case api.WorkflowNodeTypeCondition:
			// Process condition node based on metadata
			if err := s.processConditionNode(node, executeVars, output, input.Condition); err != nil {
				step.Status = api.ExecutionStepStatusFailed
				errorMsg := err.Error()
				step.Error = &errorMsg
				output["message"] = "Failed to evaluate condition"
			} else {
				// Update executeVars with output values
				for k, v := range output {
					executeVars[k] = v
				}
			}

		case api.WorkflowNodeTypeEmail:
			// Process email node based on metadata
			if err := s.processEmailNode(node, executeVars, output); err != nil {
				step.Status = api.ExecutionStepStatusFailed
				errorMsg := err.Error()
				step.Error = &errorMsg
				output["message"] = "Failed to process email"
			} else {
				// Check if email should be sent based on condition
				conditionMet, _ := executeVars["conditionMet"].(bool)
				if !conditionMet {
					step.Status = api.ExecutionStepStatusSkipped
					output["message"] = "Email alert skipped - condition not met"
				}
			}

		case api.WorkflowNodeTypeEnd:
			output["message"] = "Workflow completed successfully"
		}

		steps = append(steps, step)

		// Find next nodes to execute based on edges
		edges := adjacencyList[currentNodeId]
		for _, edge := range edges {
			// For conditional nodes, check the sourceHandle
			if node.Type == api.WorkflowNodeTypeCondition {
				// Get conditionMet from executeVars
				conditionMet, _ := executeVars["conditionMet"].(bool)

				// Check if this edge should be followed based on condition result
				if edge.SourceHandle != nil {
					if (*edge.SourceHandle == "true" && conditionMet) || (*edge.SourceHandle == "false" && !conditionMet) {
						queue = append(queue, edge.Target)
					}
				} else {
					// No sourceHandle specified, follow the edge
					queue = append(queue, edge.Target)
				}
			} else {
				// For non-conditional nodes, follow all outgoing edges
				queue = append(queue, edge.Target)
			}
		}
	}

	return steps, nil
}

// processIntegrationNode processes integration node based on its metadata configuration
func (s *Service) processIntegrationNode(ctx context.Context, node api.WorkflowNode, executeVars map[string]interface{}, output map[string]interface{}) error {
	// Check if node has metadata
	if node.Data == nil || node.Data.Metadata == nil {
		return fmt.Errorf("integration node missing metadata")
	}

	metadata := *node.Data.Metadata

	// Get inputVariables from metadata
	inputVariables, hasInputVars := metadata["inputVariables"]
	if !hasInputVars {
		return fmt.Errorf("integration node missing inputVariables in metadata")
	}

	inputVarsList, ok := inputVariables.([]interface{})
	if !ok {
		return fmt.Errorf("inputVariables must be an array")
	}

	// Check that all required input variables exist in executeVars
	inputValues := make(map[string]interface{})
	for _, varName := range inputVarsList {
		varNameStr, ok := varName.(string)
		if !ok {
			continue
		}

		value, exists := executeVars[varNameStr]
		if !exists {
			return fmt.Errorf("required input variable '%s' not found in executeVars", varNameStr)
		}
		inputValues[varNameStr] = value
	}

	// Get options from metadata to find matching configuration
	options, hasOptions := metadata["options"]
	if !hasOptions {
		return fmt.Errorf("integration node missing options in metadata")
	}

	optionsList, ok := options.([]interface{})
	if !ok {
		return fmt.Errorf("options must be an array")
	}

	// Find the matching option based on input values
	var selectedOption map[string]interface{}
	for _, opt := range optionsList {
		option, ok := opt.(map[string]interface{})
		if !ok {
			continue
		}

		// Check if this option matches our input values
		matches := true
		for key, value := range inputValues {
			if optValue, exists := option[key]; !exists || optValue != value {
				matches = false
				break
			}
		}

		if matches {
			selectedOption = option
			break
		}
	}

	if selectedOption == nil {
		return fmt.Errorf("no matching option found for input values")
	}

	// Get API endpoint template from metadata
	apiEndpoint, hasEndpoint := metadata["apiEndpoint"]
	if !hasEndpoint {
		return fmt.Errorf("integration node missing apiEndpoint in metadata")
	}

	apiEndpointStr, ok := apiEndpoint.(string)
	if !ok {
		return fmt.Errorf("apiEndpoint must be a string")
	}

	// Replace placeholders in API endpoint with values from selectedOption
	apiURL := apiEndpointStr
	for key, value := range selectedOption {
		placeholder := fmt.Sprintf("{%s}", key)
		apiURL = strings.ReplaceAll(apiURL, placeholder, fmt.Sprintf("%v", value))
	}

	// Make HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		slog.Error("Failed to create request", "error", err, "url", apiURL)
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed to call API", "error", err, "url", apiURL)
		return fmt.Errorf("failed to call API: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read API response", "error", err)
		return fmt.Errorf("failed to read API response: %w", err)
	}

	// Parse JSON response with proper number handling
	var responseData interface{}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber() // This ensures numbers are preserved properly
	if err := decoder.Decode(&responseData); err != nil {
		slog.Error("Failed to parse API response", "error", err, "body", string(body))
		return fmt.Errorf("failed to parse API response: %w", err)
	}

	// Convert to map if it's a map
	responseMap, ok := responseData.(map[string]interface{})
	if !ok {
		return fmt.Errorf("API response is not a JSON object")
	}

	// Log the response for debugging
	slog.Debug("API response received", "url", apiURL, "response", responseMap)

	// Get outputVariables from metadata
	outputVariables, hasOutputVars := metadata["outputVariables"]
	if hasOutputVars {
		outputVarsList, ok := outputVariables.([]interface{})
		if ok {
			// Extract specified output variables from response using recursive search
			for _, varName := range outputVarsList {
				varNameStr, ok := varName.(string)
				if !ok {
					continue
				}

				// Search for the variable in the response (up to 2 levels deep)
				if value := findValueInMap(responseMap, varNameStr, 0, 2); value != nil {
					output[varNameStr] = value
					slog.Debug("Found output variable", "variable", varNameStr, "value", value)
				} else {
					slog.Debug("Output variable not found in response", "variable", varNameStr)
				}
			}
		}
	}

	// Add a success message if we got temperature
	if temp, ok := output["temperature"].(float64); ok {
		if city, ok := inputValues["city"].(string); ok {
			output["message"] = fmt.Sprintf("Weather data fetched for %s: %.1f°C", city, temp)
		}
	}

	// Copy input values to output if they're also listed in outputVariables
	// This handles cases where we want to pass through input values
	if outputVarsList, ok := outputVariables.([]interface{}); ok {
		for _, varName := range outputVarsList {
			varNameStr, ok := varName.(string)
			if !ok {
				continue
			}
			// If not already in output and exists in input, copy it
			if _, exists := output[varNameStr]; !exists {
				if value, exists := inputValues[varNameStr]; exists {
					output[varNameStr] = value
				}
			}
		}
	}

	return nil
}

// findValueInMap recursively searches for a key in a map up to maxDepth levels
// It collects all matching values and returns the first numeric one if available
func findValueInMap(data map[string]interface{}, key string, currentDepth int, maxDepth int) interface{} {
	var candidates []interface{}
	findValueInMapHelper(data, key, currentDepth, maxDepth, &candidates)

	// Prefer numeric values over strings
	for _, candidate := range candidates {
		switch v := candidate.(type) {
		case float64:
			return v
		case json.Number:
			if floatVal, err := v.Float64(); err == nil {
				return floatVal
			}
		case int, int64, int32, float32:
			return v
		}
	}

	// If no numeric value found, return the first candidate if any
	if len(candidates) > 0 {
		return candidates[0]
	}

	return nil
}

// findValueInMapHelper is a helper that collects all values for a given key
func findValueInMapHelper(data map[string]interface{}, key string, currentDepth int, maxDepth int, candidates *[]interface{}) {
	// Check if the key exists at the current level
	if value, exists := data[key]; exists {
		// Handle JSON number type
		switch v := value.(type) {
		case json.Number:
			if floatVal, err := v.Float64(); err == nil {
				*candidates = append(*candidates, floatVal)
			} else {
				*candidates = append(*candidates, v)
			}
		default:
			*candidates = append(*candidates, value)
		}
	}

	// If we've reached max depth, stop searching
	if currentDepth >= maxDepth {
		return
	}

	// Recursively search in nested maps
	for _, v := range data {
		switch nested := v.(type) {
		case map[string]interface{}:
			findValueInMapHelper(nested, key, currentDepth+1, maxDepth, candidates)
		}
	}
}

// processConditionNode processes condition node based on its metadata and executeVars
func (s *Service) processConditionNode(node api.WorkflowNode, executeVars map[string]interface{}, output map[string]interface{}, condition *api.Condition) error {
	// Check if condition configuration is provided
	if condition == nil {
		return fmt.Errorf("condition configuration is missing")
	}

	// Get the value to evaluate (e.g., temperature) from executeVars
	// This should be configurable in metadata, but for now we'll use temperature
	temperature, ok := executeVars["temperature"].(float64)
	if !ok {
		return fmt.Errorf("temperature not found in executeVars or invalid type")
	}

	// Evaluate the condition
	conditionMet := evaluateCondition(temperature, string(condition.Operator), float64(condition.Threshold))

	// Store results in output
	output["conditionMet"] = conditionMet
	output["threshold"] = condition.Threshold
	output["operator"] = string(condition.Operator)
	output["actualValue"] = temperature
	output["message"] = fmt.Sprintf("Temperature %.1f°C is %s %.1f°C - condition %s",
		temperature, condition.Operator, condition.Threshold,
		map[bool]string{true: "met", false: "not met"}[conditionMet])

	return nil
}

// evaluateCondition evaluates a condition based on operator and threshold
func evaluateCondition(value float64, operator string, threshold float64) bool {
	switch operator {
	case "greater_than":
		return value > threshold
	case "less_than":
		return value < threshold
	case "equals":
		return value == threshold
	case "greater_than_or_equal":
		return value >= threshold
	case "less_than_or_equal":
		return value <= threshold
	default:
		slog.Warn("Unknown operator, defaulting to greater_than", "operator", operator)
		return value > threshold
	}
}

// processEmailNode processes email node based on its metadata configuration
func (s *Service) processEmailNode(node api.WorkflowNode, executeVars map[string]interface{}, output map[string]interface{}) error {
	// Check if node has metadata
	if node.Data == nil || node.Data.Metadata == nil {
		return fmt.Errorf("email node missing metadata")
	}

	metadata := *node.Data.Metadata

	// Check if condition was met (for conditional emails)
	conditionMet, hasCondition := executeVars["conditionMet"].(bool)
	if hasCondition && !conditionMet {
		// Condition not met, email will be skipped
		return nil
	}

	// Get inputVariables from metadata
	inputVariables, hasInputVars := metadata["inputVariables"]
	var inputValues map[string]interface{}

	if hasInputVars {
		inputVarsList, ok := inputVariables.([]interface{})
		if ok {
			inputValues = make(map[string]interface{})
			for _, varName := range inputVarsList {
				varNameStr, ok := varName.(string)
				if !ok {
					continue
				}

				// Get value from executeVars
				if value, exists := executeVars[varNameStr]; exists {
					inputValues[varNameStr] = value
				} else {
					slog.Debug("Input variable not found in executeVars", "variable", varNameStr)
				}
			}
		}
	}

	// Get email template from metadata
	emailTemplate, hasTemplate := metadata["emailTemplate"]
	if !hasTemplate {
		return fmt.Errorf("email node missing emailTemplate in metadata")
	}

	templateMap, ok := emailTemplate.(map[string]interface{})
	if !ok {
		return fmt.Errorf("emailTemplate must be an object")
	}

	// Process email template - replace placeholders with values
	subject, _ := templateMap["subject"].(string)
	body, _ := templateMap["body"].(string)

	// Replace placeholders in subject and body
	for key, value := range executeVars {
		placeholder := fmt.Sprintf("{{%s}}", key)
		subject = strings.ReplaceAll(subject, placeholder, fmt.Sprintf("%v", value))
		body = strings.ReplaceAll(body, placeholder, fmt.Sprintf("%v", value))
	}

	// Get recipient email
	email := ""
	if emailValue, exists := executeVars["email"]; exists {
		email, _ = emailValue.(string)
	}

	// Build email draft
	output["emailDraft"] = map[string]interface{}{
		"to":        email,
		"from":      "weather-alerts@example.com", // This could also come from metadata
		"subject":   subject,
		"body":      body,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	// Set delivery status
	output["deliveryStatus"] = "sent"
	output["messageId"] = fmt.Sprintf("msg_%d", time.Now().Unix())
	output["emailSent"] = true

	// Get outputVariables from metadata and set them
	if outputVariables, hasOutputVars := metadata["outputVariables"]; hasOutputVars {
		if outputVarsList, ok := outputVariables.([]interface{}); ok {
			for _, varName := range outputVarsList {
				varNameStr, ok := varName.(string)
				if !ok {
					continue
				}

				// Set the output variable if it's already defined above
				if _, exists := output[varNameStr]; !exists {
					// If the output variable is not set yet, check if it should come from input
					if value, exists := inputValues[varNameStr]; exists {
						output[varNameStr] = value
					}
				}
			}
		}
	}

	return nil
}

// processFormNode processes form node data based on its metadata configuration
func (s *Service) processFormNode(node api.WorkflowNode, executeVars map[string]interface{}, output map[string]interface{}) error {
	// Check if node has metadata
	if node.Data == nil || node.Data.Metadata == nil {
		// No metadata, just copy all executeVars to output
		for k, v := range executeVars {
			output[k] = v
		}
		return nil
	}

	metadata := *node.Data.Metadata

	// Check for outputVariables in metadata
	outputVariables, hasOutputVars := metadata["outputVariables"]
	if !hasOutputVars {
		// No outputVariables specified, copy all executeVars
		for k, v := range executeVars {
			output[k] = v
		}
		return nil
	}

	// Parse outputVariables
	outputVarsList, ok := outputVariables.([]interface{})
	if !ok {
		return fmt.Errorf("outputVariables must be an array")
	}

	// Loop through outputVariables and copy values from executeVars
	for _, varName := range outputVarsList {
		varNameStr, ok := varName.(string)
		if !ok {
			continue
		}

		// Check if this variable exists in executeVars
		if value, exists := executeVars[varNameStr]; exists {
			output[varNameStr] = value
		} else {
			// Variable not found in executeVars, set as null or skip
			slog.Debug("Variable not found in executeVars", "variable", varNameStr)
			output[varNameStr] = nil
		}
	}

	// Also check for inputFields to validate if all required fields are present
	if inputFields, hasInputFields := metadata["inputFields"]; hasInputFields {
		inputFieldsList, ok := inputFields.([]interface{})
		if ok {
			for _, field := range inputFieldsList {
				fieldStr, ok := field.(string)
				if !ok {
					continue
				}

				// Log if an expected input field is missing
				if _, exists := executeVars[fieldStr]; !exists {
					slog.Warn("Expected input field not found in executeVars", "field", fieldStr)
				}
			}
		}
	}

	return nil
}
