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

// WeatherAPIResponse represents the Open-Meteo API response
type WeatherAPIResponse struct {
	CurrentWeather struct {
		Temperature float64 `json:"temperature"`
		Time        string  `json:"time"`
	} `json:"current_weather"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// ExecuteWorkflow handles the actual workflow execution
func (s *Service) ExecuteWorkflow(ctx context.Context, workflowID string, input api.WorkflowExecutionInput) (*api.WorkflowExecutionResult, error) {
	// Initialize results
	result := &api.WorkflowExecutionResult{
		ExecutedAt: time.Now(),
		Status:     api.WorkflowExecutionResultStatusCompleted,
		Steps:      []api.ExecutionStep{},
	}

	// Get workflow definition from database
	workflow, err := s.repository.GetWorkflowByID(ctx, workflowID)
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

	// Track execution state
	var temperature float64
	conditionMet := false

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
			if err := s.processIntegrationNode(node, executeVars, output); err != nil {
				step.Status = api.ExecutionStepStatusFailed
				errorMsg := err.Error()
				step.Error = &errorMsg
				output["message"] = "Failed to process integration"
			} else {
				// Update executeVars with output values for subsequent steps
				for k, v := range output {
					executeVars[k] = v
				}
				// Special handling for temperature tracking
				if temp, ok := output["temperature"].(float64); ok {
					temperature = temp
				}
			}

		case api.WorkflowNodeTypeCondition:
			// Evaluate condition
			if input.Condition != nil {
				conditionMet = evaluateCondition(temperature, string(input.Condition.Operator), float64(input.Condition.Threshold))
				output["message"] = fmt.Sprintf("Temperature %.1f°C is %s %.1f°C - condition %s",
					temperature, input.Condition.Operator, input.Condition.Threshold,
					map[bool]string{true: "met", false: "not met"}[conditionMet])
				output["conditionMet"] = conditionMet
				output["threshold"] = input.Condition.Threshold
				output["operator"] = string(input.Condition.Operator)
				output["actualValue"] = temperature
			}

		case api.WorkflowNodeTypeEmail:
			if conditionMet {
				email, _ := executeVars["email"].(string)
				city, _ := executeVars["city"].(string)

				output["emailDraft"] = map[string]interface{}{
					"to":        email,
					"from":      "weather-alerts@example.com",
					"subject":   "Weather Alert",
					"body":      fmt.Sprintf("Weather alert for %s! Temperature is %.1f°C!", city, temperature),
					"timestamp": time.Now().Format(time.RFC3339),
				}
				output["deliveryStatus"] = "sent"
				output["messageId"] = "msg_" + fmt.Sprintf("%d", time.Now().Unix())
				output["emailSent"] = true
			} else {
				// Skip email if condition not met
				step.Status = api.ExecutionStepStatusSkipped
				output["message"] = "Email alert skipped - condition not met"
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
func (s *Service) processIntegrationNode(node api.WorkflowNode, executeVars map[string]interface{}, output map[string]interface{}) error {
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

	// Make HTTP request
	resp, err := http.Get(apiURL)
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

	// Parse JSON response
	var responseData map[string]interface{}
	if err := json.Unmarshal(body, &responseData); err != nil {
		slog.Error("Failed to parse API response", "error", err)
		return fmt.Errorf("failed to parse API response: %w", err)
	}

	// Get outputVariables from metadata
	outputVariables, hasOutputVars := metadata["outputVariables"]
	if hasOutputVars {
		outputVarsList, ok := outputVariables.([]interface{})
		if ok {
			// Extract specified output variables from response
			for _, varName := range outputVarsList {
				varNameStr, ok := varName.(string)
				if !ok {
					continue
				}

				// Special handling for nested response data
				if varNameStr == "temperature" {
					// Check for temperature in current_weather
					if currentWeather, ok := responseData["current_weather"].(map[string]interface{}); ok {
						if temp, ok := currentWeather["temperature"].(float64); ok {
							output["temperature"] = temp
						}
					}
				} else {
					// Direct extraction from response
					if value, exists := responseData[varNameStr]; exists {
						output[varNameStr] = value
					}
				}
			}
		}
	}

	// Add location info from input
	if city, exists := inputValues["city"]; exists {
		output["location"] = city
	}

	// Add success message
	if temp, ok := output["temperature"].(float64); ok {
		if city, ok := inputValues["city"].(string); ok {
			output["message"] = fmt.Sprintf("Weather data fetched for %s: %.1f°C", city, temp)
		}
	}

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

	// node.Data.Metadata is already a pointer to map[string]interface{}
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
