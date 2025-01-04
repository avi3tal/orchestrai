package main

import (
	"context"
	"fmt"

	"github.com/avi3tal/orchestrai/internal/graph"
	"github.com/avi3tal/orchestrai/pkg/checkpoints"
	"github.com/avi3tal/orchestrai/pkg/state"
	"github.com/avi3tal/orchestrai/pkg/types"
	"github.com/tmc/langchaingo/llms"
)

func AddAIMessage(text string) func(context.Context, state.MessagesState, types.Config[state.MessagesState]) (types.NodeResponse[state.MessagesState], error) {
	return func(_ context.Context, _ state.MessagesState, _ types.Config[state.MessagesState]) (types.NodeResponse[state.MessagesState], error) {
		ms := state.MessagesState{
			Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeAI, text)},
		}
		return types.NodeResponse[state.MessagesState]{
			State: ms,
		}, nil
	}
}

func PendingAgent(_ context.Context, st state.MessagesState, _ types.Config[state.MessagesState]) (types.NodeResponse[state.MessagesState], error) {
	// Simulate pending condition
	if len(st.Messages) < 3 {
		// Return pending status if the condition is met
		return types.NodeResponse[state.MessagesState]{
			State: state.MessagesState{
				Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeAI, "Sensitive tool requires approval")},
			},
			Status: types.StatusPending,
		}, nil
	}

	// Otherwise, complete the node
	return types.NodeResponse[state.MessagesState]{
		State: state.MessagesState{
			Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeAI, "sensitive tool executed successfully")},
		},
		Status: types.StatusCompleted,
	}, nil
}

func main() {
	g := graph.NewGraph[state.MessagesState]("pending-agent")

	// Add nodes
	_ = g.AddNode("StartNode", AddAIMessage("Hello, running actions"), nil)
	_ = g.AddNode("toolsAgent", PendingAgent, nil)

	_ = g.AddEdge("StartNode", "toolsAgent", nil)
	_ = g.AddEdge("toolsAgent", graph.END, nil)
	_ = g.SetEntryPoint("StartNode")

	g.PrintGraph()

	compiled, err := g.Compile(
		graph.WithDebug[state.MessagesState](),
		graph.WithCheckpointStore(checkpoints.NewMemoryStore[state.MessagesState]()),
		graph.WithTimeout[state.MessagesState](30),
		graph.WithMaxSteps[state.MessagesState](100),
	)
	if err != nil {
		panic(err)
	}

	initialState := state.MessagesState{
		Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, "Hello my name is Bowie")},
	}

	// First run
	pendingState, err := compiled.Run(context.Background(), initialState, graph.WithThreadID[state.MessagesState]("thread-1"))
	if err != nil {
		panic(err)
	}

	for _, msg := range pendingState.Messages {
		fmt.Printf("\t%s: %s\n", msg.Role, msg.Parts[0].(llms.TextContent).Text)
	}

	// Handle pending execution
	resumeState := state.MessagesState{
		Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, "Approved message")},
	}
	finalState, err := compiled.Run(context.Background(), resumeState, graph.WithThreadID[state.MessagesState]("thread-1"))
	if err != nil {
		panic(err)
	}

	for _, msg := range finalState.Messages {
		fmt.Printf("\t%s: %s\n", msg.Role, msg.Parts[0].(llms.TextContent).Text)
	}
}
