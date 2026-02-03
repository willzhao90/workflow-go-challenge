package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "workflow-code-test/api/openapi"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteFormNode(t *testing.T) {
	// Define test cases using table-driven tests (map format)
	tests := map[string]struct {
		// Input
		node        api.WorkflowNode
		executeVars map[string]any

		// Expected output
		expectedOutput map[string]any
		expectedError  bool
		errorContains  string
	}{
		"no_metadata_copies_all_vars": {
			node: api.WorkflowNode{
				Id:   "form-1",
				Type: api.WorkflowNodeTypeForm,
				Data: &api.NodeData{
					Label: strPtr("Form Node"),
					// No metadata
				},
			},
			executeVars: map[string]any{
				"name":  "John Doe",
				"email": "john@example.com",
				"age":   30,
			},
			expectedOutput: map[string]any{
				"name":  "John Doe",
				"email": "john@example.com",
				"age":   30,
			},
			expectedError: false,
		},

		"nil_data_copies_all_vars": {
			node: api.WorkflowNode{
				Id:   "form-2",
				Type: api.WorkflowNodeTypeForm,
				Data: nil, // Nil data
			},
			executeVars: map[string]any{
				"city":    "Sydney",
				"country": "Australia",
			},
			expectedOutput: map[string]any{
				"city":    "Sydney",
				"country": "Australia",
			},
			expectedError: false,
		},

		"metadata_without_output_variables_copies_all": {
			node: api.WorkflowNode{
				Id:   "form-3",
				Type: api.WorkflowNodeTypeForm,
				Data: &api.NodeData{
					Label: strPtr("Form with metadata"),
					Metadata: &map[string]any{
						"inputFields": []any{"name", "email"},
						// No outputVariables
					},
				},
			},
			executeVars: map[string]any{
				"name":  "Jane Smith",
				"email": "jane@example.com",
				"phone": "123-456-7890",
			},
			expectedOutput: map[string]any{
				"name":  "Jane Smith",
				"email": "jane@example.com",
				"phone": "123-456-7890",
			},
			expectedError: false,
		},

		"output_variables_filters_vars": {
			node: api.WorkflowNode{
				Id:   "form-4",
				Type: api.WorkflowNodeTypeForm,
				Data: &api.NodeData{
					Label: strPtr("Form with output filtering"),
					Metadata: &map[string]any{
						"outputVariables": []any{"name", "city"},
					},
				},
			},
			executeVars: map[string]any{
				"name":     "Bob Wilson",
				"email":    "bob@example.com",
				"city":     "Melbourne",
				"country":  "Australia",
				"postcode": "3000",
			},
			expectedOutput: map[string]any{
				"name": "Bob Wilson",
				"city": "Melbourne",
			},
			expectedError: false,
		},

		"output_variables_with_missing_vars": {
			node: api.WorkflowNode{
				Id:   "form-5",
				Type: api.WorkflowNodeTypeForm,
				Data: &api.NodeData{
					Label: strPtr("Form with missing vars"),
					Metadata: &map[string]any{
						"outputVariables": []any{"name", "email", "phone"},
					},
				},
			},
			executeVars: map[string]any{
				"name":  "Alice Cooper",
				"email": "alice@example.com",
				// phone is missing
			},
			expectedOutput: map[string]any{
				"name":  "Alice Cooper",
				"email": "alice@example.com",
				"phone": nil, // Missing variable set as nil
			},
			expectedError: false,
		},

		"invalid_output_variables_type": {
			node: api.WorkflowNode{
				Id:   "form-6",
				Type: api.WorkflowNodeTypeForm,
				Data: &api.NodeData{
					Label: strPtr("Form with invalid output vars"),
					Metadata: &map[string]any{
						"outputVariables": "not-an-array", // Invalid type
					},
				},
			},
			executeVars: map[string]any{
				"name": "Test User",
			},
			expectedOutput: map[string]any{},
			expectedError:  true,
			errorContains:  "outputVariables must be an array",
		},

		"output_variables_with_non_string_elements": {
			node: api.WorkflowNode{
				Id:   "form-7",
				Type: api.WorkflowNodeTypeForm,
				Data: &api.NodeData{
					Label: strPtr("Form with mixed types"),
					Metadata: &map[string]any{
						"outputVariables": []any{
							"name",
							123, // Non-string element (should be skipped)
							"email",
							true, // Non-string element (should be skipped)
							"city",
						},
					},
				},
			},
			executeVars: map[string]any{
				"name":  "Mixed User",
				"email": "mixed@example.com",
				"city":  "Brisbane",
				"age":   25,
			},
			expectedOutput: map[string]any{
				"name":  "Mixed User",
				"email": "mixed@example.com",
				"city":  "Brisbane",
			},
			expectedError: false,
		},

		"empty_output_variables_array": {
			node: api.WorkflowNode{
				Id:   "form-8",
				Type: api.WorkflowNodeTypeForm,
				Data: &api.NodeData{
					Label: strPtr("Form with empty output vars"),
					Metadata: &map[string]any{
						"outputVariables": []any{}, // Empty array
					},
				},
			},
			executeVars: map[string]any{
				"name":  "Empty Output",
				"email": "empty@example.com",
			},
			expectedOutput: map[string]any{}, // No output since array is empty
			expectedError:  false,
		},

		"complex_data_types_in_vars": {
			node: api.WorkflowNode{
				Id:   "form-9",
				Type: api.WorkflowNodeTypeForm,
				Data: &api.NodeData{
					Label: strPtr("Form with complex types"),
					Metadata: &map[string]any{
						"outputVariables": []any{"user", "settings", "count"},
					},
				},
			},
			executeVars: map[string]any{
				"user": map[string]any{
					"name": "Complex User",
					"id":   12345,
				},
				"settings": []string{"option1", "option2"},
				"count":    42,
				"ignored":  "This should not be in output",
			},
			expectedOutput: map[string]any{
				"user": map[string]any{
					"name": "Complex User",
					"id":   12345,
				},
				"settings": []string{"option1", "option2"},
				"count":    42,
			},
			expectedError: false,
		},

		"nil_execute_vars": {
			node: api.WorkflowNode{
				Id:   "form-10",
				Type: api.WorkflowNodeTypeForm,
				Data: &api.NodeData{
					Label: strPtr("Form with nil vars"),
				},
			},
			executeVars:    nil,
			expectedOutput: map[string]any{}, // Should handle nil gracefully
			expectedError:  false,
		},

		"input_fields_validation": {
			node: api.WorkflowNode{
				Id:   "form-11",
				Type: api.WorkflowNodeTypeForm,
				Data: &api.NodeData{
					Label: strPtr("Form with input field validation"),
					Metadata: &map[string]any{
						"inputFields":     []any{"requiredName", "requiredEmail", "optionalPhone"},
						"outputVariables": []any{"requiredName", "requiredEmail"},
					},
				},
			},
			executeVars: map[string]any{
				"requiredName":  "Validated User",
				"requiredEmail": "validated@example.com",
				// optionalPhone is missing - should log warning but not error
			},
			expectedOutput: map[string]any{
				"requiredName":  "Validated User",
				"requiredEmail": "validated@example.com",
			},
			expectedError: false,
		},

		"special_characters_in_variable_names": {
			node: api.WorkflowNode{
				Id:   "form-12",
				Type: api.WorkflowNodeTypeForm,
				Data: &api.NodeData{
					Label: strPtr("Form with special chars"),
					Metadata: &map[string]any{
						"outputVariables": []any{
							"user-name",
							"email_address",
							"phone.number",
							"data[0]",
						},
					},
				},
			},
			executeVars: map[string]any{
				"user-name":     "Special User",
				"email_address": "special@example.com",
				"phone.number":  "+61 400 000 000",
				"data[0]":       "First item",
				"other":         "Should be filtered",
			},
			expectedOutput: map[string]any{
				"user-name":     "Special User",
				"email_address": "special@example.com",
				"phone.number":  "+61 400 000 000",
				"data[0]":       "First item",
			},
			expectedError: false,
		},
	}

	// Run test cases
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create service (no database needed for this function)
			service := &Service{}

			// Create output map
			output := make(map[string]any)

			// Call the function
			err := service.executeFormNode(tc.node, tc.executeVars, output)

			// Check error
			if tc.expectedError {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
			}

			// Check output
			if !tc.expectedError {
				assert.Equal(t, tc.expectedOutput, output)
			}
		})
	}
}

