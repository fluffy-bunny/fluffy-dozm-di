package main

import (
	"fmt"
	"reflect"

	di "github.com/fluffy-bunny/fluffy-dozm-di"
	"github.com/fluffy-bunny/fluffy-dozm-di/reflectx"
)

// Define interfaces
type ILogger interface {
	Log(message string)
}

type INotifier interface {
	Notify(message string)
}

type IProcessor interface {
	Process(data string) string
}

// Console Logger implements ILogger
type ConsoleLogger struct {
	Prefix string
}

func (c *ConsoleLogger) Log(message string) {
	fmt.Printf("[%s] %s\n", c.Prefix, message)
}

// Email Notifier implements INotifier
type EmailNotifier struct {
	From string
}

func (e *EmailNotifier) Notify(message string) {
	fmt.Printf("📧 Email from %s: %s\n", e.From, message)
}

// SMS Notifier implements INotifier
type SMSNotifier struct {
	PhoneNumber string
}

func (s *SMSNotifier) Notify(message string) {
	fmt.Printf("📱 SMS to %s: %s\n", s.PhoneNumber, message)
}

// Data Processor implements IProcessor
type DataProcessor struct {
	Name string
}

func (d *DataProcessor) Process(data string) string {
	return fmt.Sprintf("[%s processed: %s]", d.Name, data)
}

// Service that uses multiple implementations
type MultiService struct {
	Logger     ILogger
	Notifiers  []INotifier
	Processors []IProcessor
}

func NewMultiService(logger ILogger, notifiers []INotifier, processors []IProcessor) *MultiService {
	return &MultiService{
		Logger:     logger,
		Notifiers:  notifiers,
		Processors: processors,
	}
}

func (m *MultiService) ProcessAndNotify(data string) {
	m.Logger.Log(fmt.Sprintf("Starting to process: %s", data))

	// Process with all processors
	for _, processor := range m.Processors {
		result := processor.Process(data)
		m.Logger.Log(fmt.Sprintf("Processed result: %s", result))
	}

	// Notify all notifiers
	for _, notifier := range m.Notifiers {
		notifier.Notify(fmt.Sprintf("Data processed: %s", data))
	}

	m.Logger.Log("Processing complete")
}

// Implementation that supports multiple interfaces
type UnifiedService struct {
	ServiceName string
}

func (u *UnifiedService) Log(message string) {
	fmt.Printf("[UnifiedService:%s] LOG: %s\n", u.ServiceName, message)
}

func (u *UnifiedService) Notify(message string) {
	fmt.Printf("[UnifiedService:%s] NOTIFY: %s\n", u.ServiceName, message)
}

func (u *UnifiedService) Process(data string) string {
	return fmt.Sprintf("[UnifiedService:%s] PROCESSED: %s", u.ServiceName, data)
}

