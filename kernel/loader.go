package kernel

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// loadTOMLFile reads and parses a TOML file into a nested map.
// Returns an empty map (not an error) if the file does not exist,
// so the framework works with zero configuration.
func loadTOMLFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var result map[string]any
	if err := toml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return result, nil
}

// applyEnvOverrides scans environment variables with the given prefix and
// applies them as overrides to the config map.
//
// Convention: NULLSPACE_LOG_LEVEL -> config path "log.level"
// The prefix is stripped, the remainder is lowercased, and underscores
// become dots to form the nested key path.
func applyEnvOverrides(values map[string]any, prefix string) {
	prefix = strings.ToUpper(prefix) + "_"

	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]

		upper := strings.ToUpper(key)
		if !strings.HasPrefix(upper, prefix) {
			continue
		}

		// NULLSPACE_LOG_LEVEL -> log.level
		path := strings.ToLower(upper[len(prefix):])
		path = strings.ReplaceAll(path, "_", ".")

		setNestedValue(values, path, val)
	}
}

// setNestedValue sets a value in a nested map at the given dot-separated path.
// Intermediate maps are created as needed.
func setNestedValue(m map[string]any, path string, val any) {
	parts := strings.Split(path, ".")
	current := m

	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = val
			return
		}
		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
}

// getNestedValue retrieves a value from a nested map at the given dot-separated path.
func getNestedValue(m map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var current any = m

	for _, part := range parts {
		cm, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = cm[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// mergeDefaults merges loaded config values on top of module default values.
// The loaded values take precedence; unspecified keys retain their defaults.
func mergeDefaults(defaults, loaded any) (any, error) {
	if loaded == nil {
		return defaults, nil
	}
	if defaults == nil {
		return loaded, nil
	}

	defaultMap, err := toMap(defaults)
	if err != nil {
		// Can't convert defaults to map — just use loaded values.
		return loaded, nil
	}

	loadedMap, ok := loaded.(map[string]any)
	if !ok {
		// Loaded value isn't a map — it overrides entirely.
		return loaded, nil
	}

	// Loaded values override defaults at the leaf level.
	for k, v := range loadedMap {
		defaultMap[k] = v
	}

	return defaultMap, nil
}

// toMap converts a value to map[string]any. If the value is already a map,
// it is returned directly. Otherwise, a JSON round-trip is used to convert
// structs to maps (using json struct tags for key names).
func toMap(v any) (map[string]any, error) {
	if m, ok := v.(map[string]any); ok {
		result := make(map[string]any, len(m))
		for k, val := range m {
			result[k] = val
		}
		return result, nil
	}

	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal to map: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("unmarshal to map: %w", err)
	}
	return result, nil
}

// decode converts a stored value (map[string]any or struct) into the target
// struct via a JSON round-trip. The target must be a pointer to a struct.
func decode(src, dst any) error {
	b, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("config decode marshal: %w", err)
	}
	if err := json.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("config decode unmarshal: %w", err)
	}
	return nil
}

// applyModulesSection reads the [modules] section from the loaded config
// and applies enabled/disabled states to the kernel's live config.
//
// TOML format:
//
//	[modules]
//	"data.static" = true
//	"format.query_param" = false
func applyModulesSection(raw map[string]any, cfg *Config) {
	modules, ok := raw["modules"]
	if !ok {
		return
	}

	modMap, ok := modules.(map[string]any)
	if !ok {
		return
	}

	for name, val := range modMap {
		if enabled, ok := val.(bool); ok {
			cfg.SetModuleEnabled(name, enabled)
		}
	}
}

// applyModuleConfigs merges loaded config sections with each module's defaults
// and stores the result in the kernel's live config.
func applyModuleConfigs(raw map[string]any, modules []Module, cfg *Config) error {
	for _, m := range modules {
		c, ok := m.(Configurable)
		if !ok {
			continue
		}

		mc := c.Config()
		loaded, _ := getNestedValue(raw, mc.Key)

		merged, err := mergeDefaults(mc.Default, loaded)
		if err != nil {
			return fmt.Errorf("merge config for %s: %w", mc.Key, err)
		}

		cfg.Set(mc.Key, merged)
	}
	return nil
}