func TestExecuteEmailNode(t *testing.T) {
	// Define test cases using table-driven tests (map format)
	tests := map[string]struct {
		// Input
		node        api.WorkflowNode
		executeVars map[string]any

		// Expected output
		expectedOutput map[string]any
		expectedError  bool
		errorContains  string
		checkOutput    func(t *testing.T, output map[string]any)
	}{
		"successful_email_with_condition_met": {
			node: api.WorkflowNode{
				Id:   "email-1",
				Type: api.WorkflowNodeTypeEmail,
				Data: &api.NodeData{
					Label: strPtr("Email Alert"),
					Metadata: &map[string]any{
						"emailTemplate": map[string]any{
							"subject": "Weather Alert for {{city}}",
							"body":    "Temperature in {{city}} is {{temperature}}°C which is {{operator}} {{threshold}}°C",
						},
						"inputVariables":  []any{"city", "temperature", "email"},
						"outputVariables": []any{"emailDraft", "deliveryStatus", "messageId", "emailSent"},
					},
				},
			},
			executeVars: map[string]any{
				"city":         "Sydney",
				"temperature":  35.5,
				"operator":     "greater than",
				"threshold":    30,
				"email":        "user@example.com",
				"conditionMet": true,
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				// Check email draft
				emailDraft, ok := output["emailDraft"].(map[string]any)
				require.True(t, ok, "emailDraft should be a map")
				assert.Equal(t, "user@example.com", emailDraft["to"])
				assert.Equal(t, "weather-alerts@example.com", emailDraft["from"])
				assert.Equal(t, "Weather Alert for Sydney", emailDraft["subject"])
				assert.Equal(t, "Temperature in Sydney is 35.5°C which is greater than 30°C", emailDraft["body"])

				// Check other outputs
				assert.Equal(t, "sent", output["deliveryStatus"])
				assert.NotNil(t, output["messageId"])
				assert.Equal(t, true, output["emailSent"])
			},
		},

		"email_with_missing_template": {
			node: api.WorkflowNode{
				Id:   "email-3",
				Type: api.WorkflowNodeTypeEmail,
				Data: &api.NodeData{
					Label: strPtr("Email without template"),
					Metadata: &map[string]any{
						// Missing emailTemplate
						"inputVariables": []any{"email"},
					},
				},
			},
			executeVars: map[string]any{
				"email":        "user@example.com",
				"conditionMet": true,
			},
			expectedError: true,
			errorContains: "email node missing emailTemplate in metadata",
		},

		"email_with_nil_metadata": {
			node: api.WorkflowNode{
				Id:   "email-4",
				Type: api.WorkflowNodeTypeEmail,
				Data: &api.NodeData{
					Label: strPtr("Email with nil metadata"),
					// Nil metadata
				},
			},
			executeVars: map[string]any{
				"email": "user@example.com",
			},
			expectedError: true,
			errorContains: "email node missing metadata",
		},

		"email_with_nil_data": {
			node: api.WorkflowNode{
				Id:   "email-5",
				Type: api.WorkflowNodeTypeEmail,
				Data: nil, // Nil data
			},
			executeVars: map[string]any{
				"email": "user@example.com",
			},
			expectedError: true,
			errorContains: "email node missing metadata",
		},

		"email_with_invalid_template_format": {
			node: api.WorkflowNode{
				Id:   "email-6",
				Type: api.WorkflowNodeTypeEmail,
				Data: &api.NodeData{
					Label: strPtr("Invalid template format"),
					Metadata: &map[string]any{
						"emailTemplate": "not-a-map", // Invalid format
					},
				},
			},
			executeVars: map[string]any{
				"email": "user@example.com",
			},
			expectedError: true,
			errorContains: "emailTemplate must be an object",
		},

		"email_with_placeholder_replacement": {
			node: api.WorkflowNode{
				Id:   "email-7",
				Type: api.WorkflowNodeTypeEmail,
				Data: &api.NodeData{
					Label: strPtr("Email with placeholders"),
					Metadata: &map[string]any{
						"emailTemplate": map[string]any{
							"subject": "Order #{{orderId}} - {{status}}",
							"body":    "Dear {{customerName}}, your order #{{orderId}} is {{status}}. Total: ${{amount}}",
						},
						"outputVariables": []any{"emailDraft", "deliveryStatus"},
					},
				},
			},
			executeVars: map[string]any{
				"orderId":      "ORD-12345",
				"status":       "Shipped",
				"customerName": "John Smith",
				"amount":       299.99,
				"email":        "john.smith@example.com",
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				emailDraft, ok := output["emailDraft"].(map[string]any)
				require.True(t, ok, "emailDraft should be a map")
				assert.Equal(t, "Order #ORD-12345 - Shipped", emailDraft["subject"])
				assert.Equal(t, "Dear John Smith, your order #ORD-12345 is Shipped. Total: $299.99", emailDraft["body"])
				assert.Equal(t, "sent", output["deliveryStatus"])
			},
		},

		"email_without_recipient": {
			node: api.WorkflowNode{
				Id:   "email-8",
				Type: api.WorkflowNodeTypeEmail,
				Data: &api.NodeData{
					Label: strPtr("Email without recipient"),
					Metadata: &map[string]any{
						"emailTemplate": map[string]any{
							"subject": "Test Email",
							"body":    "This is a test email",
						},
					},
				},
			},
			executeVars: map[string]any{
				// No email field
				"name": "Test User",
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				emailDraft, ok := output["emailDraft"].(map[string]any)
				require.True(t, ok, "emailDraft should be a map")
				assert.Equal(t, "", emailDraft["to"]) // Empty recipient
				assert.Equal(t, "Test Email", emailDraft["subject"])
			},
		},

		"email_with_input_variables": {
			node: api.WorkflowNode{
				Id:   "email-9",
				Type: api.WorkflowNodeTypeEmail,
				Data: &api.NodeData{
					Label: strPtr("Email with input vars"),
					Metadata: &map[string]any{
						"emailTemplate": map[string]any{
							"subject": "Notification",
							"body":    "Hello {{name}}, this is your notification.",
						},
						"inputVariables":  []any{"name", "priority"},
						"outputVariables": []any{"emailDraft", "priority"},
					},
				},
			},
			executeVars: map[string]any{
				"name":     "Alice",
				"priority": "high",
				"email":    "alice@example.com",
				"extra":    "ignored",
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				emailDraft, ok := output["emailDraft"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "Hello Alice, this is your notification.", emailDraft["body"])
				assert.Equal(t, "high", output["priority"]) // From inputVariables
			},
		},

		"email_with_complex_data_types": {
			node: api.WorkflowNode{
				Id:   "email-10",
				Type: api.WorkflowNodeTypeEmail,
				Data: &api.NodeData{
					Label: strPtr("Email with complex types"),
					Metadata: &map[string]any{
						"emailTemplate": map[string]any{
							"subject": "Data Report",
							"body":    "Count: {{count}}, Array: {{items}}, Map: {{data}}",
						},
					},
				},
			},
			executeVars: map[string]any{
				"count": 42,
				"items": []string{"item1", "item2", "item3"},
				"data": map[string]any{
					"key1": "value1",
					"key2": "value2",
				},
				"email": "report@example.com",
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				emailDraft, ok := output["emailDraft"].(map[string]any)
				require.True(t, ok)
				// Complex types are formatted as strings
				assert.Contains(t, emailDraft["body"].(string), "42")
				assert.Contains(t, emailDraft["body"].(string), "item1")
			},
		},

		"email_with_empty_template": {
			node: api.WorkflowNode{
				Id:   "email-11",
				Type: api.WorkflowNodeTypeEmail,
				Data: &api.NodeData{
					Label: strPtr("Empty template"),
					Metadata: &map[string]any{
						"emailTemplate": map[string]any{},
					},
				},
			},
			executeVars: map[string]any{
				"email": "test@example.com",
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				emailDraft, ok := output["emailDraft"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "", emailDraft["subject"])
				assert.Equal(t, "", emailDraft["body"])
				assert.Equal(t, "test@example.com", emailDraft["to"])
			},
		},

		"email_with_nil_execute_vars": {
			node: api.WorkflowNode{
				Id:   "email-12",
				Type: api.WorkflowNodeTypeEmail,
				Data: &api.NodeData{
					Label: strPtr("Email with nil vars"),
					Metadata: &map[string]any{
						"emailTemplate": map[string]any{
							"subject": "Test {{undefined}}",
							"body":    "Variable: {{missing}}",
						},
					},
				},
			},
			executeVars:   nil,
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				emailDraft, ok := output["emailDraft"].(map[string]any)
				require.True(t, ok)
				// Placeholders remain unreplaced
				assert.Equal(t, "Test {{undefined}}", emailDraft["subject"])
				assert.Equal(t, "Variable: {{missing}}", emailDraft["body"])
			},
		},

		"email_with_non_string_input_variables": {
			node: api.WorkflowNode{
				Id:   "email-13",
				Type: api.WorkflowNodeTypeEmail,
				Data: &api.NodeData{
					Label: strPtr("Email with invalid input vars"),
					Metadata: &map[string]any{
						"emailTemplate": map[string]any{
							"subject": "Test",
							"body":    "Test body",
						},
						"inputVariables": []any{
							"validVar",
							123, // Non-string (should be skipped)
							"anotherVar",
							true, // Non-string (should be skipped)
						},
					},
				},
			},
			executeVars: map[string]any{
				"validVar":   "value1",
				"anotherVar": "value2",
				"email":      "test@example.com",
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				assert.NotNil(t, output["emailDraft"])
				assert.Equal(t, "sent", output["deliveryStatus"])
			},
		},
	}

	// Run test cases
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create service (no database needed for this function)
			service := &Service{}

			// Create output map
			output := make(map[string]any)

			// Call the function
			err := service.executeEmailNode(tc.node, tc.executeVars, output)

			// Check error
			if tc.expectedError {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
			}

			// Check output
			if !tc.expectedError {
				if tc.checkOutput != nil {
					tc.checkOutput(t, output)
				} else if tc.expectedOutput != nil {
					assert.Equal(t, tc.expectedOutput, output)
				}
			}
		})
	}
}

