package tests

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/avi3tal/orchestrai/internal/graph"
	"github.com/avi3tal/orchestrai/pkg/checkpoints"
	"github.com/avi3tal/orchestrai/pkg/types"
	"github.com/stretchr/testify/require"
)

// CycleState represents a state for testing cyclic workflows
type CycleState struct {
	Value      string
	Iterations int
	Logs       []string
}

func (s CycleState) Validate() error {
	if s.Value == "" {
		return errors.New("value cannot be empty")
	}
	return nil
}

func (s CycleState) Merge(other CycleState) CycleState {
	value := s.Value
	if other.Value != "" {
		value = other.Value
	}
	iterations := s.Iterations
	if other.Iterations != 0 {
		iterations = other.Iterations
	}
	logs := s.Logs
	if len(other.Logs) > 0 {
		logs = append(logs, other.Logs...)
	}

	merged := CycleState{
		Value:      value,
		Iterations: iterations,
		Logs:       logs,
	}
	return merged
}

// TestGraphCycles tests various scenarios involving cyclic graph execution
func TestGraphCycles(t *testing.T) {
	t.Parallel()

	t.Run("max_steps_with_cycles", func(t *testing.T) {
		t.Parallel()
		g := graph.NewGraph[CycleState]("cycle-test")
		store := checkpoints.NewMemoryStore[CycleState]()

		// Add nodes that form a cycle with state modifications
		require.NoError(t, g.AddNode("initializer", func(_ context.Context, s CycleState, _ types.Config[CycleState]) (types.NodeResponse[CycleState], error) {
			newState := CycleState{
				Value:      s.Value,
				Iterations: s.Iterations,
				Logs:       []string{"initialized"},
			}
			return types.NodeResponse[CycleState]{State: newState, Status: types.StatusCompleted}, nil
		}, nil))

		require.NoError(t, g.AddNode("processor", func(_ context.Context, s CycleState, _ types.Config[CycleState]) (types.NodeResponse[CycleState], error) {
			newState := CycleState{
				Value:      fmt.Sprintf("processed-%d", s.Iterations),
				Iterations: s.Iterations + 1,
				Logs:       []string{fmt.Sprintf("processing iteration %d", s.Iterations+1)},
			}
			return types.NodeResponse[CycleState]{State: newState, Status: types.StatusCompleted}, nil
		}, nil))

		require.NoError(t, g.AddNode("validator", func(_ context.Context, s CycleState, _ types.Config[CycleState]) (types.NodeResponse[CycleState], error) {
			newState := CycleState{
				Logs: []string{fmt.Sprintf("validating iteration %d", s.Iterations)},
			}
			return types.NodeResponse[CycleState]{State: newState, Status: types.StatusCompleted}, nil
		}, nil))

		// Create cycle: initializer -> processor -> validator -> processor
		require.NoError(t, g.AddEdge("initializer", "processor", nil))
		require.NoError(t, g.AddEdge("processor", "validator", nil))
		require.NoError(t, g.AddEdge("validator", "processor", nil))
		require.NoError(t, g.SetEntryPoint("initializer"))
		require.NoError(t, g.SetEndPoint("validator"))

		require.NoError(t, g.AddBranch("validator", func(_ context.Context, s CycleState, _ types.Config[CycleState]) string {
			if s.Iterations >= 3 {
				return graph.END
			}
			return "processor"
		}, "", nil))

		testCases := []struct {
			name          string
			maxSteps      int
			expectedState CycleState
			expectError   bool
		}{
			{
				name:     "single_cycle",
				maxSteps: 3,
				expectedState: CycleState{
					Value:      "processed-0",
					Iterations: 1,
					Logs: []string{
						"initialized",
						"processing iteration 1",
						"validating iteration 1",
					},
				},
				expectError: true,
			},
			{
				name:     "multiple_cycles",
				maxSteps: 7,
				expectedState: CycleState{
					Value:      "processed-2",
					Iterations: 3,
					Logs: []string{
						"initialized",
						"processing iteration 1",
						"validating iteration 1",
						"processing iteration 2",
						"validating iteration 2",
						"processing iteration 3",
						"validating iteration 3",
					},
				},
				expectError: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Compile graph with test case configuration
				compiled, err := g.Compile(
					graph.WithMaxSteps[CycleState](tc.maxSteps),
					graph.WithCheckpointStore[CycleState](store),
					graph.WithDebug[CycleState](),
				)
				require.NoError(t, err)

				// Run with unique thread ID
				result, err := compiled.Run(
					context.Background(),
					CycleState{Value: "initial"},
					graph.WithThreadID[CycleState](tc.name),
				)

				if tc.expectError {
					require.Error(t, err)
					require.Contains(t, err.Error(), "max steps reached")
				} else {
					require.NoError(t, err)
				}

				// Verify state progression
				require.Equal(t, tc.expectedState.Value, result.Value)
				require.Equal(t, tc.expectedState.Iterations, result.Iterations)
				require.Equal(t, tc.expectedState.Logs, result.Logs)

				// Verify checkpoint
				checkpoint, err := store.Load(context.Background(), types.CheckpointKey{
					GraphID:  g.ID(),
					ThreadID: tc.name,
				})
				require.NoError(t, err)
				require.Equal(t, tc.maxSteps, checkpoint.Meta.Steps+1)
			})
		}
	})

	t.Run("cycle_with_conditional_exit", func(t *testing.T) {
		t.Parallel()
		g := graph.NewGraph[CycleState]("conditional-cycle")
		store := checkpoints.NewMemoryStore[CycleState]()

		// Add nodes for conditional cycle
		require.NoError(t, g.AddNode("start", func(_ context.Context, s CycleState, _ types.Config[CycleState]) (types.NodeResponse[CycleState], error) {
			s.Logs = append(s.Logs, "started")
			return types.NodeResponse[CycleState]{State: s, Status: types.StatusCompleted}, nil
		}, nil))

		require.NoError(t, g.AddNode("work", func(_ context.Context, s CycleState, _ types.Config[CycleState]) (types.NodeResponse[CycleState], error) {
			newState := CycleState{
				Logs:       []string{fmt.Sprintf("work iteration %d", s.Iterations+1)},
				Iterations: s.Iterations + 1,
			}
			return types.NodeResponse[CycleState]{
				State:  newState,
				Status: types.StatusCompleted,
			}, nil
		}, nil))

		require.NoError(t, g.AddNode("finish", func(_ context.Context, _ CycleState, _ types.Config[CycleState]) (types.NodeResponse[CycleState], error) {
			newState := CycleState{
				Logs:  []string{"finished"},
				Value: "completed",
			}
			return types.NodeResponse[CycleState]{State: newState, Status: types.StatusCompleted}, nil
		}, nil))

		// Add edges with conditional routing
		require.NoError(t, g.AddEdge("start", "work", nil))
		require.NoError(t, g.AddEdge("work", "work", nil))
		require.NoError(t, g.AddEdge("work", "finish", nil))
		require.NoError(t, g.AddEdge("finish", graph.END, nil))

		// Add condition to break cycle
		require.NoError(t, g.AddBranch("work", func(_ context.Context, s CycleState, _ types.Config[CycleState]) string {
			if s.Iterations >= 3 {
				return "finish"
			}
			return "work"
		}, "", nil))

		require.NoError(t, g.SetEntryPoint("start"))

		// Test execution with conditional exit
		compiled, err := g.Compile(
			graph.WithMaxSteps[CycleState](10),
			graph.WithCheckpointStore[CycleState](store),
			graph.WithDebug[CycleState](),
		)
		require.NoError(t, err)

		result, err := compiled.Run(
			context.Background(),
			CycleState{Value: "initial"},
			graph.WithThreadID[CycleState]("conditional-cycle"),
		)
		require.NoError(t, err)

		// Verify execution path
		expectedLogs := []string{
			"started",
			"work iteration 1",
			"work iteration 2",
			"work iteration 3",
			"finished",
		}
		require.Equal(t, "completed", result.Value)
		require.Equal(t, 3, result.Iterations)
		require.Equal(t, expectedLogs, result.Logs)
	})

	t.Run("concurrent_cycles", func(t *testing.T) {
		t.Parallel()
		g := graph.NewGraph[CycleState]("concurrent-cycles")
		store := checkpoints.NewMemoryStore[CycleState]()

		// Add cyclic node
		require.NoError(t, g.AddNode("cycle", func(_ context.Context, s CycleState, _ types.Config[CycleState]) (types.NodeResponse[CycleState], error) {
			time.Sleep(100 * time.Millisecond) // Simulate work
			newState := CycleState{
				Logs:       []string{fmt.Sprintf("cycle-%d", s.Iterations)},
				Iterations: s.Iterations + 1,
			}
			return types.NodeResponse[CycleState]{State: newState, Status: types.StatusCompleted}, nil
		}, nil))

		// Create self-cycle
		require.NoError(t, g.AddEdge("cycle", "cycle", nil))
		require.NoError(t, g.SetEntryPoint("cycle"))
		require.NoError(t, g.SetEndPoint("cycle"))

		// Run multiple instances concurrently
		compiled, err := g.Compile(
			graph.WithMaxSteps[CycleState](5),
			graph.WithCheckpointStore[CycleState](store),
			graph.WithDebug[CycleState](),
		)
		require.NoError(t, err)

		threads := []string{"thread-1", "thread-2", "thread-3"}
		for _, threadID := range threads {
			t.Run(threadID, func(t *testing.T) {
				result, err := compiled.Run(
					context.Background(),
					CycleState{Value: "initial-" + threadID},
					graph.WithThreadID[CycleState](threadID),
				)
				require.Error(t, err) // Should hit max steps
				require.Len(t, result.Logs, 5)
				require.Contains(t, result.Value, threadID)
			})
		}
	})
}
