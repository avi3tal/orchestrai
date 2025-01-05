package main

import (
	"context"
	"fmt"

	"github.com/avi3tal/orchestrai/pkg/agents"
	"github.com/avi3tal/orchestrai/pkg/types"
	"github.com/avi3tal/orchestrai/pkg/workflow"
)

type ArticleState struct {
	Content  string
	Valid    bool
	Approved bool
}

func (a ArticleState) Validate() error { return nil }
func (a ArticleState) Merge(other ArticleState) ArticleState {
	if other.Content != "" {
		a.Content = other.Content
	}
	a.Valid = a.Valid || other.Valid
	a.Approved = a.Approved || other.Approved
	return a
}

// Main flow's starting agent: produce some content
func makeWriterAgent() workflow.Agent[ArticleState] {
	return agents.NewSimpleAgent("Writer", func(_ context.Context, s ArticleState, _ types.Config[ArticleState]) (types.NodeResponse[ArticleState], error) {
		s.Content = "Draft content for subFlow"
		return types.NodeResponse[ArticleState]{State: s, Status: types.StatusCompleted}, nil
	}, nil)
}

// After subFlow is done, we do "publish"
func makePublishAgent() workflow.Agent[ArticleState] {
	return agents.NewSimpleAgent("Publisher", func(_ context.Context, s ArticleState, _ types.Config[ArticleState]) (types.NodeResponse[ArticleState], error) {
		if s.Approved {
			fmt.Println("Publishing content:", s.Content)
		} else {
			fmt.Println("Cannot publish unapproved content!")
		}
		return types.NodeResponse[ArticleState]{State: s, Status: types.StatusCompleted}, nil
	}, nil)
}

// We'll build a small sub-flow that does 2 steps:
//
//	subAgent1 -> subAgent2 -> end
//	(Pretend it's some complex check or multi-step approval process)
func buildValidationSubWorkflow() *workflow.Builder[ArticleState] {
	sw := workflow.NewBuilder[ArticleState]("ValidationSubFlow")

	step1 := agents.NewSimpleAgent("ValidateLength", func(_ context.Context, s ArticleState, _ types.Config[ArticleState]) (types.NodeResponse[ArticleState], error) {
		if len(s.Content) < 10 {
			s.Valid = false
		} else {
			s.Valid = true
		}
		return types.NodeResponse[ArticleState]{State: s, Status: types.StatusCompleted}, nil
	}, nil)

	step2 := agents.NewSimpleAgent("CheckApproval", func(_ context.Context, s ArticleState, _ types.Config[ArticleState]) (types.NodeResponse[ArticleState], error) {
		if s.Valid {
			s.Approved = true
		}
		return types.NodeResponse[ArticleState]{State: s, Status: types.StatusCompleted}, nil
	}, nil)

	err := sw.AddAgent(step1).
		AsEntryPoint().
		Then(step2).
		End()
	if err != nil {
		panic(fmt.Sprintf("Subflow build error: %v", err))
	}
	return sw
}

func main() {
	// Build the sub-flow (validation logic)
	validationSubFlow := buildValidationSubWorkflow()

	// Build the main flow
	mainFlow := workflow.NewBuilder[ArticleState]("MainFlowWithSub")
	writer := makeWriterAgent()
	publisher := makePublishAgent()

	err := mainFlow.AddAgent(writer).
		AsEntryPoint().
		// ThenSubWorkflow -> treat the entire validation sub-flow as one "agent"
		ThenSubWorkflow(validationSubFlow).
		Then(publisher).
		End()
	if err != nil {
		panic(fmt.Sprintf("Main flow build error: %v", err))
	}

	// Compile and run
	app, err := workflow.NewApp[ArticleState](mainFlow, workflow.WithDebug[ArticleState]())
	if err != nil {
		panic(fmt.Sprintf("Failed to create app: %v", err))
	}

	// Let's see how the sub-flow manipulates the state
	initState := ArticleState{Content: ""}
	final, err := app.Invoke(context.Background(), initState)
	if err != nil {
		fmt.Println("Run error:", err)
		return
	}
	fmt.Printf("\nFinal State => Content:%s, Valid:%t, Approved:%t\n", final.Content, final.Valid, final.Approved)
}
