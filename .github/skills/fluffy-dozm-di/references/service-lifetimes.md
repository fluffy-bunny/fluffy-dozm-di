# Service Lifetimes

`fluffy-dozm-di` supports three lifetimes. The lifetime is fixed at registration time.

| Lifetime | Function | When to use |
|----------|----------|-------------|
| Singleton | `di.AddSingleton[T]` | Config, loggers, DB pools, shared caches |
| Scoped | `di.AddScoped[T]` | Per-request state, transactions, unit-of-work |
| Transient | `di.AddTransient[T]` | Stateless lightweight services, validators |

## Registration

All three take the same shape: `(builder, ctor, ...implementedInterfaceTypes)`. The constructor's parameters are auto-resolved by the container.

```go
b := di.Builder()

// Singleton, no dependencies
di.AddSingleton[*Config](b, func() *Config { return loadConfig() })

// Scoped, with auto-resolved dependencies
di.AddScoped[*RequestHandler](b, func(c *Config, db *DB) *RequestHandler {
    return &RequestHandler{cfg: c, db: db}
})

// Transient
di.AddTransient[*Validator](b, func() *Validator { return &Validator{} })

container := b.Build()
```

## Singleton

- One instance for the lifetime of the container
- Constructor runs lazily on first resolution, then the instance is cached
- A singleton **cannot depend on a scoped service** when `ValidateScopes` is enabled — that would leak the scoped instance

## Scoped

- One instance per scope
- Created via `di.ScopeFactory.CreateScope()` — see [scope-management.md](./scope-management.md)
- Resolve scoped services from `scope.Container()`, not the root container
- Scopes implement `Dispose()`; always `defer scope.Dispose()`

## Transient

- A new instance every time the service is resolved
- Cheapest pattern to reason about; use it unless you need shared state
- Has no disposal hook

## Lifetime Conflicts

Registering the same service type with two different lifetimes panics on `Build()` (see `detectLifetimeConflicts` in `builder.go`). The container will not silently pick one.

## Validation Options

```go
b.ConfigureOptions(func(o *di.Options) {
    o.ValidateScopes = true     // panic if singleton depends on scoped
    o.ValidateOnBuild = true    // resolve all services at Build time
    o.DetectLifetimeConflicts = true
})
```

Enable both in tests and during development.
