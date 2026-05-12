# Constructor Injection

Constructor injection is the primary registration pattern. The constructor passed to `di.AddTransient` / `AddScoped` / `AddSingleton` declares its dependencies as function parameters; the container resolves them automatically when the service is requested.

## Basic Pattern

```go
type UserRepository struct{ db *DB }
type UserService struct{ repo *UserRepository }

b := di.Builder()

// Zero-dependency
di.AddSingleton[*DB](b, func() *DB { return openDB() })

// One dependency, auto-resolved
di.AddSingleton[*UserRepository](b, func(db *DB) *UserRepository {
    return &UserRepository{db: db}
})

// Multiple dependencies
di.AddTransient[*UserService](b, func(repo *UserRepository) *UserService {
    return &UserService{repo: repo}
})

c := b.Build()
svc := di.Get[*UserService](c)
```

## Resolution Process

1. `di.Get[T](container)` looks up the descriptor for `T`
2. The container inspects the constructor's parameter types via reflection
3. Each parameter is resolved recursively (subject to lifetime rules)
4. The constructor is invoked with the resolved arguments
5. The instance is returned (and cached if `Singleton` / `Scoped`)

## Constructor Signatures

The constructor must be a function. Its return type must be assignable to `T` (the registered service type). It may optionally return `(T, error)`:

```go
di.AddSingleton[*DB](b, func() (*DB, error) {
    return sql.Open("postgres", dsn)
})
```

If it returns an error, `Get[T]` panics; use `di.TryGet[T]` to receive the error.

## Multi-Level Graphs

The container resolves transitively, so you only ever ask for the leaf service:

```go
// Database -> Repository -> Service -> Controller
di.AddSingleton[*Database](b, newDatabase)
di.AddScoped[*UserRepository](b, func(db *Database) *UserRepository { ... })
di.AddTransient[*UserService](b, func(r *UserRepository) *UserService { ... })
di.AddTransient[*UserController](b, func(s *UserService) *UserController { ... })

ctrl := di.Get[*UserController](scope.Container())
// container builds the whole graph
```

## Slice Injection

If multiple services are registered for the same type `T`, `di.Get[[]T]` (or a `[]T` parameter on a constructor) returns all of them in registration order:

```go
di.AddSingleton[INotifier](b, func() INotifier { return &EmailNotifier{} })
di.AddSingleton[INotifier](b, func() INotifier { return &SMSNotifier{} })

di.AddSingleton[*Dispatcher](b, func(notifiers []INotifier) *Dispatcher {
    return &Dispatcher{notifiers: notifiers}
})
```

## Anti-patterns

- **Service locator inside business logic**: do not call `di.Get[T]` from inside a service. Inject what you need.
- **Reflection-based wiring**: there is no separate `WithFunc` / `ByInterface` API — just pass the constructor.
- **Circular dependencies**: refactor or break the cycle with an interface plus lazy lookup at the composition root.
