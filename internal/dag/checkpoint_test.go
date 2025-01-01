package dag

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestState struct {
	Value int
}

func (s TestState) Validate() error {
	if s.Value < 0 {
		return fmt.Errorf("value cannot be negative")
	}
	return nil
}

func (s TestState) Merge(other TestState) TestState {
	return TestState{Value: s.Value + other.Value}
}

// Test state validation
func TestStateValidation(t *testing.T) {
	state := TestState{Value: 5}
	assert.NoError(t, state.Validate())

	invalidState := TestState{Value: -1}
	assert.Error(t, invalidState.Validate())
}

// Test state merging
func TestStateMerge(t *testing.T) {
	state1 := TestState{Value: 5}
	state2 := TestState{Value: 3}
	mergedState := state1.Merge(state2)
	assert.Equal(t, 8, mergedState.Value)
}

func TestMemoryCheckpointer(t *testing.T) {
	checkpointer := NewStateCheckpointer[TestState](NewMemoryStore[TestState]())

	ctx := context.Background()
	config := Config[TestState]{ThreadID: "thread-1"}
	data := &CheckpointData[TestState]{
		State:       TestState{Value: 10},
		Status:      StatusCompleted,
		CurrentNode: "node1",
	}

	// Save checkpoint
	err := checkpointer.Save(ctx, config, data)
	assert.NoError(t, err)

	// Load checkpoint
	loadedData, err := checkpointer.Load(ctx, config)
	assert.NoError(t, err)
	assert.Equal(t, data, loadedData)
}

func TestMemoryCheckpointerEdgeCases(t *testing.T) {
	checkpointer := NewStateCheckpointer[TestState](NewMemoryStore[TestState]())
	ctx := context.Background()

	// Test loading nonexistent checkpoint
	config1 := Config[TestState]{ThreadID: "thread-1"}
	_, err := checkpointer.Load(ctx, config1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "thread-1")
	assert.Contains(t, err.Error(), "checkpoint not found")

	// Test overwriting checkpoints
	data1 := &CheckpointData[TestState]{
		State:       TestState{Value: 10},
		Status:      StatusCompleted,
		CurrentNode: "node1",
	}
	err = checkpointer.Save(ctx, config1, data1)
	assert.NoError(t, err)

	data2 := &CheckpointData[TestState]{
		State:       TestState{Value: 20},
		Status:      StatusPending,
		CurrentNode: "node2",
	}
	err = checkpointer.Save(ctx, config1, data2)
	assert.NoError(t, err)

	loadedData, err := checkpointer.Load(ctx, config1)
	assert.NoError(t, err)
	assert.Equal(t, data2, loadedData)

	// Test thread safety with concurrent access
	config2 := Config[TestState]{ThreadID: "thread-2"}
	data3 := &CheckpointData[TestState]{
		State:       TestState{Value: 30},
		Status:      StatusCompleted,
		CurrentNode: "node3",
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		err := checkpointer.Save(ctx, config2, data3)
		assert.NoError(t, err)
	}()

	go func() {
		defer wg.Done()
		_, err := checkpointer.Load(ctx, config2)
		assert.Error(t, err)
	}()

	wg.Wait()
}

func TestStateCheckpointer(t *testing.T) {
	checkpointer := NewStateCheckpointer[TestState](NewMemoryStore[TestState]())
	ctx := context.Background()

	// Test saving and loading checkpoints
	config := Config[TestState]{ThreadID: "thread-1"}
	data := &CheckpointData[TestState]{
		State:       TestState{Value: 100},
		Status:      StatusPending,
		CurrentNode: "node1",
	}

	// Save checkpoint
	err := checkpointer.Save(ctx, config, data)
	assert.NoError(t, err)

	// Load checkpoint
	loadedData, err := checkpointer.Load(ctx, config)
	assert.NoError(t, err)
	assert.Equal(t, data, loadedData)

	// Test loading nonexistent checkpoint
	nonexistentConfig := Config[TestState]{ThreadID: "thread-2"}
	_, err = checkpointer.Load(ctx, nonexistentConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "thread-2")
	assert.Contains(t, err.Error(), "checkpoint not found")
}
