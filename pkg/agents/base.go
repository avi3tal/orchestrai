package agents

import (
	"context"

	"github.com/avi3tal/orchestrai/pkg/state"
	"github.com/avi3tal/orchestrai/pkg/types"
)

// BaseAgent is a straightforward in-process function agent.
type BaseAgent[T state.GraphState[T]] struct {
	name     string
	fn       func(context.Context, T, types.Config[T]) (types.NodeResponse[T], error)
	metadata map[string]any
}

// NewSimpleAgent helper to create an inline agent
func NewSimpleAgent[T state.GraphState[T]](
	name string,
	fn func(context.Context, T, types.Config[T]) (types.NodeResponse[T], error),
	meta map[string]any,
) *BaseAgent[T] {
	return &BaseAgent[T]{name: name, fn: fn, metadata: meta}
}

func (a *BaseAgent[T]) Name() string {
	return a.name
}

func (a *BaseAgent[T]) Execute(ctx context.Context, s T, cfg types.Config[T]) (types.NodeResponse[T], error) {
	return a.fn(ctx, s, cfg)
}

func (a *BaseAgent[T]) Metadata() map[string]any {
	return a.metadata
}
