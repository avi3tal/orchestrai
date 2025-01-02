# OrchestAI
=======================

[![CI](https://github.com/avi3tal/orchestai/actions/workflows/ci.yml/badge.svg)](https://github.com/avi3tal/orchestai/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/avi3tal/orchestai)](https://goreportcard.com/report/github.com/avi3tal/orchestai)
[![GoDoc](https://godoc.org/github.com/avi3tal/orchestai?status.svg)](https://godoc.org/github.com/avi3tal/orchestai)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/avi3tal/orchestai/blob/main/LICENSE)

OrchestAI is a high-performance orchestration framework for distributed AI agents, designed for production-grade systems. It provides a robust infrastructure for managing and coordinating micro-agents in scalable environments.

## Features

- **Distributed Agent Orchestration**: Manage multiple AI agents across different services and locations
- **High Scalability**: Built for production environments with kubernetes-native support
- **Flexible Graph-based Workflows**: Define complex agent interactions using DAG (Directed Acyclic Graph)
- **State Management**: Robust checkpoint and state management for reliable agent execution
- **Protocol-driven**: Well-defined protocols for agent communication and state transitions

## Project Structure
/internal/
channels/     # Communication channels between agents
checkpoints/  # State persistence and recovery
graph/        # Core DAG implementation
state/        # State management
types/        # Common types and interfaces
/cmd/
examples/     # Usage examples and demos
pkg/

## Installation

Requires Go 1.23 or later.

```bash
go get github.com/avi3tal/orchestai
```

### Quick Start

Here are two examples demonstrating basic usage of OrchestAI:

### 1. Simple Agent Chain

This example shows how to create a basic chain of agents with conditional routing:


```go
package main

import (
	"context"
	"fmt"

	"github.com/avi3tal/orchestai/graph"
	"github.com/avi3tal/orchestai/state"
)

func main() {
	// Create a new graph
	g := graph.NewGraph[state.MessagesState]("simple-chain")

	// Add nodes with their handlers
	g.AddNode("greeter", func(ctx context.Context, s state.MessagesState) (*graph.Response, error) {
		return &graph.Response{
			Message: "Hello! How can I help you today?",
			Status:  graph.StatusCompleted,
		}, nil
	})

	g.AddNode("router", func(ctx context.Context, s state.MessagesState) (*graph.Response, error) {
		return &graph.Response{
			Message: "I'll help you with that request",
			Status:  graph.StatusCompleted,
		}, nil
	})

	// Add conditional routing
	g.AddConditionalEdge("greeter", []string{"router", graph.END},
		func(ctx context.Context, state state.MessagesState) string {
			// Route based on message content
			if len(state.Messages) > 0 {
				return "router"
			}
			return graph.END
		})

	// Set entry point and compile
	g.SetEntryPoint("greeter")
	compiled := g.Compile()

	// Run the graph
	result, err := compiled.Run(context.Background(), state.NewMessagesState())
	if err != nil {
		panic(err)
	}
	fmt.Println(result)
}
```

2. Approval-Required Agent

This example demonstrates how to create an agent that requires approval before proceeding:

```go
package main

import (
    "context"
    "fmt"

    "github.com/avi3tal/orchestai/graph"
    "github.com/avi3tal/orchestai/state"
)

func main() {
    // Create a new graph
    g := graph.NewGraph[state.MessagesState]("approval-flow")

    // Add an agent that requires approval
    g.AddNode("sensitive-action", func(ctx context.Context, s state.MessagesState) (*graph.Response, error) {
        if !s.HasApproval {
            return &graph.Response{
                Message: "This action requires approval. Please confirm.",
                Status:  graph.StatusPending,
            }, nil
        }
        
        return &graph.Response{
            Message: "Action completed successfully",
            Status:  graph.StatusCompleted,
        }, nil
    })

    // Compile and run the graph
    compiled := g.Compile()

    // First run - will request approval
    result, err := compiled.Run(context.Background(), state.NewMessagesState())
    if err != nil {
        fmt.Printf("First run: %s\n", result.Message)
    }

    // Second run - with approval
    approvedState := state.MessagesState{
        HasApproval: true,
    }
    result, err = compiled.Run(context.Background(), approvedState)
    if err != nil {
        fmt.Printf("Second run: %s\n", result.Message)
    }
}
```
### Contributing

Contributions are welcome! Please read our Contributing Guide for details on our code of conduct and the process for submitting pull requests.
Development

Prerequisites
* Go 1.23+
* Make

Setup

1. Clone the repository
```
git clone https://github.com/avi3tal/orchestai.git
cd orchestai
```

2. Install dependencies
```zsh
go mod download
```

3. Run tests
```zsh
make test
```

### Common Commands
```zsh
make lint          # Run linters
make test         # Run tests
make build        # Build the project
make examples     # Build example applications
```

### License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

### Acknowledgments

- Inspired by [LangGraph](https://www.langchain.com/langgraph) architecture from [LangChain](https://www.langchain.com/)
- [DAG](https://en.wikipedia.org/wiki/Directed_acyclic_graph) - Directed Acyclic Graph
- [Kubernetes](https://kubernetes.io/) - Container Orchestration

## Contributors

- [Avi Etal]()