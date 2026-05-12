package main

import (
	"fmt"
	"strings"
	"time"

	di "github.com/fluffy-bunny/fluffy-dozm-di"
)

// Define function types
type (
	// Simple function type
	GreetingFunc func(name string) string

	// Function type that returns multiple values
	ValidatorFunc func(input string) (bool, error)

	// Function type with complex logic
	TransformFunc func(input string) string

	// Function type that depends on other services
	ProcessorFunc func(data string) string
)

// A service that will be injected into a function
type Logger struct {
	Prefix string
}

func (l *Logger) Log(message string) {
	fmt.Printf("[%s] %s\n", l.Prefix, message)
}

func main() {
	fmt.Println("=== Function Injection Example ===\n")

	// Example 1: Basic Function Injection
	fmt.Println("=== Basic Function Injection ===\n")

	builder1 := di.Builder()

	// Register a simple function
	di.AddFunc[GreetingFunc](builder1, func() GreetingFunc {
		return func(name string) string {
			return fmt.Sprintf("Hello, %s! Welcome!", name)
		}
	})

	container1 := builder1.Build()

	// Get and use the function
	greet := di.Get[GreetingFunc](container1)
	fmt.Println(greet("Alice"))
	fmt.Println(greet("Bob"))

	// Example 2: Function with Dependencies
	fmt.Println("\n=== Function with Dependencies ===\n")

	builder2 := di.Builder()

	// Register logger service
	di.AddSingleton[*Logger](builder2, func() *Logger {
		fmt.Println("→ Creating Logger")
		return &Logger{Prefix: "FUNC"}
	})

	// Register function that depends on Logger
	di.AddFunc[ProcessorFunc](builder2, func(logger *Logger) ProcessorFunc {
		fmt.Println("→ Creating ProcessorFunc with Logger dependency")
		return func(data string) string {
			logger.Log(fmt.Sprintf("Processing: %s", data))
			result := strings.ToUpper(data)
			logger.Log(fmt.Sprintf("Result: %s", result))
			return result
		}
	})

	container2 := builder2.Build()

	processor := di.Get[ProcessorFunc](container2)
	result := processor("hello world")
	fmt.Printf("\nFinal result: %s\n", result)

	// Example 3: Multiple Function Implementations
	fmt.Println("\n=== Multiple Validator Functions ===\n")

	builder3 := di.Builder()

	// Register multiple validators
	di.AddFuncWithLookupKeys[ValidatorFunc](builder3,
		func() ValidatorFunc {
			return func(input string) (bool, error) {
				if len(input) == 0 {
					return false, fmt.Errorf("input cannot be empty")
				}
				return true, nil
			}
		},
		[]string{"validator:not-empty"},
		map[string]interface{}{"type": "validator"})

	di.AddFuncWithLookupKeys[ValidatorFunc](builder3,
		func() ValidatorFunc {
			return func(input string) (bool, error) {
				if len(input) < 8 {
					return false, fmt.Errorf("input must be at least 8 characters")
				}
				return true, nil
			}
		},
		[]string{"validator:min-length"},
		map[string]interface{}{"type": "validator"})

	di.AddFuncWithLookupKeys[ValidatorFunc](builder3,
		func() ValidatorFunc {
			return func(input string) (bool, error) {
				for _, ch := range input {
					if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')) {
						return false, fmt.Errorf("input must be alphanumeric only")
					}
				}
				return true, nil
			}
		},
		[]string{"validator:alphanumeric"},
		map[string]interface{}{"type": "validator"})

	container3 := builder3.Build()

	// Test with different validators
	validators := map[string]string{
		"validator:not-empty":    "Not Empty",
		"validator:min-length":   "Min Length (8)",
		"validator:alphanumeric": "Alphanumeric",
	}

	for input, test := range map[string][]string{
		"":              {"validator:not-empty", "validator:min-length", "validator:alphanumeric"},
		"short":         {"validator:not-empty", "validator:min-length", "validator:alphanumeric"},
		"invalid@input": {"validator:not-empty", "validator:min-length", "validator:alphanumeric"},
		"ValidInput123": {"validator:not-empty", "validator:min-length", "validator:alphanumeric"},
	} {
		fmt.Printf("\nTesting: '%s'\n", input)
		for _, validatorKey := range test {
			validator := di.GetByLookupKey[ValidatorFunc](container3, validatorKey)
			valid, err := validator(input)
			status := "✅"
			if !valid {
				status = "❌"
			}
			fmt.Printf("  %s %s: ", status, validators[validatorKey])
			if valid {
				fmt.Println("PASS")
			} else {
				fmt.Printf("FAIL - %v\n", err)
			}
		}
	}

	// Example 4: Transform Functions (Pipeline Pattern)
	fmt.Println("\n=== Transform Functions Pipeline ===\n")

	builder4 := di.Builder()

	// Register multiple transform functions
	di.AddFuncWithLookupKeys[TransformFunc](builder4,
		func() TransformFunc {
			return func(input string) string {
				return strings.ToUpper(input)
			}
		},
		[]string{"transform:uppercase"},
		nil)

	di.AddFuncWithLookupKeys[TransformFunc](builder4,
		func() TransformFunc {
			return func(input string) string {
				return strings.TrimSpace(input)
			}
		},
		[]string{"transform:trim"},
		nil)

	di.AddFuncWithLookupKeys[TransformFunc](builder4,
		func() TransformFunc {
			return func(input string) string {
				return strings.ReplaceAll(input, " ", "-")
			}
		},
		[]string{"transform:hyphenate"},
		nil)

	container4 := builder4.Build()

	// Build a transformation pipeline
	input := "  Hello World From DI  "
	fmt.Printf("Input: '%s'\n\n", input)

	trim := di.GetByLookupKey[TransformFunc](container4, "transform:trim")
	uppercase := di.GetByLookupKey[TransformFunc](container4, "transform:uppercase")
	hyphenate := di.GetByLookupKey[TransformFunc](container4, "transform:hyphenate")

	// Apply transformations
	step1 := trim(input)
	fmt.Printf("After trim: '%s'\n", step1)

	step2 := uppercase(step1)
	fmt.Printf("After uppercase: '%s'\n", step2)

	step3 := hyphenate(step2)
	fmt.Printf("After hyphenate: '%s'\n", step3)

	// Example 5: Factory Function Pattern
	fmt.Println("\n=== Factory Function Pattern ===\n")

	type UserCreatorFunc func(name string, age int) map[string]interface{}

	builder5 := di.Builder()

	di.AddFunc[UserCreatorFunc](builder5, func() UserCreatorFunc {
		idCounter := 0
		return func(name string, age int) map[string]interface{} {
			idCounter++
			return map[string]interface{}{
				"id":        idCounter,
				"name":      name,
				"age":       age,
				"createdAt": time.Now().Format(time.RFC3339),
			}
		}
	})

	container5 := builder5.Build()

	createUser := di.Get[UserCreatorFunc](container5)

	// Create multiple users - note that the counter is preserved (closure)
	users := []map[string]interface{}{
		createUser("Alice", 30),
		createUser("Bob", 25),
		createUser("Charlie", 35),
	}

	fmt.Println("Created users:")
	for _, user := range users {
		fmt.Printf("  ID: %v, Name: %v, Age: %v, Created: %v\n",
			user["id"], user["name"], user["age"], user["createdAt"])
	}

	// Example 6: Function as Strategy Pattern
	fmt.Println("\n=== Function as Strategy Pattern ===\n")

	type CalculationFunc func(a, b float64) float64

	builder6 := di.Builder()

	// Register multiple calculation strategies
	di.AddFuncWithLookupKeys[CalculationFunc](builder6,
		func() CalculationFunc {
			return func(a, b float64) float64 { return a + b }
		},
		[]string{"calc:add"},
		nil)

	di.AddFuncWithLookupKeys[CalculationFunc](builder6,
		func() CalculationFunc {
			return func(a, b float64) float64 { return a - b }
		},
		[]string{"calc:subtract"},
		nil)

	di.AddFuncWithLookupKeys[CalculationFunc](builder6,
		func() CalculationFunc {
			return func(a, b float64) float64 { return a * b }
		},
		[]string{"calc:multiply"},
		nil)

	di.AddFuncWithLookupKeys[CalculationFunc](builder6,
		func() CalculationFunc {
			return func(a, b float64) float64 {
				if b == 0 {
					return 0
				}
				return a / b
			}
		},
		[]string{"calc:divide"},
		nil)

	container6 := builder6.Build()

	// Use different calculation strategies
	a, b := 10.0, 5.0
	operations := []string{"calc:add", "calc:subtract", "calc:multiply", "calc:divide"}
	symbols := map[string]string{
		"calc:add":      "+",
		"calc:subtract": "-",
		"calc:multiply": "*",
		"calc:divide":   "/",
	}

	fmt.Printf("Calculations with a=%.1f, b=%.1f:\n", a, b)
	for _, op := range operations {
		calc := di.GetByLookupKey[CalculationFunc](container6, op)
		result := calc(a, b)
		fmt.Printf("  %.1f %s %.1f = %.1f\n", a, symbols[op], b, result)
	}

	// Example 7: Middleware Pattern with Functions
	fmt.Println("\n=== Middleware Pattern with Functions ===\n")

	type MiddlewareFunc func(next ProcessorFunc) ProcessorFunc

	builder7 := di.Builder()

	// Register logger
	di.AddSingleton[*Logger](builder7, func() *Logger {
		return &Logger{Prefix: "MIDDLEWARE"}
	})

	// Register middleware functions
	di.AddFuncWithLookupKeys[MiddlewareFunc](builder7,
		func(logger *Logger) MiddlewareFunc {
			return func(next ProcessorFunc) ProcessorFunc {
				return func(data string) string {
					logger.Log("Before processing")
					result := next(data)
					logger.Log("After processing")
					return result
				}
			}
		},
		[]string{"middleware:logging"},
		nil)

	di.AddFuncWithLookupKeys[MiddlewareFunc](builder7,
		func() MiddlewareFunc {
			return func(next ProcessorFunc) ProcessorFunc {
				return func(data string) string {
					result := next(data)
					return fmt.Sprintf("[WRAPPED: %s]", result)
				}
			}
		},
		[]string{"middleware:wrap"},
		nil)

	container7 := builder7.Build()

	// Base processor
	baseProcessor := func(data string) string {
		return strings.ToUpper(data)
	}

	// Apply middleware
	loggingMiddleware := di.GetByLookupKey[MiddlewareFunc](container7, "middleware:logging")
	wrapMiddleware := di.GetByLookupKey[MiddlewareFunc](container7, "middleware:wrap")

	// Chain middleware: wrap(logging(base))
	enhancedProcessor := wrapMiddleware(loggingMiddleware(baseProcessor))

	fmt.Println("\nProcessing with middleware:")
	finalResult := enhancedProcessor("hello")
	fmt.Printf("Result: %s\n", finalResult)

	fmt.Println("\n✓ Function injection example completed!")
}