func TestExecuteConditionNode(t *testing.T) {
	// Define test cases using table-driven tests (map format)
	tests := map[string]struct {
		// Input
		executeVars map[string]any
		condition   *api.Condition

		// Expected output
		expectedOutput map[string]any
		expectedError  bool
		errorContains  string
		checkOutput    func(t *testing.T, output map[string]any)
	}{
		"greater_than_condition_met": {
			executeVars: map[string]any{
				"temperature": 35.5,
			},
			condition: &api.Condition{
				Operator:  api.GreaterThan,
				Threshold: 30.0,
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				assert.Equal(t, true, output["conditionMet"])
				assert.Equal(t, float32(30.0), output["threshold"])
				assert.Equal(t, "greater_than", output["operator"])
				assert.Equal(t, 35.5, output["actualValue"])
				assert.Contains(t, output["message"], "condition met")
			},
		},

		"greater_than_condition_not_met": {
			executeVars: map[string]any{
				"temperature": 25.0,
			},
			condition: &api.Condition{
				Operator:  api.GreaterThan,
				Threshold: 30.0,
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				assert.Equal(t, false, output["conditionMet"])
				assert.Equal(t, float32(30.0), output["threshold"])
				assert.Equal(t, "greater_than", output["operator"])
				assert.Equal(t, 25.0, output["actualValue"])
				assert.Contains(t, output["message"], "condition not met")
			},
		},

		"less_than_condition_met": {
			executeVars: map[string]any{
				"temperature": 15.0,
			},
			condition: &api.Condition{
				Operator:  api.LessThan,
				Threshold: 20.0,
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				assert.Equal(t, true, output["conditionMet"])
				assert.Equal(t, float32(20.0), output["threshold"])
				assert.Equal(t, "less_than", output["operator"])
				assert.Equal(t, 15.0, output["actualValue"])
				assert.Equal(t, "Temperature 15.0°C is less_than 20.0°C - condition met", output["message"])
			},
		},

		"less_than_condition_not_met": {
			executeVars: map[string]any{
				"temperature": 25.0,
			},
			condition: &api.Condition{
				Operator:  api.LessThan,
				Threshold: 20.0,
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				assert.Equal(t, false, output["conditionMet"])
				assert.Equal(t, float32(20.0), output["threshold"])
				assert.Equal(t, "less_than", output["operator"])
				assert.Equal(t, 25.0, output["actualValue"])
				assert.Equal(t, "Temperature 25.0°C is less_than 20.0°C - condition not met", output["message"])
			},
		},

		"equals_condition_met": {
			executeVars: map[string]any{
				"temperature": 20.0,
			},
			condition: &api.Condition{
				Operator:  api.Equals,
				Threshold: 20.0,
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				assert.Equal(t, true, output["conditionMet"])
				assert.Equal(t, float32(20.0), output["threshold"])
				assert.Equal(t, "equals", output["operator"])
				assert.Equal(t, 20.0, output["actualValue"])
			},
		},

		"equals_condition_not_met": {
			executeVars: map[string]any{
				"temperature": 20.1,
			},
			condition: &api.Condition{
				Operator:  api.Equals,
				Threshold: 20.0,
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				assert.Equal(t, false, output["conditionMet"])
				assert.Equal(t, float32(20.0), output["threshold"])
				assert.Equal(t, "equals", output["operator"])
				assert.Equal(t, 20.1, output["actualValue"])
			},
		},

		"greater_than_or_equal_condition_met_exact": {
			executeVars: map[string]any{
				"temperature": 30.0,
			},
			condition: &api.Condition{
				Operator:  api.GreaterThanOrEqual,
				Threshold: 30.0,
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				assert.Equal(t, true, output["conditionMet"])
				assert.Equal(t, float32(30.0), output["threshold"])
				assert.Equal(t, "greater_than_or_equal", output["operator"])
				assert.Equal(t, 30.0, output["actualValue"])
			},
		},

		"greater_than_or_equal_condition_met_greater": {
			executeVars: map[string]any{
				"temperature": 31.5,
			},
			condition: &api.Condition{
				Operator:  api.GreaterThanOrEqual,
				Threshold: 30.0,
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				assert.Equal(t, true, output["conditionMet"])
				assert.Equal(t, float32(30.0), output["threshold"])
				assert.Equal(t, "greater_than_or_equal", output["operator"])
				assert.Equal(t, 31.5, output["actualValue"])
			},
		},

		"less_than_or_equal_condition_met_exact": {
			executeVars: map[string]any{
				"temperature": 20.0,
			},
			condition: &api.Condition{
				Operator:  api.LessThanOrEqual,
				Threshold: 20.0,
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				assert.Equal(t, true, output["conditionMet"])
				assert.Equal(t, float32(20.0), output["threshold"])
				assert.Equal(t, "less_than_or_equal", output["operator"])
				assert.Equal(t, 20.0, output["actualValue"])
			},
		},

		"less_than_or_equal_condition_met_less": {
			executeVars: map[string]any{
				"temperature": 18.5,
			},
			condition: &api.Condition{
				Operator:  api.LessThanOrEqual,
				Threshold: 20.0,
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				assert.Equal(t, true, output["conditionMet"])
				assert.Equal(t, float32(20.0), output["threshold"])
				assert.Equal(t, "less_than_or_equal", output["operator"])
				assert.Equal(t, 18.5, output["actualValue"])
			},
		},

		"nil_condition": {
			executeVars: map[string]any{
				"temperature": 25.0,
			},
			condition:     nil,
			expectedError: true,
			errorContains: "condition configuration is missing",
		},

		"missing_temperature_in_execute_vars": {
			executeVars: map[string]any{
				"humidity": 70.0, // Wrong key
			},
			condition: &api.Condition{
				Operator:  api.GreaterThan,
				Threshold: 30.0,
			},
			expectedError: true,
			errorContains: "temperature not found in executeVars or invalid type",
		},

		"invalid_temperature_type_string": {
			executeVars: map[string]any{
				"temperature": "not-a-number",
			},
			condition: &api.Condition{
				Operator:  api.GreaterThan,
				Threshold: 30.0,
			},
			expectedError: true,
			errorContains: "temperature not found in executeVars or invalid type",
		},

		"invalid_temperature_type_int": {
			executeVars: map[string]any{
				"temperature": 25, // int instead of float64
			},
			condition: &api.Condition{
				Operator:  api.GreaterThan,
				Threshold: 30.0,
			},
			expectedError: true,
			errorContains: "temperature not found in executeVars or invalid type",
		},

		"nil_execute_vars": {
			executeVars: nil,
			condition: &api.Condition{
				Operator:  api.GreaterThan,
				Threshold: 30.0,
			},
			expectedError: true,
			errorContains: "temperature not found in executeVars or invalid type",
		},

		"negative_temperature_values": {
			executeVars: map[string]any{
				"temperature": -15.5,
			},
			condition: &api.Condition{
				Operator:  api.LessThan,
				Threshold: 0.0,
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				assert.Equal(t, true, output["conditionMet"])
				assert.Equal(t, float32(0.0), output["threshold"])
				assert.Equal(t, "less_than", output["operator"])
				assert.Equal(t, -15.5, output["actualValue"])
			},
		},

		"large_temperature_values": {
			executeVars: map[string]any{
				"temperature": 99999.99,
			},
			condition: &api.Condition{
				Operator:  api.GreaterThan,
				Threshold: 1000.0,
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				assert.Equal(t, true, output["conditionMet"])
				assert.Equal(t, float32(1000.0), output["threshold"])
				assert.Equal(t, "greater_than", output["operator"])
				assert.Equal(t, 99999.99, output["actualValue"])
			},
		},

		"zero_temperature_and_threshold": {
			executeVars: map[string]any{
				"temperature": 0.0,
			},
			condition: &api.Condition{
				Operator:  api.Equals,
				Threshold: 0.0,
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				assert.Equal(t, true, output["conditionMet"])
				assert.Equal(t, float32(0.0), output["threshold"])
				assert.Equal(t, "equals", output["operator"])
				assert.Equal(t, 0.0, output["actualValue"])
			},
		},

		"decimal_precision_comparison": {
			executeVars: map[string]any{
				"temperature": 20.0,
			},
			condition: &api.Condition{
				Operator:  api.Equals,
				Threshold: 20.0,
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				assert.Equal(t, true, output["conditionMet"])
				assert.Equal(t, float32(20.0), output["threshold"])
				assert.Equal(t, "equals", output["operator"])
				assert.Equal(t, 20.0, output["actualValue"])
			},
		},
	}

	// Run test cases
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create service (no database needed for this function)
			service := &Service{}

			// Create output map
			output := make(map[string]any)

			// Call the function
			err := service.executeConditionNode(tc.executeVars, output, tc.condition)

			// Check error
			if tc.expectedError {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
			}

			// Check output
			if !tc.expectedError {
				if tc.checkOutput != nil {
					tc.checkOutput(t, output)
				} else if tc.expectedOutput != nil {
					assert.Equal(t, tc.expectedOutput, output)
				}
			}
		})
	}
}

