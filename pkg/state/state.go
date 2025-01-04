package state

type Mergeable[T any] interface {
	Merge(T) T
}

// State represents the base interface for any state type.
type State interface {
	// Validate validates the state
	Validate() error
}

// GraphState Combine both interfaces for graph states.
type GraphState[T any] interface {
	State
	Mergeable[T]
}
