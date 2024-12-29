package state

import (
	"fmt"
	"time"

	"github.com/avi3tal/orchestrai/internal/types"
)

// BaseState provides a base implementation of the State interface
type BaseState struct {
	// ID is the unique identifier for this state
	ID string `json:"id"`

	// Type is the state type
	Type string `json:"type"`

	// Data holds the state data
	Data map[string]interface{} `json:"data"`

	// Schema defines the expected state structure
	Schema *types.StateSchema `json:"schema,omitempty"`

	// Metadata holds state metadata
	Metadata types.StateMetadata `json:"metadata"`
}

// Ensure BaseState implements State interface
var _ types.State = (*BaseState)(nil)

// GetID returns the state's unique identifier
func (b *BaseState) GetID() string {
	return b.ID
}

// GetType returns the state type
func (b *BaseState) GetType() string {
	return b.Type
}

// Get retrieves a value by key
func (b *BaseState) Get(key string) (interface{}, bool) {
	val, exists := b.Data[key]
	return val, exists
}

// Set creates a new state with the updated key-value pair
func (b *BaseState) Set(key string, value interface{}) (types.State, error) {
	newState := b.Clone().(*BaseState)
	newState.Data[key] = value
	newState.Metadata.UpdatedAt = time.Now().UTC()

	if err := newState.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed after set: %w", err)
	}

	return newState, nil
}

// Delete creates a new state with the key removed
func (b *BaseState) Delete(key string) types.State {
	newState := b.Clone().(*BaseState)
	delete(newState.Data, key)
	newState.Metadata.UpdatedAt = time.Now().UTC()
	return newState
}

// Merge creates a new state by merging with another state
func (b *BaseState) Merge(other types.State) (types.State, error) {
	newState := b.Clone().(*BaseState)

	otherMap := other.ToMap()
	for k, v := range otherMap {
		newState.Data[k] = v
	}

	newState.Metadata.UpdatedAt = time.Now().UTC()

	if err := newState.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed after merge: %w", err)
	}

	return newState, nil
}

// Clone creates a deep copy of the state
func (b *BaseState) Clone() types.State {
	newState := &BaseState{
		ID:   b.ID,
		Type: b.Type,
		Data: make(map[string]interface{}),
		Metadata: types.StateMetadata{
			CreatedAt: b.Metadata.CreatedAt,
			UpdatedAt: time.Now().UTC(),
			Version:   b.Metadata.Version + 1,
			Labels:    make(map[string]string),
		},
	}

	// Deep copy the data
	for k, v := range b.Data {
		newState.Data[k] = deepCopy(v)
	}

	// Deep copy the schema if exists
	if b.Schema != nil {
		newState.Schema = &types.StateSchema{
			Type:                 b.Schema.Type,
			Properties:           make(map[string]types.PropertySchema),
			Required:             make([]string, len(b.Schema.Required)),
			AdditionalProperties: b.Schema.AdditionalProperties,
		}
		copy(newState.Schema.Required, b.Schema.Required)
		for k, v := range b.Schema.Properties {
			newState.Schema.Properties[k] = v
		}
	}

	// Deep copy labels
	for k, v := range b.Metadata.Labels {
		newState.Metadata.Labels[k] = v
	}

	return newState
}

// Validate ensures the state is valid
func (b *BaseState) Validate() error {
	if b.ID == "" {
		return fmt.Errorf("state ID is required")
	}

	if b.Type == "" {
		return fmt.Errorf("state type is required")
	}

	if b.Schema != nil {
		if err := b.validateAgainstSchema(); err != nil {
			return fmt.Errorf("schema validation failed: %w", err)
		}
	}

	return nil
}

// ToMap converts the state to a map representation
func (b *BaseState) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"id":       b.ID,
		"type":     b.Type,
		"data":     b.Data,
		"metadata": b.Metadata,
	}
}

// validateAgainstSchema validates the state data against its schema
func (b *BaseState) validateAgainstSchema() error {
	for _, required := range b.Schema.Required {
		if _, exists := b.Data[required]; !exists {
			return fmt.Errorf("missing required field: %s", required)
		}
	}

	for key, value := range b.Data {
		if propSchema, exists := b.Schema.Properties[key]; exists {
			if err := validateProperty(value, propSchema); err != nil {
				return fmt.Errorf("invalid value for %s: %w", key, err)
			}
		} else if !b.Schema.AdditionalProperties {
			return fmt.Errorf("additional property not allowed: %s", key)
		}
	}

	return nil
}
