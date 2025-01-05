package workflow

import (
	"fmt"
	"github.com/avi3tal/orchestrai/pkg/state"
)

// ensureAgent ensures that the agent is added to the graph
func ensureAgent[T state.GraphState[T]](wf *Builder[T], agent Agent[T]) error {
	err := wf.graph.AddNode(agent.Name(), agent.Execute, agent.Metadata())
	if err != nil && !isDuplicateNodeError(err) {
		return fmt.Errorf("cannot ensure agent %q: %w", agent.Name(), err)
	}
	return nil
}
