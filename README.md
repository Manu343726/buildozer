# buildozer
A peer to peer distributed build system

## Logging

### Logging Standard: Component Logger Embedding Pattern

All components in buildozer follow a consistent logging pattern for cleaner, more maintainable code.

#### Pattern Overview

Every component (struct) embeds a `*logging.Logger` as an unnamed field. This enables:
- Direct logging method calls via method promotion: `c.Debug("message")` instead of `c.logger.Debug("message")`
- Hierarchical component tracking in logs (e.g., daemon → httpServer)
- Consistent error handling pattern
- Clear ownership of logger with the component

#### Implementation Steps

1. **Embed the logger as unnamed field:**
   ```go
   type MyComponent struct {
       *logging.Logger  // unnamed for method promotion
       config ConfigType
       // ... other fields ...
   }
   ```

2. **Initialize in constructor with component name:**
   ```go
   func NewMyComponent(cfg ConfigType) *MyComponent {
       return &MyComponent{
           Logger: daemon.Log().Child("MyComponent"),  // hierarchical
           config: cfg,
       }
   }
   ```
   The component logger is a child of the daemon logger, creating a parent-child hierarchy.

3. **Log entry points with Debug and structured attributes:**
   ```go
   c.Debug("starting operation", "foo", "bar", "retries", 3)
   ```
   Use Debug at operation entry points. Include relevant context as key-value pairs.

4. **Use `Errorf()` for error handling (returns error AND logs):**
   ```go
   if err != nil {
       return c.Errorf("failed to connect: %w", err)
   }
   ```
   `Errorf()` both logs the error and returns it — eliminates the need for separate log + return statements.

5. **Log successful completions:**
   ```go
   c.Debug("operation completed", "duration_ms", elapsed)
   ```

#### Example: Complete Component

```go
package daemon

type httpServer struct {
    *logging.Logger  // unnamed embedded
    config DaemonConfig
    listener net.Listener
}

func newHTTPServer(cfg DaemonConfig) (*httpServer, error) {
    hs := &httpServer{
        Logger: Log().Child("httpServer"),  // parent is daemon logger
        config: cfg,
    }
    
    hs.Debug("starting HTTP server", "port", cfg.Port)
    
    listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
    if err != nil {
        return nil, hs.Errorf("failed to listen: %w", err)  // logs + returns error
    }
    
    hs.listener = listener
    hs.Debug("HTTP server ready", "addr", listener.Addr().String())
    return hs, nil
}

func (hs *httpServer) Shutdown(ctx context.Context) error {
    hs.Debug("shutting down HTTP server")
    err := hs.listener.Close()
    if err != nil {
        return hs.Errorf("listener close failed: %w", err)
    }
    hs.Debug("HTTP server shut down")
    return nil
}
```

#### Key Methods

- **`Debug(msg, attr1, val1, attr2, val2, ...)`** — Log entry points and completion with structured attributes
- **`Info(msg, attr1, val1, ...)`** — Log less frequent events
- **`Warn(msg, attr1, val1, ...)`** — Log warnings
- **`Errorf(format, args...)`** — Log error AND return it (supports `%w` for error wrapping)
- **`Fatal(msg, attr1, val1, ...)`** — Log and exit immediately

#### Logging Hierarchy

Each component logs as a child of its parent:
```
root
├── daemon
│   ├── httpServer
│   └── scheduler
├── client
│   ├── jobExecutor
│   └── cacheManager
└── runtime
    ├── executor
    └── detector
```

This hierarchy enables filtering logs by component, such as:
```bash
buildozer-client logs --component daemon.httpServer  # Only httpServer logs
buildozer-client logs --component daemon.*           # All daemon sub-components
buildozer-client logs --level error                  # Only errors across system
```

#### Why This Pattern

1. **Reduced boilerplate:** No `c.logger.Method()` prefix everywhere
2. **Automatic context tracking:** Parent-child hierarchy visible in logs
3. **Consistent error handling:** All errors logged and returned uniformly
4. **Remote observability:** Hierarchical structure enables querying by component
5. **Easy testing:** Components with embedded loggers easier to mock/test
6. **Method promotion:** Go's unnamed embedding reduces verbosity
