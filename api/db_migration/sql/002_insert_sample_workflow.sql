-- Insert sample workflow data
-- This matches the hardcoded workflow in the API

-- Insert the main workflow
INSERT INTO workflows (id, name, description) VALUES 
    ('550e8400-e29b-41d4-a716-446655440000', 'Weather Alert Workflow', 'Check weather conditions and send alerts when temperature exceeds threshold');

-- Insert workflow nodes
INSERT INTO workflow_nodes (workflow_id, node_id, type, position, data) VALUES
    -- Start node
    ('550e8400-e29b-41d4-a716-446655440000', 'start', 'start', 
     '{"x": -160, "y": 300}',
     '{
        "label": "Start",
        "description": "Begin weather check workflow",
        "metadata": {
            "hasHandles": {
                "source": true,
                "target": false
            }
        }
     }'),
    
    -- Form node
    ('550e8400-e29b-41d4-a716-446655440000', 'form', 'form',
     '{"x": 152, "y": 304}',
     '{
        "label": "User Input",
        "description": "Process collected data - name, email, location",
        "metadata": {
            "hasHandles": {
                "source": true,
                "target": true
            },
            "inputFields": ["name", "email", "city"],
            "outputVariables": ["name", "email", "city"]
        }
     }'),
    
    -- Weather API node
    ('550e8400-e29b-41d4-a716-446655440000', 'weather-api', 'integration',
     '{"x": 460, "y": 304}',
     '{
        "label": "Weather API",
        "description": "Fetch current temperature for {{city}}",
        "metadata": {
            "hasHandles": {
                "source": true,
                "target": true
            },
            "inputVariables": ["city"],
            "apiEndpoint": "https://api.open-meteo.com/v1/forecast?latitude={lat}&longitude={lon}&current_weather=true",
            "options": [
                {
                    "city": "Sydney",
                    "lat": -33.8688,
                    "lon": 151.2093
                },
                {
                    "city": "Melbourne",
                    "lat": -37.8136,
                    "lon": 144.9631
                },
                {
                    "city": "Brisbane",
                    "lat": -27.4698,
                    "lon": 153.0251
                },
                {
                    "city": "Perth",
                    "lat": -31.9505,
                    "lon": 115.8605
                },
                {
                    "city": "Adelaide",
                    "lat": -34.9285,
                    "lon": 138.6007
                }
            ],
            "outputVariables": ["temperature"]
        }
     }'),
    
    -- Condition node
    ('550e8400-e29b-41d4-a716-446655440000', 'condition', 'condition',
     '{"x": 794, "y": 304}',
     '{
        "label": "Check Condition",
        "description": "Evaluate temperature threshold",
        "metadata": {
            "hasHandles": {
                "source": ["true", "false"],
                "target": true
            },
            "conditionExpression": "temperature {{operator}} {{threshold}}",
            "outputVariables": ["conditionMet"]
        }
     }'),
    
    -- Email node
    ('550e8400-e29b-41d4-a716-446655440000', 'email', 'email',
     '{"x": 1096, "y": 88}',
     '{
        "label": "Send Alert",
        "description": "Email weather alert notification",
        "metadata": {
            "hasHandles": {
                "source": true,
                "target": true
            },
            "inputVariables": ["name", "city", "temperature"],
            "emailTemplate": {
                "subject": "Weather Alert",
                "body": "Weather alert for {{city}}! Temperature is {{temperature}}°C!"
            },
            "outputVariables": ["emailSent"]
        }
     }'),
    
    -- End node
    ('550e8400-e29b-41d4-a716-446655440000', 'end', 'end',
     '{"x": 1360, "y": 302}',
     '{
        "label": "Complete",
        "description": "Workflow execution finished",
        "metadata": {
            "hasHandles": {
                "source": false,
                "target": true
            }
        }
     }');

-- Insert workflow edges
INSERT INTO workflow_edges (workflow_id, edge_id, source, target, source_handle, type, animated, style, label, label_style) VALUES
    -- Edge from start to form
    ('550e8400-e29b-41d4-a716-446655440000', 'e1', 'start', 'form', NULL, 'smoothstep', true,
     '{"stroke": "#10b981", "strokeWidth": 3}',
     'Initialize', '{}'),
    
    -- Edge from form to weather-api
    ('550e8400-e29b-41d4-a716-446655440000', 'e2', 'form', 'weather-api', NULL, 'smoothstep', true,
     '{"stroke": "#3b82f6", "strokeWidth": 3}',
     'Submit Data', '{}'),
    
    -- Edge from weather-api to condition
    ('550e8400-e29b-41d4-a716-446655440000', 'e3', 'weather-api', 'condition', NULL, 'smoothstep', true,
     '{"stroke": "#f97316", "strokeWidth": 3}',
     'Temperature Data', '{}'),
    
    -- Edge from condition to email (true branch)
    ('550e8400-e29b-41d4-a716-446655440000', 'e4', 'condition', 'email', 'true', 'smoothstep', true,
     '{"stroke": "#10b981", "strokeWidth": 3}',
     '✓ Condition Met',
     '{"fill": "#10b981", "fontWeight": "bold"}'),
    
    -- Edge from condition to end (false branch)
    ('550e8400-e29b-41d4-a716-446655440000', 'e5', 'condition', 'end', 'false', 'smoothstep', true,
     '{"stroke": "#6b7280", "strokeWidth": 3}',
     '✗ No Alert Needed',
     '{"fill": "#6b7280", "fontWeight": "bold"}'),
    
    -- Edge from email to end
    ('550e8400-e29b-41d4-a716-446655440000', 'e6', 'email', 'end', NULL, 'smoothstep', true,
     '{"stroke": "#ef4444", "strokeWidth": 2}',
     'Alert Sent',
     '{"fill": "#ef4444", "fontWeight": "bold"}');