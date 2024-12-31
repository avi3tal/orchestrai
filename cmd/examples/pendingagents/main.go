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

func ApprovalNode(ctx context.Context, state MessagesState, c dag.Config[MessagesState]) (dag.NodeResponse[MessagesState], error) {
	// Simulate user approval
	userApproved := true // Replace with actual approval logic

	if !userApproved {
		return dag.NodeResponse[MessagesState]{
			Status: dag.StatusFailed,
		}, errors.New("approval denied")
	}

	// Append approval message and return to the pending agent
	ms := MessagesState{
		Messages: append(state.Messages, llms.TextParts(llms.ChatMessageTypeAI, "Approval Granted")),
	}
	return dag.NodeResponse[MessagesState]{
		State:  ms,
		Status: dag.StatusCompleted,
	}, nil
}

func main() {
	g := dag.NewGraph[MessagesState]()

	// Add nodes
	_ = g.AddNode("StartNode", AddAIMessage("Hello, running actions"), nil)
	_ = g.AddNode("toolsAgent", PendingAgent, nil)

	_ = g.AddEdge("StartNode", "toolsAgent", nil)
	_ = g.AddEdge("toolsAgent", dag.END, nil)
	_ = g.SetEntryPoint("StartNode")

	g.PrintGraph()

	// Compile the graph
	config := dag.Config[MessagesState]{
		ThreadID:     "thread-1",
		MaxSteps:     100,
		Timeout:      30,
		Debug:        true,
		Checkpointer: dag.NewMemoryCheckpointer[MessagesState](),
	}

	compiled, err := g.Compile(config)
	if err != nil {
		panic(err)
	}

	initialState := MessagesState{
		Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, "Hello my name is Bowie")},
	}

	// First run
	pendingState, err := compiled.Run(context.Background(), initialState)
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
	finalState, err := compiled.Run(context.Background(), resumeState)
	if err != nil {
		panic(err)
	}

	for _, msg := range finalState.Messages {
		fmt.Printf("\t%s: %s\n", msg.Role, msg.Parts[0].(llms.TextContent).Text)
	}
}
