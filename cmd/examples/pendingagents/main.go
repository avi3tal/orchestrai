package main

import (
	"context"
	"fmt"

	"github.com/avi3tal/orchestrai/internal/checkpoints"
	"github.com/avi3tal/orchestrai/internal/dag"
	"github.com/avi3tal/orchestrai/internal/state"
	"github.com/avi3tal/orchestrai/internal/types"
	"github.com/tmc/langchaingo/llms"
)

func AddAIMessage(text string) func(context.Context, state.MessagesState, types.Config[state.MessagesState]) (dag.NodeResponse[state.MessagesState], error) {
	return func(_ context.Context, _ state.MessagesState, _ types.Config[state.MessagesState]) (dag.NodeResponse[state.MessagesState], error) {
		ms := state.MessagesState{
			Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeAI, text)},
		}
		return dag.NodeResponse[state.MessagesState]{
			State: ms,
		}, nil
	}
}

func PendingAgent(_ context.Context, st state.MessagesState, _ types.Config[state.MessagesState]) (dag.NodeResponse[state.MessagesState], error) {
	// Simulate pending condition
	if len(st.Messages) < 3 {
		// Return pending status if the condition is met
		return dag.NodeResponse[state.MessagesState]{
			State: state.MessagesState{
				Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeAI, "Sensitive tool requires approval")},
			},
			Status: types.StatusPending,
		}, nil
	}

	// Otherwise, complete the node
	return dag.NodeResponse[state.MessagesState]{
		State: state.MessagesState{
			Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeAI, "sensitive tool executed successfully")},
		},
		Status: types.StatusCompleted,
	}, nil
}

func main() {
	g := dag.NewGraph[state.MessagesState]("pending-agent")

	// Add nodes
	_ = g.AddNode("StartNode", AddAIMessage("Hello, running actions"), nil)
	_ = g.AddNode("toolsAgent", PendingAgent, nil)

	_ = g.AddEdge("StartNode", "toolsAgent", nil)
	_ = g.AddEdge("toolsAgent", dag.END, nil)
	_ = g.SetEntryPoint("StartNode")

	g.PrintGraph()

	compiled, err := g.Compile(
		dag.WithDebug[state.MessagesState](),
		dag.WithCheckpointStore(checkpoints.NewMemoryStore[state.MessagesState]()),
		dag.WithTimeout[state.MessagesState](30),
		dag.WithMaxSteps[state.MessagesState](100),
	)
	if err != nil {
		panic(err)
	}

	initialState := state.MessagesState{
		Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, "Hello my name is Bowie")},
	}

	// First run
	pendingState, err := compiled.Run(context.Background(), initialState, dag.WithThreadID[state.MessagesState]("thread-1"))
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
	finalState, err := compiled.Run(context.Background(), resumeState, dag.WithThreadID[state.MessagesState]("thread-1"))
	if err != nil {
		panic(err)
	}

	for _, msg := range finalState.Messages {
		fmt.Printf("\t%s: %s\n", msg.Role, msg.Parts[0].(llms.TextContent).Text)
	}
}
