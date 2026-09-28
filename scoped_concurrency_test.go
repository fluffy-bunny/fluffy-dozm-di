package di

import (
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fluffy-bunny/fluffy-dozm-di/errorx"
	"github.com/stretchr/testify/require"
)

// Deterministic tests for the scoped-construction handoff in visitScopeCache:
// one caller owns a service's cache slot while constructing it, and others
// wait on the scope's pendingCond. Timing is controlled with channels, and
// waitForWaiter observes the waiting state directly, so nothing relies on sleeps.

type ccService struct{ disposed atomic.Bool }

func (s *ccService) Dispose() { s.disposed.Store(true) }

// not zero-size: pointers to distinct zero-size values may compare equal,
// which would make the require.Same identity checks meaningless
type ccPlain struct{ _ byte }

// blockingCtor returns a constructor that signals started, then blocks until
// release is closed. Call n (1-based) returns results(n).
func blockingCtor[T any](started chan<- struct{}, release <-chan struct{}, calls *atomic.Int32, results func(n int32) (T, error)) func() (T, error) {
	return func() (T, error) {
		n := calls.Add(1)
		if n == 1 {
			started <- struct{}{}
			<-release
		}
		return results(n)
	}
}

func newScope(t *testing.T, c Container) *ContainerEngineScope {
	t.Helper()
	return c.(ScopeFactory).CreateScope().(*ContainerEngineScope)
}

// waitForWaiter blocks until some goroutine is parked in acquireSlot waiting
// on scope.pendingCond. The cond is created and waited on while holding
// scope.Locker, and Wait releases it, so seeing a non-nil cond under the lock
// means the waiter is already inside Wait.
func waitForWaiter(t *testing.T, scope *ContainerEngineScope) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		scope.Locker.Lock()
		waiting := scope.pendingCond != nil
		scope.Locker.Unlock()
		if waiting {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("no goroutine began waiting on the pending service")
}

type result[T any] struct {
	v   T
	err error
}

func resolveAsync[T any](c Container) <-chan result[T] {
	ch := make(chan result[T], 1)
	go func() {
		v, err := TryGet[T](c)
		ch <- result[T]{v, err}
	}()
	return ch
}

func await[T any](t *testing.T, ch <-chan result[T]) result[T] {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock: resolution did not complete")
		return result[T]{}
	}
}

// Scope disposed while a Disposable scoped service is being constructed: the
// new instance is disposed at once, and both the owner and a waiter get errors.
func TestScopedConcurrency_DisposeDuringConstruction_Disposable(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	var built *ccService
	b := Builder()
	AddScoped[*ccService](b, blockingCtor(started, release, &calls, func(int32) (*ccService, error) {
		built = &ccService{}
		return built, nil
	}))
	scope := newScope(t, b.Build())

	owner := resolveAsync[*ccService](scope)
	<-started
	waiter := resolveAsync[*ccService](scope)
	waitForWaiter(t, scope)

	scope.Dispose()
	close(release)

	require.Error(t, await(t, owner).err)
	require.True(t, built.disposed.Load(), "instance built after dispose must be disposed immediately")

	var disposedErr *errorx.ObjectDisposedError
	require.ErrorAs(t, await(t, waiter).err, &disposedErr)
	require.EqualValues(t, 1, calls.Load(), "waiter must not construct into a disposed scope")
}

// Same, for a service that is not Disposable: storeResolved must return an
// error rather than write into the scope's cleared (nil) cache.
func TestScopedConcurrency_DisposeDuringConstruction_NotDisposable(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	b := Builder()
	AddScoped[*ccPlain](b, blockingCtor(started, release, &calls, func(int32) (*ccPlain, error) {
		return &ccPlain{}, nil
	}))
	scope := newScope(t, b.Build())

	owner := resolveAsync[*ccPlain](scope)
	<-started
	scope.Dispose()
	close(release)

	var disposedErr *errorx.ObjectDisposedError
	require.ErrorAs(t, await(t, owner).err, &disposedErr)
}

// The owner's construction fails (by error or panic) while another caller
// waits: the waiter wakes, takes over, constructs exactly once more, and the
// scope then caches that instance.
func TestScopedConcurrency_OwnerFailsWhileWaiterWaits(t *testing.T) {
	for name, fail := range map[string]func() (*ccPlain, error){
		"error": func() (*ccPlain, error) { return nil, errors.New("boom") },
		"panic": func() (*ccPlain, error) { panic("boom") },
	} {
		t.Run(name, func(t *testing.T) {
			started, release := make(chan struct{}), make(chan struct{})
			var calls atomic.Int32
			b := Builder()
			AddScoped[*ccPlain](b, blockingCtor(started, release, &calls, func(n int32) (*ccPlain, error) {
				if n == 1 {
					return fail()
				}
				return &ccPlain{}, nil
			}))
			scope := newScope(t, b.Build())
			defer scope.Dispose()

			owner := resolveAsync[*ccPlain](scope)
			<-started
			waiter := resolveAsync[*ccPlain](scope)
			waitForWaiter(t, scope)
			close(release)

			require.Error(t, await(t, owner).err)
			w := await(t, waiter)
			require.NoError(t, w.err)
			require.NotNil(t, w.v)
			require.EqualValues(t, 2, calls.Load())
			require.Same(t, w.v, Get[*ccPlain](scope))
			require.EqualValues(t, 2, calls.Load())
		})
	}
}

