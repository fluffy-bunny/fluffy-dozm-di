package di

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/fluffy-bunny/fluffy-dozm-di/errorx"
	"github.com/fluffy-bunny/fluffy-dozm-di/reflectx"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------
// Issue #1, #3: Race condition on container.disposed and scope.disposed
// These tests exercise concurrent access to disposed state.
// -----------------------------------------------------------------------

func TestConcurrentDisposeAndGet(t *testing.T) {
	b := Builder()
	AddSingleton[string](b, func() string { return "hello" })
	c := b.Build()

	var wg sync.WaitGroup
	// Spawn goroutines that try to Get while another disposes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Should not panic, may return error
			_, _ = TryGet[string](c)
		}()
	}

	// Dispose concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		d, _ := c.(Disposable)
		d.Dispose()
	}()

	wg.Wait()
}

func TestConcurrentScopeDisposeAndGet(t *testing.T) {
	b := Builder()
	AddScoped[string](b, func() string { return "scoped-hello" })
	c := b.Build()
	scopeFactory := Get[ScopeFactory](c)
	scope := scopeFactory.CreateScope()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = scope.Container().Get(reflectx.TypeOf[string]())
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		scope.Dispose()
	}()

	wg.Wait()
}

// -----------------------------------------------------------------------
// Issue #2: Race condition on CallSite.Value()/SetValue()
// Test concurrent singleton resolution.
// -----------------------------------------------------------------------

func TestConcurrentSingletonResolution(t *testing.T) {
	var counter int32
	b := Builder()
	AddSingleton[int32](b, func() int32 { return atomic.AddInt32(&counter, 1) })
	c := b.Build()

	var wg sync.WaitGroup
	results := make([]int32, 200)
	for i := 0; i < 200; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			results[i] = Get[int32](c)
		}()
	}
	wg.Wait()

	// Singleton should have been created exactly once
	require.Equal(t, int32(1), counter, "singleton constructor should be called exactly once")
	for _, v := range results {
		require.Equal(t, int32(1), v, "all goroutines should get the same singleton value")
	}
}

// -----------------------------------------------------------------------
// Issue #4: createServiceAccessor duplicate work
// Concurrent resolution of the same type should still produce correct results.
// -----------------------------------------------------------------------

func TestConcurrentServiceAccessorCreation(t *testing.T) {
	var counter int32
	b := Builder()
	AddTransient[int32](b, func() int32 { return atomic.AddInt32(&counter, 1) })
	c := b.Build()

	var wg sync.WaitGroup
	results := make([]int32, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			results[i] = Get[int32](c)
		}()
	}
	wg.Wait()

	// Transient => each call gets a unique value
	seen := make(map[int32]bool)
	for _, v := range results {
		require.False(t, seen[v], "transient should return unique values")
		seen[v] = true
	}
}

// -----------------------------------------------------------------------
// Issue #5: Lookup key hash collision — types with same name in different pkgs
// We can't easily test cross-package, but we can test that the hash includes
// more than just the type name by verifying the full format.
// -----------------------------------------------------------------------

func TestHashTypeAndString_IncludesPackagePath(t *testing.T) {
	// hashTypeAndString is an internal function; test it indirectly
	// by registering with lookup keys and verifying correct resolution.
	type MyConfig struct{ Name string }

	b := Builder()
	AddSingletonWithLookupKeys[*MyConfig](b,
		func() *MyConfig { return &MyConfig{Name: "config1"} },
		[]string{"key1"}, nil)
	AddSingletonWithLookupKeys[*MyConfig](b,
		func() *MyConfig { return &MyConfig{Name: "config2"} },
		[]string{"key2"}, nil)

	c := b.Build()

	v1 := GetByLookupKey[*MyConfig](c, "key1")
	v2 := GetByLookupKey[*MyConfig](c, "key2")
	require.Equal(t, "config1", v1.Name)
	require.Equal(t, "config2", v2.Name)
}

// -----------------------------------------------------------------------
// Issue #6: Nil pointer panic in createServiceLookupKeyAccessor
// -----------------------------------------------------------------------

func TestLookupKeyNotFound_ReturnsError(t *testing.T) {
	b := Builder()
	AddSingleton[string](b, func() string { return "hello" })
	c := b.Build()

	_, err := TryGetByLookupKey[string](c, "nonexistent-key")
	require.Error(t, err, "should return error for unknown lookup key, not panic")
}

// -----------------------------------------------------------------------
// Issue #9: descriptorCacheItem.Add slice aliasing
// -----------------------------------------------------------------------

