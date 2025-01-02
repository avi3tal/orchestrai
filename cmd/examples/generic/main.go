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

func main() {
	g := graph.NewGraph("pending-agents")

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
		func(ctx context.Context, other state.GraphState, cfg types.Config) string {
			o, ok := other.(state.MessagesState)
			if !ok {
				return graph.END
			}
			if len(o.Messages) > 5 {
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

	finalState, err := compiled.Run(context.Background(), initialState, graph.WithThreadID("thread-1"))
	if err != nil {
		panic(err)
	}

	fmt.Println(finalState)
}
