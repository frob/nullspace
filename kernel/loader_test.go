package kernel

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeTempTOML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "nullspace.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadTOMLFile(t *testing.T) {
	path := writeTempTOML(t, `
[log]
level = "debug"
format = "json"

[data.static]
dir = "./public"
`)

	raw, err := loadTOMLFile(path)
	if err != nil {
		t.Fatalf("loadTOMLFile: %v", err)
	}

	log, ok := raw["log"].(map[string]any)
	if !ok {
		t.Fatal("expected [log] section")
	}
	if log["level"] != "debug" {
		t.Fatalf("expected debug, got %v", log["level"])
	}
	if log["format"] != "json" {
		t.Fatalf("expected json, got %v", log["format"])
	}

	data, ok := raw["data"].(map[string]any)
	if !ok {
		t.Fatal("expected [data] section")
	}
	static, ok := data["static"].(map[string]any)
	if !ok {
		t.Fatal("expected [data.static] section")
	}
	if static["dir"] != "./public" {
		t.Fatalf("expected ./public, got %v", static["dir"])
	}
}

func TestLoadTOMLFileMissing(t *testing.T) {
	raw, err := loadTOMLFile("/nonexistent/nullspace.toml")
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(raw) != 0 {
		t.Fatalf("expected empty map, got %v", raw)
	}
}

func TestLoadTOMLFileInvalid(t *testing.T) {
	path := writeTempTOML(t, `[invalid toml !!!`)

	_, err := loadTOMLFile(path)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	values := map[string]any{
		"log": map[string]any{
			"level": "info",
		},
	}

	t.Setenv("NULLSPACE_LOG_LEVEL", "debug")

	applyEnvOverrides(values, "NULLSPACE")

	log := values["log"].(map[string]any)
	if log["level"] != "debug" {
		t.Fatalf("expected env override to debug, got %v", log["level"])
	}
}

func TestApplyEnvOverridesCreatesNested(t *testing.T) {
	values := make(map[string]any)

	t.Setenv("NULLSPACE_DATA_STATIC_DIR", "./assets")

	applyEnvOverrides(values, "NULLSPACE")

	data, ok := values["data"].(map[string]any)
	if !ok {
		t.Fatal("expected data section created")
	}
	static, ok := data["static"].(map[string]any)
	if !ok {
		t.Fatal("expected data.static section created")
	}
	if static["dir"] != "./assets" {
		t.Fatalf("expected ./assets, got %v", static["dir"])
	}
}

func TestApplyEnvOverridesPrefixCaseInsensitive(t *testing.T) {
	values := map[string]any{
		"log": map[string]any{
			"level": "info",
		},
	}

	t.Setenv("NULLSPACE_LOG_LEVEL", "warn")

	applyEnvOverrides(values, "nullspace") // lowercase prefix
	log := values["log"].(map[string]any)
	if log["level"] != "warn" {
		t.Fatalf("expected warn, got %v", log["level"])
	}
}

func TestMergeDefaults(t *testing.T) {
	type cfg struct {
		Dir     string `json:"dir"`
		Enabled bool   `json:"enabled"`
	}

	defaults := cfg{Dir: "./public", Enabled: true}
	loaded := map[string]any{"dir": "./assets"}

	merged, err := mergeDefaults(defaults, loaded)
	if err != nil {
		t.Fatalf("mergeDefaults: %v", err)
	}

	m, ok := merged.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", merged)
	}

	if m["dir"] != "./assets" {
		t.Fatalf("expected loaded dir to win, got %v", m["dir"])
	}
	if m["enabled"] != true {
		t.Fatalf("expected default enabled to persist, got %v", m["enabled"])
	}
}

