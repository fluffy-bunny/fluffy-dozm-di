# fluffy-dozm-di

This is a new project based on [dozm/di](https://github.com/dozm/di). The main reason for the deviation is addition of features that do not exist in the original.

## Features

The features added are;

### The ability to add an object that implements many interfaces.

I would like to add an object that MAY implement a lot of interfaces, but in this case I want to only register a subset of them. You may have an object that you would like to new with different inputs and more importantly cherry pick which interfaces get registered in the DI. You may not want to register the object itself, but only the Interface. I couldn't do this with the original dozm/di and even with asp.net's di on which dozm/di was based on.

### The ability to register by an lookup key and fetch by the lookup key

I would like to register an object by a name. i.e. "my-awesome-object".

A dependency injection module based on reflection.

## Installation

```sh
go get -u github.com/fluffy-bunny/fluffy-dozm-di
```

## Quick start

```go
package main

import (
    "fmt"
    di "github.com/fluffy-bunny/fluffy-dozm-di"
)

func main() {
    // Create a ContainerBuilder
    b := di.Builder()

    // Register some services with generic helper function.
    di.AddSingleton[string](b, func() string { return "hello" })
    di.AddTransient[int](b, func() int { return 1 })
    di.AddScoped[int](b, func() int { return 2 })

    // Build the container
    c := b.Build()

    // Usually, you should not resolve a service directly from the root scope.
    // So, get the di.ScopeFactory (it's a built-in service) to create a scope.
    // Typically, in web application we create a scope for per HTTP request.
    scopeFactory := di.Get[di.ScopeFactory](c)
    scope := scopeFactory.CreateScope()
    c = scope.Container()

    // Get a service from the container
    s := di.Get[string](c)
    fmt.Println(s)

    // Get all of the services with the type int as a slice.
    intSlice := di.Get[[]int](c)
    fmt.Println(intSlice)
}
```

## Register a service that supports many interfaces.

```go
import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/fluffy-bunny/fluffy-dozm-di/reflectx"
	"github.com/stretchr/testify/require"
)
type department struct {
    Name       string
    SecretName string
    Time       ITime
}

func AddSingletonDepartments(b ContainerBuilder, names ...string) {
	// pointer to interface type
	typeIDepartment := reflect.TypeOf((*IDepartment)(nil))
	// elem of pointer to interface type
	typeIDepartment2 := reflectx.TypeOf[IDepartment2]()

	for idx := range names {
		name := names[idx]
		secretName := fmt.Sprintf("%s-FBI", name)
		fmt.Println("registering department:", name, " secretname:", secretName)
		AddSingleton[*department](b, func(tt ITime) *department {
			return &department{
				Name:       name,
				Time:       tt,
				SecretName: secretName,
			}
		}, typeIDepartment, typeIDepartment2)
	}
}
```

### Same registration using `ImplementedInterfaceType[T]()`

```go
func AddSingletonDepartments(b di.ContainerBuilder, names ...string) {
	for idx := range names {
		name := names[idx]
		secretName := fmt.Sprintf("%s-FBI", name)

		di.AddSingleton[*department](b,
			func(tt ITime) *department {
				return &department{
					Name:       name,
					Time:       tt,
					SecretName: secretName,
				}
			},
			di.ImplementedInterfaceType[IDepartment](),
			di.ImplementedInterfaceType[IDepartment2](),
		)
	}
}
```

This avoids manual `reflect.TypeOf((*MyInterface)(nil))` calls and keeps interface registration strongly typed.

## Add by lookup key

```go
func AddSingletonEmployeesWithLookupKeys(b ContainerBuilder) {
	AddSingletonWithLookupKeys[*employee](b,
		func() *employee {
			return &employee{Name: "1"}
		}, []string{"1"}, map[string]interface{}{"name": "1"},
		reflect.TypeOf((*IEmployee)(nil)))
	AddSingletonWithLookupKeys[*employee](b,
		func() *employee {
			return &employee{Name: "2a"}
		}, []string{"2"}, map[string]interface{}{"name": "2a"},
		reflect.TypeOf((*IEmployee)(nil)))
	AddSingletonWithLookupKeys[*employee](b,
		func() *employee {
			return &employee{Name: "2"}
		}, []string{"2"}, map[string]interface{}{"name": "2"},
		reflect.TypeOf((*IEmployee)(nil)))
}

func TestManyWithSingletonWithLookupKeys(t *testing.T) {
	b := Builder()
	// Build the container
	AddSingletonEmployeesWithLookupKeys(b)
	c := b.Build()
	scopeFactory := Get[ScopeFactory](c)
	scope1 := scopeFactory.CreateScope()
	employees := Get[[]IEmployee](scope1.Container())
	require.Equal(t, 3, len(employees))
	require.NotPanics(t, func() {
		h := GetByLookupKey[IEmployee](c, "1")
		require.NotNil(t, h)
		require.Equal(t, "1", h.GetName())
	})
	require.NotPanics(t, func() {
		h := GetByLookupKey[IEmployee](c, "2")
		require.NotNil(t, h)
		require.Equal(t, "2", h.GetName())
	})
}
```

## Resolution Order: Last Registration Wins

When the same service type (interface) is registered multiple times with different lifetimes, the **last registration wins**. This follows standard DI convention (same as ASP.NET Core).

### Rules

| Container | Who Wins? | Rule |
|-----------|-----------|------|
| **Root container** | The **last registered** descriptor, regardless of lifetime | `descriptor.Last()` is used during call site creation |
| **Scoped container** | The **last registered** descriptor, regardless of lifetime | Same call site factory is shared; resolution order is identical |

The lifetime of the winning descriptor then determines caching behavior:

| Lifetime | Behavior |
|----------|----------|
| **Singleton** | One instance for the lifetime of the root container |
| **Scoped** | One instance per scope (acts as singleton when resolved from root) |
| **Transient** | New instance every time |

### Example

```go
b := di.Builder()

// Register ISomething three times with different lifetimes
di.AddTransient[ISomething](b, func() ISomething { return &somethingTransient{} })
di.AddSingleton[ISomething](b, func() ISomething { return &somethingSingleton{} })
di.AddScoped[ISomething](b, func() ISomething { return &somethingScoped{} })

c := b.Build()

// Scoped was registered last, so it wins
result := di.Get[ISomething](c) // returns somethingScoped

// ALL registrations are still available as a slice
all := di.Get[[]ISomething](c) // returns [somethingTransient, somethingSingleton, somethingScoped]
```

### Detecting Lifetime Conflicts

If you want to **panic at build time** when the same service type is registered with different lifetimes, enable `DetectLifetimeConflicts`:

```go
b := di.Builder()
b.ConfigureOptions(func(o *di.Options) {
    o.DetectLifetimeConflicts = true
})

di.AddTransient[ISomething](b, func() ISomething { return &somethingTransient{} })
di.AddSingleton[ISomething](b, func() ISomething { return &somethingSingleton{} })

// This will panic with:
//   "service type 'ISomething' is registered with conflicting lifetimes: [Transient Singleton]"
c := b.Build()
```

> **Note**: Multiple registrations with the **same** lifetime are allowed and will not trigger a panic. Only mixed lifetimes for the same type are flagged.
