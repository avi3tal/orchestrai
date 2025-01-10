package workflow

import (
	"context"
	"fmt"

	"github.com/avi3tal/orchestrai/internal/graph"
	"github.com/avi3tal/orchestrai/pkg/state"
	"github.com/avi3tal/orchestrai/pkg/types"
)

// Builder is the top-level DSL object. Wraps an internal graph.
type Builder[T state.GraphState[T]] struct {
	name  string
	graph *graph.Graph[T]
}

// NewBuilder creates a new DSL workflow with an underlying graph.
func NewBuilder[T state.GraphState[T]](name string, opts ...graph.Option[T]) *Builder[T] {
	g := graph.NewGraph[T](name, opts...)
	return &Builder[T]{name: name, graph: g}
}

// Compile compiles the underlying graph using the internal engine.
func (wf *Builder[T]) Compile(opts ...graph.CompilationOption[T]) (*graph.CompiledGraph[T], error) {
	return wf.graph.Compile(opts...)
}

// AddAgent adds a new agent (node) to the workflow.
func (wf *Builder[T]) AddAgent(agent Agent[T]) *FlowAgent[T] {
	err := wf.graph.AddNode(agent.Name(), agent.Execute, agent.Metadata())
	if err != nil {
		return &FlowAgent[T]{wf: wf, agent: agent, err: fmt.Errorf("AddAgent(%q) failed: %w", agent.Name(), err)}
	}
	return &FlowAgent[T]{wf: wf, agent: agent, err: nil}
}

// FlowAgent references a node that was just added (an Agent).
type FlowAgent[T state.GraphState[T]] struct {
	wf    *Builder[T]
	agent Agent[T]
	err   error

	// Internal fields used for looping, if needed:
	loopPredicate func(context.Context, T, types.Config[T]) bool
	loopMode      bool

	// track possible branch targets from a ThenIf/OnCondition
	branchTargets []string
}

func (fa *FlowAgent[T]) Err() error {
	return fa.err
}

// AsEntryPoint marks the current agent as the graph’s entry point.
func (fa *FlowAgent[T]) AsEntryPoint() *FlowAgent[T] {
	if fa.err != nil {
		return fa
	}
	if err := fa.wf.graph.SetEntryPoint(fa.agent.Name()); err != nil {
		fa.err = fmt.Errorf("AsEntryPoint failed: %w", err)
	}
	return fa
}

// Then creates a simple sequential link from fa.agent -> nextAgent.
func (fa *FlowAgent[T]) Then(nextAgent Agent[T]) *FlowAgent[T] {
	if fa.err != nil {
		return fa
	}

	// Ensure next node is in the graph
	err := fa.wf.graph.AddNode(nextAgent.Name(), nextAgent.Execute, nextAgent.Metadata())
	if err != nil && !isDuplicateNodeError(err) {
		fa.err = err
		return fa
	}

	// If we're in loop mode, create a conditional that loops back
	if fa.loopMode && fa.loopPredicate != nil {
		localPredicate := fa.loopPredicate
		localAgent := fa.agent
		fa.loopMode = false
		fa.loopPredicate = nil

		possibleTargets := []string{localAgent.Name(), nextAgent.Name()}
		cond := func(ctx context.Context, s T, cfg types.Config[T]) string {
			if localPredicate(ctx, s, cfg) {
				return localAgent.Name()
			}
			return nextAgent.Name()
		}

		e := fa.wf.graph.AddConditionalEdge(localAgent.Name(), possibleTargets, cond, nil)
		if e != nil {
			fa.err = fmt.Errorf("Then(loopMode) failed: %w", e)
			return fa
		}
	} else {
		// Normal case: direct edge
		if e := fa.wf.graph.AddEdge(fa.agent.Name(), nextAgent.Name(), nil); e != nil {
			fa.err = e
			return fa
		}
	}

	// Return a new FlowAgent referencing nextAgent
	return &FlowAgent[T]{wf: fa.wf, agent: nextAgent, err: fa.err}
}

// End marks the current agent as pointing to the END node.
func (fa *FlowAgent[T]) End() error {
	if fa.err != nil {
		return fa.err
	}
	if len(fa.branchTargets) > 0 {
		// We just came from a ThenIf or OnCondition with multiple possible branches
		for _, targetName := range fa.branchTargets {
			e := fa.wf.graph.AddEdge(targetName, graph.END, nil)
			if e != nil {
				fa.err = fmt.Errorf("[End]: AddEdge(%q->END) failed: %w", targetName, e)
				return fa.err
			}
		}
		// We’ve handled all branches. Clear them out
		fa.branchTargets = nil

	} else {
		// Normal single chain scenario
		e := fa.wf.graph.AddEdge(fa.agent.Name(), graph.END, nil)
		if e != nil {
			fa.err = fmt.Errorf("[End]: AddEdge(%q->END) failed: %w", fa.agent.Name(), e)
		}
	}
	return fa.err
}