func TestMergeDefaultsNilLoaded(t *testing.T) {
	defaults := map[string]any{"key": "value"}
	merged, err := mergeDefaults(defaults, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := merged.(map[string]any)
	if m["key"] != "value" {
		t.Fatal("expected defaults when loaded is nil")
	}
}

func TestMergeDefaultsNilDefaults(t *testing.T) {
	loaded := map[string]any{"key": "value"}
	merged, err := mergeDefaults(nil, loaded)
	if err != nil {
		t.Fatal(err)
	}
	m := merged.(map[string]any)
	if m["key"] != "value" {
		t.Fatal("expected loaded when defaults is nil")
	}
}

func TestDecodeMapToStruct(t *testing.T) {
	type cfg struct {
		Dir    string `json:"dir"`
		Port   int    `json:"port"`
		Debug  bool   `json:"debug"`
	}

	src := map[string]any{
		"dir":   "./public",
		"port":  float64(8080), // JSON numbers are float64
		"debug": true,
	}

	var dst cfg
	if err := decode(src, &dst); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if dst.Dir != "./public" {
		t.Fatalf("expected ./public, got %s", dst.Dir)
	}
	if dst.Port != 8080 {
		t.Fatalf("expected 8080, got %d", dst.Port)
	}
	if !dst.Debug {
		t.Fatal("expected debug=true")
	}
}

func TestApplyModulesSection(t *testing.T) {
	raw := map[string]any{
		"modules": map[string]any{
			"data.static":     true,
			"format.query_param": false,
		},
	}

	cfg := NewConfig()
	applyModulesSection(raw, cfg)

	if !cfg.ModuleEnabled("data.static") {
		t.Fatal("expected data.static enabled")
	}
	if cfg.ModuleEnabled("format.query_param") {
		t.Fatal("expected format.query_param disabled")
	}
}

func TestConfigDecode(t *testing.T) {
	type logCfg struct {
		Level  string `json:"level"`
		Format string `json:"format"`
	}

	cfg := NewConfig()
	cfg.Set("log", map[string]any{"level": "debug", "format": "json"})

	var lc logCfg
	if err := cfg.Decode("log", &lc); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if lc.Level != "debug" || lc.Format != "json" {
		t.Fatalf("unexpected: %+v", lc)
	}
}

func TestSnapshotDecode(t *testing.T) {
	type logCfg struct {
		Level string `json:"level"`
	}

	cfg := NewConfig()
	cfg.Set("log", map[string]any{"level": "info"})
	snap := cfg.Snapshot()

	var lc logCfg
	if err := snap.Decode("log", &lc); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if lc.Level != "info" {
		t.Fatalf("expected info, got %s", lc.Level)
	}
}

func TestConfigDecodeMissing(t *testing.T) {
	cfg := NewConfig()
	var target struct{}
	if err := cfg.Decode("nonexistent", &target); err == nil {
		t.Fatal("expected error for missing section")
	}
}

// configModule is a test helper combining Module + Configurable.
type configModule struct {
	testModule
	modConfig ModuleConfig
}

func (m *configModule) Config() ModuleConfig {
	return m.modConfig
}

func TestKernelInitWithTOML(t *testing.T) {
	type staticCfg struct {
		Dir string `json:"dir"`
	}

	tomlContent := `
[modules]
"data.static" = true

[data.static]
dir = "./assets"
`
	path := writeTempTOML(t, tomlContent)

	k := New(WithConfigFile(path))

	m := &configModule{
		testModule: testModule{name: "data.static"},
		modConfig: ModuleConfig{
			Key:            "data.static",
			Default:        staticCfg{Dir: "./public"},
			DefaultEnabled: true,
		},
	}
	k.Use(m)

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Module should be initialized (enabled in config).
	if !m.initCalled {
		t.Fatal("expected module to be initialized")
	}

	// Config should have TOML values merged over defaults.
	var cfg staticCfg
	if err := k.Config().Decode("data.static", &cfg); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if cfg.Dir != "./assets" {
		t.Fatalf("expected TOML dir ./assets, got %s", cfg.Dir)
	}
}

func TestKernelInitWithEnvOverride(t *testing.T) {
	type staticCfg struct {
		Dir string `json:"dir"`
	}

	tomlContent := `
[data.static]
dir = "./public"
`
	path := writeTempTOML(t, tomlContent)
	t.Setenv("NULLSPACE_DATA_STATIC_DIR", "./from-env")

	k := New(WithConfigFile(path))

	m := &configModule{
		testModule: testModule{name: "data.static"},
		modConfig: ModuleConfig{
			Key:            "data.static",
			Default:        staticCfg{Dir: "./default"},
			DefaultEnabled: true,
		},
	}
	k.Use(m)

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	var cfg staticCfg
	if err := k.Config().Decode("data.static", &cfg); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if cfg.Dir != "./from-env" {
		t.Fatalf("expected env override ./from-env, got %s", cfg.Dir)
	}
}

func TestKernelInitDisabledByConfig(t *testing.T) {
	tomlContent := `
[modules]
"optional" = false
`
	path := writeTempTOML(t, tomlContent)

	k := New(WithConfigFile(path))

	m := &configModule{
		testModule: testModule{name: "optional"},
		modConfig: ModuleConfig{
			Key:            "optional",
			DefaultEnabled: true, // default enabled, but config disables it
		},
	}
	k.Use(m)

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if m.initCalled {
		t.Fatal("module should be disabled by config file")
	}
}

func TestKernelInitNoConfigFile(t *testing.T) {
	k := New(WithConfigFile("/nonexistent/nullspace.toml"))

	m := &testModule{name: "basic"}
	k.Use(m)

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		t.Fatalf("Init should succeed without config file: %v", err)
	}
	if !m.initCalled {
		t.Fatal("module should still be initialized")
	}
}
