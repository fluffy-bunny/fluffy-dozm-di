# Lookup Keys

Lookup keys let you register multiple instances of the same service type and resolve a specific one by string key. They also support metadata.

## Registration

Three keyed variants exist, mirroring the lifetimes:

- `di.AddSingletonWithLookupKeys[T]`
- `di.AddScopedWithLookupKeys[T]`
- `di.AddTransientWithLookupKeys[T]`

Signature: `(builder, ctor, []string{keys...}, map[string]any{metadata}, ...implementedInterfaceTypes)`.

```go
b := di.Builder()

di.AddSingletonWithLookupKeys[*Config](b,
    func() *Config { return loadDevConfig() },
    []string{"dev-config", "development"},
    map[string]interface{}{"env": "dev"},
)

di.AddSingletonWithLookupKeys[*Config](b,
    func() *Config { return loadProdConfig() },
    []string{"prod-config", "production"},
    map[string]interface{}{"env": "prod"},
)

c := b.Build()

dev  := di.GetByLookupKey[*Config](c, "dev-config")
prod := di.GetByLookupKey[*Config](c, "production")  // alternate key
```

Each key is hashed against the service type, so the same key string is fine to reuse across different `T`s.

## Resolution

```go
v := di.GetByLookupKey[T](container, key)         // panics if missing
v, err := di.TryGetByLookupKey[T](container, key) // returns error
```

Resolving without a key (`di.Get[T]`) returns the **last-registered** descriptor for that type.

Resolving `[]T` returns all registered instances regardless of key.

## With Interfaces

Combine keyed registration with `ImplementedInterfaceType` to expose the same instance via an interface:

```go
di.AddSingletonWithLookupKeys[*StripeProcessor](b,
    func() *StripeProcessor { return &StripeProcessor{} },
    []string{"stripe"},
    nil,
    di.ImplementedInterfaceType[PaymentProcessor](),
)

p := di.GetByLookupKey[PaymentProcessor](container, "stripe")
```

## Use Cases

- **Environment selection**: dev / staging / prod configs
- **Feature flags**: register flag objects under `feature:foo`
- **Strategy selection**: payment providers, cache backends, storage drivers
- **Tenant or region routing**: keyed connection pools

## Anti-patterns

- Resolving keys from inside business logic (do it at the composition root and inject the chosen instance)
- Using lookup keys when an interface + single registration would do
- Letting the key string spread through the codebase — keep it in one config map
