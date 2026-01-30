-- Workflow API Database Schema (Simple Version)
-- Version: 1.0.0
-- Description: Core schema for workflow management system

-- Table: workflows
-- Stores the main workflow definitions
CREATE TABLE IF NOT EXISTS workflows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Table: workflow_nodes
-- Stores individual nodes in a workflow
CREATE TABLE IF NOT EXISTS workflow_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    node_id VARCHAR(100) NOT NULL, -- The internal node identifier (e.g., 'start', 'form', 'weather-api')
    type VARCHAR(50) NOT NULL, -- Node type: 'start', 'end', 'form', 'integration', 'condition', 'email'
    position JSONB NOT NULL, -- {"x": 100, "y": 200}
    data JSONB DEFAULT '{}', -- Contains label, description, metadata and other node-specific data
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(workflow_id, node_id)
);

-- Table: workflow_edges
-- Stores connections between nodes
CREATE TABLE IF NOT EXISTS workflow_edges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    edge_id VARCHAR(100) NOT NULL, -- The internal edge identifier (e.g., 'e1', 'e2')
    source VARCHAR(100) NOT NULL, -- Source node_id
    target VARCHAR(100) NOT NULL, -- Target node_id
    source_handle VARCHAR(50), -- For conditional nodes with multiple outputs (e.g., 'true', 'false')
    type VARCHAR(50) DEFAULT 'smoothstep', -- Edge type
    animated BOOLEAN DEFAULT false,
    style JSONB DEFAULT '{}', -- Styling information (stroke, strokeWidth, etc.)
    label VARCHAR(255), -- Edge label
    label_style JSONB DEFAULT '{}', -- Label styling
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(workflow_id, edge_id)
);

-- Create indexes for better query performance
CREATE INDEX idx_workflows_created_at ON workflows(created_at DESC);
CREATE INDEX idx_workflow_nodes_workflow_id ON workflow_nodes(workflow_id);
CREATE INDEX idx_workflow_edges_workflow_id ON workflow_edges(workflow_id);

-- Create update trigger for updated_at columns
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_workflows_updated_at BEFORE UPDATE ON workflows
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_workflow_nodes_updated_at BEFORE UPDATE ON workflow_nodes
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_workflow_edges_updated_at BEFORE UPDATE ON workflow_edges
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();