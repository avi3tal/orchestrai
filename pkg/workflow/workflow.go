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
// You can pass in graph.CompilationOptions (e.g. WithMaxSteps, WithCheckpointStore, etc.)
func (wf *Builder[T]) Compile(opts ...graph.CompilationOption[T]) (*graph.CompiledGraph[T], error) {
	return wf.graph.Compile(opts...)
}

// AddAgent adds a new agent (node) to the workflow.
// Returns a FlowAgent, which helps chain subsequent calls.
func (wf *Builder[T]) AddAgent(agent Agent[T]) *FlowAgent[T] {
	// Attempt to add the node
	err := wf.graph.AddNode(agent.Name(), agent.Execute, agent.Metadata())
	if err != nil {
		return &FlowAgent[T]{wf: wf, agent: agent, err: fmt.Errorf("AddAgent(%q) failed: %w", agent.Name(), err)}
	}
	return &FlowAgent[T]{wf: wf, agent: agent, err: nil}
}

// FlowAgent references a node that was just added (an Agent).
// Provides chainable methods like Then(), End(), or advanced branching.
type FlowAgent[T state.GraphState[T]] struct {
	wf    *Builder[T] // reference to the parent workflow
	agent Agent[T]    // reference to the current agent

	// err accumulates any error encountered in the chain.
	err error

	// For loop/branch building:
	loopPredicate func(context.Context, T, types.Config[T]) bool
	loopMode      bool
}

// Err returns the error state of the current agent in the chain.
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

// Then links the current agent to the next agent in the flow.
// Returns a new FlowAgent referencing the next agent.
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

	// If we are in loop mode, create a conditional edge with two targets:
	// 1) fa.agent (loop back) if predicate is true
	// 2) nextAgent (continue) if predicate is false
	if fa.loopMode && fa.loopPredicate != nil {
		localPredicate := fa.loopPredicate
		localAgent := fa.agent // keep a local reference to the "looping" agent
		fa.loopMode = false
		fa.loopPredicate = nil

		possibleTargets := []string{localAgent.Name(), nextAgent.Name()}
		cond := func(ctx context.Context, s T, cfg types.Config[T]) string {
			if localPredicate(ctx, s, cfg) {
				return localAgent.Name()
			}
			return nextAgent.Name()
		}

		e := fa.wf.graph.AddConditionalEdge(
			localAgent.Name(),
			possibleTargets,
			cond,
			nil, // metadata
		)
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
	return &FlowAgent[T]{
		wf:    fa.wf,
		agent: nextAgent,
		err:   fa.err,
	}
}

// End links the current agent to the END node, returning the final error status.
func (fa *FlowAgent[T]) End() error {
	if fa.err != nil {
		return fa.err
	}
	if e := fa.wf.graph.AddEdge(fa.agent.Name(), graph.END, nil); e != nil {
		fa.err = fmt.Errorf("end: AddEdge(%q->END) failed: %w", fa.agent.Name(), e)
	}
	return fa.err
}

// ThenIf ThenElse adds a binary router to the current agent.
func (fa *FlowAgent[T]) ThenIf(
	predicate func(ctx context.Context, s T, cfg types.Config[T]) bool,
	ifTrueAgent Agent[T],
	ifFalseAgent Agent[T],
) *FlowAgent[T] {
	if fa.err != nil {
		return fa
	}

	// ensure both agents exist
	for _, ag := range []Agent[T]{ifTrueAgent, ifFalseAgent} {
		if fa.wf.graph.HasNode(ag.Name()) {
			continue
		}
		e := fa.wf.graph.AddNode(ag.Name(), ag.Execute, ag.Metadata())
		if e != nil && !isDuplicateNodeError(e) {
			fa.err = e
			return fa
		}
	}

	possibleTargets := []string{ifTrueAgent.Name(), ifFalseAgent.Name()}
	cond := func(ctx context.Context, s T, cfg types.Config[T]) string {
		if predicate(ctx, s, cfg) {
			return ifTrueAgent.Name()
		}
		return ifFalseAgent.Name()
	}

	e := fa.wf.graph.AddConditionalEdge(fa.agent.Name(), possibleTargets, cond, nil)
	if e != nil {
		fa.err = fmt.Errorf("ThenElse failed: %w", e)
		return fa
	}
	// Return fa referencing the same node, so you can chain further from here
	return fa
}

// Example condition check to see if it’s “already exists” error (stub).
func isDuplicateNodeError(err error) bool {
	// you might parse the error string or check the actual type
	return false
}
