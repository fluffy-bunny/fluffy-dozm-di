package main

import (
	"fmt"

	di "github.com/fluffy-bunny/fluffy-dozm-di"
)

// Demonstrate advanced DI patterns

// Strategy Pattern with DI
type PaymentStrategy interface {
	ProcessPayment(amount float64) string
}

type CreditCardPayment struct {
	Fee float64
}

func (c *CreditCardPayment) ProcessPayment(amount float64) string {
	total := amount + c.Fee
	return fmt.Sprintf("Credit Card: $%.2f (includes $%.2f fee)", total, c.Fee)
}

type PayPalPayment struct {
	Fee float64
}

func (p *PayPalPayment) ProcessPayment(amount float64) string {
	total := amount + p.Fee
	return fmt.Sprintf("PayPal: $%.2f (includes $%.2f fee)", total, p.Fee)
}

type CryptoPayment struct{}

func (c *CryptoPayment) ProcessPayment(amount float64) string {
	return fmt.Sprintf("Crypto: $%.2f (no fees)", amount)
}

// Chain of Responsibility with DI
type ValidationHandler interface {
	Validate(data string) error
	SetNext(handler ValidationHandler)
}

type LengthValidator struct {
	MinLength int
	Next      ValidationHandler
}

func (l *LengthValidator) Validate(data string) error {
	if len(data) < l.MinLength {
		return fmt.Errorf("length must be at least %d characters", l.MinLength)
	}
	if l.Next != nil {
		return l.Next.Validate(data)
	}
	return nil
}

func (l *LengthValidator) SetNext(handler ValidationHandler) {
	l.Next = handler
}

type AlphaNumericValidator struct {
	Next ValidationHandler
}

func (a *AlphaNumericValidator) Validate(data string) error {
	for _, char := range data {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
			return fmt.Errorf("must contain only alphanumeric characters")
		}
	}
	if a.Next != nil {
		return a.Next.Validate(data)
	}
	return nil
}

func (a *AlphaNumericValidator) SetNext(handler ValidationHandler) {
	a.Next = handler
}

// Decorator Pattern with DI
type MessageService interface {
	Send(message string) string
}

type BasicMessageService struct{}

func (b *BasicMessageService) Send(message string) string {
	return message
}

type EncryptionDecorator struct {
	Inner MessageService
}

func (e *EncryptionDecorator) Send(message string) string {
	encrypted := e.Inner.Send(message)
	return fmt.Sprintf("[ENCRYPTED: %s]", encrypted)
}

type LoggingDecorator struct {
	Inner MessageService
}

func (l *LoggingDecorator) Send(message string) string {
	fmt.Println("  [LOG] Sending message...")
	result := l.Inner.Send(message)
	fmt.Println("  [LOG] Message sent")
	return result
}

type CompressionDecorator struct {
	Inner MessageService
}

func (c *CompressionDecorator) Send(message string) string {
	compressed := c.Inner.Send(message)
	return fmt.Sprintf("[COMPRESSED: %s]", compressed)
}

