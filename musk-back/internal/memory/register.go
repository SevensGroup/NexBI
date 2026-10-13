package memory

import (
	"fmt"

	"autonomous-agent/internal/tools"
)

// RegisterTools adds search_memory and save_memory to the Tool Registry.
// The agent depends on the registry, never on this package's concrete types.
func RegisterTools(reg tools.Registry, store MemoryStore) error {
	if reg == nil {
		return fmt.Errorf("tool registry is required")
	}
	if store == nil {
		return fmt.Errorf("memory store is required")
	}
	if err := reg.Register(NewSearchTool(store), tools.RiskLow); err != nil {
		return err
	}
	if err := reg.Register(NewSaveTool(store), tools.RiskMedium); err != nil {
		return err
	}
	return nil
}
