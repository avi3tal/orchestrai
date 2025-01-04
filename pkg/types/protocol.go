package types

import "github.com/avi3tal/orchestrai/pkg/state"

// NodeResponse encapsulates the execution result
type NodeResponse[T state.GraphState[T]] struct {
	State  T
	Status NodeExecutionStatus
}