func main() {
	fmt.Println("=== Advanced Patterns Example ===\n")

	// Example 1: Strategy Pattern
	fmt.Println("=== Strategy Pattern ===\n")

	builder1 := di.Builder()

	// Register multiple payment strategies
	di.AddTransient[PaymentStrategy](builder1, func() PaymentStrategy {
		return &CreditCardPayment{Fee: 2.50}
	})

	di.AddTransient[PaymentStrategy](builder1, func() PaymentStrategy {
		return &PayPalPayment{Fee: 1.50}
	})

	di.AddTransient[PaymentStrategy](builder1, func() PaymentStrategy {
		return &CryptoPayment{}
	})

	container1 := builder1.Build()

	// Get all payment strategies
	strategies := di.Get[[]PaymentStrategy](container1)
	fmt.Printf("Available payment methods: %d\n\n", len(strategies))

	amount := 100.00
	for i, strategy := range strategies {
		result := strategy.ProcessPayment(amount)
		fmt.Printf("%d. %s\n", i+1, result)
	}

	// Example 2: Chain of Responsibility
	fmt.Println("\n=== Chain of Responsibility ===\n")

	builder2 := di.Builder()

	// Register validators
	di.AddSingleton[*LengthValidator](builder2, func() *LengthValidator {
		return &LengthValidator{MinLength: 8}
	})

	di.AddSingleton[*AlphaNumericValidator](builder2, func() *AlphaNumericValidator {
		return &AlphaNumericValidator{}
	})

	// Create the validation chain
	di.AddSingleton[ValidationHandler](builder2, func(
		lengthVal *LengthValidator,
		alphaNumVal *AlphaNumericValidator,
	) ValidationHandler {
		// Set up the chain
		lengthVal.SetNext(alphaNumVal)
		return lengthVal
	})

	container2 := builder2.Build()

	validator := di.Get[ValidationHandler](container2)

	// Test validation
	testCases := []string{
		"short",
		"thisIsValid123",
		"invalid!@#",
		"ValidPass123",
	}

	for _, test := range testCases {
		err := validator.Validate(test)
		if err != nil {
			fmt.Printf("❌ '%s': %v\n", test, err)
		} else {
			fmt.Printf("✅ '%s': Valid\n", test)
		}
	}

	// Example 3: Decorator Pattern
	fmt.Println("\n=== Decorator Pattern ===\n")

	builder3 := di.Builder()

	// Register the base service
	di.AddSingleton[*BasicMessageService](builder3, func() *BasicMessageService {
		return &BasicMessageService{}
	})

	// Register decorated service with multiple decorators
	di.AddSingleton[MessageService](builder3, func(base *BasicMessageService) MessageService {
		// Wrap with multiple decorators
		var service MessageService = base
		service = &EncryptionDecorator{Inner: service}
		service = &CompressionDecorator{Inner: service}
		service = &LoggingDecorator{Inner: service}
		return service
	})

	container3 := builder3.Build()

	messageService := di.Get[MessageService](container3)
	result := messageService.Send("Hello, World!")
	fmt.Printf("\nFinal result: %s\n", result)

	// Example 4: Combining patterns - Service Locator with validation
	fmt.Println("\n=== Service Locator Pattern ===\n")

	builder4 := di.Builder()

	// Register multiple services with lookup keys
	di.AddSingletonWithLookupKeys[PaymentStrategy](builder4,
		func() PaymentStrategy {
			return &CreditCardPayment{Fee: 2.50}
		},
		[]string{"payment:credit-card"},
		map[string]interface{}{"type": "payment"})

	di.AddSingletonWithLookupKeys[PaymentStrategy](builder4,
		func() PaymentStrategy {
			return &PayPalPayment{Fee: 1.50}
		},
		[]string{"payment:paypal"},
		map[string]interface{}{"type": "payment"})

	di.AddSingletonWithLookupKeys[PaymentStrategy](builder4,
		func() PaymentStrategy {
			return &CryptoPayment{}
		},
		[]string{"payment:crypto"},
		map[string]interface{}{"type": "payment"})

	container4 := builder4.Build()

	// Service locator function
	getPaymentMethod := func(method string) PaymentStrategy {
		key := fmt.Sprintf("payment:%s", method)
		return di.GetByLookupKey[PaymentStrategy](container4, key)
	}

	// Use service locator
	methods := []string{"credit-card", "paypal", "crypto"}
	testAmount := 50.00

	for _, method := range methods {
		payment := getPaymentMethod(method)
		result := payment.ProcessPayment(testAmount)
		fmt.Printf("Using %s: %s\n", method, result)
	}

	// Example 5: Builder pattern with DI
	fmt.Println("\n=== Builder Pattern with DI ===\n")

	type EmailBuilder struct {
		From    string
		To      []string
		Subject string
		Body    string
	}

	type Email struct {
		From    string
		To      []string
		Subject string
		Body    string
	}

	builder5 := di.Builder()

	// Register builder factory
	di.AddTransient[*EmailBuilder](builder5, func() *EmailBuilder {
		return &EmailBuilder{
			From: "system@example.com",
			To:   make([]string, 0),
		}
	})

	// Register build function
	di.AddSingleton[func(*EmailBuilder) *Email](builder5, func() func(*EmailBuilder) *Email {
		return func(b *EmailBuilder) *Email {
			return &Email{
				From:    b.From,
				To:      b.To,
				Subject: b.Subject,
				Body:    b.Body,
			}
		}
	})

	container5 := builder5.Build()

	// Use the builder
	emailBuilder := di.Get[*EmailBuilder](container5)
	emailBuilder.To = []string{"user1@example.com", "user2@example.com"}
	emailBuilder.Subject = "Important Message"
	emailBuilder.Body = "This is an important message."

	buildFunc := di.Get[func(*EmailBuilder) *Email](container5)
	email := buildFunc(emailBuilder)

	fmt.Printf("Email created:\n")
	fmt.Printf("  From: %s\n", email.From)
	fmt.Printf("  To: %v\n", email.To)
	fmt.Printf("  Subject: %s\n", email.Subject)
	fmt.Printf("  Body: %s\n", email.Body)

	fmt.Println("\n✓ Advanced patterns example completed!")
}