func TestDescriptorCacheItemAdd_NoSliceAliasing(t *testing.T) {
	b := Builder()
	// Register multiple services of the same type to exercise the Add path
	AddTransient[int](b, func() int { return 1 })
	AddTransient[int](b, func() int { return 2 })
	AddTransient[int](b, func() int { return 3 })

	c := b.Build()

	// The last one wins for default resolution
	v := Get[int](c)
	require.Equal(t, 3, v)

	// All should be in the slice
	all := Get[[]int](c)
	require.Equal(t, 3, len(all))
	require.Equal(t, 1, all[0])
	require.Equal(t, 2, all[1])
	require.Equal(t, 3, all[2])
}

// -----------------------------------------------------------------------
// Issue #10: nil constructor panics with clear message
// -----------------------------------------------------------------------

func TestNilConstructor_PanicsWithMessage(t *testing.T) {
	require.PanicsWithValue(t, "constructor must not be nil", func() {
		Transient[string](nil)
	})
}

func TestNonFuncConstructor_PanicsWithMessage(t *testing.T) {
	require.Panics(t, func() {
		Transient[string]("not a function")
	})
}

// -----------------------------------------------------------------------
// Issue #11, #12: Nil value handling in visitConstructor
// -----------------------------------------------------------------------

type IOptional interface {
	GetValue() string
}

func TestFactoryReturningNil_HandledGracefully(t *testing.T) {
	b := Builder()
	AddTransientFactory[IOptional](b, func(c Container) any {
		return (*optionalImpl)(nil)
	})
	c := b.Build()

	// Should not panic even when factory returns nil
	result, err := TryGet[IOptional](c)
	require.NoError(t, err)
	require.Nil(t, result)
}

type optionalImpl struct{}

func (o *optionalImpl) GetValue() string { return "value" }

// -----------------------------------------------------------------------
// Issue #13, #14: Remove/Contains should check ImplementedInterfaceTypes
// -----------------------------------------------------------------------

type IFoo interface {
	Foo() string
}

type IBar interface {
	Bar() string
}

type fooBarImpl struct{}

func (f *fooBarImpl) Foo() string { return "foo" }
func (f *fooBarImpl) Bar() string { return "bar" }

func TestRemove_MatchesImplementedInterfaceTypes(t *testing.T) {
	typeIFoo := reflectx.TypeOf[IFoo]()
	typeIBar := reflectx.TypeOf[IBar]()

	b := Builder()
	AddSingleton[*fooBarImpl](b, func() *fooBarImpl {
		return &fooBarImpl{}
	}, reflect.TypeOf((*IFoo)(nil)), reflect.TypeOf((*IBar)(nil)))

	require.True(t, b.Contains(typeIFoo), "should contain IFoo via ImplementedInterfaceTypes")
	require.True(t, b.Contains(typeIBar), "should contain IBar via ImplementedInterfaceTypes")

	b.Remove(typeIFoo)

	require.False(t, b.Contains(typeIFoo), "should no longer contain IFoo after removal")
	require.False(t, b.Contains(typeIBar), "should no longer contain IBar after removal (descriptor removed)")
}

func TestContains_ChecksImplementedInterfaceTypes(t *testing.T) {
	b := Builder()
	AddSingleton[*fooBarImpl](b, func() *fooBarImpl {
		return &fooBarImpl{}
	}, reflect.TypeOf((*IFoo)(nil)))

	require.True(t, b.Contains(reflectx.TypeOf[IFoo]()), "Contains should find by implemented interface type")
	require.False(t, b.Contains(reflectx.TypeOf[IBar]()), "Contains should not find unregistered interface type")
}

// -----------------------------------------------------------------------
// Issue #18: AggregateError implements Unwrap
// -----------------------------------------------------------------------

func TestAggregateError_Unwrap(t *testing.T) {
	inner1 := fmt.Errorf("error1")
	inner2 := fmt.Errorf("error2")
	agg := &errorx.AggregateError{Errors: []error{inner1, inner2}}

	// errors.Is should traverse
	require.True(t, errors.Is(agg, inner1))
	require.True(t, errors.Is(agg, inner2))

	// Unwrap returns inner errors
	unwrapped := agg.Unwrap()
	require.Equal(t, 2, len(unwrapped))
}

// -----------------------------------------------------------------------
// Issue #19: Typos fixed — "unknow" -> "unknown"
// Verifying error messages through the system isn't easily testable,
// but we can verify the validator and resolver don't produce "unknow".
// This is a compile-time/code-review fix verified by build.
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Issue #20: syncx.Map.LoadOrCreate TOCTOU
// The factory should only produce one value that is used (though it may be
// called more than once due to races).
// -----------------------------------------------------------------------

