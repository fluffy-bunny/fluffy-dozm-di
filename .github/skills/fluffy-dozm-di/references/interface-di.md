# Interface-Based DI

Two ways to register a concrete type so it can be resolved by an interface.

## 1. Register `T` as the interface

The simplest pattern: set the type parameter `T` to the interface.

```go
type Logger interface{ Info(msg string) }

type fileLogger struct{ path string }
func (f *fileLogger) Info(msg string) { /* ... */ }

di.AddSingleton[Logger](b, func() Logger {
    return &fileLogger{path: "/var/log/app.log"}
})

log := di.Get[Logger](container)
```

Use this when only one interface registration is needed and you don't care about resolving the concrete type directly.

## 2. Register the concrete type with `ImplementedInterfaceType`

Register `T` as the concrete pointer, but tell the container which interfaces it implements. Both the concrete type and each interface become resolvable.

```go
di.AddSingleton[*fileLogger](b,
    func(cfg *Config) *fileLogger {
        return &fileLogger{path: cfg.LogPath}
    },
    di.ImplementedInterfaceType[Logger](),
    di.ImplementedInterfaceType[io.Closer](),
)

log := di.Get[Logger](container)       // works
fl  := di.Get[*fileLogger](container)  // also works
cl  := di.Get[io.Closer](container)    // also works
```

Use this when:
- A single concrete type satisfies multiple interfaces
- You sometimes need the concrete type (e.g. for tests or internal code)

## Multiple Implementations of One Interface

Register more than once. Resolving the interface returns the most-recently-registered one; resolving `[]Iface` returns all of them.

```go
di.AddSingleton[INotifier](b, func() INotifier { return &EmailNotifier{} })
di.AddSingleton[INotifier](b, func() INotifier { return &SMSNotifier{} })

last := di.Get[INotifier](container)        // SMSNotifier
all  := di.Get[[]INotifier](container)      // [Email, SMS]
```

For named selection between multiple implementations, see [lookup-keys.md](./lookup-keys.md).

## Rules

- Define the interface near the consumer (the caller), not next to the implementation
- Keep interfaces narrow — only the methods the consumer needs
- Inject the interface, not the concrete type, in production code
- In tests, register a fake implementation of the same interface
