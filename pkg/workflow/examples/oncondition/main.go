package main

import (
	"context"
	"fmt"

	"github.com/avi3tal/orchestrai/pkg/agents"
	"github.com/avi3tal/orchestrai/pkg/types"
	"github.com/avi3tal/orchestrai/pkg/workflow"
)

type AccountState struct {
	UserName string
	PlanType string // e.g. "free", "paid", "enterprise"
}

func (a AccountState) Validate() error { return nil }
func (a AccountState) Merge(other AccountState) AccountState {
	if other.UserName != "" {
		a.UserName = other.UserName
	}
	if other.PlanType != "" {
		a.PlanType = other.PlanType
	}
	return a
}

// Agents
func makeCheckUserAgent() workflow.Agent[AccountState] {
	return agents.NewSimpleAgent("CheckUser", func(_ context.Context, s AccountState, _ types.Config[AccountState]) (types.NodeResponse[AccountState], error) {
		fmt.Println("CheckUser: current plan is:", s.PlanType)
		return types.NodeResponse[AccountState]{State: s, Status: types.StatusCompleted}, nil
	}, nil)
}

func makeUpgradeFreeAgent() workflow.Agent[AccountState] {
	return agents.NewSimpleAgent("UpgradeFreeAgent", func(_ context.Context, s AccountState, _ types.Config[AccountState]) (types.NodeResponse[AccountState], error) {
		fmt.Println("Upgrading free user to paid plan.")
		s.PlanType = "paid"
		return types.NodeResponse[AccountState]{State: s, Status: types.StatusCompleted}, nil
	}, nil)
}

func makePaidWelcomeAgent() workflow.Agent[AccountState] {
	return agents.NewSimpleAgent("PaidWelcomeAgent", func(_ context.Context, s AccountState, _ types.Config[AccountState]) (types.NodeResponse[AccountState], error) {
		fmt.Println("Welcome to the paid plan, user:", s.UserName)
		return types.NodeResponse[AccountState]{State: s, Status: types.StatusCompleted}, nil
	}, nil)
}

func makeEnterpriseFlowAgent() workflow.Agent[AccountState] {
	return agents.NewSimpleAgent("EnterpriseFlow", func(_ context.Context, s AccountState, _ types.Config[AccountState]) (types.NodeResponse[AccountState], error) {
		fmt.Println("Proceed to enterprise-specific flow for:", s.UserName)
		return types.NodeResponse[AccountState]{State: s, Status: types.StatusCompleted}, nil
	}, nil)
}

func main() {
	wf := workflow.NewBuilder[AccountState]("OnCondition-Demo")

	checkUser := makeCheckUserAgent()
	upgradeAgent := makeUpgradeFreeAgent()
	paidWelcome := makePaidWelcomeAgent()
	enterpriseFlow := makeEnterpriseFlowAgent()

	err := wf.AddAgent(checkUser).
		AsEntryPoint().
		// OnCondition returns a branchKey => we map them to agents
		OnCondition(func(_ context.Context, s AccountState, _ types.Config[AccountState]) string {
			switch s.PlanType {
			case "free":
				return "upgrade" // route to UpgradeFreeAgent
			case "paid":
				return "paidWelcome" // route to PaidWelcomeAgent
			case "enterprise":
				return "enterprise" // route to EnterpriseFlow
			default:
				// unknown plan => go to END
				return "no_such_plan"
			}
		}, map[string]workflow.Agent[AccountState]{
			"upgrade":     upgradeAgent,
			"paidWelcome": paidWelcome,
			"enterprise":  enterpriseFlow,
		}).
		End() // This single End() will place edges from each branch to END, if you implemented that multi-branch logic
	if err != nil {
		panic(fmt.Sprintf("Error building flow: %v", err))
	}

	app, err := workflow.NewApp[AccountState](wf, workflow.WithDebug[AccountState]())
	if err != nil {
		panic(fmt.Sprintf("Error creating app: %v", err))
	}

	// Test run with a "free" user => should route to UpgradeFreeAgent
	initState := AccountState{UserName: "Alice", PlanType: "free"}
	fmt.Println("\n--- Invoking with 'free' plan ---")
	_, err = app.Invoke(context.Background(), initState)
	if err != nil {
		fmt.Println("Invoke error:", err)
	}

	// Test run with a "paid" user => should route to PaidWelcomeAgent
	initState2 := AccountState{UserName: "Bob", PlanType: "paid"}
	fmt.Println("\n--- Invoking with 'paid' plan ---")
	_, err = app.Invoke(context.Background(), initState2)
	if err != nil {
		fmt.Println("Invoke error:", err)
	}

	// Test run with an "enterprise" user => EnterpriseFlow
	initState3 := AccountState{UserName: "Eve", PlanType: "enterprise"}
	fmt.Println("\n--- Invoking with 'enterprise' plan ---")
	_, err = app.Invoke(context.Background(), initState3)
	if err != nil {
		fmt.Println("Invoke error:", err)
	}

	// Test run with unknown => flows to END directly
	initState4 := AccountState{UserName: "X", PlanType: "weird"}
	fmt.Println("\n--- Invoking with 'weird' plan ---")
	_, err = app.Invoke(context.Background(), initState4)
	if err != nil {
		fmt.Println("Invoke error:", err)
	}
}
