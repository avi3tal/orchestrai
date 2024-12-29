package state

import (
	"fmt"
	"github.com/avi3tal/orchestrai/internal/types"
)

// deepCopy performs a deep copy of an interface{}
func deepCopy(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case map[string]interface{}:
		newMap := make(map[string]interface{})
		for k, v := range val {
			newMap[k] = deepCopy(v)
		}
		return newMap
	case []interface{}:
		newSlice := make([]interface{}, len(val))
		for i, v := range val {
			newSlice[i] = deepCopy(v)
		}
		return newSlice
	case string, int, int64, float64, bool:
		return val // These are immutable, no need to copy
	default:
		// For other types, we might want to implement specific copy logic
		// or return as is if we know they're immutable
		return val
	}
}

// validateProperty validates a single property against its schema
func validateProperty(value interface{}, schema types.PropertySchema) error {
	switch schema.Type {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string value")
		}
		// Add pattern validation if needed

	case "number":
		num, ok := value.(float64)
		if !ok {
			return fmt.Errorf("expected number value")
		}
		if schema.Minimum != nil && num < *schema.Minimum {
			return fmt.Errorf("value below minimum: %v", *schema.Minimum)
		}
		if schema.Maximum != nil && num > *schema.Maximum {
			return fmt.Errorf("value above maximum: %v", *schema.Maximum)
		}

	case "array":
		arr, ok := value.([]interface{})
		if !ok {
			return fmt.Errorf("expected array value")
		}
		if schema.Items != nil {
			for i, item := range arr {
				if err := validateProperty(item, *schema.Items); err != nil {
					return fmt.Errorf("invalid array item at index %d: %w", i, err)
				}
			}
		}

	case "object":
		obj, ok := value.(map[string]interface{})
		if !ok {
			return fmt.Errorf("expected object value")
		}
		for key, propSchema := range schema.Properties {
			if val, exists := obj[key]; exists {
				if err := validateProperty(val, propSchema); err != nil {
					return fmt.Errorf("invalid property %s: %w", key, err)
				}
			}
		}
	}

	return nil
}
