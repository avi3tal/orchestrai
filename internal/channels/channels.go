package channels

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/avi3tal/orchestrai/internal/state"
	"github.com/avi3tal/orchestrai/internal/types"
)

// BaseChannel provides common channel functionality
type BaseChannel struct {
	mu   sync.RWMutex
	data any
}

// LastValue is a channel that only keeps the most recent state
type LastValue struct {
	BaseChannel
}

func NewLastValue() *LastValue {
	return &LastValue{}
}

func (l *LastValue) Read(_ context.Context, _ types.Config) (any, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.data, nil
}

func (l *LastValue) Write(_ context.Context, value state.GraphState, _ types.Config) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.data = value
	return nil
}

// BarrierChannel waits for all required inputs before allowing reads
type BarrierChannel struct {
	BaseChannel
	inputs map[string]any // Track inputs from each source
}

func NewBarrierChannel(required []string) *BarrierChannel {
	inputs := make(map[string]any, len(required))
	for _, r := range required {
		inputs[r] = nil
	}
	return &BarrierChannel{
		inputs: inputs,
	}
}

func (b *BarrierChannel) Read(_ context.Context, _ types.Config) (any, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Check if all inputs are received
	for source, input := range b.inputs {
		if input == nil {
			return nil, fmt.Errorf("waiting for input from: %s", source)
		}
	}
	return b.data, nil
}

func (b *BarrierChannel) Write(_ context.Context, value state.GraphState, config types.Config) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	nodeID := config.ThreadID // Use ThreadID which we already have
	if _, exists := b.inputs[nodeID]; !exists {
		return fmt.Errorf("unexpected input from: %s", nodeID)
	}

	// Store input and update state
	b.inputs[nodeID] = value.Clone()
	b.data = value

	return nil
}

type DynamicBarrierChannel struct {
	BaseChannel
	required sync.Map // Track required nodes
	received sync.Map // Track received responses
}

func NewDynamicBarrierChannel() *DynamicBarrierChannel {
	return &DynamicBarrierChannel{}
}

func (d *DynamicBarrierChannel) AddRequired(nodeID string) {
	d.required.Store(nodeID, struct{}{})
}

func (d *DynamicBarrierChannel) Write(_ context.Context, st state.GraphState, config types.Config) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Verify node is required
	if _, ok := d.required.Load(config.ThreadID); !ok {
		return fmt.Errorf("unexpected input from: %s", config.ThreadID)
	}

	d.received.Store(config.ThreadID, struct{}{})
	d.data = st
	return nil
}

func (d *DynamicBarrierChannel) Read(_ context.Context, _ types.Config) (any, error) {
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
		return nil, errors.New("waiting for required inputs")
	}

	return d.data, nil
}
