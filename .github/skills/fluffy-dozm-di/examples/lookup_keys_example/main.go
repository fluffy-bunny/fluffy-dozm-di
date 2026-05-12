package main

import (
	"fmt"

	di "github.com/fluffy-bunny/fluffy-dozm-di"
)

// Configuration service
type Config struct {
	Environment string
	DatabaseURL string
	APIKey      string
	Port        int
}

// Database connection
type DatabaseConnection struct {
	Config *Config
}

func (d *DatabaseConnection) Connect() string {
	return fmt.Sprintf("Connected to %s (env: %s)", d.Config.DatabaseURL, d.Config.Environment)
}

// API Client
type APIClient struct {
	Config *Config
}

func (a *APIClient) MakeRequest() string {
	return fmt.Sprintf("API request with key: %s on port %d", a.Config.APIKey, a.Config.Port)
}

// Feature flags
type FeatureFlag struct {
	Name    string
	Enabled bool
}

func main() {
	fmt.Println("=== Lookup Keys Example ===\n")

	// Create container builder
	builder := di.Builder()

	// Register multiple configurations with lookup keys
	// Development configuration
	di.AddSingletonWithLookupKeys[*Config](builder,
		func() *Config {
			fmt.Println("→ Creating Development Config")
			return &Config{
				Environment: "development",
				DatabaseURL: "localhost:5432/dev_db",
				APIKey:      "dev-api-key-12345",
				Port:        8080,
			}
		},
		[]string{"dev-config", "development"},
		map[string]interface{}{
			"type": "config",
			"env":  "dev",
		})

	// Production configuration
	di.AddSingletonWithLookupKeys[*Config](builder,
		func() *Config {
			fmt.Println("→ Creating Production Config")
			return &Config{
				Environment: "production",
				DatabaseURL: "prod-server:5432/prod_db",
				APIKey:      "prod-api-key-67890",
				Port:        443,
			}
		},
		[]string{"prod-config", "production"},
		map[string]interface{}{
			"type": "config",
			"env":  "prod",
		})

	// Staging configuration
	di.AddSingletonWithLookupKeys[*Config](builder,
		func() *Config {
			fmt.Println("→ Creating Staging Config")
			return &Config{
				Environment: "staging",
				DatabaseURL: "staging-server:5432/staging_db",
				APIKey:      "staging-api-key-11111",
				Port:        8443,
			}
		},
		[]string{"staging-config", "staging"},
		map[string]interface{}{
			"type": "config",
			"env":  "staging",
		})

	// Register feature flags with names
	di.AddSingletonWithLookupKeys[*FeatureFlag](builder,
		func() *FeatureFlag {
			fmt.Println("→ Creating NewUIFeature")
			return &FeatureFlag{Name: "NewUI", Enabled: true}
		},
		[]string{"feature:new-ui"},
		map[string]interface{}{"feature": true})

	di.AddSingletonWithLookupKeys[*FeatureFlag](builder,
		func() *FeatureFlag {
			fmt.Println("→ Creating BetaFeature")
			return &FeatureFlag{Name: "Beta", Enabled: false}
		},
		[]string{"feature:beta"},
		map[string]interface{}{"feature": true})

	di.AddSingletonWithLookupKeys[*FeatureFlag](builder,
		func() *FeatureFlag {
			fmt.Println("→ Creating DarkModeFeature")
			return &FeatureFlag{Name: "DarkMode", Enabled: true}
		},
		[]string{"feature:dark-mode"},
		map[string]interface{}{"feature": true})

	// Register services that can use specific configs
	di.AddSingletonWithLookupKeys[*DatabaseConnection](builder,
		func() *DatabaseConnection {
			// This will resolve without lookup key, getting the last registered Config
			return &DatabaseConnection{}
		},
		[]string{"dev-db"},
		nil)

	// Build container
	fmt.Println("\nBuilding container...\n")
	container := builder.Build()

	// Retrieve configurations by lookup key
	fmt.Println("=== Retrieving Configurations by Key ===\n")

	devConfig := di.GetByLookupKey[*Config](container, "dev-config")
	fmt.Printf("Dev Config: %+v\n", devConfig)

	prodConfig := di.GetByLookupKey[*Config](container, "prod-config")
	fmt.Printf("Prod Config: %+v\n", prodConfig)

	stagingConfig := di.GetByLookupKey[*Config](container, "staging-config")
	fmt.Printf("Staging Config: %+v\n\n", stagingConfig)

	// Alternative lookup key names
	devConfigAlt := di.GetByLookupKey[*Config](container, "development")
	fmt.Printf("Dev Config (alternative key): %+v\n", devConfigAlt)
	fmt.Printf("Same instance: %v\n\n", devConfig == devConfigAlt)

	// Use configurations
	fmt.Println("=== Using Configurations ===\n")

	devDB := &DatabaseConnection{Config: devConfig}
	fmt.Println(devDB.Connect())

	prodDB := &DatabaseConnection{Config: prodConfig}
	fmt.Println(prodDB.Connect())

	devAPI := &APIClient{Config: devConfig}
	fmt.Println(devAPI.MakeRequest())

	prodAPI := &APIClient{Config: prodConfig}
	fmt.Println(prodAPI.MakeRequest())

	// Retrieve feature flags by lookup key
	fmt.Println("\n=== Feature Flags ===\n")

	newUI := di.GetByLookupKey[*FeatureFlag](container, "feature:new-ui")
	fmt.Printf("Feature: %s, Enabled: %v\n", newUI.Name, newUI.Enabled)

	beta := di.GetByLookupKey[*FeatureFlag](container, "feature:beta")
	fmt.Printf("Feature: %s, Enabled: %v\n", beta.Name, beta.Enabled)

	darkMode := di.GetByLookupKey[*FeatureFlag](container, "feature:dark-mode")
	fmt.Printf("Feature: %s, Enabled: %v\n", darkMode.Name, darkMode.Enabled)

	// Get all configs as a slice
	fmt.Println("\n=== Getting All Configs ===\n")
	allConfigs := di.Get[[](*Config)](container)
	fmt.Printf("Total configurations registered: %d\n", len(allConfigs))
	for i, cfg := range allConfigs {
		fmt.Printf("%d. %s environment on port %d\n", i+1, cfg.Environment, cfg.Port)
	}

	// Get all feature flags
	fmt.Println("\n=== Getting All Feature Flags ===\n")
	allFlags := di.Get[[](*FeatureFlag)](container)
	fmt.Printf("Total feature flags registered: %d\n", len(allFlags))
	for _, flag := range allFlags {
		status := "❌"
		if flag.Enabled {
			status = "✅"
		}
		fmt.Printf("%s %s\n", status, flag.Name)
	}

	// Example: Environment-specific service loading
	fmt.Println("\n=== Environment-Specific Service Loading ===\n")

	currentEnv := "development" // This would come from environment variable
	fmt.Printf("Loading services for environment: %s\n\n", currentEnv)

	var envConfig *Config
	switch currentEnv {
	case "development":
		envConfig = di.GetByLookupKey[*Config](container, "dev-config")
	case "staging":
		envConfig = di.GetByLookupKey[*Config](container, "staging-config")
	case "production":
		envConfig = di.GetByLookupKey[*Config](container, "prod-config")
	}

	fmt.Printf("Loaded config: %s environment\n", envConfig.Environment)
	fmt.Printf("Database: %s\n", envConfig.DatabaseURL)
	fmt.Printf("API Key: %s\n", envConfig.APIKey)
	fmt.Printf("Port: %d\n", envConfig.Port)

	// Advanced: Using metadata
	fmt.Println("\n=== Using Metadata ===")
	descriptors := container.GetDescriptors()
	fmt.Println("\nServices with metadata:")
	for _, desc := range descriptors {
		if desc.Metadata != nil && len(desc.Metadata) > 0 {
			fmt.Printf("- Type: %v, Metadata: %v, Keys: %v\n",
				desc.ServiceType, desc.Metadata, len(desc.LookupKeys))
		}
	}

	// Example with function types
	fmt.Println("\n=== Function Type with Lookup Keys ===\n")

	type ValidationFunc func(string) bool

	builder2 := di.Builder()

	di.AddFuncWithLookupKeys[ValidationFunc](builder2,
		func() ValidationFunc {
			return func(s string) bool {
				return len(s) > 0
			}
		},
		[]string{"validate:not-empty"},
		nil)

	di.AddFuncWithLookupKeys[ValidationFunc](builder2,
		func() ValidationFunc {
			return func(s string) bool {
				return len(s) >= 8
			}
		},
		[]string{"validate:min-length"},
		nil)

	container2 := builder2.Build()

	notEmptyValidator := di.GetByLookupKey[ValidationFunc](container2, "validate:not-empty")
	minLengthValidator := di.GetByLookupKey[ValidationFunc](container2, "validate:min-length")

	testString := "password123"
	fmt.Printf("Testing '%s':\n", testString)
	fmt.Printf("  Not empty: %v\n", notEmptyValidator(testString))
	fmt.Printf("  Min length (8): %v\n", minLengthValidator(testString))

	fmt.Println("\n✓ Lookup keys example completed!")
}
