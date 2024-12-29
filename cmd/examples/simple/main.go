package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/avi3tal/orchestrai/internal/graph"
	"github.com/avi3tal/orchestrai/internal/types"
)

type SimpleState struct {
	Value int
}

func (s *SimpleState) GetID() string {
	return "simple_state"
}

func (s *SimpleState) GetType() string {
	return "number_processor"
}

func (s *SimpleState) Get(key string) (interface{}, bool) {
	if key == "value" {
		return s.Value, true
	}
	return nil, false
}

func (s *SimpleState) Set(key string, value interface{}) (types.State, error) {
	if key == "value" {
		val, ok := value.(int)
		if !ok {
			return nil, fmt.Errorf("value must be an int")
		}
		return &SimpleState{Value: val}, nil
	}
	return nil, fmt.Errorf("invalid key: %s", key)
}

func (s *SimpleState) Delete(key string) types.State {
	return s // Immutable for this example
}

func (s *SimpleState) Merge(other types.State) (types.State, error) {
	otherSimple, ok := other.(*SimpleState)
	if !ok {
		return nil, fmt.Errorf("can only merge with SimpleState")
	}
	return &SimpleState{Value: otherSimple.Value}, nil
}

func (s *SimpleState) Clone() types.State {
	return &SimpleState{Value: s.Value}
}

func (s *SimpleState) Validate() error {
	return nil // Always valid for this example
}

func (s *SimpleState) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"value": s.Value,
	}
}

// SimpleNode implements a basic processing node
type SimpleNode struct {
	*graph.Node
	transformer types.StateTransformer
}

func NewSimpleNode(id string, transformer types.StateTransformer) *SimpleNode {
	return &SimpleNode{
		Node:        graph.NewNode(id, id, "processor"),
		transformer: transformer,
	}
}

func (n *SimpleNode) Execute(ctx context.Context, state types.State) (types.State, error) {
	return n.transformer(ctx, state)
}

func main() {
	// Create nodes
	doubleNode := NewSimpleNode("double", func(ctx context.Context, state types.State) (types.State, error) {
		s := state.(*SimpleState)
		return &SimpleState{Value: s.Value * 2}, nil
	})

	addTenNode := NewSimpleNode("add_ten", func(ctx context.Context, state types.State) (types.State, error) {
		s := state.(*SimpleState)
		return &SimpleState{Value: s.Value + 10}, nil
	})

	divideByThreeNode := NewSimpleNode("divide_by_three", func(ctx context.Context, state types.State) (types.State, error) {
		s := state.(*SimpleState)
		return &SimpleState{Value: s.Value / 3}, nil
	})

	// Create graph
	g := graph.NewGraph(
		graph.WithDebug(true),
		graph.WithVersion("1.0.0"),
	)

	// Add nodes
	err := g.AddNode(doubleNode)
	if err != nil {
		log.Fatalf("Failed to add double node: %v", err)
	}

	err = g.AddNode(addTenNode)
	if err != nil {
		log.Fatalf("Failed to add add_ten node: %v", err)
	}

	err = g.AddNode(divideByThreeNode)
	if err != nil {
		log.Fatalf("Failed to add divide_by_three node: %v", err)
	}

	// Add edges
	err = g.AddEdge(types.START, "double")
	if err != nil {
		log.Fatalf("Failed to add start edge: %v", err)
	}

	err = g.AddEdge("double", "add_ten")
	if err != nil {
		log.Fatalf("Failed to add double->add_ten edge: %v", err)
	}

	err = g.AddEdge("add_ten", "divide_by_three")
	if err != nil {
		log.Fatalf("Failed to add add_ten->divide_by_three edge: %v", err)
	}

	err = g.AddEdge("divide_by_three", types.END)
	if err != nil {
		log.Fatalf("Failed to add end edge: %v", err)
	}

	// Compile graph
	compiledGraph, err := g.Compile()
	if err != nil {
		log.Fatalf("Failed to compile graph: %v", err)
	}

	// Execute graph
	initialState := &SimpleState{Value: 5}

	fmt.Printf("Initial value: %d\n", initialState.Value)

	start := time.Now()
	finalState, err := compiledGraph.Execute(context.Background(), initialState)
	if err != nil {
		log.Fatalf("Failed to execute graph: %v", err)
	}

	finalValue := finalState.(*SimpleState).Value
	duration := time.Since(start)

	fmt.Printf("Final value: %d\n", finalValue)
	fmt.Printf("Execution time: %v\n", duration)

	// Expected flow:
	// 5 -> double -> 10 -> add_ten -> 20 -> divide_by_three -> ~6
}
