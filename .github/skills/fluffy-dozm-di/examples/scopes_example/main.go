package main

import (
	"fmt"

	di "github.com/fluffy-bunny/fluffy-dozm-di"
)

// Request counter to demonstrate scoped lifetime
type RequestCounter struct {
	count int
}

func (r *RequestCounter) Increment() int {
	r.count++
	return r.count
}

func (r *RequestCounter) GetCount() int {
	return r.count
}

// Database connection (singleton)
type Database struct {
	ConnectionString string
	Connections      int
}

func (d *Database) Connect() {
	d.Connections++
	fmt.Printf("  [Database] New connection created (total: %d)\n", d.Connections)
}

// Request handler (scoped per request)
type RequestHandler struct {
	RequestID string
	Counter   *RequestCounter
	DB        *Database
}

func NewRequestHandler(counter *RequestCounter, db *Database) *RequestHandler {
	handler := &RequestHandler{
		RequestID: fmt.Sprintf("req-%d", counter.GetCount()),
		Counter:   counter,
		DB:        db,
	}
	handler.DB.Connect()
	return handler
}

func (r *RequestHandler) Process(data string) {
	operationNum := r.Counter.Increment()
	fmt.Printf("  [%s] Processing operation #%d: %s\n", r.RequestID, operationNum, data)
}

func main() {
	fmt.Println("=== Scopes Example ===\n")

	// Create container builder
	builder := di.Builder()

	// Configure validation
	builder.ConfigureOptions(func(o *di.Options) {
		o.ValidateScopes = true // Prevent scoped services from root container
		o.ValidateOnBuild = true
	})

	// Register singleton Database - shared across all scopes
	di.AddSingleton[*Database](builder, func() *Database {
		fmt.Println("→ Creating Database (singleton)")
		return &Database{
			ConnectionString: "server=localhost;database=mydb",
			Connections:      0,
		}
	})

	// Register scoped RequestCounter - one per scope
	di.AddScoped[*RequestCounter](builder, func() *RequestCounter {
		fmt.Println("→ Creating RequestCounter (scoped)")
		return &RequestCounter{count: 0}
	})

	// Register scoped RequestHandler - one per scope
	di.AddScoped[*RequestHandler](builder, func(counter *RequestCounter, db *Database) *RequestHandler {
		fmt.Println("→ Creating RequestHandler (scoped)")
		return NewRequestHandler(counter, db)
	})

	// Build container
	container := builder.Build()

	// Get scope factory (built-in singleton service)
	scopeFactory := di.Get[di.ScopeFactory](container)

	// Simulate HTTP Request 1
	fmt.Println("=== Simulating HTTP Request 1 ===")
	processRequest(scopeFactory, "Request 1 - Operation A", "Request 1 - Operation B")

	fmt.Println()

	// Simulate HTTP Request 2
	fmt.Println("=== Simulating HTTP Request 2 ===")
	processRequest(scopeFactory, "Request 2 - Operation A", "Request 2 - Operation B", "Request 2 - Operation C")

	fmt.Println()

	// Simulate HTTP Request 3
	fmt.Println("=== Simulating HTTP Request 3 ===")
	processRequest(scopeFactory, "Request 3 - Operation A")

	// Demonstrate scope isolation
	fmt.Println("\n=== Demonstrating Scope Isolation ===")

	scope1 := scopeFactory.CreateScope()
	defer scope1.Dispose()

	scope2 := scopeFactory.CreateScope()
	defer scope2.Dispose()

	// Get handlers from different scopes
	handler1a := di.Get[*RequestHandler](scope1.Container())
	handler1b := di.Get[*RequestHandler](scope1.Container())

	handler2a := di.Get[*RequestHandler](scope2.Container())
	handler2b := di.Get[*RequestHandler](scope2.Container())

	fmt.Printf("\nScope 1: handler1a == handler1b: %v (same scope, same instance)\n", handler1a == handler1b)
	fmt.Printf("Scope 2: handler2a == handler2b: %v (same scope, same instance)\n", handler2a == handler2b)
	fmt.Printf("Cross-scope: handler1a == handler2a: %v (different scopes, different instances)\n", handler1a == handler2a)

	// Show that singleton Database is shared
	db1 := di.Get[*Database](scope1.Container())
	db2 := di.Get[*Database](scope2.Container())
	fmt.Printf("\nSingleton: db1 == db2: %v (same singleton across scopes)\n", db1 == db2)
	fmt.Printf("Total database connections: %d\n", db1.Connections)

	// Demonstrate scope disposal
	fmt.Println("\n=== Scope Disposal ===")
	testScope := scopeFactory.CreateScope()
	fmt.Println("Scope created")
	_ = di.Get[*RequestHandler](testScope.Container())
	fmt.Println("Service resolved")
	testScope.Dispose()
	fmt.Println("Scope disposed (resources cleaned up)")

	// Try to resolve from root container (will fail with validation enabled)
	fmt.Println("\n=== Attempting to resolve scoped service from root (should panic) ===")
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("✓ Expected panic caught: Cannot resolve scoped service from root container\n")
		}
	}()

	// This will panic because we enabled ValidateScopes
	// _ = di.Get[*RequestHandler](container)

	fmt.Println("\n✓ Scopes example completed!")
}

func processRequest(scopeFactory di.ScopeFactory, operations ...string) {
	// Create a new scope for this request
	scope := scopeFactory.CreateScope()
	defer scope.Dispose()

	// Get the scoped request handler
	handler := di.Get[*RequestHandler](scope.Container())

	// Process operations
	for _, op := range operations {
		handler.Process(op)
	}

	// Within the same scope, we get the same instance
	handler2 := di.Get[*RequestHandler](scope.Container())
	fmt.Printf("  [Verification] Same handler instance in scope: %v\n", handler == handler2)
}
