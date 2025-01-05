package workflow

import (
	"context"

	"github.com/avi3tal/orchestrai/pkg/state"
	"github.com/avi3tal/orchestrai/pkg/types"
)

// Agent represents a node in the user-facing workflow DSL.
type Agent[T state.GraphState[T]] interface {
	Name() string
	Execute(ctx context.Context, s T, cfg types.Config[T]) (types.NodeResponse[T], error)
	Metadata() map[string]any
}
