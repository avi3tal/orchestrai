package graph

import "github.com/avi3tal/orchestrai/internal/types"

// Option configures a Graph
type Option func(*Graph)

// WithCheckpointer sets the checkpointer for the graph
func WithCheckpointer(checkpointer types.Checkpointer) Option {
	return func(g *Graph) {
		g.checkpointer = checkpointer
	}
}

// WithStore sets the store for the graph
func WithStore(store types.Store) Option {
	return func(g *Graph) {
		g.store = store
	}
}

// WithVersion sets the version for the graph
func WithVersion(version string) Option {
	return func(g *Graph) {
		g.version = version
	}
}

// WithDebug enables or disables debug mode
func WithDebug(debug bool) Option {
	return func(g *Graph) {
		g.debug = debug
	}
}

// WithID sets a custom ID for the graph
func WithID(id string) Option {
	return func(g *Graph) {
		g.id = id
	}
}

// WithMetadata sets initial metadata for the graph
func WithMetadata(metadata map[string]interface{}) Option {
	return func(g *Graph) {
		for k, v := range metadata {
			g.metadata[k] = v
		}
	}
}

// WithMaxConcurrency sets the maximum number of concurrent node executions
func WithMaxConcurrency(max int) Option {
	return func(g *Graph) {
		g.maxConcurrency = max
	}
}

// WithErrorHandler sets a custom error handler for the graph
func WithErrorHandler(handler func(error) error) Option {
	return func(g *Graph) {
		g.errorHandler = handler
	}
}
