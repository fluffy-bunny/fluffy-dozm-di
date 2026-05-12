# Scope Management

Scopes provide instance isolation for `Scoped` services. Each scope has its own container view; resolving a scoped service from two different scopes yields two different instances.

## Creating a Scope

The container exposes a built-in `di.ScopeFactory` singleton. Resolve it once, then create scopes from it.

```go
c := b.Build()
sf := di.Get[di.ScopeFactory](c)

scope := sf.CreateScope()
defer scope.Dispose()

handler := di.Get[*RequestHandler](scope.Container())
```

> Resolve scoped services from `scope.Container()`, **never from the root container**. With `ValidateScopes: true`, doing so panics.

## HTTP Handler Pattern

```go
func handle(sf di.ScopeFactory, w http.ResponseWriter, r *http.Request) {
    scope := sf.CreateScope()
    defer scope.Dispose()

    h := di.Get[*RequestHandler](scope.Container())
    h.Serve(w, r)
}
```

Wire `sf` once at startup; never call `CreateScope` from inside business logic.

## Disposal

`scope.Dispose()` releases all scoped instances created within that scope. If a scoped service implements `di.Disposable` (`Dispose()`), its `Dispose` is called.

Always pair creation with `defer scope.Dispose()`. Forgetting to dispose leaks every scoped instance for that scope's lifetime.

## Lifetime Interactions

| Dependency | Allowed in Singleton | Allowed in Scoped | Allowed in Transient |
|------------|---------------------|-------------------|----------------------|
| Singleton  | yes                 | yes               | yes                  |
| Scoped     | **no**              | yes               | yes (within a scope) |
| Transient  | yes                 | yes               | yes                  |

When a singleton needs scoped data, inject `di.ScopeFactory` (or a small factory function) and create a scope on demand inside the singleton's method.

## Validation

Enable scope validation during development and tests:

```go
b.ConfigureOptions(func(o *di.Options) {
    o.ValidateScopes  = true
    o.ValidateOnBuild = true
})
```

`ValidateScopes` rejects singleton-from-scope leaks. `ValidateOnBuild` resolves every registered service at `Build()` time so wiring errors surface immediately.

## Common Mistakes

- Resolving from the root container after a scope is created (use `scope.Container()`)
- Storing a scoped service in a singleton field
- Forgetting `defer scope.Dispose()`
- Calling `Build()` more than once on the same builder
