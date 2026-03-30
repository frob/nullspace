# Configuration Specification

Configuration is a kernel primitive, not a module. It loads before any module initializes and provides per-request immutable snapshots.

## Sources and Precedence

```
1. Module defaults (compiled in)     — lowest priority
2. TOML config file                  — overrides defaults
3. Environment variables             — overrides TOML (for secrets, dev overrides)
```

No flags. Anything that would be a flag is an env var or config file entry.

## TOML Config File

Default location: `nullspace.toml` in the working directory.

```toml
[kernel]
name = "myapp"

[modules]
format.query_param = true
format.content_negotiate = true
format.route_override = false

[data.static]
dir = "./public"
enabled = true

[data.sql]
driver = "sqlite"
dsn = "./data.db"
enabled = true

[data.file]
dir = "./content"
format = "markdown"
enabled = true

[log]
level = "info"
format = "text"

[request]
addr = ":8080"
```

## Environment Variable Convention

Environment variables override TOML values using a prefix + path convention:

```
NULLSPACE_<SECTION>_<KEY>=value
```

Nested keys use underscores:
- `NULLSPACE_DATA_STATIC_DIR=./assets` overrides `[data.static] dir`
- `NULLSPACE_DATA_SQL_DSN=postgres://...` overrides `[data.sql] dsn`
- `NULLSPACE_LOG_LEVEL=debug` overrides `[log] level`
- `NULLSPACE_REQUEST_ADDR=:9090` overrides `[request] addr`

Case-insensitive matching. The prefix (`NULLSPACE`) is configurable at kernel construction.

## Module-Declared Config

Each module declares its config section, key prefix, and defaults:

```go
type ModuleConfig struct {
    Key     string      // TOML section path, e.g., "data.static"
    Default interface{} // Default config struct
}

func (m *StaticModule) Config() ModuleConfig {
    return ModuleConfig{
        Key:     "data.static",
        Default: StaticConfig{Dir: "./public", Enabled: true},
    }
}
```

The kernel:
1. Collects all module config declarations
2. Loads TOML file
3. Applies env var overrides
4. Unmarshals each section into the module's config struct
5. Passes the populated config struct back to the module during Init

## Config Lifecycle

### Startup
1. Kernel loads TOML file
2. Kernel applies env var overrides
3. Kernel resolves module enabled/disabled states
4. Kernel passes config sections to modules during Init

### Runtime Mutation
- The kernel holds a "live" config that can be mutated at runtime
- Mutations are thread-safe (sync.RWMutex or copy-on-write)
- Mutations take effect at the next request boundary, never mid-request

### Per-Request Snapshots
- When a request starts, the live config is snapshotted
- The snapshot is immutable and attached to the request context
- All hook resolution, module enablement, and format negotiation reads from the snapshot
- Two concurrent requests may run under different configurations

```
Request arrives
    -> RLock live config
    -> Copy to snapshot (immutable)
    -> RUnlock
    -> Attach snapshot to request context
    -> All processing uses snapshot
```

## Dependencies

- `pelletier/go-toml` for TOML parsing
- `os.Getenv` for environment variables
- No other external dependencies
