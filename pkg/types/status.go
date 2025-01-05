package types

// NodeExecutionStatus represents the current state of node execution
type NodeExecutionStatus string

const (
	StatusCompleted NodeExecutionStatus = "completed"
	StatusPending   NodeExecutionStatus = "pending" // Waiting for user input
	StatusReady     NodeExecutionStatus = "ready"   // Ready to execute
	StatusFailed    NodeExecutionStatus = "failed"
)
