package workflow

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/avi3tal/orchestrai/internal/graph"
	"github.com/avi3tal/orchestrai/pkg/state"
	"github.com/avi3tal/orchestrai/pkg/types"
)

// Listener is polled or awaited for new inputs to run in the workflow.
// For example, it might be reading from a queue, an HTTP endpoint, etc.
type Listener[T any] interface {
	// WaitForEvent blocks until a new event is available or context is done.
	// Returns the input state for the workflow.
	WaitForEvent(ctx context.Context) (T, error)
}

// Callback is invoked after execution (success or error).
type Callback[T any] interface {
	OnComplete(ctx context.Context, output T) error
	OnError(ctx context.Context, err error) error
}

// App represents a compiled workflow plus optional config like a checkpoint store.
type App[T state.GraphState[T]] struct {
	workflow *Builder[T]
	compiled *graph.CompiledGraph[T]
	listener Listener[T]
	callback Callback[T]

	// Additional config
	store types.CheckpointStore[T]
	debug bool
}

// AppOption is a functional option that configures the App before finalizing.
type AppOption[T state.GraphState[T]] func(*App[T])

func WithListener[T state.GraphState[T]](l Listener[T]) AppOption[T] {
	return func(a *App[T]) {
		a.listener = l
	}
}

func WithCallback[T state.GraphState[T]](cb Callback[T]) AppOption[T] {
	return func(a *App[T]) {
		a.callback = cb
	}
}

func WithCheckpointStore[T state.GraphState[T]](store types.CheckpointStore[T]) AppOption[T] {
	return func(a *App[T]) {
		a.store = store
	}
}

func WithDebug[T state.GraphState[T]]() AppOption[T] {
	return func(a *App[T]) {
		a.debug = true
	}
}

// NewApp compiles the Builder and sets up optional Listener, Callback, etc.
func NewApp[T state.GraphState[T]](wf *Builder[T], opts ...AppOption[T]) (*App[T], error) {
	app := &App[T]{workflow: wf}

	// Apply user-provided options
	for _, opt := range opts {
		opt(app)
	}

	// Prepare compilation options for the internal engine
	var compileOpts []graph.CompilationOption[T]
	if app.store != nil {
		compileOpts = append(compileOpts, graph.WithCheckpointStore(app.store))
	}
	if app.debug {
		compileOpts = append(compileOpts, graph.WithDebug[T]())
	}

	// Compile the workflow
	cg, err := wf.Compile(compileOpts...)
	if err != nil {
		return nil, fmt.Errorf("NewApp: failed to compile workflow: %w", err)
	}
	app.compiled = cg

	return app, nil
}

// Invoke runs the compiled workflow *once* with a given input.
// If the App has a callback set, OnComplete/OnError is called here.
func (app *App[T]) Invoke(
	ctx context.Context,
	input T,
	execOpts ...graph.ExecutionOption[T],
) (T, error) {
	// Actually run the flow
	out, err := app.compiled.Run(ctx, input, execOpts...)
	if err != nil {
		if app.callback != nil {
			_ = app.callback.OnError(ctx, err)
		}
		return out, errors.Wrap(err, "invoke: workflow failed")
	}
	// Success => OnComplete
	if app.callback != nil {
		if cbErr := app.callback.OnComplete(ctx, out); cbErr != nil {
			// Decide if you want to treat callback error as critical
			// or just log it
			return out, fmt.Errorf("invoke: callback OnComplete failed: %w", cbErr)
		}
	}
	return out, nil
}

// Start runs in a loop, continuously invoking the workflow for each incoming event from the Listener.
// It blocks until context is cancelled or an unrecoverable error occurs (depending on your design).
func (app *App[T]) Start(ctx context.Context, execOpts ...graph.ExecutionOption[T]) error {
	if app.listener == nil {
		return errors.New("start called, but no Listener is configured")
	}

	// Keep listening for new events
	for {
		select {
		case <-ctx.Done():
			// Time to shut down gracefully
			return errors.Wrap(ctx.Err(), "context is done!")

		default:
			// Wait for an event from the listener
			input, err := app.listener.WaitForEvent(ctx)
			if err != nil {
				// If the listener fails, decide how to handle
				if app.callback != nil {
					_ = app.callback.OnError(ctx, err)
				}
				// Possibly continue, or return error if it's fatal
				continue
			}

			// We got an input => run the workflow
			_, runErr := app.Invoke(ctx, input, execOpts...)
			if runErr != nil {
				// OnError has been called in app.Invoke
				// Possibly decide to continue or abort
				continue
			}
			// OnComplete was triggered within .Invoke on success
			// Loop for the next event
		}
	}
}
