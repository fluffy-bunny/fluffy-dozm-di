package main

import (
	"fmt"
	"reflect"

	di "github.com/fluffy-bunny/fluffy-dozm-di"
)

// Service that receives the container as a dependency
type ServiceLocator struct {
	Container di.Container
}

func (s *ServiceLocator) GetService(serviceType reflect.Type) (interface{}, error) {
	return s.Container.Get(serviceType)
}

// Dynamic service resolver that uses container
type DynamicResolver struct {
	Container di.Container
	Logger    *Logger
}

func (d *DynamicResolver) ResolveByName(serviceName string) interface{} {
	d.Logger.Log(fmt.Sprintf("Resolving service: %s", serviceName))

	switch serviceName {
	case "config":
		return di.Get[*Config](d.Container)
	case "database":
		return di.Get[*Database](d.Container)
	case "cache":
		return di.Get[*Cache](d.Container)
	default:
		d.Logger.Log(fmt.Sprintf("Unknown service: %s", serviceName))
		return nil
	}
}

// Plugin system that uses container for dynamic resolution
type PluginManager struct {
	Container di.Container
	Plugins   []string
}

func (p *PluginManager) LoadPlugin(pluginName string) interface{} {
	fmt.Printf("Loading plugin: %s\n", pluginName)

	// Use container to resolve plugin dynamically
	switch pluginName {
	case "email":
		return di.Get[*EmailPlugin](p.Container)
	case "sms":
		return di.Get[*SMSPlugin](p.Container)
	default:
		return nil
	}
}

func (p *PluginManager) ExecuteAll() {
	for _, pluginName := range p.Plugins {
		plugin := p.LoadPlugin(pluginName)
		if executor, ok := plugin.(PluginExecutor); ok {
			executor.Execute()
		}
	}
}

// Plugin interface and implementations
type PluginExecutor interface {
	Execute()
}

type EmailPlugin struct {
	Config *Config
}

func (e *EmailPlugin) Execute() {
	fmt.Printf("  [EmailPlugin] Sending email via %s\n", e.Config.Provider)
}

type SMSPlugin struct {
	Config *Config
}

func (s *SMSPlugin) Execute() {
	fmt.Printf("  [SMSPlugin] Sending SMS via %s\n", s.Config.Provider)
}

// Lazy initialization service
type LazyService struct {
	Container di.Container
	database  *Database
}

func (l *LazyService) GetDatabase() *Database {
	if l.database == nil {
		fmt.Println("  [LazyService] Lazy loading database on first access")
		l.database = di.Get[*Database](l.Container)
	}
	return l.database
}

func (l *LazyService) DoWork() {
	fmt.Println("  [LazyService] Doing work...")
	db := l.GetDatabase()
	db.Query("SELECT * FROM users")
}

// Factory service that creates instances dynamically
type EntityFactory struct {
	Container di.Container
}

func (e *EntityFactory) CreateUser(name string) *User {
	// Get dependencies from container
	logger := di.Get[*Logger](e.Container)
	config := di.Get[*Config](e.Container)

	logger.Log(fmt.Sprintf("Creating user: %s with config: %s", name, config.Provider))

	return &User{
		Name:   name,
		Config: config,
		Logger: logger,
	}
}

// Supporting types
type Logger struct {
	Prefix string
}

func (l *Logger) Log(message string) {
	fmt.Printf("[%s] %s\n", l.Prefix, message)
}

type Config struct {
	Provider string
	Env      string
}

type Database struct {
	ConnectionString string
}

func (d *Database) Query(sql string) {
	fmt.Printf("  [Database] Executing: %s\n", sql)
}

type Cache struct {
	Type string
}

type User struct {
	Name   string
	Config *Config
	Logger *Logger
}

// Service that validates container scope
type ScopeAwareService struct {
	Container di.Container
	ID        int
}

func (s *ScopeAwareService) CheckScope() string {
	// Try to get scope factory to determine if we're in root or scoped container
	scopeFactory := di.Get[di.ScopeFactory](s.Container)
	if scopeFactory != nil {
		return fmt.Sprintf("Service #%d has access to scope factory", s.ID)
	}
	return fmt.Sprintf("Service #%d is in unknown scope", s.ID)
}

