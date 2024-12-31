package dag

import "time"

// NodeExecutionStatus represents the current state of node execution
type NodeExecutionStatus string

const (
	StatusCompleted NodeExecutionStatus = "completed"
	StatusPending   NodeExecutionStatus = "pending" // Waiting for user input
	StatusFailed    NodeExecutionStatus = "failed"
	StatusReady     NodeExecutionStatus = "ready" // Ready to execute
)

// NodeResponse encapsulates the execution result
type NodeResponse[T GraphState[T]] struct {
	State  T
	Status NodeExecutionStatus
}

// ExecutionContext contains session identifiers
type ExecutionContext struct {
	GraphID  string
	ThreadID string
	RunID    string
	NodeID   string
}

// ApprovalRequest tracks pending user approvals
type ApprovalRequest struct {
	ID        string
	Context   ExecutionContext
	Message   string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// ApprovalResponse captures user decision
type ApprovalResponse struct {
	ID       string
	Approved bool
	Metadata map[string]interface{} // Additional user input if needed
}