// Many waiters on one slow construction all receive the same instance.
func TestScopedConcurrency_ManyWaitersShareInstance(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	b := Builder()
	AddScoped[*ccPlain](b, blockingCtor(started, release, &calls, func(int32) (*ccPlain, error) {
		return &ccPlain{}, nil
	}))
	scope := newScope(t, b.Build())
	defer scope.Dispose()

	owner := resolveAsync[*ccPlain](scope)
	<-started
	waiters := make([]<-chan result[*ccPlain], 16)
	for i := range waiters {
		waiters[i] = resolveAsync[*ccPlain](scope)
	}
	waitForWaiter(t, scope)
	close(release)

	want := await(t, owner).v
	for _, w := range waiters {
		r := await(t, w)
		require.NoError(t, r.err)
		require.Same(t, want, r.v)
	}
	require.EqualValues(t, 1, calls.Load())
}

// A slow scoped service must not block other services in the same scope:
// construction is serialized per service, not per scope.
func TestScopedConcurrency_UnrelatedServiceNotBlocked(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	b := Builder()
	AddScoped[*ccService](b, blockingCtor(started, release, &calls, func(int32) (*ccService, error) {
		return &ccService{}, nil
	}))
	AddScoped[*ccPlain](b, func() *ccPlain { return &ccPlain{} })
	scope := newScope(t, b.Build())
	defer scope.Dispose()

	slow := resolveAsync[*ccService](scope)
	<-started
	require.NoError(t, await(t, resolveAsync[*ccPlain](scope)).err)
	close(release)
	require.NoError(t, await(t, slow).err)
}

// Each scope gets its own instance even when scopes resolve concurrently.
func TestScopedConcurrency_ScopesIsolated(t *testing.T) {
	b := Builder()
	AddScoped[*ccPlain](b, func() *ccPlain { return &ccPlain{} })
	c := b.Build()

	const n = 16
	got := make([]*ccPlain, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := newScope(t, c)
			defer s.Dispose()
			got[i] = Get[*ccPlain](s)
		}()
	}
	wg.Wait()
	seen := map[*ccPlain]bool{}
	for _, v := range got {
		require.False(t, seen[v], "scopes must not share scoped instances")
		seen[v] = true
	}
}

// First resolution of a singleton through its interface and its concrete type
// at the same moment used to be able to build two call sites (and so two
// singletons); tryCreateExact now publishes call sites with LoadOrStore.
// This is a stress test: it detects a regression with high probability, not certainty.
func TestScopedConcurrency_SingletonFirstResolutionRace(t *testing.T) {
	for range 200 {
		var calls atomic.Int32
		b := Builder()
		AddSingleton[*regA](b, func() *regA { calls.Add(1); return &regA{} }, ImplementedInterfaceType[regIFoo]())
		c := b.Build()

		var viaIface regIFoo
		var viaConcrete *regA
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); <-start; viaIface = Get[regIFoo](c) }()
		go func() { defer wg.Done(); <-start; viaConcrete = Get[*regA](c) }()
		close(start)
		wg.Wait()

		require.Same(t, viaConcrete, viaIface.(*regA))
		require.EqualValues(t, 1, calls.Load())
	}
}

// Lookup-key resolution converts constructor panics into errors, like Get.
func TestLookupKey_PanicBecomesError(t *testing.T) {
	for name, p := range map[string]any{"error": errors.New("boom"), "string": "boom"} {
		t.Run(name, func(t *testing.T) {
			b := Builder()
			AddTransientWithLookupKeys[*ccPlain](b, func() *ccPlain { panic(p) }, []string{"k"}, nil)
			c := b.Build()

			_, err := TryGetByLookupKey[*ccPlain](c, "k")
			require.EqualError(t, err, "boom")
		})
	}
}

// Interfaces may be passed in pointer form, reflect.TypeOf((*I)(nil)); the key
// hash strips the pointer so lookups by the interface type itself still match.
func TestLookupKey_PointerFormInterfaceMatches(t *testing.T) {
	b := Builder()
	AddSingletonWithLookupKeys[*regA](b, func() *regA { return &regA{7} }, []string{"k"}, nil,
		reflect.TypeOf((*regIFoo)(nil)))
	c := b.Build()

	require.Equal(t, 7, GetByLookupKey[regIFoo](c, "k").(*regA).n)
	require.Equal(t, 7, GetByLookupKey[*regA](c, "k").n)
}

func TestLookupKey_DisposedContainer(t *testing.T) {
	b := Builder()
	AddSingletonWithLookupKeys[*ccPlain](b, func() *ccPlain { return &ccPlain{} }, []string{"k"}, nil)
	c := b.Build()
	c.(Disposable).Dispose()

	_, err := TryGetByLookupKey[*ccPlain](c, "k")
	require.ErrorContains(t, err, "disposed")
}