func TestExecuteIntegrationNode(t *testing.T) {
	// Define test cases using table-driven tests (map format)
	tests := map[string]struct {
		// Input
		node        api.WorkflowNode
		executeVars map[string]any
		mockServer  func() *httptest.Server

		// Expected output
		expectedError bool
		errorContains string
		checkOutput   func(t *testing.T, output map[string]any)
	}{
		"successful_weather_api_call": {
			node: api.WorkflowNode{
				Id:   "integration-1",
				Type: api.WorkflowNodeTypeIntegration,
				Data: &api.NodeData{
					Label: strPtr("Weather API"),
					Metadata: &map[string]any{
						"inputVariables": []any{"city"},
						"apiEndpoint":    "http://test-server/weather/{city}",
						"options": []any{
							map[string]any{
								"city": "Sydney",
							},
							map[string]any{
								"city": "Melbourne",
							},
						},
						"outputVariables": []any{"temperature", "city", "humidity"},
					},
				},
			},
			executeVars: map[string]any{
				"city": "Sydney",
			},
			mockServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]any{
						"temperature": 25.5,
						"humidity":    65,
						"conditions":  "sunny",
					})
				}))
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				assert.Equal(t, 25.5, output["temperature"])
				assert.Equal(t, float64(65), output["humidity"])
				assert.Equal(t, "Sydney", output["city"])
				assert.Contains(t, output["message"], "Weather data fetched for Sydney")
			},
		},

		"nested_json_response": {
			node: api.WorkflowNode{
				Id:   "integration-2",
				Type: api.WorkflowNodeTypeIntegration,
				Data: &api.NodeData{
					Label: strPtr("Nested API"),
					Metadata: &map[string]any{
						"inputVariables": []any{"id"},
						"apiEndpoint":    "http://test-server/data/{id}",
						"options": []any{
							map[string]any{"id": "123"},
						},
						"outputVariables": []any{"temperature", "status"},
					},
				},
			},
			executeVars: map[string]any{
				"id": "123",
			},
			mockServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]any{
						"data": map[string]any{
							"temperature": 30.0,
							"nested": map[string]any{
								"status": "active",
							},
						},
					})
				}))
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				assert.Equal(t, 30.0, output["temperature"])
				assert.Equal(t, "active", output["status"])
			},
		},

		"missing_metadata": {
			node: api.WorkflowNode{
				Id:   "integration-3",
				Type: api.WorkflowNodeTypeIntegration,
				Data: &api.NodeData{
					Label: strPtr("No metadata"),
				},
			},
			executeVars: map[string]any{
				"city": "Sydney",
			},
			mockServer: func() *httptest.Server {
				return nil // No server needed
			},
			expectedError: true,
			errorContains: "integration node missing metadata",
		},

		"missing_input_variables": {
			node: api.WorkflowNode{
				Id:   "integration-4",
				Type: api.WorkflowNodeTypeIntegration,
				Data: &api.NodeData{
					Label: strPtr("Missing input vars"),
					Metadata: &map[string]any{
						// Missing inputVariables
						"apiEndpoint": "http://test-server/api",
						"options":     []any{},
					},
				},
			},
			executeVars: map[string]any{},
			mockServer: func() *httptest.Server {
				return nil
			},
			expectedError: true,
			errorContains: "integration node missing inputVariables in metadata",
		},

		"missing_required_variable_in_execute_vars": {
			node: api.WorkflowNode{
				Id:   "integration-5",
				Type: api.WorkflowNodeTypeIntegration,
				Data: &api.NodeData{
					Label: strPtr("Missing required var"),
					Metadata: &map[string]any{
						"inputVariables": []any{"city", "country"},
						"apiEndpoint":    "http://test-server/api",
						"options":        []any{},
					},
				},
			},
			executeVars: map[string]any{
				"city": "Sydney",
				// country is missing
			},
			mockServer: func() *httptest.Server {
				return nil
			},
			expectedError: true,
			errorContains: "required input variable 'country' not found in executeVars",
		},

		"no_matching_option": {
			node: api.WorkflowNode{
				Id:   "integration-6",
				Type: api.WorkflowNodeTypeIntegration,
				Data: &api.NodeData{
					Label: strPtr("No matching option"),
					Metadata: &map[string]any{
						"inputVariables": []any{"city"},
						"apiEndpoint":    "http://test-server/api/{city}",
						"options": []any{
							map[string]any{"city": "Sydney"},
							map[string]any{"city": "Melbourne"},
						},
					},
				},
			},
			executeVars: map[string]any{
				"city": "Brisbane", // No matching option
			},
			mockServer: func() *httptest.Server {
				return nil
			},
			expectedError: true,
			errorContains: "no matching option found for input values",
		},

		"api_returns_error": {
			node: api.WorkflowNode{
				Id:   "integration-7",
				Type: api.WorkflowNodeTypeIntegration,
				Data: &api.NodeData{
					Label: strPtr("API error"),
					Metadata: &map[string]any{
						"inputVariables": []any{"id"},
						"apiEndpoint":    "http://test-server/error/{id}",
						"options": []any{
							map[string]any{"id": "123"},
						},
						"outputVariables": []any{"data"},
					},
				},
			},
			executeVars: map[string]any{
				"id": "123",
			},
			mockServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(map[string]any{
						"error": "Internal Server Error",
					})
				}))
			},
			expectedError: true, // API error with 500 status should fail
			errorContains: "API returned status 500",
		},

		"invalid_json_response": {
			node: api.WorkflowNode{
				Id:   "integration-8",
				Type: api.WorkflowNodeTypeIntegration,
				Data: &api.NodeData{
					Label: strPtr("Invalid JSON"),
					Metadata: &map[string]any{
						"inputVariables": []any{"id"},
						"apiEndpoint":    "http://test-server/invalid/{id}",
						"options": []any{
							map[string]any{"id": "123"},
						},
					},
				},
			},
			executeVars: map[string]any{
				"id": "123",
			},
			mockServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte("not valid json"))
				}))
			},
			expectedError: true,
			errorContains: "failed to parse API response",
		},

		"multiple_input_variables": {
			node: api.WorkflowNode{
				Id:   "integration-9",
				Type: api.WorkflowNodeTypeIntegration,
				Data: &api.NodeData{
					Label: strPtr("Multiple inputs"),
					Metadata: &map[string]any{
						"inputVariables": []any{"city", "date"},
						"apiEndpoint":    "http://test-server/weather/{city}/{date}",
						"options": []any{
							map[string]any{
								"city": "Sydney",
								"date": "2024-01-01",
							},
						},
						"outputVariables": []any{"temperature"},
					},
				},
			},
			executeVars: map[string]any{
				"city": "Sydney",
				"date": "2024-01-01",
			},
			mockServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]any{
						"temperature": 28.5,
					})
				}))
			},
			expectedError: false,
			checkOutput: func(t *testing.T, output map[string]any) {
				assert.Equal(t, 28.5, output["temperature"])
				assert.Contains(t, output["message"], "Weather data fetched for Sydney")
			},
		},

		"nil_data": {
			node: api.WorkflowNode{
				Id:   "integration-10",
				Type: api.WorkflowNodeTypeIntegration,
				Data: nil,
			},
			executeVars: map[string]any{},
			mockServer: func() *httptest.Server {
				return nil
			},
			expectedError: true,
			errorContains: "integration node missing metadata",
		},

		"missing_api_endpoint": {
			node: api.WorkflowNode{
				Id:   "integration-11",
				Type: api.WorkflowNodeTypeIntegration,
				Data: &api.NodeData{
					Label: strPtr("No endpoint"),
					Metadata: &map[string]any{
						"inputVariables": []any{"id"},
						"options": []any{
							map[string]any{"id": "123"},
						},
					},
				},
			},
			executeVars: map[string]any{
				"id": "123",
			},
			mockServer: func() *httptest.Server {
				return nil
			},
			expectedError: true,
			errorContains: "integration node missing apiEndpoint in metadata",
		},

		"invalid_options_format": {
			node: api.WorkflowNode{
				Id:   "integration-12",
				Type: api.WorkflowNodeTypeIntegration,
				Data: &api.NodeData{
					Label: strPtr("Invalid options"),
					Metadata: &map[string]any{
						"inputVariables": []any{"id"},
						"apiEndpoint":    "http://test-server/api",
						"options":        "not-an-array", // Invalid format
					},
				},
			},
			executeVars: map[string]any{
				"id": "123",
			},
			mockServer: func() *httptest.Server {
				return nil
			},
			expectedError: true,
			errorContains: "options must be an array",
		},
	}

	// Run test cases
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create mock server if needed
			var server *httptest.Server
			if tc.mockServer != nil {
				server = tc.mockServer()
				if server != nil {
					defer server.Close()
					// Replace the test server URL in the API endpoint
					if tc.node.Data != nil && tc.node.Data.Metadata != nil {
						metadata := *tc.node.Data.Metadata
						if endpoint, ok := metadata["apiEndpoint"].(string); ok {
							metadata["apiEndpoint"] = strings.Replace(endpoint, "http://test-server", server.URL, 1)
						}
					}
				}
			}

			// Create service
			service := &Service{}

			// Create output map
			output := make(map[string]any)

			// Call the function
			err := service.executeIntegrationNode(context.Background(), tc.node, tc.executeVars, output)

			// Check error
			if tc.expectedError {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
			}

			// Check output
			if !tc.expectedError && tc.checkOutput != nil {
				tc.checkOutput(t, output)
			}
		})
	}
}

