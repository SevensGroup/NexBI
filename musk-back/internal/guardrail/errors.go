package guardrail

import "errors"

var (
	// ErrInvalidApproval is returned when an approval request is malformed.
	ErrInvalidApproval = errors.New("invalid approval")
	// ErrNotFound is returned when an approval or checkpoint does not exist.
	ErrNotFound = errors.New("not found")
	// ErrNotPending is returned when Approve/Reject/Expire is called on a decided request.
	ErrNotPending = errors.New("approval is not pending")
	// ErrExpired is returned when an approval TTL has elapsed.
	ErrExpired = errors.New("approval expired")
	// ErrRejected is returned when Resume is attempted on a rejected approval.
	ErrRejected = errors.New("approval rejected")
	// ErrWaitingApproval is the sentinel wrapped by WaitingError after interrupt.
	ErrWaitingApproval = errors.New("waiting for approval")
	// ErrNoCheckpoint is returned when Resume cannot find a saved run.
	ErrNoCheckpoint = errors.New("checkpoint not found")
	// ErrInvalidAction is returned when the risk engine is given an empty action.
	ErrInvalidAction = errors.New("invalid action")
)
