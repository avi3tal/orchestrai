// channels.go
package dag

import (
	"context"
	"fmt"
	"sync"
)

// ChannelType represents different types of state management channels
type ChannelType string

const (
	LastValueChannelType ChannelType = "last_value"
	BarrierChannelType   ChannelType = "barrier"
)

// BaseChannel provides common channel functionality
type BaseChannel[T State] struct {
	mu    sync.RWMutex
	state T
}

// LastValue is a channel that only keeps the most recent state
type LastValue[T State] struct {
	BaseChannel[T]
}

func NewLastValue[T State]() *LastValue[T] {
	return &LastValue[T]{}
}

func (l *LastValue[T]) Read(ctx context.Context, config Config[T]) (T, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.state, nil
}

func (l *LastValue[T]) Write(ctx context.Context, value T, config Config[T]) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.state = value
	return nil
}

// BarrierChannel waits for all required inputs before allowing reads
type BarrierChannel[T State] struct {
	BaseChannel[T]
	inputs map[string]*T // Track inputs from each source
}

func NewBarrierChannel[T State](required []string) *BarrierChannel[T] {
	inputs := make(map[string]*T, len(required))
	for _, r := range required {
		inputs[r] = nil
	}
	return &BarrierChannel[T]{
		inputs: inputs,
	}
}

func (b *BarrierChannel[T]) Read(ctx context.Context, config Config[T]) (T, error) {
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

func (b *BarrierChannel[T]) Write(ctx context.Context, value T, config Config[T]) error {
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
