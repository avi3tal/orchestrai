package channels

import (
	"context"
	"fmt"
	"sync"

	"github.com/avi3tal/orchestrai/internal/dag"
)

// BaseChannel provides common channel functionality
type BaseChannel[T dag.GraphState[T]] struct {
	mu    sync.RWMutex
	state T
}

// LastValue is a channel that only keeps the most recent state
type LastValue[T dag.GraphState[T]] struct {
	BaseChannel[T]
}

func NewLastValue[T dag.GraphState[T]]() *LastValue[T] {
	return &LastValue[T]{}
}

func (l *LastValue[T]) Read(_ context.Context, _ dag.Config[T]) (T, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.state, nil
}

func (l *LastValue[T]) Write(_ context.Context, value T, _ dag.Config[T]) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.state = value
	return nil
}

// BarrierChannel waits for all required inputs before allowing reads
type BarrierChannel[T dag.GraphState[T]] struct {
	BaseChannel[T]
	inputs map[string]*T // Track inputs from each source
}

func NewBarrierChannel[T dag.GraphState[T]](required []string) *BarrierChannel[T] {
	inputs := make(map[string]*T, len(required))
	for _, r := range required {
		inputs[r] = nil
	}
	return &BarrierChannel[T]{
		inputs: inputs,
	}
}

func (b *BarrierChannel[T]) Read(_ context.Context, _ dag.Config[T]) (T, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Check if all inputs are received
	for source, input := range b.inputs {
		if input == nil {
			var zero T
			return zero, fmt.Errorf("waiting for input from: %s", source)
		}
	}
	return b.state, nil
}

func (b *BarrierChannel[T]) Write(_ context.Context, value T, config dag.Config[T]) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	nodeID := config.ThreadID // Use ThreadID which we already have
	if _, exists := b.inputs[nodeID]; !exists {
		return fmt.Errorf("unexpected input from: %s", nodeID)
	}

	// Store input and update state
	valueCopy := value // Make a copy to store
	b.inputs[nodeID] = &valueCopy
	b.state = value

	return nil
}

type DynamicBarrierChannel[T dag.GraphState[T]] struct {
	BaseChannel[T]
	required sync.Map // Track required nodes
	received sync.Map // Track received responses
}

func NewDynamicBarrierChannel[T dag.GraphState[T]]() *DynamicBarrierChannel[T] {
	return &DynamicBarrierChannel[T]{}
}

func (d *DynamicBarrierChannel[T]) AddRequired(nodeID string) {
	d.required.Store(nodeID, struct{}{})
}

func (d *DynamicBarrierChannel[T]) Write(_ context.Context, value T, config dag.Config[T]) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Verify node is required
	if _, ok := d.required.Load(config.ThreadID); !ok {
		return fmt.Errorf("unexpected input from: %s", config.ThreadID)
	}

	d.received.Store(config.ThreadID, struct{}{})
	d.state = value
	return nil
}

func (d *DynamicBarrierChannel[T]) Read(_ context.Context, _ dag.Config[T]) (T, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	allReceived := true
	d.required.Range(func(nodeID, _ interface{}) bool {
		if _, ok := d.received.Load(nodeID); !ok {
			allReceived = false
			return false // Stop iteration
		}
		return true
	})

	if !allReceived {
		var zero T
		return zero, fmt.Errorf("waiting for required inputs")
	}

	return d.state, nil
}
