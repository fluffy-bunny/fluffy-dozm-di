# Factory Patterns

There are two distinct "factory" mechanisms:

1. **Constructor factories** — the function passed to `AddTransient` / `AddScoped` / `AddSingleton`. The container resolves its parameters automatically. Use this 95% of the time.
2. **Container factories** — explicit `di.Factory` registered via `AddTransientFactory` / `AddScopedFactory` / `AddSingletonFactory`. The function receives the `di.Container` directly. Use when you need conditional logic, dynamic key lookup, or to construct multiple things from one factory call.

## Constructor Factory (Default)

```go
di.AddSingleton[*ConnectionPool](b, func(cfg *Config) *ConnectionPool {
    return &ConnectionPool{
        Max: cfg.MaxConns,
        DSN: cfg.DSN,
    }
})
```

Parameters are resolved by the container. Prefer this whenever the inputs are themselves DI services.

## Container Factory

```go
type Factory func(Container) any  // from descriptor.go

di.AddSingletonFactory[*ComplexService](b, func(c di.Container) any {
    cfg := di.Get[*Config](c)
    log := di.Get[Logger](c)
    return &ComplexService{cfg: cfg, log: log, builtAt: time.Now()}
})

di.AddTransientFactory[*Job](b, func(c di.Container) any {
    return newJobFromContext(c)
})

di.AddScopedFactory[*RequestContext](b, func(c di.Container) any {
    return &RequestContext{ID: xid.New().String()}
})
```

Use when:
- The factory needs the container to make decisions (`if cfg.UseRedis ...`)
- The same factory function constructs different concrete types based on runtime state
- You need to call `di.GetByLookupKey` from inside the factory

Avoid when a constructor with parameters would be clearer. Container factories are an escape hatch, not the default.

## Pre-built Instances

For values that already exist (e.g. parsed config from `main`), use `AddInstance`:

```go
cfg := loadConfig()
di.AddInstance[*Config](b, cfg)

// Or expose the same instance via an interface
di.AddInstance[*MetricsClient](b, mc, di.ImplementedInterfaceType[Metrics]())
```

For keyed pre-built instances: `di.AddInstanceWithLookupKeys[T]`.

## Bridging Containers

To pull a service from one container into another (rare, but useful for plugins or test harnesses):

```go
di.AddSingletonFromContainer[Logger](targetBuilder, sourceContainer)
```

The target container will lazily call `Get[Logger]` on `sourceContainer` when needed.

## Anti-patterns

- Using a container factory just to avoid declaring constructor parameters
- Calling `Build()` more than once
- Constructing a service in `init()` and registering it with `AddInstance` when a constructor factory would defer the work to actual use
