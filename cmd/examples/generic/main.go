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

func AddAIMessage(text string) func(context.Context, state.MessagesState, types.Config[state.MessagesState]) (graph.NodeResponse[state.MessagesState], error) {
	return func(_ context.Context, _ state.MessagesState, _ types.Config[state.MessagesState]) (graph.NodeResponse[state.MessagesState], error) {
		ms := state.MessagesState{
			Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeAI, text)},
		}
		return graph.NodeResponse[state.MessagesState]{
			State: ms,
		}, nil
	}
}

func main() {
	g := graph.NewGraph[state.MessagesState]("pending-agents")

	// Add nodes
	if err := g.AddNode("agentA", AddAIMessage("Hello"), nil); err != nil {
		panic(err)
	}
	if err := g.AddNode("agentB", AddAIMessage("Follow up"), nil); err != nil {
		panic(err)
	}

	// Add direct edge
	if err := g.AddEdge("agentB", graph.END, nil); err != nil {
		panic(err)
	}

	// Add conditional edge
	if err := g.AddConditionalEdge(
		"agentA",
		[]string{"agentB", graph.END},
		func(ctx context.Context, state state.MessagesState, cfg types.Config[state.MessagesState]) string {
			if len(state.Messages) > 5 {
				return graph.END
			}
			return "agentB"
		},
		nil,
	); err != nil {
		panic(err)
	}

	if err := g.SetEntryPoint("agentA"); err != nil {
		panic(err)
	}

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

	finalState, err := compiled.Run(context.Background(), initialState, graph.WithThreadID[state.MessagesState]("thread-1"))
	if err != nil {
		panic(err)
	}

	fmt.Println(finalState)
}
