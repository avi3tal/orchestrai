package agents

import (
	"context"

	"github.com/avi3tal/orchestrai/pkg/state"
	"github.com/avi3tal/orchestrai/pkg/types"
)

// RemoteAgent can call an external microservice, etc.
type RemoteAgent[T state.GraphState[T]] struct {
	name     string
	endpoint string
	metadata map[string]any
}

func NewRemoteAgent[T state.GraphState[T]](name, endpoint string, meta map[string]any) *RemoteAgent[T] {
	return &RemoteAgent[T]{name: name, endpoint: endpoint, metadata: meta}
}

func (ra *RemoteAgent[T]) Name() string {
	return ra.name
}

func (ra *RemoteAgent[T]) Execute(ctx context.Context, s T, cfg types.Config[T]) (types.NodeResponse[T], error) {
	// Pseudo: do HTTP/gRPC call with s + cfg, parse result
	// e.g. s2 := callRemote(ra.endpoint, s)
	// for now, pretend no transformation
	resp := types.NodeResponse[T]{
		State:  s, // unchanged
		Status: types.StatusCompleted,
	}
	return resp, nil
}

func (ra *RemoteAgent[T]) Metadata() map[string]any {
	return ra.metadata
}
