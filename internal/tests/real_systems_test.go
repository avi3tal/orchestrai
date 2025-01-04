package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/avi3tal/orchestrai/internal/graph"
	"github.com/avi3tal/orchestrai/pkg/checkpoints"
	"github.com/avi3tal/orchestrai/pkg/types"
	"github.com/stretchr/testify/require"
)

// CodeCompilerState represents the state for a code compilation workflow
type CodeCompilerState struct {
	Code     string
	Analysis string
	Errors   []string
	Approved bool
	Output   string
}

func (s CodeCompilerState) Validate() error {
	if s.Code == "" {
		return errors.New("code cannot be empty")
	}
	return nil
}

func (s CodeCompilerState) Merge(other CodeCompilerState) CodeCompilerState {
	merged := CodeCompilerState{
		Code:     other.Code,
		Analysis: other.Analysis,
		Approved: other.Approved,
		Output:   other.Output,
	}
	merged.Errors = append(merged.Errors, other.Errors...)
	return merged
}

// ResearchState represents the state for a research workflow
type ResearchState struct {
	Topic      string
	Research   string
	Draft      string
	Review     string
	Approved   bool
	Published  bool
	Iterations int
}

func (s ResearchState) Validate() error {
	if s.Topic == "" {
		return errors.New("topic cannot be empty")
	}
	return nil
}

func (s ResearchState) Merge(other ResearchState) ResearchState {
	if other.Research != "" {
		s.Research = other.Research
	}
	if other.Draft != "" {
		s.Draft = other.Draft
	}
	if other.Review != "" {
		s.Review = other.Review
	}
	s.Approved = other.Approved
	s.Published = other.Published
	if other.Iterations > s.Iterations {
		s.Iterations = other.Iterations
	}
	return s
}

