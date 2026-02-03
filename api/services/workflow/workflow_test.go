package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	api "workflow-code-test/api/openapi"
	"workflow-code-test/api/pkg/cache"
	cachemocks "workflow-code-test/api/pkg/cache/mocks"
	dbmocks "workflow-code-test/api/pkg/db/mocks"
	"workflow-code-test/api/pkg/db/models"

	"github.com/aarondl/null/v8"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleGetWorkflow(t *testing.T) {
	// Define test cases using table-driven tests (map format)
	tests := map[string]struct {
		// Input
		workflowID string

		// Mock setup
		setupMock func(mockDB *dbmocks.MockWorkFlowDB, mockCache *cachemocks.MockCache)

		// Expected response
		expectedStatus int
		expectedBody   interface{} // Can be api.Workflow or api.Error
		checkResponse  func(t *testing.T, body []byte)
	}{
		"success_with_workflow_data": {
			workflowID: "550e8400-e29b-41d4-a716-446655440000",
			setupMock: func(mockDB *dbmocks.MockWorkFlowDB, mockCache *cachemocks.MockCache) {
				// Mock cache miss so it goes to database
				cacheKey := "workflow:550e8400-e29b-41d4-a716-446655440000"
				mockCache.EXPECT().
					Get(gomock.Any(), cacheKey, gomock.Any()).
					Return(cache.ErrCacheMiss{Key: cacheKey})

				// Create workflow without relationships (R field will be nil)
				// The mapper handles nil R field gracefully
				workflow := &models.Workflow{
					ID:          "550e8400-e29b-41d4-a716-446655440000",
					Name:        "Test Workflow",
					Description: null.StringFrom("Test Description"),
					CreatedAt:   null.TimeFrom(time.Now()),
					UpdatedAt:   null.TimeFrom(time.Now()),
					R:           nil, // Relationships not loaded - mapper will handle this
				}

				mockDB.EXPECT().
					GetWorkflowByID(gomock.Any(), "550e8400-e29b-41d4-a716-446655440000").
					Return(workflow, nil)

				// Mock cache set after retrieving from DB
				mockCache.EXPECT().
					Set(gomock.Any(), cacheKey, gomock.Any(), gomock.Any()).
					Return(nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var response api.Workflow
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)

				expectedUUID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
				assert.Equal(t, expectedUUID, uuid.UUID(response.Id))
				assert.NotNil(t, response.Name)
				assert.Equal(t, "Test Workflow", *response.Name)
				assert.NotNil(t, response.Description)
				assert.Equal(t, "Test Description", *response.Description)

				// When R is nil, nodes and edges should be nil
				assert.Nil(t, response.Nodes)
				assert.Nil(t, response.Edges)
			},
		},

		"workflow_with_complete_nodes_and_edges": {
			workflowID: "550e8400-e29b-41d4-a716-446655440000",
			setupMock: func(mockDB *dbmocks.MockWorkFlowDB, mockCache *cachemocks.MockCache) {
				// Mock cache miss so it goes to database
				cacheKey := "workflow:550e8400-e29b-41d4-a716-446655440000"
				mockCache.EXPECT().
					Get(gomock.Any(), cacheKey, gomock.Any()).
					Return(cache.ErrCacheMiss{Key: cacheKey})

				// Create workflow with complete node and edge data
				workflow := &models.Workflow{
					ID:          "550e8400-e29b-41d4-a716-446655440000",
					Name:        "Complete Workflow",
					Description: null.StringFrom("Workflow with nodes and edges"),
					CreatedAt:   null.TimeFrom(time.Now()),
					UpdatedAt:   null.TimeFrom(time.Now()),
				}

				// Initialize relationships with nodes and edges
				workflow.R = workflow.R.NewStruct()

				// Add workflow nodes
				workflow.R.WorkflowNodes = models.WorkflowNodeSlice{
					&models.WorkflowNode{
						ID:         "node-1",
						WorkflowID: "550e8400-e29b-41d4-a716-446655440000",
						NodeID:     "node-1",
						Type:       "start",
						Position:   []byte(`{"x":100,"y":100}`),
						Data:       null.JSONFrom([]byte(`{"label":"Start Node","description":"Start of workflow"}`)),
					},
					&models.WorkflowNode{
						ID:         "node-2",
						WorkflowID: "550e8400-e29b-41d4-a716-446655440000",
						NodeID:     "node-2",
						Type:       "form",
						Position:   []byte(`{"x":300,"y":100}`),
						Data:       null.JSONFrom([]byte(`{"label":"Form Input","fields":["name","email"]}`)),
					},
					&models.WorkflowNode{
						ID:         "node-3",
						WorkflowID: "550e8400-e29b-41d4-a716-446655440000",
						NodeID:     "node-3",
						Type:       "end",
						Position:   []byte(`{"x":500,"y":100}`),
						Data:       null.JSONFrom([]byte(`{"label":"End Node","description":"End of workflow"}`)),
					},
				}

				// Add workflow edges
				workflow.R.WorkflowEdges = models.WorkflowEdgeSlice{
					&models.WorkflowEdge{
						ID:         "edge-1",
						WorkflowID: "550e8400-e29b-41d4-a716-446655440000",
						EdgeID:     "edge-1",
						Source:     "node-1",
						Target:     "node-2",
						Label:      null.StringFrom("To Form"),
					},
					&models.WorkflowEdge{
						ID:         "edge-2",
						WorkflowID: "550e8400-e29b-41d4-a716-446655440000",
						EdgeID:     "edge-2",
						Source:     "node-2",
						Target:     "node-3",
						Label:      null.StringFrom("To End"),
					},
				}

				mockDB.EXPECT().
					GetWorkflowByID(gomock.Any(), "550e8400-e29b-41d4-a716-446655440000").
					Return(workflow, nil)

				// Mock cache set after retrieving from DB
				mockCache.EXPECT().
					Set(gomock.Any(), cacheKey, gomock.Any(), gomock.Any()).
					Return(nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var response api.Workflow
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)

				expectedUUID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
				assert.Equal(t, expectedUUID, uuid.UUID(response.Id))
				assert.NotNil(t, response.Name)
				assert.Equal(t, "Complete Workflow", *response.Name)
				assert.NotNil(t, response.Description)
				assert.Equal(t, "Workflow with nodes and edges", *response.Description)

				// Verify nodes are present
				require.NotNil(t, response.Nodes)
				assert.Len(t, *response.Nodes, 3)

				// Check first node details
				nodes := *response.Nodes
				assert.Equal(t, "node-1", nodes[0].Id)
				assert.Equal(t, api.WorkflowNodeType("start"), nodes[0].Type)
				if nodes[0].Data != nil && nodes[0].Data.Label != nil {
					assert.Equal(t, "Start Node", *nodes[0].Data.Label)
				}

				// Verify edges are present
				require.NotNil(t, response.Edges)
				assert.Len(t, *response.Edges, 2)

				// Check first edge details
				edges := *response.Edges
				assert.Equal(t, "edge-1", edges[0].Id)
				assert.Equal(t, "node-1", edges[0].Source)
				assert.Equal(t, "node-2", edges[0].Target)
			},
		},

		"workflow_not_found": {
			workflowID: "non-existent-id",
			setupMock: func(mockDB *dbmocks.MockWorkFlowDB, mockCache *cachemocks.MockCache) {
				// Mock cache miss so it goes to database
				cacheKey := "workflow:non-existent-id"
				mockCache.EXPECT().
					Get(gomock.Any(), cacheKey, gomock.Any()).
					Return(cache.ErrCacheMiss{Key: cacheKey})

				mockDB.EXPECT().
					GetWorkflowByID(gomock.Any(), "non-existent-id").
					Return(nil, fmt.Errorf("workflow not found: non-existent-id"))
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body []byte) {
				var response api.Error
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)
				assert.Equal(t, "Workflow not found", response.Error)
			},
		},

		"database_error": {
			workflowID: "550e8400-e29b-41d4-a716-446655440000",
			setupMock: func(mockDB *dbmocks.MockWorkFlowDB, mockCache *cachemocks.MockCache) {
				// Mock cache miss so it goes to database
				cacheKey := "workflow:550e8400-e29b-41d4-a716-446655440000"
				mockCache.EXPECT().
					Get(gomock.Any(), cacheKey, gomock.Any()).
					Return(cache.ErrCacheMiss{Key: cacheKey})

				mockDB.EXPECT().
					GetWorkflowByID(gomock.Any(), "550e8400-e29b-41d4-a716-446655440000").
					Return(nil, errors.New("database connection error"))
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, body []byte) {
				var response api.Error
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)
				assert.Equal(t, "Failed to retrieve workflow", response.Error)
			},
		},

		"invalid_workflow_id_format": {
			workflowID: "invalid-uuid",
			setupMock: func(mockDB *dbmocks.MockWorkFlowDB, mockCache *cachemocks.MockCache) {
				// Mock cache miss so it goes to database
				cacheKey := "workflow:invalid-uuid"
				mockCache.EXPECT().
					Get(gomock.Any(), cacheKey, gomock.Any()).
					Return(cache.ErrCacheMiss{Key: cacheKey})

				workflow := &models.Workflow{
					ID:          "invalid-uuid", // This will fail UUID parsing
					Name:        "Test Workflow",
					Description: null.StringFrom("Test Description"),
				}

				mockDB.EXPECT().
					GetWorkflowByID(gomock.Any(), "invalid-uuid").
					Return(workflow, nil)
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, body []byte) {
				var response api.Error
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)
				assert.Equal(t, "Failed to retrieve workflow", response.Error)
			},
		},

		"workflow_with_nil_relationships": {
			workflowID: "550e8400-e29b-41d4-a716-446655440000",
			setupMock: func(mockDB *dbmocks.MockWorkFlowDB, mockCache *cachemocks.MockCache) {
				// Mock cache miss so it goes to database
				cacheKey := "workflow:550e8400-e29b-41d4-a716-446655440000"
				mockCache.EXPECT().
					Get(gomock.Any(), cacheKey, gomock.Any()).
					Return(cache.ErrCacheMiss{Key: cacheKey})

				workflow := &models.Workflow{
					ID:          "550e8400-e29b-41d4-a716-446655440000",
					Name:        "Minimal Workflow",
					Description: null.StringFrom("Minimal Description"),
					R:           nil, // No relationships loaded
				}

				mockDB.EXPECT().
					GetWorkflowByID(gomock.Any(), "550e8400-e29b-41d4-a716-446655440000").
					Return(workflow, nil)

				// Mock cache set after retrieving from DB
				mockCache.EXPECT().
					Set(gomock.Any(), cacheKey, gomock.Any(), gomock.Any()).
					Return(nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var response api.Workflow
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)

				expectedUUID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
				assert.Equal(t, expectedUUID, uuid.UUID(response.Id))
				assert.NotNil(t, response.Name)
				assert.Equal(t, "Minimal Workflow", *response.Name)

				// Nodes and edges should be nil when relationships are not loaded
				assert.Nil(t, response.Nodes)
				assert.Nil(t, response.Edges)
			},
		},

		"workflow_with_complex_conditional_edges": {
			workflowID: "550e8400-e29b-41d4-a716-446655440001",
			setupMock: func(mockDB *dbmocks.MockWorkFlowDB, mockCache *cachemocks.MockCache) {
				// Mock cache miss so it goes to database
				cacheKey := "workflow:550e8400-e29b-41d4-a716-446655440001"
				mockCache.EXPECT().
					Get(gomock.Any(), cacheKey, gomock.Any()).
					Return(cache.ErrCacheMiss{Key: cacheKey})

				// Create workflow with conditional branching
				workflow := &models.Workflow{
					ID:          "550e8400-e29b-41d4-a716-446655440001",
					Name:        "Conditional Workflow",
					Description: null.StringFrom("Workflow with conditional branching"),
					CreatedAt:   null.TimeFrom(time.Now()),
					UpdatedAt:   null.TimeFrom(time.Now()),
				}

				// Initialize relationships
				workflow.R = workflow.R.NewStruct()

				// Add nodes including decision node
				workflow.R.WorkflowNodes = models.WorkflowNodeSlice{
					&models.WorkflowNode{
						ID:         "node-start",
						WorkflowID: "550e8400-e29b-41d4-a716-446655440001",
						NodeID:     "node-start",
						Type:       "start",
						Position:   []byte(`{"x":100,"y":200}`),
						Data:       null.JSONFrom([]byte(`{"label":"Start"}`)),
					},
					&models.WorkflowNode{
						ID:         "node-decision",
						WorkflowID: "550e8400-e29b-41d4-a716-446655440001",
						NodeID:     "node-decision",
						Type:       "condition",
						Position:   []byte(`{"x":300,"y":200}`),
						Data:       null.JSONFrom([]byte(`{"label":"Decision Point","field":"amount","operator":">"}`)),
					},
					&models.WorkflowNode{
						ID:         "node-branch-a",
						WorkflowID: "550e8400-e29b-41d4-a716-446655440001",
						NodeID:     "node-branch-a",
						Type:       "integration",
						Position:   []byte(`{"x":500,"y":100}`),
						Data:       null.JSONFrom([]byte(`{"label":"High Value Action","action":"premium"}`)),
					},
					&models.WorkflowNode{
						ID:         "node-branch-b",
						WorkflowID: "550e8400-e29b-41d4-a716-446655440001",
						NodeID:     "node-branch-b",
						Type:       "integration",
						Position:   []byte(`{"x":500,"y":300}`),
						Data:       null.JSONFrom([]byte(`{"label":"Regular Action","action":"standard"}`)),
					},
					&models.WorkflowNode{
						ID:         "node-end",
						WorkflowID: "550e8400-e29b-41d4-a716-446655440001",
						NodeID:     "node-end",
						Type:       "end",
						Position:   []byte(`{"x":700,"y":200}`),
						Data:       null.JSONFrom([]byte(`{"label":"End"}`)),
					},
				}

				// Add conditional edges
				workflow.R.WorkflowEdges = models.WorkflowEdgeSlice{
					&models.WorkflowEdge{
						ID:         "edge-start",
						WorkflowID: "550e8400-e29b-41d4-a716-446655440001",
						EdgeID:     "edge-start",
						Source:     "node-start",
						Target:     "node-decision",
					},
					&models.WorkflowEdge{
						ID:           "edge-high",
						WorkflowID:   "550e8400-e29b-41d4-a716-446655440001",
						EdgeID:       "edge-high",
						Source:       "node-decision",
						Target:       "node-branch-a",
						SourceHandle: null.StringFrom("true"),
						Label:        null.StringFrom(">1000"),
						Style:        null.JSONFrom([]byte(`{"stroke":"green"}`)),
					},
					&models.WorkflowEdge{
						ID:           "edge-low",
						WorkflowID:   "550e8400-e29b-41d4-a716-446655440001",
						EdgeID:       "edge-low",
						Source:       "node-decision",
						Target:       "node-branch-b",
						SourceHandle: null.StringFrom("false"),
						Label:        null.StringFrom("<=1000"),
						Style:        null.JSONFrom([]byte(`{"stroke":"orange"}`)),
					},
					&models.WorkflowEdge{
						ID:         "edge-merge-a",
						WorkflowID: "550e8400-e29b-41d4-a716-446655440001",
						EdgeID:     "edge-merge-a",
						Source:     "node-branch-a",
						Target:     "node-end",
					},
					&models.WorkflowEdge{
						ID:         "edge-merge-b",
						WorkflowID: "550e8400-e29b-41d4-a716-446655440001",
						EdgeID:     "edge-merge-b",
						Source:     "node-branch-b",
						Target:     "node-end",
					},
				}

				mockDB.EXPECT().
					GetWorkflowByID(gomock.Any(), "550e8400-e29b-41d4-a716-446655440001").
					Return(workflow, nil)

				// Mock cache set after retrieving from DB
				mockCache.EXPECT().
					Set(gomock.Any(), cacheKey, gomock.Any(), gomock.Any()).
					Return(nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var response api.Workflow
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)

				assert.NotNil(t, response.Name)
				assert.Equal(t, "Conditional Workflow", *response.Name)

				// Verify nodes
				require.NotNil(t, response.Nodes)
				assert.Len(t, *response.Nodes, 5)

				// Verify edges with conditions
				require.NotNil(t, response.Edges)
				assert.Len(t, *response.Edges, 5)

				// Check that conditional edges have labels
				edges := *response.Edges
				for _, edge := range edges {
					if edge.Id == "edge-high" || edge.Id == "edge-low" {
						assert.NotNil(t, edge.Label, "Conditional edge should have label")
					}
				}
			},
		},
	}

	// Run test cases
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create mock controller
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Create mocks
			mockDB := dbmocks.NewMockWorkFlowDB(ctrl)
			mockCache := cachemocks.NewMockCache(ctrl)

			// Setup expectations
			tc.setupMock(mockDB, mockCache)

			// Create service with mock
			service := &Service{
				db:    mockDB,
				cache: mockCache,
			}

			// Create test request
			req, err := http.NewRequest("GET", fmt.Sprintf("/workflows/%s", tc.workflowID), nil)
			require.NoError(t, err)

			// Add route variables
			req = mux.SetURLVars(req, map[string]string{"id": tc.workflowID})

			// Create response recorder
			rr := httptest.NewRecorder()

			// Call the handler
			service.HandleGetWorkflow(rr, req)

			// Check status code
			assert.Equal(t, tc.expectedStatus, rr.Code)

			// Check response body
			if tc.checkResponse != nil {
				tc.checkResponse(t, rr.Body.Bytes())
			}

			// Check content-type header
			contentType := rr.Header().Get("Content-Type")
			assert.Equal(t, "application/json", contentType)
		})
	}
}

