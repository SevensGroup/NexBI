package multi

import "errors"

var (
	// ErrUnknownAgent is returned when DELEGATE_AGENT names a role that is not on the team.
	ErrUnknownAgent = errors.New("unknown agent")
	// ErrInvalidDelegate is returned when agent or task is missing.
	ErrInvalidDelegate = errors.New("invalid DELEGATE_AGENT")
	// ErrNestedDelegate is returned when a specialist attempts DELEGATE_AGENT.
	ErrNestedDelegate = errors.New("specialist cannot DELEGATE_AGENT")
	// ErrNoRunner is returned when Agent-as-Tool has no spawn function.
	ErrNoRunner = errors.New("agent runner is not configured")
)
