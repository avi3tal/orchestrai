package main

import (
	"context"
	"fmt"
	"github.com/avi3tal/orchestrai/pkg/agents"
	"github.com/avi3tal/orchestrai/pkg/types"
	"github.com/avi3tal/orchestrai/pkg/workflow"
)

// Flow Explanation:
//	1.	Incrementer will keep looping back to itself while Value < 5. Each loop increments the value.
//	2.	Once Value hits 5, we go to Printer, which prints Value=5.
//	3.	Then we call .End(), ending the flow.

type CounterState struct {
	Value int
}

func (cs CounterState) Validate() error { return nil }
func (cs CounterState) Merge(other CounterState) CounterState {
	cs.Value = other.Value
	return cs
}

func makeLectureAgent() workflow.Agent[CounterState] {
	return agents.NewSimpleAgent[CounterState](
		"Lecturer",
		func(_ context.Context, s CounterState, _ types.Config[CounterState]) (types.NodeResponse[CounterState], error) {
			fmt.Println("Lecture: Welcome to the loop-while demo!")
			return types.NodeResponse[CounterState]{State: s, Status: types.StatusCompleted}, nil
		},
		nil,
	)
}

// Increments the Value
func makeIncrementAgent(name string) workflow.Agent[CounterState] {
	return agents.NewSimpleAgent[CounterState](
		name,
		func(_ context.Context, s CounterState, _ types.Config[CounterState]) (types.NodeResponse[CounterState], error) {
			s.Value++
			return types.NodeResponse[CounterState]{State: s, Status: types.StatusCompleted}, nil
		},
		nil,
	)
}

// Prints and returns the state
func makePrintAgent(name string) workflow.Agent[CounterState] {
	return agents.NewSimpleAgent[CounterState](
		name,
		func(_ context.Context, s CounterState, _ types.Config[CounterState]) (types.NodeResponse[CounterState], error) {
			fmt.Printf("[%s] Current Value: %d\n", name, s.Value)
			return types.NodeResponse[CounterState]{State: s, Status: types.StatusCompleted}, nil
		},
		nil,
	)
}

func main() {
	wf := workflow.NewBuilder[CounterState]("loop-while-demo")

	// Agents
	lectureAgent := makeIncrementAgent("Lecturer")
	incrAgent := makeIncrementAgent("Incrementer")
	printAgent := makePrintAgent("Printer")

	// Build chain:
	//  1) AddAgent(incrAgent) as entry
	//  2) ThenIf(Value < 5)
	//  3) Then(printAgent)
	//  4) End()
	// Meaning: "Incrementer" loops to itself while Value < 5, else it proceeds to "Printer"

	err := wf.AddAgent(lectureAgent).
		AsEntryPoint().
		Then(incrAgent).
		ThenIf(func(_ context.Context, s CounterState, _ types.Config[CounterState]) bool {
			return s.Value < 5
		}, incrAgent, printAgent).
		End()

	if err != nil {
		fmt.Println("Error building the loop flow:", err)
		return
	}

	app, err := workflow.NewApp[CounterState](wf, workflow.WithDebug[CounterState]())
	if err != nil {
		fmt.Println("Error building the Application flow:", err)
		return
	}

	res, err := app.Invoke(context.Background(), CounterState{Value: 0})
	if err != nil {
		fmt.Println("Run error:", err)
		return
	}

	fmt.Println("Final State:", res.Value)
}