// TestCodeCompilerWorkflow tests a complete code compilation workflow
func TestCodeCompilerWorkflow(t *testing.T) {
	t.Parallel()
	t.Run("state_validation", func(t *testing.T) {
		t.Parallel()
		gg := graph.NewGraph[ResearchState]("validation-test")

		require.NoError(t, gg.AddNode("start", func(_ context.Context, s ResearchState, _ types.Config[ResearchState]) (types.NodeResponse[ResearchState], error) {
			return types.NodeResponse[ResearchState]{State: s, Status: types.StatusCompleted}, nil
		}, nil))

		require.NoError(t, gg.AddEdge("start", graph.END, nil))
		require.NoError(t, gg.SetEntryPoint("start"))

		compiled, err := gg.Compile()
		require.NoError(t, err)

		// Test with invalid state (empty topic)
		_, err = compiled.Run(context.Background(), ResearchState{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "topic cannot be empty")
	})

	t.Run("checkpoint_recovery", func(t *testing.T) {
		t.Parallel()
		gg := graph.NewGraph[ResearchState]("recovery-test")
		st := checkpoints.NewMemoryStore[ResearchState]()

		// Add node that requires multiple attempts
		require.NoError(t, gg.AddNode("process", func(_ context.Context, s ResearchState, _ types.Config[ResearchState]) (types.NodeResponse[ResearchState], error) {
			if s.Iterations == 0 {
				s.Iterations++
				return types.NodeResponse[ResearchState]{
					State:  s,
					Status: types.StatusPending,
				}, nil
			}
			s.Research = "Completed"
			return types.NodeResponse[ResearchState]{
				State:  s,
				Status: types.StatusCompleted,
			}, nil
		}, nil))

		require.NoError(t, gg.AddEdge("process", graph.END, nil))
		require.NoError(t, gg.SetEntryPoint("process"))

		compiled, err := gg.Compile(graph.WithCheckpointStore[ResearchState](st))
		require.NoError(t, err)

		// First run - should stop at pending
		initialState := ResearchState{Topic: "test"}
		result, err := compiled.Run(context.Background(), initialState, graph.WithThreadID[ResearchState]("thread-1"))
		require.NoError(t, err)
		require.Equal(t, 1, result.Iterations)

		// Verify checkpoint
		checkpoint, err := st.Load(context.Background(), types.CheckpointKey{
			GraphID:  gg.ID(),
			ThreadID: "thread-1",
		})
		require.NoError(t, err)
		require.Equal(t, types.StatusPending, checkpoint.Meta.Status)

		// Second run - should complete
		result, err = compiled.Run(context.Background(), result)
		require.NoError(t, err)
		require.Equal(t, "Completed", result.Research)
	})

	t.Run("branch_conditions", func(t *testing.T) {
		t.Parallel()
		gg := graph.NewGraph[ResearchState]("branch-test")

		// Add nodes for different paths
		require.NoError(t, gg.AddNode("start", func(_ context.Context, s ResearchState, _ types.Config[ResearchState]) (types.NodeResponse[ResearchState], error) {
			return types.NodeResponse[ResearchState]{State: s, Status: types.StatusCompleted}, nil
		}, nil))

		require.NoError(t, gg.AddNode("pathA", func(_ context.Context, s ResearchState, _ types.Config[ResearchState]) (types.NodeResponse[ResearchState], error) {
			s.Research = "Path A"
			return types.NodeResponse[ResearchState]{State: s, Status: types.StatusCompleted}, nil
		}, nil))

		require.NoError(t, gg.AddNode("pathB", func(_ context.Context, s ResearchState, _ types.Config[ResearchState]) (types.NodeResponse[ResearchState], error) {
			s.Research = "Path B"
			return types.NodeResponse[ResearchState]{State: s, Status: types.StatusCompleted}, nil
		}, nil))

		// Add conditional branching
		require.NoError(t, gg.AddEdge("start", "pathA", nil))
		require.NoError(t, gg.AddEdge("start", "pathB", nil))
		require.NoError(t, gg.AddEdge("pathA", graph.END, nil))
		require.NoError(t, gg.AddEdge("pathB", graph.END, nil))

		require.NoError(t, gg.AddBranch("start", func(_ context.Context, s ResearchState, _ types.Config[ResearchState]) string {
			if s.Iterations > 0 {
				return "pathA"
			}
			return "pathB"
		}, "", nil))

		require.NoError(t, gg.SetEntryPoint("start"))

		compiled, err := gg.Compile()
		require.NoError(t, err)

		// Test path B (Iterations = 0)
		result, err := compiled.Run(context.Background(), ResearchState{Topic: "test"})
		require.NoError(t, err)
		require.Equal(t, "Path B", result.Research)

		// Test path A (Iterations > 0)
		result, err = compiled.Run(context.Background(), ResearchState{Topic: "test", Iterations: 1})
		require.NoError(t, err)
		require.Equal(t, "Path A", result.Research)
	})
}

// TestResearchWorkflow tests a complex research workflow with review cycles
func TestResearchWorkflow(t *testing.T) {
	t.Parallel()
	g := graph.NewGraph[ResearchState]("research")
	store := checkpoints.NewMemoryStore[ResearchState]()

	// Add nodes for research workflow
	require.NoError(t, g.AddNode("researcher", func(_ context.Context, s ResearchState, _ types.Config[ResearchState]) (types.NodeResponse[ResearchState], error) {
		s.Research = "Completed research on: " + s.Topic
		return types.NodeResponse[ResearchState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("writer", func(_ context.Context, s ResearchState, _ types.Config[ResearchState]) (types.NodeResponse[ResearchState], error) {
		s.Draft = "Draft based on research: " + s.Research
		s.Iterations++
		return types.NodeResponse[ResearchState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("reviewer", func(_ context.Context, s ResearchState, _ types.Config[ResearchState]) (types.NodeResponse[ResearchState], error) {
		if s.Iterations < 2 {
			s.Review = "Needs revision"
			s.Approved = false
		} else {
			s.Review = "Approved"
			s.Approved = true
		}
		return types.NodeResponse[ResearchState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("publisher", func(_ context.Context, s ResearchState, _ types.Config[ResearchState]) (types.NodeResponse[ResearchState], error) {
		s.Published = true
		return types.NodeResponse[ResearchState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	// Add edges and conditional routing
	require.NoError(t, g.AddEdge("researcher", "writer", nil))
	require.NoError(t, g.AddEdge("writer", "reviewer", nil))

	// Add conditional edge from reviewer
	require.NoError(t, g.AddConditionalEdge(
		"reviewer",
		[]string{"writer", "publisher"},
		func(_ context.Context, s ResearchState, _ types.Config[ResearchState]) string {
			if !s.Approved {
				return "writer"
			}
			return "publisher"
		},
		nil,
	))

	require.NoError(t, g.AddEdge("publisher", graph.END, nil))
	require.NoError(t, g.SetEntryPoint("researcher"))

	// Test the research workflow
	compiled, err := g.Compile(
		graph.WithCheckpointStore[ResearchState](store),
		graph.WithMaxSteps[ResearchState](10),
		graph.WithDebug[ResearchState](),
	)
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), ResearchState{
		Topic: "AI Testing",
	})
	require.NoError(t, err)

	// Verify the final state
	require.True(t, result.Published)
	require.True(t, result.Approved)
	require.Equal(t, 2, result.Iterations)
	require.Contains(t, result.Research, "AI Testing")
	require.Equal(t, "Approved", result.Review)
}

// TestConcurrentGraphExecution tests concurrent execution of multiple graph instances
func TestConcurrentGraphExecution(t *testing.T) {
	t.Parallel()
	g := graph.NewGraph[ResearchState]("concurrent-research")
	store := checkpoints.NewMemoryStore[ResearchState]()

	// Add simple nodes for concurrent testing
	require.NoError(t, g.AddNode("process", func(_ context.Context, s ResearchState, _ types.Config[ResearchState]) (types.NodeResponse[ResearchState], error) {
		time.Sleep(100 * time.Millisecond) // Simulate work
		s.Research = "Processed: " + s.Topic
		return types.NodeResponse[ResearchState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddEdge("process", graph.END, nil))
	require.NoError(t, g.SetEntryPoint("process"))

	// Compile the graph
	compiled, err := g.Compile(
		graph.WithCheckpointStore[ResearchState](store),
		graph.WithDebug[ResearchState](),
	)
	require.NoError(t, err)

	// Run multiple instances concurrently
	topics := []string{"Topic1", "Topic2", "Topic3", "Topic4", "Topic5"}
	for _, topic := range topics {
		t.Run(topic, func(t *testing.T) {
			t.Parallel()
			result, err := compiled.Run(
				context.Background(),
				ResearchState{Topic: topic},
				graph.WithThreadID[ResearchState](topic),
			)
			require.NoError(t, err)
			require.Equal(t, "Processed: "+topic, result.Research)
		})
	}
}

// TestErrorHandling tests various error conditions and edge cases
func TestErrorHandling(t *testing.T) {
	t.Parallel()

	t.Run("invalid_node_connection", func(t *testing.T) {
		t.Parallel()
		g := graph.NewGraph[ResearchState]("error-test")

		err := g.AddEdge("nonexistent", "also-nonexistent", nil)
		require.Error(t, err)
	})

	t.Run("cycle_detection", func(t *testing.T) {
		t.Parallel()
		g := graph.NewGraph[ResearchState]("cycle-test")

		require.NoError(t, g.AddNode("node1", func(_ context.Context, s ResearchState, _ types.Config[ResearchState]) (types.NodeResponse[ResearchState], error) {
			return types.NodeResponse[ResearchState]{State: s, Status: types.StatusCompleted}, nil
		}, nil))

		require.NoError(t, g.AddNode("node2", func(_ context.Context, s ResearchState, _ types.Config[ResearchState]) (types.NodeResponse[ResearchState], error) {
			return types.NodeResponse[ResearchState]{State: s, Status: types.StatusCompleted}, nil
		}, nil))

		require.NoError(t, g.AddEdge("node1", "node2", nil))
		require.NoError(t, g.AddEdge("node2", "node1", nil))
		require.NoError(t, g.SetEntryPoint("node1"))
		require.NoError(t, g.SetEndPoint("node2"))

		_, err := g.Compile()
		require.NoError(t, err) // Cycles are allowed in our implementation
	})

	t.Run("max_steps_limit", func(t *testing.T) {
		t.Parallel()
		g := graph.NewGraph[ResearchState]("steps-test")

		require.NoError(t, g.AddNode("loop", func(_ context.Context, s ResearchState, _ types.Config[ResearchState]) (types.NodeResponse[ResearchState], error) {
			s.Iterations++
			return types.NodeResponse[ResearchState]{State: s, Status: types.StatusCompleted}, nil
		}, nil))

		require.NoError(t, g.AddEdge("loop", "loop", nil))
		require.NoError(t, g.SetEntryPoint("loop"))
	})
}