func main() {
	fmt.Println("=== Interface Registration Example ===\n")

	// Create container builder
	builder := di.Builder()

	// Register services by interface type
	// Method 1: Simple interface registration
	di.AddSingleton[ILogger](builder, func() ILogger {
		fmt.Println("→ Creating ConsoleLogger")
		return &ConsoleLogger{Prefix: "APP"}
	})

	// Method 2: Register multiple implementations of the same interface
	// These will be available as a slice
	di.AddSingleton[INotifier](builder, func() INotifier {
		fmt.Println("→ Creating EmailNotifier")
		return &EmailNotifier{From: "system@example.com"}
	})

	di.AddSingleton[INotifier](builder, func() INotifier {
		fmt.Println("→ Creating SMSNotifier")
		return &SMSNotifier{PhoneNumber: "+1-555-0100"}
	})

	// Register multiple processors
	di.AddSingleton[IProcessor](builder, func() IProcessor {
		fmt.Println("→ Creating DataProcessor 1")
		return &DataProcessor{Name: "Processor-Alpha"}
	})

	di.AddSingleton[IProcessor](builder, func() IProcessor {
		fmt.Println("→ Creating DataProcessor 2")
		return &DataProcessor{Name: "Processor-Beta"}
	})

	// Register MultiService with automatic injection of interfaces
	di.AddSingleton[*MultiService](builder, func(
		logger ILogger,
		notifiers []INotifier,
		processors []IProcessor,
	) *MultiService {
		fmt.Println("→ Creating MultiService")
		fmt.Printf("  - Received %d notifiers\n", len(notifiers))
		fmt.Printf("  - Received %d processors\n", len(processors))
		return NewMultiService(logger, notifiers, processors)
	})

	// Build container
	fmt.Println("\nBuilding container...\n")
	container := builder.Build()

	// Use the service
	fmt.Println("=== Using MultiService ===\n")
	service := di.Get[*MultiService](container)
	service.ProcessAndNotify("Important Data")

	// Get individual interfaces
	fmt.Println("\n=== Getting Individual Interfaces ===\n")
	logger := di.Get[ILogger](container)
	logger.Log("Direct logger access")

	// Get all notifiers as a slice
	notifiers := di.Get[[]INotifier](container)
	fmt.Printf("\nTotal notifiers registered: %d\n", len(notifiers))
	for i, notifier := range notifiers {
		notifier.Notify(fmt.Sprintf("Notification #%d", i+1))
	}

	// Get all processors as a slice
	processors := di.Get[[]IProcessor](container)
	fmt.Printf("\nTotal processors registered: %d\n", len(processors))
	for _, processor := range processors {
		result := processor.Process("test-data")
		fmt.Println(result)
	}

	// Example 2: Service implementing multiple interfaces
	fmt.Println("\n=== Multiple Interface Support ===\n")
	builder2 := di.Builder()

	// Get the reflect types for the interfaces
	typeILogger := reflect.TypeOf((*ILogger)(nil))
	typeINotifier := reflect.TypeOf((*INotifier)(nil))
	typeIProcessor := reflect.TypeOf((*IProcessor)(nil))

	// Register UnifiedService that implements all three interfaces
	di.AddSingleton[*UnifiedService](builder2,
		func() *UnifiedService {
			fmt.Println("→ Creating UnifiedService")
			return &UnifiedService{ServiceName: "Unified"}
		},
		typeILogger, typeINotifier, typeIProcessor)

	container2 := builder2.Build()

	// Retrieve as different interfaces - all point to the same instance
	unifiedLogger := di.Get[ILogger](container2)
	unifiedNotifier := di.Get[INotifier](container2)
	unifiedProcessor := di.Get[IProcessor](container2)

	fmt.Println("\nUsing unified service through different interfaces:")
	unifiedLogger.Log("Test message")
	unifiedNotifier.Notify("Test notification")
	result := unifiedProcessor.Process("test-data")
	fmt.Println(result)

	// Verify they're the same underlying instance
	fmt.Println("\nVerifying same instance:")
	unified := di.Get[*UnifiedService](container2)
	fmt.Printf("All interfaces point to same instance: %v\n",
		fmt.Sprintf("%p", unified) == fmt.Sprintf("%p", unifiedLogger.(*UnifiedService)))

	// Example 3: Using IsService to check registration
	fmt.Println("\n=== Checking Service Registration ===")
	isService := di.Get[di.IsService](container)

	fmt.Printf("ILogger registered: %v\n", isService.IsService(reflectx.TypeOf[ILogger]()))
	fmt.Printf("INotifier registered: %v\n", isService.IsService(reflectx.TypeOf[INotifier]()))
	fmt.Printf("IProcessor registered: %v\n", isService.IsService(reflectx.TypeOf[IProcessor]()))
	fmt.Printf("String registered: %v\n", isService.IsService(reflectx.TypeOf[string]()))

	fmt.Println("\n✓ Interface example completed!")
}
