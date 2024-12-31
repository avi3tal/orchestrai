package dag

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemoryCheckpointer(t *testing.T) {
	checkpointer := NewMemoryCheckpointer[TestState]()

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
	checkpointer := NewMemoryCheckpointer[TestState]()
	ctx := context.Background()

	// Test loading nonexistent checkpoint
	config1 := Config[TestState]{ThreadID: "thread-1"}
	_, err := checkpointer.Load(ctx, config1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no chckpoint found for thread thread-1")

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

type MockCheckpointStore struct {
	data map[string]*CheckpointData[TestState]
	mu   sync.RWMutex
}

func NewMockCheckpointStore() *MockCheckpointStore {
	return &MockCheckpointStore{
		data: make(map[string]*CheckpointData[TestState]),
	}
}

func (m *MockCheckpointStore) Save(ctx context.Context, threadID string, data *CheckpointData[TestState]) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[threadID] = data
	return nil
}

func (m *MockCheckpointStore) Load(ctx context.Context, threadID string) (*CheckpointData[TestState], error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, exists := m.data[threadID]
	if !exists {
		return nil, fmt.Errorf("no checkpoint found for thread %s", threadID)
	}
	return data, nil
}

func TestStateCheckpointer(t *testing.T) {
	mockStore := NewMockCheckpointStore()
	checkpointer := NewStateCheckpointer[TestState](mockStore)
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
	assert.Contains(t, err.Error(), "no checkpoint found for thread thread-2")
}
