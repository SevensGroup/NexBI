package semantic

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type metricYAML struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Formula     string   `yaml:"formula"`
	Field       string   `yaml:"field"`
	Aggregation string   `yaml:"aggregation"`
	Aliases     []string `yaml:"aliases"`
}

type dimensionYAML struct {
	Name        string              `yaml:"name"`
	Description string              `yaml:"description"`
	Field       string              `yaml:"field"`
	Type        string              `yaml:"type"`
	Entity      string              `yaml:"entity"`
	Aliases     []string            `yaml:"aliases"`
	Values      map[string][]string `yaml:"values"`
}

type entityYAML struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Key         string   `yaml:"key"`
	Dimensions  []string `yaml:"dimensions"`
}

type dataSourceYAML struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Index       string `yaml:"index"`
	DateField   string `yaml:"date_field"`
	Default     bool   `yaml:"default"`
}

type catalogYAML struct {
	Metrics     map[string]metricYAML     `yaml:"metrics"`
	Dimensions  map[string]dimensionYAML  `yaml:"dimensions"`
	Entities    map[string]entityYAML     `yaml:"entities"`
	DataSources map[string]dataSourceYAML `yaml:"datasources"`
}

// Load reads YAML files from dir into a SemanticRegistry.
func Load(dir string) (*Registry, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("semantic dir is required")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read semantic dir %s: %w", dir, err)
	}

	reg := NewRegistry()
	loaded := 0
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		path := filepath.Join(dir, name)
		if err := loadFile(reg, path); err != nil {
			return nil, err
		}
		loaded++
	}
	if loaded == 0 {
		return nil, fmt.Errorf("no semantic YAML files in %s", dir)
	}
	return reg, nil
}

func loadFile(reg *Registry, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	var wrapped catalogYAML
	if err := yaml.Unmarshal(data, &wrapped); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if wrapped.Metrics != nil || wrapped.Dimensions != nil || wrapped.Entities != nil || wrapped.DataSources != nil {
		return registerCatalog(reg, wrapped)
	}

	base := strings.ToLower(filepath.Base(path))
	switch {
	case strings.Contains(base, "metric"):
		var raw map[string]metricYAML
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		return registerMetrics(reg, raw)
	case strings.Contains(base, "dimension"):
		var raw map[string]dimensionYAML
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		return registerDimensions(reg, raw)
	case strings.Contains(base, "entit"):
		var raw map[string]entityYAML
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		return registerEntities(reg, raw)
	case strings.Contains(base, "datasource") || strings.Contains(base, "source"):
		var raw map[string]dataSourceYAML
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		return registerDataSources(reg, raw)
	default:
		return fmt.Errorf("unrecognized semantic YAML %s", path)
	}
}

func registerCatalog(reg *Registry, c catalogYAML) error {
	if err := registerMetrics(reg, c.Metrics); err != nil {
		return err
	}
	if err := registerDimensions(reg, c.Dimensions); err != nil {
		return err
	}
	if err := registerEntities(reg, c.Entities); err != nil {
		return err
	}
	return registerDataSources(reg, c.DataSources)
}

func registerMetrics(reg *Registry, raw map[string]metricYAML) error {
	for key, item := range raw {
		m := Metric{
			Key:         key,
			Name:        firstNonEmpty(item.Name, key),
			Description: item.Description,
			Field:       item.Field,
			Aggregation: item.Aggregation,
			Aliases:     item.Aliases,
		}
		if strings.TrimSpace(item.Formula) != "" {
			m.Formula = &Formula{Expression: item.Formula}
		}
		if err := reg.RegisterMetric(m); err != nil {
			return fmt.Errorf("register metric %s: %w", key, err)
		}
	}
	return nil
}

func registerDimensions(reg *Registry, raw map[string]dimensionYAML) error {
	for key, item := range raw {
		d := Dimension{
			Key:         key,
			Name:        firstNonEmpty(item.Name, key),
			Description: item.Description,
			Field:       item.Field,
			Type:        item.Type,
			Entity:      item.Entity,
			Aliases:     item.Aliases,
			Values:      item.Values,
		}
		if err := reg.RegisterDimension(d); err != nil {
			return fmt.Errorf("register dimension %s: %w", key, err)
		}
	}
	return nil
}

func registerEntities(reg *Registry, raw map[string]entityYAML) error {
	for key, item := range raw {
		e := Entity{
			Key:         firstNonEmpty(key, item.Name),
			Name:        firstNonEmpty(item.Name, key),
			Description: item.Description,
			KeyField:    item.Key,
			Dimensions:  item.Dimensions,
		}
		if err := reg.RegisterEntity(e); err != nil {
			return fmt.Errorf("register entity %s: %w", key, err)
		}
	}
	return nil
}

func registerDataSources(reg *Registry, raw map[string]dataSourceYAML) error {
	for key, item := range raw {
		s := DataSource{
			Key:         key,
			Name:        firstNonEmpty(item.Name, key),
			Description: item.Description,
			Index:       item.Index,
			DateField:   item.DateField,
			Default:     item.Default,
		}
		if err := reg.RegisterDataSource(s); err != nil {
			return fmt.Errorf("register data source %s: %w", key, err)
		}
	}
	return nil
}
