package main

import (
	"fmt"

	di "github.com/fluffy-bunny/fluffy-dozm-di"
)

// Simple service with no dependencies
type GreetingService struct {
	Message string
}

func (g *GreetingService) Greet(name string) string {
	return fmt.Sprintf("%s, %s!", g.Message, name)
}

// Counter service to demonstrate different lifetimes
type Counter struct {
	Value int
}

func (c *Counter) Increment() int {
	c.Value++
	return c.Value
}

func main() {
	fmt.Println("=== Basic DI Example ===\n")

	// Create a container builder
	builder := di.Builder()

	// Register a singleton string service
	di.AddSingleton[string](builder, func() string {
		return "Hello from Singleton"
	})

	// Register a singleton GreetingService
	di.AddSingleton[*GreetingService](builder, func() *GreetingService {
		return &GreetingService{Message: "Welcome"}
	})

	// Register three different lifetime counters
	di.AddSingleton[*Counter](builder, func() *Counter {
		fmt.Println("Creating Singleton Counter")
		return &Counter{Value: 100}
	})

	// Build the container
	container := builder.Build()

	// Get the string service
	message := di.Get[string](container)
	fmt.Printf("Message: %s\n", message)

	// Get the greeting service
	greeter := di.Get[*GreetingService](container)
	fmt.Printf("Greeting: %s\n", greeter.Greet("World"))

	// Get the singleton counter multiple times - same instance
	counter1 := di.Get[*Counter](container)
	counter2 := di.Get[*Counter](container)

	fmt.Printf("\nSingleton Counter (first): %d\n", counter1.Increment())
	fmt.Printf("Singleton Counter (second): %d\n", counter2.Increment())
	fmt.Printf("Are they the same instance? %v\n", counter1 == counter2)
	fmt.Printf("Final value: %d\n\n", counter1.Value)

	// Demonstrate transient services
	fmt.Println("=== Transient Lifetime ===")
	builder2 := di.Builder()
	di.AddTransient[*Counter](builder2, func() *Counter {
		fmt.Println("Creating Transient Counter")
		return &Counter{Value: 0}
	})
	container2 := builder2.Build()

	trans1 := di.Get[*Counter](container2)
	trans2 := di.Get[*Counter](container2)
	fmt.Printf("Transient Counter 1: %d\n", trans1.Increment())
	fmt.Printf("Transient Counter 2: %d\n", trans2.Increment())
	fmt.Printf("Are they the same instance? %v\n\n", trans1 == trans2)

	// Demonstrate scoped services
	fmt.Println("=== Scoped Lifetime ===")
	builder3 := di.Builder()
	di.AddScoped[*Counter](builder3, func() *Counter {
		fmt.Println("Creating Scoped Counter")
		return &Counter{Value: 0}
	})
	container3 := builder3.Build()

	// Create first scope
	scopeFactory := di.Get[di.ScopeFactory](container3)
	scope1 := scopeFactory.CreateScope()
	defer scope1.Dispose()

	scoped1a := di.Get[*Counter](scope1.Container())
	scoped1b := di.Get[*Counter](scope1.Container())
	fmt.Printf("Scope 1 - Counter A: %d\n", scoped1a.Increment())
	fmt.Printf("Scope 1 - Counter B: %d\n", scoped1b.Increment())
	fmt.Printf("Are they the same in scope 1? %v\n", scoped1a == scoped1b)

	// Create second scope
	scope2 := scopeFactory.CreateScope()
	defer scope2.Dispose()

	scoped2a := di.Get[*Counter](scope2.Container())
	fmt.Printf("\nScope 2 - Counter A: %d\n", scoped2a.Increment())
	fmt.Printf("Are scope 1 and scope 2 the same? %v\n", scoped1a == scoped2a)

	fmt.Println("\n✓ Basic example completed!")
}
