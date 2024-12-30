// graph_visualizer.go
package dag

import (
	"fmt"
	"strings"
)

// GraphInfo represents the graph structure for visualization
type GraphInfo struct {
	Nodes []string
	Edges []EdgeInfo
}

type EdgeInfo struct {
	From          string
	Targets       []string
	IsConditional bool
}

// GetGraphInfo returns a representation of the graph structure
func (g *Graph[T]) GetGraphInfo() *GraphInfo {
	info := &GraphInfo{
		Nodes: make([]string, 0, len(g.nodes)),
	}

	// Add nodes
	for nodeName := range g.nodes {
		info.Nodes = append(info.Nodes, nodeName)
	}

	// Add edges
	for _, edge := range g.edges {
		info.Edges = append(info.Edges, EdgeInfo{
			From:          edge.From,
			Targets:       edge.To.PossibleTargets,
			IsConditional: len(edge.To.PossibleTargets) > 1,
		})
	}

	return info
}

// PrintGraph prints a simple text representation of the graph
func (g *Graph[T]) PrintGraph() {
	info := g.GetGraphInfo()

	fmt.Println("Graph Structure:")
	fmt.Println("Entry Point:", g.entryPoint)
	fmt.Println("\nNodes:")
	for _, node := range info.Nodes {
		if node == g.entryPoint {
			fmt.Printf("  * %s (Entry)\n", node)
		} else {
			fmt.Printf("  - %s\n", node)
		}
	}

	fmt.Println("\nEdges:")
	for _, edge := range info.Edges {
		if edge.IsConditional {
			fmt.Printf("  %s --[conditional]--> %s\n", edge.From, strings.Join(edge.Targets, " or "))
		} else {
			fmt.Printf("  %s --> %s\n", edge.From, edge.Targets[0])
		}
	}
}
