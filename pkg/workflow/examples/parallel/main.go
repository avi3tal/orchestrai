package main

import (
	"context"
	"fmt"

	"github.com/avi3tal/orchestrai/pkg/agents"
	"github.com/avi3tal/orchestrai/pkg/types"
	"github.com/avi3tal/orchestrai/pkg/workflow"
)

// State includes user data for email & address.
type UserDataState struct {
	Email   string
	Address string
}

// Implement GraphState interface
func (u UserDataState) Validate() error { return nil }
func (u UserDataState) Merge(other UserDataState) UserDataState {
	if other.Email != "" {
		u.Email = other.Email
	}
	if other.Address != "" {
		u.Address = other.Address
	}
	return u
}

// Agents
func makeGetEmailAgent() workflow.Agent[UserDataState] {
	return agents.NewSimpleAgent("GetEmail", func(_ context.Context, s UserDataState, _ types.Config[UserDataState]) (types.NodeResponse[UserDataState], error) {
		// Simulate retrieving an email from user or database
		s.Email = "alice@example.com"
		return types.NodeResponse[UserDataState]{State: s, Status: types.StatusCompleted}, nil
	}, nil)
}

func makeGetAddressAgent() workflow.Agent[UserDataState] {
	return agents.NewSimpleAgent("GetAddress", func(_ context.Context, s UserDataState, _ types.Config[UserDataState]) (types.NodeResponse[UserDataState], error) {
		// Simulate retrieving a shipping address
		s.Address = "1234 Main St."
		return types.NodeResponse[UserDataState]{State: s, Status: types.StatusCompleted}, nil
	}, nil)
}

func makeFinalAgent() workflow.Agent[UserDataState] {
	return agents.NewSimpleAgent("DoSomething", func(_ context.Context, s UserDataState, _ types.Config[UserDataState]) (types.NodeResponse[UserDataState], error) {
		fmt.Println("Final Agent: got Email =", s.Email, "and Address =", s.Address)
		return types.NodeResponse[UserDataState]{State: s, Status: types.StatusCompleted}, nil
	}, nil)
}

func main() {
	// Build the workflow
	wf := workflow.NewBuilder[UserDataState]("Parallel-Join-Demo")

	getEmail := makeGetEmailAgent()
	getAddress := makeGetAddressAgent()
	finalAgent := makeFinalAgent()

	err := wf.AddAgent(getEmail).
		AsEntryPoint().
		// ThenAll => run getEmail and getAddress in parallel
		ThenAll(getAddress).
		// Join => merges the partial states from both agents
		Join(func(ctx context.Context, states []UserDataState, _ types.Config[UserDataState]) (types.NodeResponse[UserDataState], error) {
			// Aggregate the states
			merged := states[0]
			for i := 1; i < len(states); i++ {
				merged = merged.Merge(states[i])
			}
			return types.NodeResponse[UserDataState]{State: merged, Status: types.StatusCompleted}, nil
		}).
		Then(finalAgent).
		End()
	if err != nil {
		panic(fmt.Sprintf("Failed building flow: %v", err))
	}

	// Create the application
	app, err := workflow.NewApp[UserDataState](wf, workflow.WithDebug[UserDataState]())
	if err != nil {
		panic(fmt.Sprintf("Failed creating app: %v", err))
	}

	// Run it
	initState := UserDataState{}
	result, err := app.Invoke(context.Background(), initState)
	if err != nil {
		fmt.Println("Invoke error:", err)
		return
	}
	fmt.Println("Workflow done. Final state:", result)
}
