package multi

import (
	"context"
	"strings"

	"autonomous-agent/internal/tools"
)

// FilterRegistry exposes only allowlisted tools. nil allow means no filtering.
// An empty allow list exposes nothing. Specialists get independent tool scope.
func FilterRegistry(inner tools.Registry, allow []string) tools.Registry {
	if inner == nil {
		return inner
	}
	if allow == nil {
		return inner
	}
	set := make(map[string]struct{}, len(allow))
	for _, name := range allow {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			set[name] = struct{}{}
		}
	}
	return &filterRegistry{inner: inner, allow: set}
}

type filterRegistry struct {
	inner tools.Registry
	allow map[string]struct{}
}

func (f *filterRegistry) allowed(name string) bool {
	if f == nil {
		return false
	}
	_, ok := f.allow[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

func (f *filterRegistry) Register(tools.Tool, tools.RiskLevel) error {
	return tools.ErrInvalidTool
}

func (f *filterRegistry) Get(name string) (tools.Tool, bool) {
	if f == nil || !f.allowed(name) {
		return nil, false
	}
	return f.inner.Get(name)
}

func (f *filterRegistry) List() []tools.Info {
	if f == nil || f.inner == nil {
		return nil
	}
	all := f.inner.List()
	out := make([]tools.Info, 0, len(f.allow))
	for _, info := range all {
		if f.allowed(info.Name) {
			out = append(out, info)
		}
	}
	return out
}

func (f *filterRegistry) Execute(ctx context.Context, name string, input map[string]any) (tools.ToolResult, error) {
	if f == nil || !f.allowed(name) {
		te := &tools.ToolError{
			Code:    tools.CodeNotFound,
			Tool:    strings.ToLower(strings.TrimSpace(name)),
			Message: "not in agent tool scope",
			Err:     tools.ErrNotFound,
		}
		return tools.ToolResult{
			Success: false,
			Error:   te.Error(),
			Metadata: map[string]any{
				"tool":       te.Tool,
				"error_code": te.Code,
			},
		}, te
	}
	return f.inner.Execute(ctx, name, input)
}
