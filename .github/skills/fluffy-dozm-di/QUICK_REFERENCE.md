# Quick Reference - fluffy-dozm-di

## Table of Contents

1. [Service Registration](#service-registration)
2. [Service Lifetimes](#service-lifetimes)
3. [Service Resolution](#service-resolution)
4. [Constructor Injection](#constructor-injection)
5. [Interface Support](#interface-support)
6. [Lookup Keys](#lookup-keys)
7. [Scopes](#scopes)
8. [Factory Functions](#factory-functions)
9. [Function Injection](#function-injection)
10. [Container Injection](#container-injection)
11. [Validation](#validation)
12. [Common Patterns](#common-patterns)

---

## Service Registration

### Basic Registration

```go
// Create builder
builder := di.Builder()

// Register services
di.AddTransient[*MyService](builder, constructor)
di.AddScoped[*MyService](builder, constructor)
di.AddSingleton[*MyService](builder, constructor)

// Build container
container := builder.Build()
```

### With Implemented Interfaces

```go
di.AddSingleton[*MyImpl](builder,
    func() *MyImpl { return &MyImpl{} },
    di.ImplementedInterfaceType[IMyInterface]())
```

### With Lookup Keys

```go
di.AddSingletonWithLookupKeys[*Config](builder,
    func() *Config { return &Config{} },
    []string{"prod-config", "production"},
    map[string]interface{}{"env": "prod"})
```

---

## Service Lifetimes

### Transient

New instance every time:

```go
di.AddTransient[*Service](builder, func() *Service {
    return &Service{}
})
```

### Scoped

One instance per scope:

```go
di.AddScoped[*Service](builder, func() *Service {
    return &Service{}
})
```

### Singleton

One instance for the entire application:

```go
di.AddSingleton[*Service](builder, func() *Service {
    return &Service{}
})
```

---

## Service Resolution

### Get Service

```go
// Panics if not found
service := di.Get[*MyService](container)
```

### Try Get Service

```go
// Returns error if not found
service, err := di.TryGet[*MyService](container)
if err != nil {
    // Handle error
}
```

### Get by Lookup Key

```go
config := di.GetByLookupKey[*Config](container, "prod-config")
```

### Try Get by Lookup Key

```go
config, err := di.TryGetByLookupKey[*Config](container, "prod-config")
```

### Get All Implementations

```go
// Get all services implementing IService
services := di.Get[[]IService](container)
```

---

## Constructor Injection

The container automatically resolves dependencies:

```go
// Logger has no dependencies
di.AddSingleton[*Logger](builder, func() *Logger {
    return &Logger{}
})

// Repository depends on Logger
di.AddSingleton[*Repository](builder, func(logger *Logger) *Repository {
    return &Repository{Logger: logger}
})

// Service depends on Repository (which depends on Logger)
di.AddSingleton[*Service](builder, func(repo *Repository) *Service {
    return &Service{Repo: repo}
})

// Container resolves the entire dependency chain
service := di.Get[*Service](container)
```

---

## Interface Support

### Register by Interface

```go
type ILogger interface {
    Log(message string)
}

di.AddSingleton[ILogger](builder, func() ILogger {
    return &ConsoleLogger{}
})

logger := di.Get[ILogger](container)
```

### Multiple Interfaces

```go
type ILogger interface { Log(string) }
type INotifier interface { Notify(string) }

type Service struct{}
func (s *Service) Log(msg string) { /* ... */ }
func (s *Service) Notify(msg string) { /* ... */ }

typeILogger := di.ImplementedInterfaceType[ILogger]()
typeINotifier := di.ImplementedInterfaceType[INotifier]()

di.AddSingleton[*Service](builder,
    func() *Service { return &Service{} },
    typeILogger, typeINotifier)

// Can be retrieved as either interface
logger := di.Get[ILogger](container)
notifier := di.Get[INotifier](container)
```

### Multiple Implementations

```go
// Register multiple implementations
di.AddSingleton[INotifier](builder, func() INotifier {
    return &EmailNotifier{}
})

di.AddSingleton[INotifier](builder, func() INotifier {
    return &SMSNotifier{}
})

// Get all as slice
notifiers := di.Get[[]INotifier](container)
for _, n := range notifiers {
    n.Notify("message")
}
```

---

## Lookup Keys

### Register with Keys

```go
di.AddSingletonWithLookupKeys[*Config](builder,
    func() *Config {
        return &Config{Env: "dev"}
    },
    []string{"dev-config", "development"},
    map[string]interface{}{
        "environment": "development",
        "type": "config",
    })
```

### Retrieve by Key

```go
devConfig := di.GetByLookupKey[*Config](container, "dev-config")
// or
devConfig := di.GetByLookupKey[*Config](container, "development")
```

### Function with Lookup Keys

```go
type ValidatorFunc func(string) bool

di.AddFuncWithLookupKeys[ValidatorFunc](builder,
    func() ValidatorFunc {
        return func(s string) bool { return len(s) > 0 }
    },
    []string{"validator:not-empty"},
    nil)

validator := di.GetByLookupKey[ValidatorFunc](container, "validator:not-empty")
```

---

## Scopes

### Create and Use Scopes

```go
// Get scope factory (built-in service)
scopeFactory := di.Get[di.ScopeFactory](container)

// Create scope
scope := scopeFactory.CreateScope()
defer scope.Dispose() // Always dispose

// Get services from scope
service := di.Get[*MyService](scope.Container())
```

### Scope Isolation

```go
scope1 := scopeFactory.CreateScope()
scope2 := scopeFactory.CreateScope()

service1 := di.Get[*ScopedService](scope1.Container())
service2 := di.Get[*ScopedService](scope2.Container())

// Different instances in different scopes
fmt.Println(service1 != service2) // true

// Same instance within a scope
service1b := di.Get[*ScopedService](scope1.Container())
fmt.Println(service1 == service1b) // true
```

---

## Factory Functions

### Singleton Factory

```go
di.AddSingletonFactory[*Service](builder, func(c di.Container) any {
    // Access other services
    dep := di.Get[*Dependency](c)
    
    return &Service{
        Dep: dep,
        InitTime: time.Now(),
    }
})
```

### Scoped Factory

```go
di.AddScopedFactory[*Service](builder, func(c di.Container) any {
    return &Service{
        ID: generateID(),
    }
})
```

### Transient Factory

```go
di.AddTransientFactory[*Service](builder, func(c di.Container) any {
    return &Service{
        Timestamp: time.Now(),
    }
})
```

---

## Function Injection

### Basic Function Registration

```go
type GreetingFunc func(name string) string

// Register a function
di.AddFunc[GreetingFunc](builder, func() GreetingFunc {
    return func(name string) string {
        return fmt.Sprintf("Hello, %s!", name)
    }
})

// Use the function
greet := di.Get[GreetingFunc](container)
message := greet("World")
```

### Function with Dependencies

```go
type ProcessorFunc func(data string) string

// Function depends on Logger service
di.AddFunc[ProcessorFunc](builder, func(logger *Logger) ProcessorFunc {
    return func(data string) string {
        logger.Log("Processing: " + data)
        return strings.ToUpper(data)
    }
})
```

### Functions with Lookup Keys

```go
type ValidatorFunc func(input string) (bool, error)

// Register validator with lookup key
di.AddFuncWithLookupKeys[ValidatorFunc](builder,
    func() ValidatorFunc {
        return func(input string) (bool, error) {
            if len(input) < 8 {
                return false, fmt.Errorf("too short")
            }
            return true, nil
        }
    },
    []string{"validator:min-length"},
    nil)

// Retrieve by key
validator := di.GetByLookupKey[ValidatorFunc](container, "validator:min-length")
valid, err := validator("test")
```

### Strategy Pattern with Functions

```go
type CalculationFunc func(a, b float64) float64

// Register multiple strategies
di.AddFuncWithLookupKeys[CalculationFunc](builder,
    func() CalculationFunc {
        return func(a, b float64) float64 { return a + b }
    },
    []string{"calc:add"},
    nil)

di.AddFuncWithLookupKeys[CalculationFunc](builder,
    func() CalculationFunc {
        return func(a, b float64) float64 { return a * b }
    },
    []string{"calc:multiply"},
    nil)

// Use specific strategy
add := di.GetByLookupKey[CalculationFunc](container, "calc:add")
result := add(10, 5) // 15
```

### Pipeline/Transform Functions

```go
type TransformFunc func(input string) string

// Register multiple transformers
di.AddFuncWithLookupKeys[TransformFunc](builder,
    func() TransformFunc {
        return func(s string) string { return strings.ToUpper(s) }
    },
    []string{"transform:upper"},
    nil)

// Build transformation pipeline
trim := di.GetByLookupKey[TransformFunc](container, "transform:trim")
upper := di.GetByLookupKey[TransformFunc](container, "transform:upper")

result := upper(trim("  hello  ")) // "HELLO"
```

### Middleware Pattern

```go
type MiddlewareFunc func(next ProcessorFunc) ProcessorFunc

di.AddFuncWithLookupKeys[MiddlewareFunc](builder,
    func(logger *Logger) MiddlewareFunc {
        return func(next ProcessorFunc) ProcessorFunc {
            return func(data string) string {
                logger.Log("Before")
                result := next(data)
                logger.Log("After")
                return result
            }
        }
    },
    []string{"middleware:logging"},
    nil)

// Apply middleware
baseProcessor := func(s string) string { return strings.ToUpper(s) }
logging := di.GetByLookupKey[MiddlewareFunc](container, "middleware:logging")
enhanced := logging(baseProcessor)
```

---

## Container Injection

### Basic Container Injection

The DI container itself can be injected as a dependency:

```go
type ServiceLocator struct {
    Container di.Container
}

// Register service with container dependency
di.AddSingleton[*ServiceLocator](builder, func(container di.Container) *ServiceLocator {
    return &ServiceLocator{Container: container}
})

// Use the injected container
locator := di.Get[*ServiceLocator](container)
service, _ := locator.Container.Get(someType)
```

### Dynamic Resolution

```go
type DynamicResolver struct {
    Container di.Container
}

func (d *DynamicResolver) ResolveByName(name string) interface{} {
    switch name {
    case "config":
        return di.Get[*Config](d.Container)
    case "database":
        return di.Get[*Database](d.Container)
    }
    return nil
}

di.AddSingleton[*DynamicResolver](builder, func(c di.Container) *DynamicResolver {
    return &DynamicResolver{Container: c}
})
```

### Lazy Initialization

```go
type LazyService struct {
    Container di.Container
    database  *Database
}

func (l *LazyService) GetDatabase() *Database {
    if l.database == nil {
        // Lazy load on first access
        l.database = di.Get[*Database](l.Container)
    }
    return l.database
}

di.AddSingleton[*LazyService](builder, func(c di.Container) *LazyService {
    return &LazyService{Container: c}
})
```

### Plugin System

```go
type PluginManager struct {
    Container di.Container
}

func (p *PluginManager) LoadPlugin(name string) interface{} {
    switch name {
    case "email":
        return di.Get[*EmailPlugin](p.Container)
    case "sms":
        return di.Get[*SMSPlugin](p.Container)
    }
    return nil
}

di.AddSingleton[*PluginManager](builder, func(c di.Container) *PluginManager {
    return &PluginManager{Container: c}
})
```

### Factory Pattern

```go
type EntityFactory struct {
    Container di.Container
}

func (e *EntityFactory) CreateUser(name string) *User {
    // Resolve dependencies dynamically
    logger := di.Get[*Logger](e.Container)
    config := di.Get[*Config](e.Container)
    
    return &User{
        Name:   name,
        Logger: logger,
        Config: config,
    }
}

di.AddSingleton[*EntityFactory](builder, func(c di.Container) *EntityFactory {
    return &EntityFactory{Container: c}
})
```

### Service Validation

```go
type ValidatingService struct {
    Container di.Container
}

di.AddSingleton[*ValidatingService](builder, func(c di.Container) *ValidatingService {
    // Validate required services are registered
    isService := di.Get[di.IsService](c)
    
    if !isService.IsService(reflectx.TypeOf[*Config]()) {
        panic("Config service not registered")
    }
    
    return &ValidatingService{Container: c}
})
```

**⚠️ Important Notes:**
- Container injection enables powerful patterns but can hide dependencies
- Prefer explicit constructor injection when possible
- Use container injection for: dynamic resolution, lazy loading, plugin systems, factories
- Avoid using container as a service locator anti-pattern

---

## Validation

### Enable Validation

```go
builder.ConfigureOptions(func(o *di.Options) {
    o.ValidateScopes = true      // Prevent scoped service from root
    o.ValidateOnBuild = true     // Validate at build time
})
```

### Check if Service is Registered

```go
isService := di.Get[di.IsService](container)

if isService.IsService(reflectx.TypeOf[*MyService]()) {
    // Service is registered
}
```

---

## Common Patterns

### HTTP Request Handler

```go
func HandleRequest(w http.ResponseWriter, r *http.Request) {
    // Create scope for this request
    scope := scopeFactory.CreateScope()
    defer scope.Dispose()
    
    // Get request-scoped services
    handler := di.Get[*RequestHandler](scope.Container())
    handler.Process(r)
}
```

### Strategy Pattern

```go
type Strategy interface {
    Execute(data string) string
}

// Register multiple strategies
di.AddTransient[Strategy](builder, func() Strategy {
    return &StrategyA{}
})
di.AddTransient[Strategy](builder, func() Strategy {
    return &StrategyB{}
})

// Get all strategies
strategies := di.Get[[]Strategy](container)
```

### Decorator Pattern

```go
di.AddSingleton[*BaseService](builder, func() *BaseService {
    return &BaseService{}
})

di.AddSingleton[IService](builder, func(base *BaseService) IService {
    var service IService = base
    service = &LoggingDecorator{Inner: service}
    service = &CachingDecorator{Inner: service}
    return service
})
```

### Configuration per Environment

```go
di.AddSingletonWithLookupKeys[*Config](builder,
    func() *Config { return &Config{Env: "dev"} },
    []string{"dev-config"},
    nil)

di.AddSingletonWithLookupKeys[*Config](builder,
    func() *Config { return &Config{Env: "prod"} },
    []string{"prod-config"},
    nil)

// Load based on environment
env := os.Getenv("ENVIRONMENT")
config := di.GetByLookupKey[*Config](container, env+"-config")
```

---

## Full Example

```go
package main

import (
    "fmt"
    di "github.com/fluffy-bunny/fluffy-dozm-di"
)

type ILogger interface {
    Log(message string)
}

type Logger struct{}

func (l *Logger) Log(message string) {
    fmt.Println("LOG:", message)
}

type Service struct {
    Logger ILogger
}

func main() {
    // Build container
    builder := di.Builder()
    
    di.AddSingleton[ILogger](builder, func() ILogger {
        return &Logger{}
    })
    
    di.AddSingleton[*Service](builder, func(logger ILogger) *Service {
        return &Service{Logger: logger}
    })
    
    container := builder.Build()
    
    // Use service
    service := di.Get[*Service](container)
    service.Logger.Log("Hello, DI!")
}
```

---

## Tips

1. **Always dispose scopes**: Use `defer scope.Dispose()`
2. **Validate early**: Enable `ValidateOnBuild` during development
3. **Use interfaces**: Register services by interface for flexibility
4. **Avoid root container for scoped services**: Always use scopes
5. **Constructor injection**: Let the container resolve dependencies automatically
6. **Lookup keys**: Use for environment-specific configurations
7. **Factory functions**: Use when you need access to the container during construction

---

For more details, see:

- [README.md](README.md) - Comprehensive guide
- [SETUP.md](SETUP.md) - Installation and setup
- [examples/](examples/) - Working code examples
- [GitHub Repository](https://github.com/fluffy-bunny/fluffy-dozm-di)