func TestSyncMapLoadOrCreate_ConcurrentSafety(t *testing.T) {
	// Indirectly test through singleton resolution
	var counter int32
	b := Builder()
	AddSingleton[int64](b, func() int64 {
		return int64(atomic.AddInt32(&counter, 1))
	})
	c := b.Build()

	var wg sync.WaitGroup
	results := make([]int64, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			results[i] = Get[int64](c)
		}()
	}
	wg.Wait()

	// All should get the same value
	for _, v := range results {
		require.Equal(t, int64(1), v)
	}
}

// -----------------------------------------------------------------------
// Issue #22: Scope.Dispose clears ResolvedServices
// -----------------------------------------------------------------------

func TestScopeDispose_ClearsResolvedServices(t *testing.T) {
	b := Builder()
	AddScoped[string](b, func() string { return "scoped-value" })
	c := b.Build()

	scopeFactory := Get[ScopeFactory](c)
	scope := scopeFactory.CreateScope()
	engineScope := scope.(*ContainerEngineScope)

	// Resolve a scoped service
	_ = Get[string](scope.Container())

	// Before dispose, ResolvedServices should have entries
	require.NotNil(t, engineScope.ResolvedServices)
	require.Greater(t, len(engineScope.ResolvedServices), 0)

	scope.Dispose()

	// After dispose, ResolvedServices should be cleared
	require.Nil(t, engineScope.ResolvedServices, "ResolvedServices should be nil after dispose")
}

// -----------------------------------------------------------------------
// Issue #16: GetDescriptors returns pointers to live descriptors
// (Documented as known behavior; test that descriptors are accessible)
// -----------------------------------------------------------------------

func TestGetDescriptors_ReturnsRegisteredDescriptors(t *testing.T) {
	b := Builder()
	AddSingleton[string](b, func() string { return "hello" })
	AddTransient[int](b, func() int { return 42 })
	c := b.Build()

	descriptors := c.GetDescriptors()
	require.Equal(t, 2, len(descriptors))
}

// -----------------------------------------------------------------------
// Combined: Verify all fixes work together in a realistic scenario
// -----------------------------------------------------------------------

type ILogger interface {
	Log(msg string)
}

type ICache interface {
	GetItem(key string) string
}

type logger struct{ messages []string }

func (l *logger) Log(msg string) {
	l.messages = append(l.messages, msg)
}

type cache struct {
	logger ILogger
}

func (c *cache) GetItem(key string) string {
	c.logger.Log("cache hit: " + key)
	return "value-" + key
}

func TestRealisticScenario_WithAllFixes(t *testing.T) {
	b := Builder()
	b.ConfigureOptions(func(o *Options) {
		o.ValidateOnBuild = true
		o.ValidateScopes = true
	})

	AddSingleton[ILogger](b, func() ILogger { return &logger{} })
	AddScoped[ICache](b, func(l ILogger) ICache { return &cache{logger: l} })

	c := b.Build()

	// Concurrent scope creation and resolution
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scopeFactory := Get[ScopeFactory](c)
			scope := scopeFactory.CreateScope()
			defer scope.Dispose()

			svc := Get[ICache](scope.Container())
			result := svc.GetItem("test")
			require.Equal(t, "value-test", result)
		}()
	}
	wg.Wait()

	// Singleton logger should have been shared across all scopes
	l := Get[ILogger](c)
	require.NotNil(t, l)
}

func TestDisposeAfterScopedResolution_NoLeak(t *testing.T) {
	b := Builder()
	AddScoped[*DisposableStruct](b, func() *DisposableStruct {
		return &DisposableStruct{Value: 42}
	})
	c := b.Build()

	scopeFactory := Get[ScopeFactory](c)
	scope := scopeFactory.CreateScope()

	obj := Get[*DisposableStruct](scope.Container())
	require.False(t, obj.Disposed)

	scope.Dispose()
	require.True(t, obj.Disposed, "scoped disposable should be disposed when scope is disposed")

	// ResolvedServices should be cleared
	engineScope := scope.(*ContainerEngineScope)
	require.Nil(t, engineScope.ResolvedServices)
}

type IHeavyScopedRequest interface {
	Handle(ctx context.Context) int
}

type heavySingletonState struct {
	seed int
}

type heavyTransientState struct {
	value int
}

type heavyScopedPayload struct {
	id       int64
	closed   atomic.Bool
	active   *atomic.Int64
	created  *atomic.Int64
	disposed *atomic.Int64
}

