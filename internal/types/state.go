package types

import (
	"time"
)

// State represents the workflow state
type State interface {
	// GetID returns the state's unique identifier
	GetID() string

	// GetType returns the state type
	GetType() string

	// Get retrieves a value by key
	Get(key string) (interface{}, bool)

	// Set creates a new state with the updated key-value pair
	Set(key string, value interface{}) (State, error)

	// Delete creates a new state with the key removed
	Delete(key string) State

	// Merge creates a new state by merging with another state
	Merge(other State) (State, error)

	// Clone creates a deep copy of the state
	Clone() State

	// Validate ensures the state is valid
	Validate() error

	// ToMap converts the state to a map representation
	ToMap() map[string]interface{}
}

// StateType represents different types of state data
type StateType string

const (
	// StateTypeGeneric is for general purpose state
	StateTypeGeneric StateType = "generic"

	// StateTypeMessage is for message-based state
	StateTypeMessage StateType = "message"

	// StateTypeAgent is for agent-specific state
	StateTypeAgent StateType = "agent"

	// StateTypeTool is for tool-specific state
	StateTypeTool StateType = "tool"
)

// StateMetadata holds metadata about the state
type StateMetadata struct {
	// CreatedAt is when the state was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the state was last updated
	UpdatedAt time.Time `json:"updated_at"`

	// Version is the state version number
	Version int64 `json:"version"`

	// PreviousID is the ID of the previous state
	PreviousID string `json:"previous_id,omitempty"`

	// NodeID is the ID of the node that created this state
	NodeID string `json:"node_id,omitempty"`

	// Labels are optional key-value pairs
	Labels map[string]string `json:"labels,omitempty"`
}

// StateSchema defines the structure and validation rules for state data
type StateSchema struct {
	// Type is the schema type (object, array, etc)
	Type string `json:"type"`

	// Properties defines the schema for each property
	Properties map[string]PropertySchema `json:"properties,omitempty"`

	// Required lists required property names
	Required []string `json:"required,omitempty"`

	// AdditionalProperties indicates if extra properties are allowed
	AdditionalProperties bool `json:"additional_properties"`
}

// PropertySchema defines the schema for a single property
type PropertySchema struct {
	// Type is the property type
	Type string `json:"type"`

	// Description explains the property
	Description string `json:"description,omitempty"`

	// Pattern is a regex pattern for string validation
	Pattern string `json:"pattern,omitempty"`

	// Enum lists allowed values
	Enum []interface{} `json:"enum,omitempty"`

	// Minimum value for numbers
	Minimum *float64 `json:"minimum,omitempty"`

	// Maximum value for numbers
	Maximum *float64 `json:"maximum,omitempty"`

	// Items defines the schema for array items
	Items *PropertySchema `json:"items,omitempty"`

	// Properties defines nested object properties
	Properties map[string]PropertySchema `json:"properties,omitempty"`
}

//
//// StateBuilder helps construct new states
//type StateBuilder struct {
//	state *BaseState
//}
//
//// NewStateBuilder creates a new StateBuilder
//func NewStateBuilder() *StateBuilder {
//	return &StateBuilder{
//		state: &BaseState{
//			Data: make(map[string]interface{}),
//			Metadata: StateMetadata{
//				CreatedAt: time.Now().UTC(),
//				UpdatedAt: time.Now().UTC(),
//				Version:   1,
//				Labels:    make(map[string]string),
//			},
//		},
//	}
//}
//
//// WithID sets the state ID
//func (b *StateBuilder) WithID(id string) *StateBuilder {
//	b.state.ID = id
//	return b
//}
//
//// WithType sets the state type
//func (b *StateBuilder) WithType(typ StateType) *StateBuilder {
//	b.state.Type = typ
//	return b
//}
//
//// WithSchema sets the state schema
//func (b *StateBuilder) WithSchema(schema *StateSchema) *StateBuilder {
//	b.state.Schema = schema
//	return b
//}
//
//// WithData sets the initial state data
//func (b *StateBuilder) WithData(data map[string]interface{}) *StateBuilder {
//	b.state.Data = data
//	return b
//}
//
//// WithLabel adds a label to the state
//func (b *StateBuilder) WithLabel(key, value string) *StateBuilder {
//	b.state.Metadata.Labels[key] = value
//	return b
//}
//
//// Build creates the final state
//func (b *StateBuilder) Build() (State, error) {
//	if err := b.validate(); err != nil {
//		return nil, fmt.Errorf("invalid state: %w", err)
//	}
//	return b.state.Clone(), nil
//}
//
//// validate checks if the state is valid
//func (b *StateBuilder) validate() error {
//	if b.state.ID == "" {
//		return fmt.Errorf("state ID is required")
//	}
//	if b.state.Type == "" {
//		return fmt.Errorf("state type is required")
//	}
//	if b.state.Schema != nil {
//		if err := b.validateSchema(); err != nil {
//			return fmt.Errorf("schema validation failed: %w", err)
//		}
//	}
//	return nil
//}
//
//// validateSchema validates the state data against its schema
//func (b *StateBuilder) validateSchema() error {
//	// Basic schema validation implementation
//	for _, required := range b.state.Schema.Required {
//		if _, exists := b.state.Data[required]; !exists {
//			return fmt.Errorf("missing required field: %s", required)
//		}
//	}
//
//	for key, value := range b.state.Data {
//		if propSchema, exists := b.state.Schema.Properties[key]; exists {
//			if err := validateProperty(value, propSchema); err != nil {
//				return fmt.Errorf("invalid value for %s: %w", key, err)
//			}
//		} else if !b.state.Schema.AdditionalProperties {
//			return fmt.Errorf("additional property not allowed: %s", key)
//		}
//	}
//
//	return nil
//}
