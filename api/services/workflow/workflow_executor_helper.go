package workflow

import (
	"encoding/json"
	"log/slog"
)

// findValueInMap recursively searches for a key in a map up to maxDepth levels
// It collects all matching values and returns the first numeric one if available
func findValueInMap(data map[string]any, key string, currentDepth int, maxDepth int) any {
	var candidates []any
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
func findValueInMapHelper(data map[string]any, key string, currentDepth int, maxDepth int, candidates *[]any) {
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
		case map[string]any:
			findValueInMapHelper(nested, key, currentDepth+1, maxDepth, candidates)
		}
	}
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