func TestHandleExecuteWorkflow(t *testing.T) {
	tests := map[string]struct {
		// Input
		workflowID  string
		requestBody interface{}

		// Mock setup
		setupMock func(mockDB *dbmocks.MockWorkFlowDB, mockCache *cachemocks.MockCache)

		// Expected response
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		"successful_execution": {
			workflowID: "550e8400-e29b-41d4-a716-446655440000",
			requestBody: api.WorkflowExecutionInput{
				FormData: &map[string]interface{}{
					"name":  "John Doe",
					"email": "john@example.com",
					"city":  "Sydney",
				},
				Condition: &api.Condition{
					Operator:  api.GreaterThan,
					Threshold: 20.0,
				},
			},
			setupMock: func(mockDB *dbmocks.MockWorkFlowDB, mockCache *cachemocks.MockCache) {
				// Mock cache miss so it goes to database
				cacheKey := "workflow:550e8400-e29b-41d4-a716-446655440000"
				mockCache.EXPECT().
					Get(gomock.Any(), cacheKey, gomock.Any()).
					Return(cache.ErrCacheMiss{Key: cacheKey})

				// For execution test, we need a workflow with at least start and end nodes
				workflow := &models.Workflow{
					ID:   "550e8400-e29b-41d4-a716-446655440000",
					Name: "Test Workflow",
				}

				// Initialize relationships
				workflow.R = workflow.R.NewStruct()

				// Add basic nodes for execution
				workflow.R.WorkflowNodes = models.WorkflowNodeSlice{
					&models.WorkflowNode{
						ID:         "start",
						WorkflowID: "550e8400-e29b-41d4-a716-446655440000",
						NodeID:     "start",
						Type:       "start",
						Position:   []byte(`{"x":100,"y":100}`),
						Data:       null.JSONFrom([]byte(`{"label":"Start"}`)),
					},
					&models.WorkflowNode{
						ID:         "node-form",
						WorkflowID: "550e8400-e29b-41d4-a716-446655440000",
						NodeID:     "node-form",
						Type:       "form",
						Position:   []byte(`{"x":200,"y":100}`),
						Data:       null.JSONFrom([]byte(`{"label":"Form Input","metadata":{"outputVariables":["name","email","city"]}}`)),
					},
					&models.WorkflowNode{
						ID:         "node-end",
						WorkflowID: "550e8400-e29b-41d4-a716-446655440000",
						NodeID:     "node-end",
						Type:       "end",
						Position:   []byte(`{"x":300,"y":100}`),
						Data:       null.JSONFrom([]byte(`{"label":"End"}`)),
					},
				}

				// Add edges connecting the nodes
				workflow.R.WorkflowEdges = models.WorkflowEdgeSlice{
					&models.WorkflowEdge{
						ID:         "edge-1",
						WorkflowID: "550e8400-e29b-41d4-a716-446655440000",
						EdgeID:     "edge-1",
						Source:     "start",
						Target:     "node-form",
					},
					&models.WorkflowEdge{
						ID:         "edge-2",
						WorkflowID: "550e8400-e29b-41d4-a716-446655440000",
						EdgeID:     "edge-2",
						Source:     "node-form",
						Target:     "node-end",
					},
				}

				mockDB.EXPECT().
					GetWorkflowByID(gomock.Any(), "550e8400-e29b-41d4-a716-446655440000").
					Return(workflow, nil)

				// Mock cache set after retrieving from DB
				mockCache.EXPECT().
					Set(gomock.Any(), cacheKey, gomock.Any(), gomock.Any()).
					Return(nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var response api.WorkflowExecutionResult
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)
				assert.Equal(t, api.WorkflowExecutionResultStatusCompleted, response.Status)
				assert.NotEmpty(t, response.Steps)
			},
		},

		"invalid_request_body": {
			workflowID:  "550e8400-e29b-41d4-a716-446655440000",
			requestBody: "invalid json",
			setupMock: func(mockDB *dbmocks.MockWorkFlowDB, mockCache *cachemocks.MockCache) {
				// No DB call expected for invalid request body
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body []byte) {
				var response api.Error
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)
				assert.Equal(t, "Invalid request body", response.Error)
			},
		},

		"workflow_not_found_during_execution": {
			workflowID: "non-existent-id",
			requestBody: api.WorkflowExecutionInput{
				FormData: &map[string]interface{}{
					"name": "John Doe",
				},
			},
			setupMock: func(mockDB *dbmocks.MockWorkFlowDB, mockCache *cachemocks.MockCache) {
				// Mock cache miss so it goes to database
				cacheKey := "workflow:non-existent-id"
				mockCache.EXPECT().
					Get(gomock.Any(), cacheKey, gomock.Any()).
					Return(cache.ErrCacheMiss{Key: cacheKey})

				mockDB.EXPECT().
					GetWorkflowByID(gomock.Any(), "non-existent-id").
					Return(nil, fmt.Errorf("workflow not found: non-existent-id"))
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body []byte) {
				var response api.Error
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)
				assert.Equal(t, "Workflow not found", response.Error)
			},
		},

		"execution_error": {
			workflowID: "550e8400-e29b-41d4-a716-446655440000",
			requestBody: api.WorkflowExecutionInput{
				FormData: &map[string]interface{}{
					"name": "John Doe",
				},
			},
			setupMock: func(mockDB *dbmocks.MockWorkFlowDB, mockCache *cachemocks.MockCache) {
				// Mock cache miss so it goes to database
				cacheKey := "workflow:550e8400-e29b-41d4-a716-446655440000"
				mockCache.EXPECT().
					Get(gomock.Any(), cacheKey, gomock.Any()).
					Return(cache.ErrCacheMiss{Key: cacheKey})

				mockDB.EXPECT().
					GetWorkflowByID(gomock.Any(), "550e8400-e29b-41d4-a716-446655440000").
					Return(nil, errors.New("execution failed"))
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, body []byte) {
				var response api.Error
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)
				assert.Equal(t, "Failed to execute workflow", response.Error)
			},
		},
	}

	// Run test cases
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create mock controller
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Create mocks
			mockDB := dbmocks.NewMockWorkFlowDB(ctrl)
			mockCache := cachemocks.NewMockCache(ctrl)

			// Setup expectations
			tc.setupMock(mockDB, mockCache)

			// Create service with mock
			service := &Service{
				db:    mockDB,
				cache: mockCache,
			}

			// Prepare request body
			var reqBody []byte
			var err error
			if str, ok := tc.requestBody.(string); ok {
				reqBody = []byte(str)
			} else {
				reqBody, err = json.Marshal(tc.requestBody)
				require.NoError(t, err)
			}

			// Create test request
			req, err := http.NewRequest("POST", fmt.Sprintf("/workflows/%s/execute", tc.workflowID), bytes.NewBuffer(reqBody))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			// Add route variables
			req = mux.SetURLVars(req, map[string]string{"id": tc.workflowID})

			// Create response recorder
			rr := httptest.NewRecorder()

			// Call the handler
			service.HandleExecuteWorkflow(rr, req)

			// Check status code
			assert.Equal(t, tc.expectedStatus, rr.Code)

			// Check response body
			if tc.checkResponse != nil {
				tc.checkResponse(t, rr.Body.Bytes())
			}

			// Check content-type header
			contentType := rr.Header().Get("Content-Type")
			assert.Equal(t, "application/json", contentType)
		})
	}
}
