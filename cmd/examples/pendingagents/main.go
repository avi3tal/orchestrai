package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/avi3tal/orchestrai/internal/dag"
	"github.com/tmc/langchaingo/llms"
)

type MessagesState struct {
	Messages []llms.MessageContent
}

func (m MessagesState) Validate() error {
	if len(m.Messages) == 0 {
		return errors.New("no messages")
	}
	return nil
}

func (m MessagesState) Merge(other MessagesState) MessagesState {
	return MessagesState{
		Messages: append(m.Messages, other.Messages...),
	}
}

func AddAIMessage(text string) func(context.Context, MessagesState, dag.Config[MessagesState]) (dag.NodeResponse[MessagesState], error) {
	return func(_ context.Context, state MessagesState, c dag.Config[MessagesState]) (dag.NodeResponse[MessagesState], error) {
		ms := MessagesState{
			Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeAI, text)},
		}
		return dag.NodeResponse[MessagesState]{
			State: ms,
		}, nil
	}
}

func PendingAgent(ctx context.Context, state MessagesState, c dag.Config[MessagesState]) (dag.NodeResponse[MessagesState], error) {
	// Simulate pending condition
	if len(state.Messages) < 3 {
		// Return pending status if the condition is met
		return dag.NodeResponse[MessagesState]{
			State: MessagesState{
				Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeAI, "Sensitive tool requires approval")},
			},
			Status: dag.StatusPending,
		}, nil
	}

	// Otherwise, complete the node
	return dag.NodeResponse[MessagesState]{
		State: MessagesState{
			Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeAI, "sensitive tool executed successfully")},
		},
		Status: dag.StatusCompleted,
	}, nil
}

func main() {
	g := dag.NewGraph[MessagesState]("pending-agent")

	// Add nodes
	_ = g.AddNode("StartNode", AddAIMessage("Hello, running actions"), nil)
	_ = g.AddNode("toolsAgent", PendingAgent, nil)

	_ = g.AddEdge("StartNode", "toolsAgent", nil)
	_ = g.AddEdge("toolsAgent", dag.END, nil)
	_ = g.SetEntryPoint("StartNode")

	g.PrintGraph()

	compiled, err := g.Compile(
		dag.WithDebug[MessagesState](),
		dag.WithCheckpointStore(dag.NewMemoryStore[MessagesState]()),
		dag.WithTimeout[MessagesState](30),
		dag.WithMaxSteps[MessagesState](100),
	)
	if err != nil {
		panic(err)
	}

	initialState := MessagesState{
		Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, "Hello my name is Bowie")},
	}

	// First run
	pendingState, err := compiled.Run(context.Background(), initialState, dag.WithThreadID[MessagesState]("thread-1"))
	if err != nil {
		panic(err)
	}

	for _, msg := range pendingState.Messages {
		fmt.Printf("\t%s: %s\n", msg.Role, msg.Parts[0].(llms.TextContent).Text)
	}

	// Handle pending execution
	resumeState := MessagesState{
		Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, "Approved message")},
	}
	finalState, err := compiled.Run(context.Background(), resumeState, dag.WithThreadID[MessagesState]("thread-1"))
	if err != nil {
		panic(err)
	}

	for _, msg := range finalState.Messages {
		fmt.Printf("\t%s: %s\n", msg.Role, msg.Parts[0].(llms.TextContent).Text)
	}
}
