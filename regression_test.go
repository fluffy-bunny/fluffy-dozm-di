package di

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type regDisposable struct{ disposed bool }

func (d *regDisposable) Dispose() { d.disposed = true }

type regScoped struct{ D *regDisposable }
type regScopedDep struct{ _ byte } // not zero-size, so require.Same is meaningful

type regIFoo interface{ Foo() }

type regA struct{ n int }

func (*regA) Foo() {}

type regB struct{ _ byte }

func (*regB) Foo() {}

// failIfBlocked fails the test if f does not return within a few seconds.
func failIfBlocked(t *testing.T, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); f() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock: resolution did not complete")
	}
}

// A scoped service depending on a transient Disposable used to deadlock:
// the scope mutex was held during construction and CaptureDisposable took it again.
func TestRegression_ScopedDependsOnTransientDisposable(t *testing.T) {
	b := Builder()
	AddTransient[*regDisposable](b, func() *regDisposable { return &regDisposable{} })
	AddScoped[*regScoped](b, func(d *regDisposable) *regScoped { return &regScoped{d} })
	c := b.Build()

	var s *regScoped
	scope := c.(ScopeFactory).CreateScope()
	failIfBlocked(t, func() { s = Get[*regScoped](scope.Container()) })

	scope.Dispose()
	require.True(t, s.D.disposed, "transient dependency should be disposed with the scope")
}

// A scoped factory that resolves another scoped service from the same scope
// used to deadlock for the same reason.
func TestRegression_ScopedFactoryResolvesScoped(t *testing.T) {
	b := Builder()
	AddScoped[*regScopedDep](b, func() *regScopedDep { return &regScopedDep{} })
	AddScopedFactory[*regScoped](b, func(c Container) any {
		Get[*regScopedDep](c)
		return &regScoped{}
	})
	c := b.Build()

	scope := c.(ScopeFactory).CreateScope()
	defer scope.Dispose()
	failIfBlocked(t, func() {
		s1 := Get[*regScoped](scope.Container())
		s2 := Get[*regScoped](scope.Container())
		require.Same(t, s1, s2)
	})
}

func TestRegression_ScopedConcurrentSingleInstance(t *testing.T) {
	b := Builder()
	var mu sync.Mutex
	calls := 0
	AddScoped[*regScopedDep](b, func() *regScopedDep {
		mu.Lock()
		calls++
		mu.Unlock()
		time.Sleep(time.Millisecond)
		return &regScopedDep{}
	})
	c := b.Build()

	scope := c.(ScopeFactory).CreateScope()
	defer scope.Dispose()

	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Get[*regScopedDep](scope.Container())
		}()
	}
	wg.Wait()
	require.Equal(t, 1, calls, "scoped constructor must run once per scope")
}

// Reaching a singleton through an interface slice used to create a second instance.
func TestRegression_SingletonIdentityViaInterfaceSlice(t *testing.T) {
	b := Builder()
	calls := 0
	AddSingleton[*regA](b, func() *regA { calls++; return &regA{calls} }, ImplementedInterfaceType[regIFoo]())
	AddSingleton[*regB](b, func() *regB { return &regB{} }, ImplementedInterfaceType[regIFoo]())
	c := b.Build()

	a := Get[*regA](c)
	all := Get[[]regIFoo](c)
	require.Len(t, all, 2)
	require.Same(t, a, all[0])
	require.Same(t, a, Get[[]regIFoo](c)[0])
	require.Equal(t, 1, calls)
}

func TestRegression_ScopedIdentityViaInterfaceSlice(t *testing.T) {
	b := Builder()
	AddScoped[*regA](b, func() *regA { return &regA{} }, ImplementedInterfaceType[regIFoo]())
	AddScoped[*regB](b, func() *regB { return &regB{} }, ImplementedInterfaceType[regIFoo]())
	c := b.Build()

	scope := c.(ScopeFactory).CreateScope()
	defer scope.Dispose()
	a := Get[*regA](scope.Container())
	require.Same(t, a, Get[[]regIFoo](scope.Container())[0])
}

// Resolving by interface used to return a registration that never declared the interface.
func TestRegression_InterfaceResolvesCorrectDescriptor(t *testing.T) {
	b := Builder()
	AddSingleton[*regA](b, func() *regA { return &regA{1} }, ImplementedInterfaceType[regIFoo]())
	AddSingleton[*regA](b, func() *regA { return &regA{2} })
	c := b.Build()

	require.Equal(t, 2, Get[*regA](c).n, "concrete type resolves the last registration")
	require.Equal(t, 1, Get[regIFoo](c).(*regA).n, "interface resolves the registration that exposes it")
	require.Len(t, Get[[]*regA](c), 2)
}

// Unnamed types (Name() == "") used to share one lookup-key hash.
func TestRegression_LookupKeyUnnamedTypes(t *testing.T) {
	b := Builder()
	AddSingletonWithLookupKeys[func() int](b,
		func() func() int { return func() int { return 1 } }, []string{"k"}, nil)
	AddSingletonWithLookupKeys[func() string](b,
		func() func() string { return func() string { return "s" } }, []string{"k"}, nil)
	c := b.Build()

	require.Equal(t, 1, GetByLookupKey[func() int](c, "k")())
	require.Equal(t, "s", GetByLookupKey[func() string](c, "k")())
}

func TestRegression_LookupKeyScopedFromRootValidated(t *testing.T) {
	b := Builder()
	b.ConfigureOptions(func(o *Options) { o.ValidateScopes = true })
	AddScopedWithLookupKeys[*regA](b, func() *regA { return &regA{} }, []string{"k"}, nil,
		ImplementedInterfaceType[regIFoo]())
	c := b.Build()

	_, err := TryGetByLookupKey[regIFoo](c, "k")
	require.Error(t, err, "scoped keyed service must not resolve from the root scope")

	scope := c.(ScopeFactory).CreateScope()
	defer scope.Dispose()
	_, err = TryGetByLookupKey[regIFoo](scope.Container(), "k")
	require.NoError(t, err)
}

func TestRegression_NilInstancePanicsWithMessage(t *testing.T) {
	require.PanicsWithError(t, "the instance of type 'di.regIFoo' must not be nil", func() {
		Instance[regIFoo](nil)
	})
}

func TestRegression_InvokeWithNilDependency(t *testing.T) {
	b := Builder()
	AddTransientFactory[regIFoo](b, func(Container) any { return nil })
	c := b.Build()

	_, err := Invoke(c, func(f regIFoo) { require.Nil(t, f) })
	require.NoError(t, err)
}

// A scoped constructor that panics must not leave its cache slot locked:
// the next resolution in the same scope retries instead of deadlocking.
func TestRegression_ScopedPanicThenRetry(t *testing.T) {
	b := Builder()
	calls := 0
	AddScoped[*regScopedDep](b, func() *regScopedDep {
		calls++
		if calls == 1 {
			panic("boom")
		}
		return &regScopedDep{}
	})
	c := b.Build()

	scope := c.(ScopeFactory).CreateScope()
	defer scope.Dispose()
	_, err := TryGet[*regScopedDep](scope.Container())
	require.Error(t, err)
	failIfBlocked(t, func() {
		s1 := Get[*regScopedDep](scope.Container())
		require.Same(t, s1, Get[*regScopedDep](scope.Container()))
	})
	require.Equal(t, 2, calls)
}