// ThenIf creates a 2-branch condition: if predicate => ifTrueAgent else ifFalseAgent.
func (fa *FlowAgent[T]) ThenIf(
	predicate func(ctx context.Context, s T, cfg types.Config[T]) bool,
	ifTrueAgent Agent[T],
	ifFalseAgent Agent[T],
) *FlowAgent[T] {
	if fa.err != nil {
		return fa
	}

	for _, ag := range []Agent[T]{ifTrueAgent, ifFalseAgent} {
		if !fa.wf.graph.HasNode(ag.Name()) {
			e := fa.wf.graph.AddNode(ag.Name(), ag.Execute, ag.Metadata())
			if e != nil && !isDuplicateNodeError(e) {
				fa.err = e
				return fa
			}
		}
	}

	possibleTargets := []string{ifTrueAgent.Name(), ifFalseAgent.Name()}
	cond := func(ctx context.Context, s T, cfg types.Config[T]) string {
		if predicate(ctx, s, cfg) {
			return ifTrueAgent.Name()
		}
		return ifFalseAgent.Name()
	}

	err := fa.wf.graph.AddConditionalEdge(fa.agent.Name(), possibleTargets, cond, nil)
	if err != nil {
		fa.err = fmt.Errorf("ThenIf failed: %w", err)
	}
	fa.branchTargets = possibleTargets

	return fa
}

// ThenAll spawns multiple agents in parallel (similar to a “fork”).
// We’ll return a ParallelBuilder that captures the parallel group
// for a subsequent .Join() call.
func (fa *FlowAgent[T]) ThenAll(parallelAgents ...Agent[T]) *ParallelBuilder[T] {
	pb := &ParallelBuilder[T]{
		wf:             fa.wf,
		precedingAgent: fa.agent,
		parallelAgents: parallelAgents,
		err:            fa.err,
	}
	return pb.init()
}

// ParallelBuilder holds references to the set of parallel agents.
type ParallelBuilder[T state.GraphState[T]] struct {
	wf             *Builder[T]
	precedingAgent Agent[T]
	parallelAgents []Agent[T]
	joinNodeName   string // internal name for the "join" node

	err error
}

// init wires up the edges from precedingAgent -> each parallel agent.
func (pb *ParallelBuilder[T]) init() *ParallelBuilder[T] {
	if pb.err != nil {
		return pb
	}
	for _, ag := range pb.parallelAgents {
		// Add node if not present
		err := pb.wf.graph.AddNode(ag.Name(), ag.Execute, ag.Metadata())
		if err != nil && !isDuplicateNodeError(err) {
			pb.err = fmt.Errorf("[ThenAll]: could not add node %q: %w", ag.Name(), err)
			return pb
		}
		// Add edge
		e := pb.wf.graph.AddEdge(pb.precedingAgent.Name(), ag.Name(), nil)
		if e != nil {
			pb.err = fmt.Errorf("[ThenAll]: addEdge failed: %w", e)
			return pb
		}
	}
	return pb
}

// Join sets up a “barrier” or “join node” that merges all parallel agents’ outputs.
// joinFunc can handle aggregated results, returning a single combined state (or pass-through).
// We’ll return a FlowAgent referencing the new “join node.”
func (pb *ParallelBuilder[T]) Join(
	joinFunc func(ctx context.Context, states []T, cfg types.Config[T]) (types.NodeResponse[T], error),
) *FlowAgent[T] {

	if pb.err != nil {
		return &FlowAgent[T]{wf: pb.wf, err: pb.err}
	}

	// Create a synthetic "join" agent name
	pb.joinNodeName = pb.precedingAgent.Name() + "_join"

	// We treat the "join" node as a single agent that “executes” once all branches are done.
	joinAgent := &joinAgent[T]{
		name:     pb.joinNodeName,
		joinFunc: joinFunc,
	}

	// Add the join node
	err := pb.wf.graph.AddNode(joinAgent.Name(), joinAgent.Execute, joinAgent.Metadata())
	if err != nil && !isDuplicateNodeError(err) {
		pb.err = fmt.Errorf("[Join]: AddNode failed: %w", err)
		return &FlowAgent[T]{wf: pb.wf, err: pb.err}
	}

	// Now link each parallel agent -> join node
	for _, ag := range pb.parallelAgents {
		e := pb.wf.graph.AddEdge(ag.Name(), joinAgent.Name(), nil)
		if e != nil {
			pb.err = fmt.Errorf("[Join]: addEdge(%s->%s) failed: %w", ag.Name(), joinAgent.Name(), e)
			return &FlowAgent[T]{wf: pb.wf, err: pb.err}
		}
	}

	return &FlowAgent[T]{
		wf:    pb.wf,
		agent: joinAgent, // from now on, we proceed from the join node
		err:   pb.err,
	}
}

