package main

import (
	"context"
	"fmt"

	"github.com/avi3tal/orchestrai/pkg/agents"
	"github.com/avi3tal/orchestrai/pkg/types"
	"github.com/avi3tal/orchestrai/pkg/workflow"
)

// ReviewState Our custom state
type ReviewState struct {
	Content     string
	NeedsReview bool
	Approved    bool
}

func (rs ReviewState) Validate() error { return nil }
func (rs ReviewState) Merge(other ReviewState) ReviewState {
	if other.Content != "" {
		rs.Content = other.Content
	}
	rs.NeedsReview = rs.NeedsReview || other.NeedsReview
	rs.Approved = rs.Approved || other.Approved
	return rs
}

func makeWriteAgent() workflow.Agent[ReviewState] {
	counter := 0
	return agents.NewSimpleAgent[ReviewState]("Writer", func(_ context.Context, s ReviewState, _ types.Config[ReviewState]) (types.NodeResponse[ReviewState], error) {
		s.Content = "Draft Content"
		s.NeedsReview = true
		if counter > 3 {
			s.Content = "Official Content with more than 10 characters"
			s.NeedsReview = false
		}
		counter++
		return types.NodeResponse[ReviewState]{State: s, Status: types.StatusCompleted}, nil
	}, nil)
}

func makeReviewerAgent() workflow.Agent[ReviewState] {
	return agents.NewSimpleAgent[ReviewState]("Reviewer", func(_ context.Context, s ReviewState, _ types.Config[ReviewState]) (types.NodeResponse[ReviewState], error) {
		if len(s.Content) < 10 {
			// needs more writing
			s.NeedsReview = true
			s.Approved = false
		} else {
			s.NeedsReview = false
			s.Approved = true
		}
		return types.NodeResponse[ReviewState]{State: s, Status: types.StatusCompleted}, nil
	}, nil)
}

func makePublishAgent() workflow.Agent[ReviewState] {
	return agents.NewSimpleAgent[ReviewState]("Publisher", func(_ context.Context, s ReviewState, _ types.Config[ReviewState]) (types.NodeResponse[ReviewState], error) {
		fmt.Println("Publishing content:", s.Content)
		return types.NodeResponse[ReviewState]{State: s, Status: types.StatusCompleted}, nil
	}, nil)
}

func buildReviewWorkflow() (*workflow.Builder[ReviewState], error) {
	wf := workflow.NewBuilder[ReviewState]("review-flow")

	writer := makeWriteAgent()
	reviewer := makeReviewerAgent()
	publisher := makePublishAgent()

	err := wf.AddAgent(writer).
		AsEntryPoint().
		Then(reviewer).
		ThenIf(
			func(_ context.Context, s ReviewState, _ types.Config[ReviewState]) bool {
				return !s.NeedsReview
			},
			publisher,
			writer,
		).
		// We return the same FlowAgent referencing "writer" node for chaining,
		// but we want to do one more Then(...) call to finalize. For simplicity,
		// let's just do End() to mark an explicit end. Or you could link more.
		End()
	if err != nil {
		fmt.Println("Error building flow:", err)
		return nil, err
	}

	return wf, nil
}

func main() {
	wf, err := buildReviewWorkflow()
	if err != nil {
		panic(fmt.Sprintf("Failed building workflow: %v", err))
	}
	// Create the app with optional config (debug, store, etc.)
	app, err := workflow.NewApp[ReviewState](wf,
		workflow.WithDebug[ReviewState](),
		// workflow.WithCheckpointStore(myStore)  // if you want
	)
	if err != nil {
		panic(fmt.Sprintf("Failed creating app: %v", err))
	}

	// Finally, invoke with optional listener & callback
	initState := ReviewState{Content: "", NeedsReview: false}
	result, err := app.Invoke(context.Background(), initState)
	if err != nil {
		fmt.Println("Invoke failed:", err)
		return
	}
	fmt.Println("Result from app.Invoke:", result)
}
