package state

type Mergeable interface {
	Merge(State) State
}

// State represents the base interface for any state type.
type State interface {
	// Validate validates the state
	Validate() error
}

// GraphState Combine both interfaces for graph states.
type GraphState interface {
	Validate() error
	Merge(GraphState) GraphState
	Clone() GraphState
	Dump() ([]byte, error)
	Load([]byte) (GraphState, error)
}