// OnCondition a more flexible version of ThenIf, possibly returning multiple branches
// or referencing an LLM-based condition. For simplicity, we’ll do a 2-branch example.
func (fa *FlowAgent[T]) OnCondition(
	condition func(ctx context.Context, s T, cfg types.Config[T]) string, // returns branchKey
	branchMap map[string]Agent[T], // branchKey -> Agent
) *FlowAgent[T] {
	if fa.err != nil {
		return fa
	}

	// Ensure all branch agents exist
	var targets []string
	for _, ag := range branchMap {
		if !fa.wf.graph.HasNode(ag.Name()) {
			e := fa.wf.graph.AddNode(ag.Name(), ag.Execute, ag.Metadata())
			if e != nil && !isDuplicateNodeError(e) {
				fa.err = e
				return fa
			}
		}
		targets = append(targets, ag.Name())
	}

	// Add a conditional edge that picks the correct agent based on branchKey
	wrapperCond := func(ctx context.Context, s T, cfg types.Config[T]) string {
		key := condition(ctx, s, cfg)
		agent, ok := branchMap[key]
		if !ok {
			// fallback if the condition returns an unknown key
			return graph.END
		}
		return agent.Name()
	}

	err := fa.wf.graph.AddConditionalEdge(fa.agent.Name(), targets, wrapperCond, nil)
	if err != nil {
		fa.err = fmt.Errorf("OnCondition failed: %w", err)
	}

	fa.branchTargets = targets
	return fa
}

// ThenSubWorkflow treats a separate workflow Builder as a “sub-workflow agent.”
func (fa *FlowAgent[T]) ThenSubWorkflow(subWf *Builder[T]) *FlowAgent[T] {
	if fa.err != nil {
		return fa
	}

	// Idea: compile the sub-workflow into a "SubWorkflowAgent".
	// Or treat it as black-box if you have an existing way to do so.
	subAgent := &SubWorkflowAgent[T]{
		name: "subWorkflow:" + subWf.name,
		wf:   subWf,
	}

	// Make sure we add sub-agent as a node
	err := fa.wf.graph.AddNode(subAgent.Name(), subAgent.Execute, subAgent.Metadata())
	if err != nil && !isDuplicateNodeError(err) {
		fa.err = fmt.Errorf("ThenSubWorkflow: AddNode failed: %w", err)
		return fa
	}

	// Link current agent -> subAgent
	e := fa.wf.graph.AddEdge(fa.agent.Name(), subAgent.Name(), nil)
	if e != nil {
		fa.err = fmt.Errorf("ThenSubWorkflow: addEdge(%s->%s) failed: %w", fa.agent.Name(), subAgent.Name(), e)
		return fa
	}

	// Return a new FlowAgent referencing subAgent
	return &FlowAgent[T]{
		wf:    fa.wf,
		agent: subAgent,
		err:   fa.err,
	}
}

// isDuplicateNodeError is a small helper to detect “already exists” errors.
func isDuplicateNodeError(err error) bool {
	// Implementation detail, or just always return false if you want
	return false
}

// ----------------------------------------------------------------------------
// Internal joinAgent that runs once parallel branches complete
// ----------------------------------------------------------------------------

type joinAgent[T state.GraphState[T]] struct {
	name     string
	joinFunc func(ctx context.Context, states []T, cfg types.Config[T]) (types.NodeResponse[T], error)
}

func (ja *joinAgent[T]) Name() string {
	return ja.name
}

func (ja *joinAgent[T]) Metadata() map[string]any {
	// Optionally store metadata about the join
	return map[string]any{"join": true}
}

// Execute is called once the engine sees all inbound edges are completed.
// The engine must gather the states from each inbound agent. Then pass them
// to joinFunc for merging. Pseudocode: you might store them in the checkpoint
// until all arrive.
func (ja *joinAgent[T]) Execute(ctx context.Context, s T, cfg types.Config[T]) (types.NodeResponse[T], error) {
	// If your underlying engine merges parallel states automatically,
	// you can pull them from the checkpoint and pass them to joinFunc.

	// Minimal stub example:
	aggregatedStates := []T{s} // in reality, you'd gather from each inbound edge
	return ja.joinFunc(ctx, aggregatedStates, cfg)
}

// ----------------------------------------------------------------------------
// Example SubWorkflowAgent wrapper
// ----------------------------------------------------------------------------

// SubWorkflowAgent treats an entire sub-workflow as a single "agent."
type SubWorkflowAgent[T state.GraphState[T]] struct {
	name string
	wf   *Builder[T] // the sub-workflow builder
}

func (sw *SubWorkflowAgent[T]) Name() string {
	return sw.name
}

func (sw *SubWorkflowAgent[T]) Metadata() map[string]any {
	return map[string]any{"subWorkflow": sw.wf.name}
}

// Execute runs the sub-workflow internally. One approach is to compile it on the fly
// and execute. You might keep a compiled version in memory, etc.
func (sw *SubWorkflowAgent[T]) Execute(ctx context.Context, s T, cfg types.Config[T]) (types.NodeResponse[T], error) {
	compiledSub, err := sw.wf.Compile()
	if err != nil {
		return types.NodeResponse[T]{State: s, Status: types.StatusFailed}, err
	}

	outState, execErr := compiledSub.Run(ctx, s)
	if execErr != nil {
		return types.NodeResponse[T]{State: outState, Status: types.StatusFailed}, execErr
	}
	// Return success with final sub-workflow state
	return types.NodeResponse[T]{State: outState, Status: types.StatusCompleted}, nil
}
