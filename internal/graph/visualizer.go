package graph

import (
	"fmt"
	"strings"
)

// Info represents the graph structure for visualization
type Info struct {
	Nodes []string
	Edges []EdgeInfo
}

// EdgeInfo Info can stay the same but Edge info becomes simpler
type EdgeInfo struct {
	From     string
	To       string
	Type     string // "direct", "branch", or "conditional"
	Metadata map[string]any
}

func (g *Graph) GetGraphInfo() *Info {
	info := &Info{
		Nodes: make([]string, 0, len(g.nodes)),
	}

	// Add nodes
	for nodeName := range g.nodes {
		info.Nodes = append(info.Nodes, nodeName)
	}

	// Add regular edges
	for _, edge := range g.edges {
		info.Edges = append(info.Edges, EdgeInfo{
			From:     edge.From,
			To:       edge.To,
			Type:     "direct",
			Metadata: edge.Metadata,
		})
	}

	// Add branch edges - showing conditional routing
	for source, branches := range g.branches {
		// Find all possible targets from edges for this source
		targets := make([]string, 0)
		for _, edge := range g.edges {
			if edge.From == source {
				targets = append(targets, edge.To)
			}
		}

		// Add conditional routing edge
		if len(targets) > 0 {
			info.Edges = append(info.Edges, EdgeInfo{
				From:     source,
				To:       strings.Join(targets, ","),
				Type:     "conditional",
				Metadata: branches[0].Metadata, // Using first branch metadata
			})
		}

		// Add then edges if present
		for _, branch := range branches {
			if branch.Then != "" {
				info.Edges = append(info.Edges, EdgeInfo{
					From:     source,
					To:       branch.Then,
					Type:     "branch",
					Metadata: branch.Metadata,
				})
			}
		}
	}

	return info
}

func (g *Graph) PrintGraph() {
	info := g.GetGraphInfo()

	fmt.Println("Graph Structure:")
	fmt.Printf("Entry Point: %s\n\n", g.entryPoint)

	fmt.Println("Nodes:")
	for _, node := range info.Nodes {
		if node == g.entryPoint {
			fmt.Printf("  * %s (Entry)\n", node)
		} else {
			fmt.Printf("  - %s\n", node)
		}
	}

	fmt.Println("\nEdges:")
	for _, edge := range info.Edges {
		switch edge.Type {
		case "direct":
			fmt.Printf("  %s --> %s\n", edge.From, edge.To)
		case "conditional":
			fmt.Printf("  %s --[condition]--> [%s]\n", edge.From, edge.To)
		case "branch":
			fmt.Printf("  %s ==then==> %s\n", edge.From, edge.To)
		}
	}
}
