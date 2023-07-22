# fluffy-dozm-di

This is a new project based on [dozm/di](https://github.com/dozm/di).  The main reason for the deviation is addition of features that do not exist in the original.

## Features

The features added are;

### The ability to add an object that implements many interfaces.  

I would like to add an object that MAY implement a lot of interfaces, but in this case I want to only register a subset of them.  You may have an object that you would like to new with different inputs and more importantly cherry pick which interfaces get registered in the DI.  You may not want to register the object itself, but only the Interface.  I couldn't do this with the original dozm/di and even with asp.net's di on which dozm/di was based on.

### The ability to register by an lookup key and fetch by the lookup key

I would like to register an object by a name.  i.e. "my-awesome-object".



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
