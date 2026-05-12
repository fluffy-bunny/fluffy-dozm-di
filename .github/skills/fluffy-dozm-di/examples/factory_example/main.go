package main

import (
	"fmt"
	"sync/atomic"

	di "github.com/fluffy-bunny/fluffy-dozm-di"
)

// Counter to track factory invocations
var factoryCallCount int32

// Database connection pool
type ConnectionPool struct {
	MaxConnections int
	ActiveCount    int32
}

func (cp *ConnectionPool) GetConnection() int32 {
	return atomic.AddInt32(&cp.ActiveCount, 1)
}

func (cp *ConnectionPool) ReleaseConnection() {
	atomic.AddInt32(&cp.ActiveCount, -1)
}

// Service that needs complex initialization
type ComplexService struct {
	ID            int32
	Configuration map[string]string
	Dependencies  []string
}

func main() {
	fmt.Println("=== Factory Functions Example ===\n")

	// Create container builder
	builder := di.Builder()

	// Example 1: Factory with access to container
	fmt.Println("=== Factory with Container Access ===\n")

	// Register a dependency
	di.AddSingleton[string](builder, func() string {
		return "SharedConfig"
	})

	// Register service using factory that accesses container
	di.AddSingletonFactory[*ComplexService](builder, func(c di.Container) any {
		callNum := atomic.AddInt32(&factoryCallCount, 1)
		fmt.Printf("→ Factory called (call #%d)\n", callNum)

		// Access other services from container
		config := di.Get[string](c)
		fmt.Printf("  - Retrieved config from container: %s\n", config)

		return &ComplexService{
			ID: callNum,
			Configuration: map[string]string{
				"config": config,
				"source": "factory",
			},
			Dependencies: []string{"dep1", "dep2"},
		}
	})

	// Build container
	container := builder.Build()

	// Get the service - factory is called once for singleton
	fmt.Println("\nResolving ComplexService:")
	svc1 := di.Get[*ComplexService](container)
	fmt.Printf("Service ID: %d, Config: %v\n", svc1.ID, svc1.Configuration)

	svc2 := di.Get[*ComplexService](container)
	fmt.Printf("Same instance: %v\n", svc1 == svc2)

	// Example 2: Transient factory (called every time)
	fmt.Println("\n=== Transient Factory ===\n")

	builder2 := di.Builder()
	factoryCallCount = 0 // reset counter

	di.AddTransientFactory[*ComplexService](builder2, func(c di.Container) any {
		callNum := atomic.AddInt32(&factoryCallCount, 1)
		fmt.Printf("→ Transient factory called (call #%d)\n", callNum)

		return &ComplexService{
			ID: callNum,
			Configuration: map[string]string{
				"lifetime": "transient",
				"call":     fmt.Sprintf("%d", callNum),
			},
			Dependencies: []string{"transient-dep"},
		}
	})

	container2 := builder2.Build()

	fmt.Println("\nResolving transient services:")
	trans1 := di.Get[*ComplexService](container2)
	fmt.Printf("Service 1 - ID: %d\n", trans1.ID)

	trans2 := di.Get[*ComplexService](container2)
	fmt.Printf("Service 2 - ID: %d\n", trans2.ID)

	trans3 := di.Get[*ComplexService](container2)
	fmt.Printf("Service 3 - ID: %d\n", trans3.ID)

	fmt.Printf("Different instances: %v\n", trans1 != trans2 && trans2 != trans3)

	// Example 3: Scoped factory
	fmt.Println("\n=== Scoped Factory ===\n")

	builder3 := di.Builder()
	factoryCallCount = 0 // reset counter

	di.AddScopedFactory[*ComplexService](builder3, func(c di.Container) any {
		callNum := atomic.AddInt32(&factoryCallCount, 1)
		fmt.Printf("→ Scoped factory called (call #%d)\n", callNum)

		return &ComplexService{
			ID: callNum,
			Configuration: map[string]string{
				"lifetime": "scoped",
			},
			Dependencies: []string{"scoped-dep"},
		}
	})

	container3 := builder3.Build()
	scopeFactory := di.Get[di.ScopeFactory](container3)

	// First scope
	scope1 := scopeFactory.CreateScope()
	defer scope1.Dispose()

	fmt.Println("\nScope 1:")
	scoped1a := di.Get[*ComplexService](scope1.Container())
	fmt.Printf("  Service A - ID: %d\n", scoped1a.ID)

	scoped1b := di.Get[*ComplexService](scope1.Container())
	fmt.Printf("  Service B - ID: %d\n", scoped1b.ID)
	fmt.Printf("  Same in scope: %v\n", scoped1a == scoped1b)

	// Second scope
	scope2 := scopeFactory.CreateScope()
	defer scope2.Dispose()

	fmt.Println("\nScope 2:")
	scoped2a := di.Get[*ComplexService](scope2.Container())
	fmt.Printf("  Service A - ID: %d\n", scoped2a.ID)
	fmt.Printf("  Different from scope 1: %v\n", scoped1a != scoped2a)

	// Example 4: Factory for resource management
	fmt.Println("\n=== Factory for Resource Management ===\n")

	builder4 := di.Builder()

	di.AddSingleton[*ConnectionPool](builder4, func() *ConnectionPool {
		fmt.Println("→ Creating ConnectionPool")
		return &ConnectionPool{MaxConnections: 10, ActiveCount: 0}
	})

	di.AddScopedFactory[string](builder4, func(c di.Container) any {
		pool := di.Get[*ConnectionPool](c)
		connID := pool.GetConnection()
		fmt.Printf("  → Acquired connection #%d (active: %d/%d)\n",
			connID, pool.ActiveCount, pool.MaxConnections)

		return fmt.Sprintf("connection-%d", connID)
	})

	container4 := builder4.Build()
	scopeFactory4 := di.Get[di.ScopeFactory](container4)

	// Create multiple scopes to simulate multiple requests
	fmt.Println("\nSimulating multiple requests with connection pool:")
	for i := 1; i <= 3; i++ {
		fmt.Printf("\nRequest %d:\n", i)
		scope := scopeFactory4.CreateScope()

		conn := di.Get[string](scope.Container())
		fmt.Printf("  Using %s\n", conn)

		// In real scenario, scope.Dispose() would release the connection
		scope.Dispose()

		pool := di.Get[*ConnectionPool](container4)
		fmt.Printf("  Active connections after scope disposal: %d\n", pool.ActiveCount)
	}

	// Example 5: Conditional factory logic
	fmt.Println("\n=== Conditional Factory Logic ===\n")

	type Environment string
	const (
		Development Environment = "dev"
		Production  Environment = "prod"
	)

	type ServiceConfig struct {
		Env     Environment
		Debug   bool
		Timeout int
	}

	builder5 := di.Builder()

	// Current environment (would come from env variable)
	currentEnv := Development

	di.AddSingleton[Environment](builder5, func() Environment {
		return currentEnv
	})

	di.AddSingletonFactory[*ServiceConfig](builder5, func(c di.Container) any {
		env := di.Get[Environment](c)
		fmt.Printf("→ Creating config for environment: %s\n", env)

		var config *ServiceConfig

		// Different configuration based on environment
		switch env {
		case Development:
			config = &ServiceConfig{
				Env:     env,
				Debug:   true,
				Timeout: 30,
			}
		case Production:
			config = &ServiceConfig{
				Env:     env,
				Debug:   false,
				Timeout: 5,
			}
		default:
			config = &ServiceConfig{
				Env:     env,
				Debug:   false,
				Timeout: 10,
			}
		}

		fmt.Printf("  Config: Debug=%v, Timeout=%d\n", config.Debug, config.Timeout)
		return config
	})

	container5 := builder5.Build()

	config := di.Get[*ServiceConfig](container5)
	fmt.Printf("\nFinal Config: Env=%s, Debug=%v, Timeout=%ds\n",
		config.Env, config.Debug, config.Timeout)

	fmt.Println("\n✓ Factory functions example completed!")
}
