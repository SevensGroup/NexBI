package analyst

import "errors"

var (
	// ErrEmptyGoal is returned when Analyze is called without a user goal.
	ErrEmptyGoal = errors.New("analyst goal is required")
	// ErrNoSemanticTool is returned when semantic_query is not registered.
	ErrNoSemanticTool = errors.New("semantic_query is required")
	// ErrNoDataSource is returned when Elasticsearch MCP / aggregate is not registered.
	ErrNoDataSource = errors.New("advertising data source is not configured: elasticsearch aggregate tool is not registered (set ELASTICSEARCH_MCP_URL)")
	// ErrInsufficientEvidence is returned when the loop stops without EvidenceSufficient.
	ErrInsufficientEvidence = errors.New("evidence is not sufficient")
	// ErrUncitedCause is returned when a root cause has no evidence.
	ErrUncitedCause = errors.New("root cause has no evidence")
)
