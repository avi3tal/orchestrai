package dag

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test LastValueChannel
func TestLastValueChannel(t *testing.T) {
	channel := NewLastValue[TestState]()

	ctx := context.Background()
	state := TestState{Value: 5}
	config := Config[TestState]{}

	// Write state
	err := channel.Write(ctx, state, config)
	assert.NoError(t, err)

	// Read state
	readState, err := channel.Read(ctx, config)
	assert.NoError(t, err)
	assert.Equal(t, state, readState)
}

// Test BarrierChannel
func TestBarrierChannel(t *testing.T) {
	channel := NewBarrierChannel[TestState]([]string{"node1", "node2"})

	ctx := context.Background()
	state := TestState{Value: 5}
	config := Config[TestState]{ThreadID: "node1"}

	// Write state from one node
	err := channel.Write(ctx, state, config)
	assert.NoError(t, err)

	// Attempt to read before all inputs are received
	_, err = channel.Read(ctx, config)
	assert.Error(t, err)
}