func TestExecuteSingleNode(t *testing.T) {
	// Define test cases using table-driven tests (map format)
	tests := map[string]struct {
		// Input
		node        api.WorkflowNode
		executeVars map[string]any
		input       api.WorkflowExecutionInput

		// Expected
		expectedStatus   api.ExecutionStepStatus
		checkStep        func(t *testing.T, step api.ExecutionStep)
		checkExecuteVars func(t *testing.T, executeVars map[string]any)
	}{
		"start_node": {
			node: api.WorkflowNode{
				Id:   "start-1",
				Type: api.WorkflowNodeTypeStart,
				Data: &api.NodeData{
					Label:       strPtr("Start Node"),
					Description: strPtr("Beginning of workflow"),
				},
			},
			executeVars:    map[string]any{},
			input:          api.WorkflowExecutionInput{},
			expectedStatus: api.ExecutionStepStatusCompleted,
			checkStep: func(t *testing.T, step api.ExecutionStep) {
				assert.Equal(t, "start-1", step.NodeId)
				assert.Equal(t, "start", step.Type)
				assert.Equal(t, api.ExecutionStepStatusCompleted, step.Status)
				assert.Equal(t, "Start Node", *step.Label)
				assert.Equal(t, "Beginning of workflow", *step.Description)

				output := *step.Output
				assert.Equal(t, "Workflow started successfully", output["message"])
			},
		},

		"end_node": {
			node: api.WorkflowNode{
				Id:   "end-1",
				Type: api.WorkflowNodeTypeEnd,
				Data: &api.NodeData{
					Label:       strPtr("End Node"),
					Description: strPtr("End of workflow"),
				},
			},
			executeVars: map[string]any{
				"result": "success",
			},
			input:          api.WorkflowExecutionInput{},
			expectedStatus: api.ExecutionStepStatusCompleted,
			checkStep: func(t *testing.T, step api.ExecutionStep) {
				assert.Equal(t, "end-1", step.NodeId)
				assert.Equal(t, "end", step.Type)
				assert.Equal(t, api.ExecutionStepStatusCompleted, step.Status)

				output := *step.Output
				assert.Equal(t, "Workflow completed successfully", output["message"])
			},
		},

		"form_node_success": {
			node: api.WorkflowNode{
				Id:   "form-1",
				Type: api.WorkflowNodeTypeForm,
				Data: &api.NodeData{
					Label: strPtr("User Form"),
					Metadata: &map[string]any{
						"outputVariables": []any{"name", "email"},
					},
				},
			},
			executeVars: map[string]any{
				"name":  "John Doe",
				"email": "john@example.com",
				"extra": "ignored",
			},
			input:          api.WorkflowExecutionInput{},
			expectedStatus: api.ExecutionStepStatusCompleted,
			checkStep: func(t *testing.T, step api.ExecutionStep) {
				assert.Equal(t, api.ExecutionStepStatusCompleted, step.Status)
				assert.Nil(t, step.Error)

				output := *step.Output
				assert.Equal(t, "Form data executed successfully", output["message"])
				assert.Equal(t, "John Doe", output["name"])
				assert.Equal(t, "john@example.com", output["email"])
			},
		},

		"form_node_failure": {
			node: api.WorkflowNode{
				Id:   "form-2",
				Type: api.WorkflowNodeTypeForm,
				Data: &api.NodeData{
					Label: strPtr("Invalid Form"),
					Metadata: &map[string]any{
						"outputVariables": "not-an-array", // Invalid format
					},
				},
			},
			executeVars: map[string]any{
				"name": "Test",
			},
			input:          api.WorkflowExecutionInput{},
			expectedStatus: api.ExecutionStepStatusFailed,
			checkStep: func(t *testing.T, step api.ExecutionStep) {
				assert.Equal(t, api.ExecutionStepStatusFailed, step.Status)
				assert.NotNil(t, step.Error)
				assert.Contains(t, *step.Error, "outputVariables must be an array")

				output := *step.Output
				assert.Equal(t, "Failed to execute form data", output["message"])
			},
		},

		"condition_node_success": {
			node: api.WorkflowNode{
				Id:   "condition-1",
				Type: api.WorkflowNodeTypeCondition,
				Data: &api.NodeData{
					Label:       strPtr("Temperature Check"),
					Description: strPtr("Check if temperature exceeds threshold"),
				},
			},
			executeVars: map[string]any{
				"temperature": 35.5,
			},
			input: api.WorkflowExecutionInput{
				Condition: &api.Condition{
					Operator:  api.GreaterThan,
					Threshold: 30.0,
				},
			},
			expectedStatus: api.ExecutionStepStatusCompleted,
			checkStep: func(t *testing.T, step api.ExecutionStep) {
				assert.Equal(t, api.ExecutionStepStatusCompleted, step.Status)
				assert.Nil(t, step.Error)

				output := *step.Output
				assert.Equal(t, true, output["conditionMet"])
				assert.Contains(t, output["message"], "condition met")
			},
			checkExecuteVars: func(t *testing.T, executeVars map[string]any) {
				// Check that condition result was added to executeVars
				assert.Equal(t, true, executeVars["conditionMet"])
			},
		},

		"condition_node_failure": {
			node: api.WorkflowNode{
				Id:   "condition-2",
				Type: api.WorkflowNodeTypeCondition,
				Data: &api.NodeData{
					Label: strPtr("Missing Condition"),
				},
			},
			executeVars: map[string]any{
				"temperature": 25.0,
			},
			input: api.WorkflowExecutionInput{
				// Missing condition
			},
			expectedStatus: api.ExecutionStepStatusFailed,
			checkStep: func(t *testing.T, step api.ExecutionStep) {
				assert.Equal(t, api.ExecutionStepStatusFailed, step.Status)
				assert.NotNil(t, step.Error)
				assert.Contains(t, *step.Error, "condition configuration is missing")

				output := *step.Output
				assert.Equal(t, "Failed to evaluate condition", output["message"])
			},
		},

		"email_node_success": {
			node: api.WorkflowNode{
				Id:   "email-1",
				Type: api.WorkflowNodeTypeEmail,
				Data: &api.NodeData{
					Label: strPtr("Send Alert"),
					Metadata: &map[string]any{
						"emailTemplate": map[string]any{
							"subject": "Alert: {{city}}",
							"body":    "Temperature is {{temperature}}°C",
						},
					},
				},
			},
			executeVars: map[string]any{
				"city":         "Sydney",
				"temperature":  35.5,
				"email":        "user@example.com",
				"conditionMet": true,
			},
			input:          api.WorkflowExecutionInput{},
			expectedStatus: api.ExecutionStepStatusCompleted,
			checkStep: func(t *testing.T, step api.ExecutionStep) {
				assert.Equal(t, api.ExecutionStepStatusCompleted, step.Status)
				assert.Nil(t, step.Error)

				output := *step.Output
				emailDraft := output["emailDraft"].(map[string]any)
				assert.Equal(t, "Alert: Sydney", emailDraft["subject"])
				assert.Equal(t, "Temperature is 35.5°C", emailDraft["body"])
			},
		},

		"email_node_skipped": {
			node: api.WorkflowNode{
				Id:   "email-2",
				Type: api.WorkflowNodeTypeEmail,
				Data: &api.NodeData{
					Label: strPtr("Conditional Email"),
					Metadata: &map[string]any{
						"emailTemplate": map[string]any{
							"subject": "Alert",
							"body":    "Condition not met",
						},
					},
				},
			},
			executeVars: map[string]any{
				"email":        "user@example.com",
				"conditionMet": false, // Condition not met
			},
			input:          api.WorkflowExecutionInput{},
			expectedStatus: api.ExecutionStepStatusSkipped,
			checkStep: func(t *testing.T, step api.ExecutionStep) {
				assert.Equal(t, api.ExecutionStepStatusSkipped, step.Status)
				assert.Nil(t, step.Error)

				output := *step.Output
				assert.Equal(t, "Email alert skipped - condition not met", output["message"])
			},
		},

		"email_node_failure": {
			node: api.WorkflowNode{
				Id:   "email-3",
				Type: api.WorkflowNodeTypeEmail,
				Data: &api.NodeData{
					Label: strPtr("Invalid Email"),
					// Missing metadata
				},
			},
			executeVars: map[string]any{
				"email": "user@example.com",
			},
			input:          api.WorkflowExecutionInput{},
			expectedStatus: api.ExecutionStepStatusFailed,
			checkStep: func(t *testing.T, step api.ExecutionStep) {
				assert.Equal(t, api.ExecutionStepStatusFailed, step.Status)
				assert.NotNil(t, step.Error)
				assert.Contains(t, *step.Error, "email node missing metadata")

				output := *step.Output
				assert.Equal(t, "Failed to execute email", output["message"])
			},
		},

		"integration_node_with_description_placeholders": {
			node: api.WorkflowNode{
				Id:   "integration-1",
				Type: api.WorkflowNodeTypeIntegration,
				Data: &api.NodeData{
					Label:       strPtr("Weather API"),
					Description: strPtr("Fetching weather for {{city}}: {{temperature}}°C"),
					Metadata: &map[string]any{
						"inputVariables": []any{"city"},
						"apiEndpoint":    "http://test-server/weather/{city}",
						"options": []any{
							map[string]any{"city": "Sydney"},
						},
						"outputVariables": []any{"temperature"},
					},
				},
			},
			executeVars: map[string]any{
				"city": "Sydney",
			},
			input:          api.WorkflowExecutionInput{},
			expectedStatus: api.ExecutionStepStatusFailed, // Will fail due to no mock server
			checkStep: func(t *testing.T, step api.ExecutionStep) {
				// Even though it fails, we can check the basic step structure
				assert.Equal(t, "integration-1", step.NodeId)
				assert.Equal(t, "integration", step.Type)
				assert.Equal(t, "Weather API", *step.Label)
			},
		},

		"node_with_nil_data": {
			node: api.WorkflowNode{
				Id:   "node-nil",
				Type: api.WorkflowNodeTypeStart,
				Data: nil, // Nil data
			},
			executeVars:    map[string]any{},
			input:          api.WorkflowExecutionInput{},
			expectedStatus: api.ExecutionStepStatusCompleted,
			checkStep: func(t *testing.T, step api.ExecutionStep) {
				assert.Equal(t, "node-nil", step.NodeId)
				assert.Equal(t, "start", step.Type)
				assert.Equal(t, api.ExecutionStepStatusCompleted, step.Status)
				assert.Equal(t, "", *step.Label)       // Empty label
				assert.Equal(t, "", *step.Description) // Empty description
			},
		},
	}

	// Run test cases
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create service
			service := &Service{}

			// Create a copy of executeVars to check mutations
			executeVarsCopy := make(map[string]any)
			for k, v := range tc.executeVars {
				executeVarsCopy[k] = v
			}

			// Call the function
			step := service.executeSingleNode(
				context.Background(),
				tc.node,
				executeVarsCopy,
				tc.input,
			)

			// Check basic step properties
			assert.Equal(t, tc.node.Id, step.NodeId)
			assert.Equal(t, string(tc.node.Type), step.Type)
			assert.Equal(t, tc.expectedStatus, step.Status)

			// Run custom checks
			if tc.checkStep != nil {
				tc.checkStep(t, step)
			}

			// Check executeVars mutations if specified
			if tc.checkExecuteVars != nil {
				tc.checkExecuteVars(t, executeVarsCopy)
			}
		})
	}
}

// Helper function to create string pointers
func strPtr(s string) *string {
	return &s
}
