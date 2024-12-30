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

func AddAIMessage(text string) func(context.Context, MessagesState, dag.Config[MessagesState]) (MessagesState, error) {
	return func(_ context.Context, state MessagesState, c dag.Config[MessagesState]) (MessagesState, error) {
		state.Messages = append(state.Messages, llms.TextParts(llms.ChatMessageTypeAI, text))
		return state, nil
	}
}

func main() {
	g := dag.NewGraph[MessagesState]()

	// Add nodes
	if err := g.AddNode("agentA", AddAIMessage("Hello"), nil); err != nil {
		panic(err)
	}
	if err := g.AddNode("agentB", AddAIMessage("Follow up"), nil); err != nil {
		panic(err)
	}

	// Add direct edge
	if err := g.AddEdge("agentB", dag.END, nil); err != nil {
		panic(err)
	}

	// Add conditional edge
	if err := g.AddConditionalEdge(
		"agentA",
		[]string{"agentB", dag.END},
		func(ctx context.Context, state MessagesState, cfg dag.Config[MessagesState]) string {
			if len(state.Messages) > 5 {
				return dag.END
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

	// Compile and run
	config := dag.Config[MessagesState]{
		ThreadID: "thread-1",
		MaxSteps: 100,
		Timeout:  30,
		Debug:    true,
	}

	compiled, err := g.Compile(config)
	if err != nil {
		panic(err)
	}

	initialState := MessagesState{
		Messages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, "Hello my name is Bowie")},
	}

	finalState, err := compiled.Run(context.Background(), initialState)
	if err != nil {
		panic(err)
	}

	fmt.Println(finalState)
}
