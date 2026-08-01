package di

import (
	"errors"
	"reflect"

	"github.com/fluffy-bunny/fluffy-dozm-di/errorx"
	"github.com/fluffy-bunny/fluffy-dozm-di/reflectx"
)

type Container interface {
	Get(reflect.Type) (any, error)
	GetDescriptors() []*Descriptor
	GetByLookupKey(reflect.Type, string) (any, error)
}

type Scope interface {
	Container() Container
	Dispose()
}

type ScopeFactory interface {
	CreateScope() Scope
}

// Optional service used to determine if the specified type is available from the Container.
type IsService interface {
	IsService(serviceType reflect.Type) bool
}

type Disposable interface {
	Dispose()
}

// Get service of the type T from the container c
func Get[T any](c Container) T {
	result, err := TryGet[T](c)
	if err != nil {
		panic(err)
	}
	return result
}

// Get service of the type T from the container c
func GetByLookupKey[T any](c Container, key string) T {
	result, err := TryGetByLookupKey[T](c, key)
	if err != nil {
		panic(err)
	}
	return result
}
func TryGetByLookupKey[T any](c Container, key string) (result T, err error) {
	t := reflectx.TypeOf[T]()
	v, err := c.GetByLookupKey(t, key)
	if err != nil {
		return
	}

	result, ok := v.(T)
	if !ok {
		err = &errorx.TypeIncompatibilityError{To: t, From: reflect.TypeOf(v)}
		return
	}

	return
}
func TryGet[T any](c Container) (result T, err error) {
	t := reflectx.TypeOf[T]()
	v, err := c.Get(t)
	if err != nil {
		return
	}

	result, ok := v.(T)
	if !ok {
		err = &errorx.TypeIncompatibilityError{To: t, From: reflect.TypeOf(v)}
		return
	}

	return
}

// InvokeScoped creates a fresh Scope from factory, resolves T from that
// scope's Container, and invokes fn with the resolved value -- disposing the
// scope once fn returns, whether fn returns an error, returns nil, or panics.
//
// Use this instead of resolving a Scoped registration once and reusing that
// one instance across many calls (e.g. via a long-lived "forever scope" kept
// around for callers with no natural per-call scope of their own) whenever T
// -- or anything reachable from its dependency graph -- holds mutable state
// that must not be shared across concurrent callers. Each call gets its own
// instance of T, and Dispose is guaranteed to run exactly once per call.
func InvokeScoped[T any](factory ScopeFactory, fn func(T) error) (err error) {
	scope := factory.CreateScope()
	defer scope.Dispose()

	service, err := TryGet[T](scope.Container())
	if err != nil {
		return err
	}
	return fn(service)
}

// InvokeScopedResult is InvokeScoped for callers that also need to return a
// value out of fn, in addition to an error.
func InvokeScopedResult[T any, R any](factory ScopeFactory, fn func(T) (R, error)) (result R, err error) {
	scope := factory.CreateScope()
	defer scope.Dispose()

	service, err := TryGet[T](scope.Container())
	if err != nil {
		return
	}
	return fn(service)
}

// Invoke the function fn.
// the input paramenters of the fn function will be resolved from the Container c.
func Invoke(c Container, fn any) (fnReturn []any, err error) {
	vfn := reflect.ValueOf(fn)
	if vfn.Kind() != reflect.Func {
		err = errors.New("fn is not a function")
		return
	}

	inputTypes := reflectx.GetInParameters(vfn.Type())

	inputs := make([]reflect.Value, len(inputTypes))
	for i, t := range inputTypes {
		v, e := c.Get(t)
		if e != nil {
			err = e
			return
		}

		inputs[i] = reflect.ValueOf(v)
	}

	ouputs := vfn.Call(inputs)
	numOutputs := len(ouputs)
	if numOutputs > 0 {
		fnReturn = make([]any, numOutputs)
		for i, v := range ouputs {
			fnReturn[i] = v.Interface()
		}
	}

	return
}
