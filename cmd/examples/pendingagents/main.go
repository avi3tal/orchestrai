package main

import (
	"context"
	"fmt"

	"github.com/avi3tal/orchestrai/internal/checkpoints"
	"github.com/avi3tal/orchestrai/internal/graph"
	"github.com/avi3tal/orchestrai/internal/state"
	"github.com/avi3tal/orchestrai/internal/types"
	"github.com/tmc/langchaingo/llms"
)

func AddAIMessage(text string) func(context.Context, state.GraphState, types.Config) (graph.NodeResponse, error) {
	return func(_ context.Context, _ state.GraphState, _ types.Config) (graph.NodeResponse, error) {
		ms := state.MessagesState{
			Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeAI, text)},
		}
		return graph.NodeResponse{
			State: ms,
		}, nil
	}
}

func PendingAgent(_ context.Context, other state.GraphState, _ types.Config) (graph.NodeResponse, error) {
	st, _ := other.(state.MessagesState)
	// Simulate pending condition
	if len(st.Messages) < 3 {
		// Return pending status if the condition is met
		return graph.NodeResponse{
			State: state.MessagesState{
				Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeAI, "Sensitive tool requires approval")},
			},
			Status: types.StatusPending,
		}, nil
	}

	// Otherwise, complete the node
	return graph.NodeResponse{
		State: state.MessagesState{
			Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeAI, "sensitive tool executed successfully")},
		},
		Status: types.StatusCompleted,
	}, nil
}

func main() {
	g := graph.NewGraph("pending-agent")

	// Add nodes
	_ = g.AddNode("StartNode", AddAIMessage("Hello, running actions"), nil)
	_ = g.AddNode("toolsAgent", PendingAgent, nil)

	_ = g.AddEdge("StartNode", "toolsAgent", nil)
	_ = g.AddEdge("toolsAgent", graph.END, nil)
	_ = g.SetEntryPoint("StartNode")

	g.PrintGraph()

	compiled, err := g.Compile(
		graph.WithDebug(),
		graph.WithCheckpointStore(checkpoints.NewMemoryStore()),
		graph.WithTimeout(30),
		graph.WithMaxSteps(100),
	)
	if err != nil {
		panic(err)
	}

	initialState := state.MessagesState{
		Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, "Hello my name is Bowie")},
	}

	// First run
	pendingState, err := compiled.Run(context.Background(), initialState, graph.WithThreadID("thread-1"))
	if err != nil {
		panic(err)
	}
	ps, ok := pendingState.(state.MessagesState)
	if !ok {
		panic("invalid state type")
	}

	for _, msg := range ps.Messages {
		fmt.Printf("\t%s: %s\n", msg.Role, msg.Parts[0].(llms.TextContent).Text)
	}

	// Handle pending execution
	resumeState := state.MessagesState{
		Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, "Approved message")},
	}
	finalState, err := compiled.Run(context.Background(), resumeState, graph.WithThreadID("thread-1"))
	if err != nil {
		panic(err)
	}

	fs, ok := finalState.(state.MessagesState)
	if !ok {
		panic("invalid state type")
	}

	for _, msg := range fs.Messages {
		fmt.Printf("\t%s: %s\n", msg.Role, msg.Parts[0].(llms.TextContent).Text)
	}
}