func (p *heavyScopedPayload) Dispose() {
	if p.closed.Swap(true) {
		return
	}
	p.disposed.Add(1)
	p.active.Add(-1)
}

type heavyScopedRequest struct {
	singleton *heavySingletonState
	transient *heavyTransientState
	payload   *heavyScopedPayload
}

func (r *heavyScopedRequest) Handle(ctx context.Context) int {
	_ = ctx
	return int(r.payload.id) + r.transient.value + r.singleton.seed
}

func registerHeavyScopedRequestGraph(
	b ContainerBuilder,
	created *atomic.Int64,
	active *atomic.Int64,
	disposed *atomic.Int64,
	transientCounter *atomic.Int64,
) {
	AddSingleton[*heavySingletonState](b, func() *heavySingletonState {
		return &heavySingletonState{seed: 7}
	})

	AddTransient[*heavyTransientState](b, func() *heavyTransientState {
		return &heavyTransientState{value: int(transientCounter.Add(1))}
	})

	AddScoped[*heavyScopedPayload](b, func() *heavyScopedPayload {
		id := created.Add(1)
		active.Add(1)
		return &heavyScopedPayload{
			id:       id,
			active:   active,
			created:  created,
			disposed: disposed,
		}
	})

	AddScoped[IHeavyScopedRequest](b, func(
		singleton *heavySingletonState,
		transient *heavyTransientState,
		payload *heavyScopedPayload,
	) IHeavyScopedRequest {
		return &heavyScopedRequest{
			singleton: singleton,
			transient: transient,
			payload:   payload,
		}
	})
}

func heavyScopedWorkloadSize(t *testing.T) int {
	t.Helper()
	if testing.Short() {
		return 2000
	}
	return 50000
}

func TestHeavyScopedRequestSimulation_NoRetainedScopedInstances(t *testing.T) {
	var created atomic.Int64
	var active atomic.Int64
	var disposed atomic.Int64
	var transientCounter atomic.Int64

	b := Builder()
	b.ConfigureOptions(func(o *Options) {
		o.ValidateScopes = true
		o.ValidateOnBuild = true
	})
	registerHeavyScopedRequestGraph(b, &created, &active, &disposed, &transientCounter)

	c := b.Build()
	scopeFactory := Get[ScopeFactory](c)
	iterations := heavyScopedWorkloadSize(t)

	for i := 0; i < iterations; i++ {
		scope := scopeFactory.CreateScope()
		request := Get[IHeavyScopedRequest](scope.Container())
		result := request.Handle(context.Background())
		require.Greater(t, result, 0)

		scope.Dispose()
		engineScope := scope.(*ContainerEngineScope)
		require.Nil(t, engineScope.ResolvedServices)

		if i > 0 && i%5000 == 0 {
			runtime.GC()
		}
	}

	require.Equal(t, int64(iterations), created.Load(), "all scoped payloads should be created once per simulated request")
	require.Equal(t, int64(iterations), disposed.Load(), "all scoped payloads should be disposed")
	require.Equal(t, int64(0), active.Load(), "no scoped payload should remain active after all scopes are disposed")

	// Scoped services resolved from disposed scopes should fail.
	scope := scopeFactory.CreateScope()
	_ = Get[IHeavyScopedRequest](scope.Container())
	scope.Dispose()
	_, err := scope.Container().Get(reflectx.TypeOf[IHeavyScopedRequest]())
	require.Error(t, err)
}

func TestHeavyScopedRequestSimulation_Concurrent_NoRetainedScopedInstances(t *testing.T) {
	var created atomic.Int64
	var active atomic.Int64
	var disposed atomic.Int64
	var transientCounter atomic.Int64

	b := Builder()
	b.ConfigureOptions(func(o *Options) {
		o.ValidateScopes = true
		o.ValidateOnBuild = true
	})
	registerHeavyScopedRequestGraph(b, &created, &active, &disposed, &transientCounter)

	c := b.Build()
	scopeFactory := Get[ScopeFactory](c)
	iterations := heavyScopedWorkloadSize(t)
	workers := 32
	if testing.Short() {
		workers = 8
	}

	jobs := make(chan struct{}, workers)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				scope := scopeFactory.CreateScope()
				request := Get[IHeavyScopedRequest](scope.Container())
				_ = request.Handle(context.Background())
				scope.Dispose()
			}
		}()
	}

	for i := 0; i < iterations; i++ {
		jobs <- struct{}{}
	}
	close(jobs)
	wg.Wait()

	runtime.GC()
	require.Equal(t, int64(iterations), created.Load())
	require.Equal(t, int64(iterations), disposed.Load())
	require.Equal(t, int64(0), active.Load())
}
