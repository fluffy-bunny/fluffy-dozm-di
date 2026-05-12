---
applyTo: "**/*.go"
---

# Dependency Injection with fluffy-dozm-di

This repo uses [fluffy-dozm-di](https://github.com/fluffy-bunny/fluffy-dozm-di) for dependency injection. Not every Go file needs DI — small helpers, pure functions, and value types are fine as-is.

## When to Reach for DI

Strongly prefer DI whenever you see (or are about to write) any of the following:

- A struct with methods and a `NewMyStruct(...)` constructor that wires dependencies by hand
- Constructors being called inside other constructors (manual dependency graphs)
- Package-level singletons or `init()` side effects used to share state
- Tests that need to swap a dependency but can't, because it was constructed inline
- Anything that needs a different lifetime in production vs. tests (DB, clock, logger, HTTP client)

If a type is a stateless utility function or a plain data struct, leave it alone.

## Service Registration

The real API takes a constructor function directly — there is no separate `WithFunc` variant. The constructor's parameters are the dependencies and the container resolves them.

```go
b := di.Builder()

// Zero-dependency service
di.AddSingleton[*Config](b, func() *Config { return loadConfig() })

// Service with dependencies — params are auto-resolved
di.AddScoped[*UserService](b, func(cfg *Config, log Logger) *UserService {
    return &UserService{cfg: cfg, log: log}
})

// Register the concrete type as one or more interfaces
di.AddSingleton[*fileLogger](b,
    func(cfg *Config) *fileLogger { return &fileLogger{path: cfg.LogPath} },
    di.ImplementedInterfaceType[Logger](),
)

// Or: register directly by interface type as T
di.AddTransient[Validator](b, func() Validator { return &emailValidator{} })

c := b.Build()
```

Available registration funcs (from `builder.go`):

- `di.AddTransient[T]`, `di.AddScoped[T]`, `di.AddSingleton[T]` — primary registration; constructor + optional `implementedInterfaceTypes...`
- `di.AddTransientWithLookupKeys[T]`, `di.AddScopedWithLookupKeys[T]`, `di.AddSingletonWithLookupKeys[T]` — keyed/named registration with metadata
- `di.AddTransientFactory[T]`, `di.AddScopedFactory[T]`, `di.AddSingletonFactory[T]` — register via a `Factory` interface
- `di.AddInstance[T]` / `di.AddInstanceWithLookupKeys[T]` — register a pre-built instance
- `di.AddFunc[T]` — singleton convenience for function-typed services
- `di.AddTransientFromContainer[T]` / `AddScopedFromContainer` / `AddSingletonFromContainer` — bridge from another container

Rules:

- Use `AddSingleton` / `AddScoped` / `AddTransient` for everything normal — pass the constructor as the second arg
- To register a concrete type as an interface, either set `T` to the interface type, or list interface types via `di.ImplementedInterfaceType[Iface]()` after the constructor
- Do not register the same service type with multiple lifetimes — `Build()` will panic via `detectLifetimeConflicts`

## Lifetime Rules

| Lifetime | Use For |
|----------|---------|
| Singleton | Config, loggers, DB connection pools, shared caches |
| Scoped | Per-request state, DB transactions, unit-of-work objects |
| Transient | Stateless lightweight services, validators, transformers |

## Constructor Injection

- Declare all dependencies as constructor function parameters — the container resolves them automatically
- Resolve only at composition roots (main, HTTP handler entry, test setup) using `di.Get[T](container)` or `di.TryGet[T](container)`
- Never call `di.Get[T]` inside business logic — that is the service locator anti-pattern

## Scope Lifecycle

Scopes are created via the `ScopeFactory` service that the container exposes automatically:

```go
c := b.Build()
sf := di.Get[di.ScopeFactory](c)

scope := sf.CreateScope()
defer scope.Dispose()

svc := di.Get[*UserService](scope.Container())
```

- Always pair `sf.CreateScope()` with `defer scope.Dispose()`
- Resolve scoped services from `scope.Container()`, not from the root container
- Never store a scoped service in a struct that outlives the scope
- Never inject a scoped service into a singleton
- For services that require both scoped and singleton dependencies, split the service into separate components or use a factory pattern to manage the scoped dependencies
- For circular dependencies, use lazy initialization or refactor the design to remove the cycle

## Interface-First Design

- Define an interface before the implementation
- Inject the interface type, not the concrete struct
- Keep interfaces narrow — only the methods the consumer needs

## Testing

- Build the test container with `di.Builder()` then `b.Build()` — call `Build()` exactly once per builder
- Register mock implementations of interfaces in a test-only container (use `di.AddSingleton[Iface]` with a fake)
- For pre-built mock instances, use `di.AddInstance[Iface](b, mock)`
- Never patch global variables or use `init()` side effects to swap dependencies