func main() {
	fmt.Println("=== Container Injection Example ===\n")

	// Example 1: Basic Container Injection
	fmt.Println("=== Basic Container Injection ===\n")

	builder1 := di.Builder()

	// Register basic services
	di.AddSingleton[*Logger](builder1, func() *Logger {
		return &Logger{Prefix: "APP"}
	})

	di.AddSingleton[*Config](builder1, func() *Config {
		return &Config{Provider: "AWS", Env: "production"}
	})

	// Register service that receives container
	di.AddSingleton[*ServiceLocator](builder1, func(container di.Container) *ServiceLocator {
		fmt.Println("→ Creating ServiceLocator with injected container")
		return &ServiceLocator{Container: container}
	})

	container1 := builder1.Build()

	locator := di.Get[*ServiceLocator](container1)

	// Use service locator to resolve services dynamically
	logger, _ := locator.GetService(reflect.TypeOf(&Logger{}))
	if l, ok := logger.(*Logger); ok {
		l.Log("Retrieved via ServiceLocator")
	}

	// Example 2: Dynamic Resolver with Container
	fmt.Println("\n=== Dynamic Resolver ===\n")

	builder2 := di.Builder()

	di.AddSingleton[*Logger](builder2, func() *Logger {
		return &Logger{Prefix: "RESOLVER"}
	})

	di.AddSingleton[*Config](builder2, func() *Config {
		return &Config{Provider: "Azure", Env: "staging"}
	})

	di.AddSingleton[*Database](builder2, func() *Database {
		return &Database{ConnectionString: "server=localhost;db=mydb"}
	})

	di.AddSingleton[*Cache](builder2, func() *Cache {
		return &Cache{Type: "Redis"}
	})

	di.AddSingleton[*DynamicResolver](builder2, func(container di.Container, logger *Logger) *DynamicResolver {
		fmt.Println("→ Creating DynamicResolver with injected container")
		return &DynamicResolver{Container: container, Logger: logger}
	})

	container2 := builder2.Build()

	resolver := di.Get[*DynamicResolver](container2)

	// Resolve services dynamically by name
	config := resolver.ResolveByName("config").(*Config)
	fmt.Printf("Resolved config: Provider=%s, Env=%s\n", config.Provider, config.Env)

	database := resolver.ResolveByName("database").(*Database)
	fmt.Printf("Resolved database: %s\n", database.ConnectionString)

	cache := resolver.ResolveByName("cache").(*Cache)
	fmt.Printf("Resolved cache: Type=%s\n", cache.Type)

	// Example 3: Plugin System with Container
	fmt.Println("\n=== Plugin System ===\n")

	builder3 := di.Builder()

	di.AddSingleton[*Config](builder3, func() *Config {
		return &Config{Provider: "SendGrid", Env: "production"}
	})

	di.AddTransient[*EmailPlugin](builder3, func(config *Config) *EmailPlugin {
		return &EmailPlugin{Config: config}
	})

	di.AddTransient[*SMSPlugin](builder3, func(config *Config) *SMSPlugin {
		return &SMSPlugin{Config: config}
	})

	di.AddSingleton[*PluginManager](builder3, func(container di.Container) *PluginManager {
		fmt.Println("→ Creating PluginManager with injected container")
		return &PluginManager{
			Container: container,
			Plugins:   []string{"email", "sms"},
		}
	})

	container3 := builder3.Build()

	pluginMgr := di.Get[*PluginManager](container3)
	pluginMgr.ExecuteAll()

	// Example 4: Lazy Initialization
	fmt.Println("\n=== Lazy Initialization ===\n")

	builder4 := di.Builder()

	di.AddSingleton[*Database](builder4, func() *Database {
		fmt.Println("→ Creating Database instance")
		return &Database{ConnectionString: "server=prod;db=maindb"}
	})

	di.AddSingleton[*LazyService](builder4, func(container di.Container) *LazyService {
		fmt.Println("→ Creating LazyService (database not loaded yet)")
		return &LazyService{Container: container}
	})

	container4 := builder4.Build()

	fmt.Println("Getting LazyService...")
	lazyService := di.Get[*LazyService](container4)

	fmt.Println("\nFirst call to DoWork (will lazy load database):")
	lazyService.DoWork()

	fmt.Println("\nSecond call to DoWork (database already loaded):")
	lazyService.DoWork()

	// Example 5: Factory Pattern with Container
	fmt.Println("\n=== Factory Pattern ===\n")

	builder5 := di.Builder()

	di.AddSingleton[*Logger](builder5, func() *Logger {
		return &Logger{Prefix: "FACTORY"}
	})

	di.AddSingleton[*Config](builder5, func() *Config {
		return &Config{Provider: "GCP", Env: "production"}
	})

	di.AddSingleton[*EntityFactory](builder5, func(container di.Container) *EntityFactory {
		fmt.Println("→ Creating EntityFactory with injected container")
		return &EntityFactory{Container: container}
	})

	container5 := builder5.Build()

	factory := di.Get[*EntityFactory](container5)

	// Create entities dynamically
	user1 := factory.CreateUser("Alice")
	user2 := factory.CreateUser("Bob")
	user3 := factory.CreateUser("Charlie")

	fmt.Printf("\nCreated %d users\n", 3)
	user1.Logger.Log(fmt.Sprintf("User: %s", user1.Name))
	user2.Logger.Log(fmt.Sprintf("User: %s", user2.Name))
	user3.Logger.Log(fmt.Sprintf("User: %s", user3.Name))

	// Example 6: Scope-Aware Services
	fmt.Println("\n=== Scope-Aware Services ===\n")

	builder6 := di.Builder()

	serviceID := 0
	di.AddScoped[*ScopeAwareService](builder6, func(container di.Container) *ScopeAwareService {
		serviceID++
		fmt.Printf("→ Creating ScopeAwareService #%d with container access\n", serviceID)
		return &ScopeAwareService{Container: container, ID: serviceID}
	})

	container6 := builder6.Build()

	scopeFactory := di.Get[di.ScopeFactory](container6)

	// Create first scope
	scope1 := scopeFactory.CreateScope()
	defer scope1.Dispose()

	service1 := di.Get[*ScopeAwareService](scope1.Container())
	fmt.Println(service1.CheckScope())

	// Create second scope
	scope2 := scopeFactory.CreateScope()
	defer scope2.Dispose()

	service2 := di.Get[*ScopeAwareService](scope2.Container())
	fmt.Println(service2.CheckScope())

	fmt.Printf("\nServices are in different scopes: %v\n", service1.ID != service2.ID)

	// Example 7: Container as Service Locator (Anti-pattern demonstration)
	fmt.Println("\n=== Service Locator Pattern (Use with Caution) ===\n")

	builder7 := di.Builder()

	di.AddSingleton[*Logger](builder7, func() *Logger {
		return &Logger{Prefix: "LOCATOR"}
	})

	// Service that uses container as service locator
	type ServiceWithLocator struct {
		Container di.Container
	}

	di.AddSingleton[*ServiceWithLocator](builder7, func(container di.Container) *ServiceWithLocator {
		return &ServiceWithLocator{Container: container}
	})

	container7 := builder7.Build()

	serviceLocator := di.Get[*ServiceWithLocator](container7)

	// This works but hides dependencies - prefer explicit constructor injection
	logger7 := di.Get[*Logger](serviceLocator.Container)
	logger7.Log("⚠️  Service Locator pattern hides dependencies")
	logger7.Log("✅ Prefer explicit constructor injection when possible")

	// Example 8: Container validation
	fmt.Println("\n=== Container Validation ===\n")

	builder8 := di.Builder()

	di.AddSingleton[*Config](builder8, func() *Config {
		return &Config{Provider: "Local", Env: "development"}
	})

	type ValidatingService struct {
		Container di.Container
	}

	di.AddSingleton[*ValidatingService](builder8, func(container di.Container) *ValidatingService {
		// Use container to check if required services are registered
		isService := di.Get[di.IsService](container)

		fmt.Println("→ Validating service registrations:")

		if isService.IsService(reflect.TypeOf(&Config{})) {
			fmt.Println("  ✓ Config is registered")
		}

		if !isService.IsService(reflect.TypeOf(&Database{})) {
			fmt.Println("  ⚠️  Database is NOT registered")
		}

		return &ValidatingService{Container: container}
	})

	container8 := builder8.Build()
	_ = di.Get[*ValidatingService](container8)

	fmt.Println("\n✓ Container injection example completed!")
}
