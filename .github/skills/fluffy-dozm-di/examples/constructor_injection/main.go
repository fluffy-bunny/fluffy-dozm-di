package main

import (
	"fmt"
	"time"

	di "github.com/fluffy-bunny/fluffy-dozm-di"
)

// Repository interface and implementation
type Repository interface {
	Save(data string) error
	Get(id string) (string, error)
}

type MemoryRepository struct {
	data map[string]string
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		data: make(map[string]string),
	}
}

func (r *MemoryRepository) Save(data string) error {
	id := fmt.Sprintf("id-%d", len(r.data)+1)
	r.data[id] = data
	fmt.Printf("  [Repository] Saved data: %s with id: %s\n", data, id)
	return nil
}

func (r *MemoryRepository) Get(id string) (string, error) {
	if data, ok := r.data[id]; ok {
		return data, nil
	}
	return "", fmt.Errorf("not found")
}

// Logger service
type Logger struct {
	prefix string
}

func NewLogger(prefix string) *Logger {
	return &Logger{prefix: prefix}
}

func (l *Logger) Log(message string) {
	fmt.Printf("[%s] %s: %s\n", time.Now().Format("15:04:05"), l.prefix, message)
}

// Service that depends on both Repository and Logger
type UserService struct {
	repo   Repository
	logger *Logger
}

// Constructor with automatic dependency injection
func NewUserService(repo Repository, logger *Logger) *UserService {
	return &UserService{
		repo:   repo,
		logger: logger,
	}
}

func (s *UserService) CreateUser(name string) error {
	s.logger.Log(fmt.Sprintf("Creating user: %s", name))
	return s.repo.Save(name)
}

// Higher level service that depends on UserService
type ApplicationService struct {
	userService *UserService
	logger      *Logger
}

func NewApplicationService(userService *UserService, logger *Logger) *ApplicationService {
	return &ApplicationService{
		userService: userService,
		logger:      logger,
	}
}

func (a *ApplicationService) ProcessRequest(userName string) {
	a.logger.Log("Processing request")
	a.userService.CreateUser(userName)
	a.logger.Log("Request completed")
}

func main() {
	fmt.Println("=== Constructor Injection Example ===\n")

	// Create container builder
	builder := di.Builder()

	// Register Logger as a singleton
	di.AddSingleton[*Logger](builder, func() *Logger {
		fmt.Println("→ Creating Logger instance")
		return NewLogger("APP")
	})

	// Register Repository interface with MemoryRepository implementation
	// The container will automatically inject it wherever Repository is needed
	di.AddSingleton[Repository](builder, func() Repository {
		fmt.Println("→ Creating MemoryRepository instance")
		return NewMemoryRepository()
	})

	// Register UserService with automatic constructor injection
	// The container will automatically resolve Repository and Logger
	di.AddSingleton[*UserService](builder, func(repo Repository, logger *Logger) *UserService {
		fmt.Println("→ Creating UserService instance")
		return NewUserService(repo, logger)
	})

	// Register ApplicationService with automatic constructor injection
	// The container will automatically resolve UserService and Logger
	di.AddSingleton[*ApplicationService](builder, func(userService *UserService, logger *Logger) *ApplicationService {
		fmt.Println("→ Creating ApplicationService instance")
		return NewApplicationService(userService, logger)
	})

	// Build the container
	fmt.Println("Building container...\n")
	container := builder.Build()

	// Resolve ApplicationService - the container automatically creates the entire dependency chain
	fmt.Println("Resolving ApplicationService (this will create the entire dependency graph):\n")
	app := di.Get[*ApplicationService](container)

	// Use the service
	fmt.Println("\nUsing the application service:\n")
	app.ProcessRequest("Alice")
	app.ProcessRequest("Bob")
	app.ProcessRequest("Charlie")

	// Demonstrate that dependencies are shared (singletons)
	fmt.Println("\n=== Verifying Singleton Behavior ===\n")
	app2 := di.Get[*ApplicationService](container)
	fmt.Printf("app and app2 are the same instance: %v\n", app == app2)

	userService1 := di.Get[*UserService](container)
	userService2 := di.Get[*UserService](container)
	fmt.Printf("UserService instances are the same: %v\n", userService1 == userService2)

	// Example with transient services
	fmt.Println("\n=== Transient Constructor Injection ===\n")
	builder2 := di.Builder()

	di.AddSingleton[*Logger](builder2, func() *Logger {
		return NewLogger("TRANSIENT-DEMO")
	})

	di.AddTransient[Repository](builder2, func() Repository {
		fmt.Println("→ Creating NEW MemoryRepository instance")
		return NewMemoryRepository()
	})

	di.AddTransient[*UserService](builder2, func(repo Repository, logger *Logger) *UserService {
		fmt.Println("→ Creating NEW UserService instance")
		return NewUserService(repo, logger)
	})

	container2 := builder2.Build()

	// Each Get call creates a new instance of UserService and Repository
	svc1 := di.Get[*UserService](container2)
	svc2 := di.Get[*UserService](container2)

	fmt.Printf("\nTransient services are different instances: %v\n", svc1 != svc2)

	svc1.CreateUser("User1")
	svc2.CreateUser("User2")

	fmt.Println("\n✓ Constructor injection example completed!")
}
