package di

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// ISomething is a test interface to prove resolution order.
type ISomething interface {
	GetName() string
}

type somethingTransient struct{ callCount int32 }
type somethingSingleton struct{ callCount int32 }
type somethingScoped struct{ callCount int32 }

func (s *somethingTransient) GetName() string { return "transient" }
func (s *somethingSingleton) GetName() string { return "singleton" }
func (s *somethingScoped) GetName() string    { return "scoped" }

// TestLastRegistrationWins_TransientThenSingletonThenScoped proves that when
// the same interface is registered as transient, singleton, and scoped (in that order),
// the last registration (scoped) wins.
func TestLastRegistrationWins_TransientThenSingletonThenScoped(t *testing.T) {
	b := Builder()
	AddTransient[ISomething](b, func() ISomething { return &somethingTransient{} })
	AddSingleton[ISomething](b, func() ISomething { return &somethingSingleton{} })
	AddScoped[ISomething](b, func() ISomething { return &somethingScoped{} })

	c := b.Build()

	// From root container, last registered (scoped) wins.
	// Scoped resolved from root behaves like singleton.
	result := Get[ISomething](c)
	require.Equal(t, "scoped", result.GetName(), "last registration (scoped) should win from root")
}

// TestLastRegistrationWins_ScopedThenSingletonThenTransient proves that when
// the same interface is registered as scoped, singleton, and transient (in that order),
// the last registration (transient) wins.
func TestLastRegistrationWins_ScopedThenSingletonThenTransient(t *testing.T) {
	b := Builder()
	AddScoped[ISomething](b, func() ISomething { return &somethingScoped{} })
	AddSingleton[ISomething](b, func() ISomething { return &somethingSingleton{} })
	AddTransient[ISomething](b, func() ISomething { return &somethingTransient{} })

	c := b.Build()

	// From root container, last registered (transient) wins.
	result := Get[ISomething](c)
	require.Equal(t, "transient", result.GetName(), "last registration (transient) should win from root")

	// Each call returns a new instance (transient behavior)
	result2 := Get[ISomething](c)
	require.Equal(t, "transient", result2.GetName())
}

// TestLastRegistrationWins_TransientThenScopedThenSingleton proves singleton wins when last
func TestLastRegistrationWins_TransientThenScopedThenSingleton(t *testing.T) {
	b := Builder()
	AddTransient[ISomething](b, func() ISomething { return &somethingTransient{} })
	AddScoped[ISomething](b, func() ISomething { return &somethingScoped{} })
	AddSingleton[ISomething](b, func() ISomething { return &somethingSingleton{} })

	c := b.Build()

	// From root container, last registered (singleton) wins.
	result := Get[ISomething](c)
	require.Equal(t, "singleton", result.GetName(), "last registration (singleton) should win from root")
}

// TestLastRegistrationWins_RootContainer_TransientVsSingleton proves that
// given the root container, the last registered wins between transient and singleton.
func TestLastRegistrationWins_RootContainer_TransientVsSingleton(t *testing.T) {
	t.Run("transient last wins", func(t *testing.T) {
		b := Builder()
		AddSingleton[ISomething](b, func() ISomething { return &somethingSingleton{} })
		AddTransient[ISomething](b, func() ISomething { return &somethingTransient{} })
		c := b.Build()

		result := Get[ISomething](c)
		require.Equal(t, "transient", result.GetName(), "transient registered last should win")
	})

	t.Run("singleton last wins", func(t *testing.T) {
		b := Builder()
		AddTransient[ISomething](b, func() ISomething { return &somethingTransient{} })
		AddSingleton[ISomething](b, func() ISomething { return &somethingSingleton{} })
		c := b.Build()

		result := Get[ISomething](c)
		require.Equal(t, "singleton", result.GetName(), "singleton registered last should win")
	})
}

// TestLastRegistrationWins_ScopedContainer proves that in a scoped container
// the last registration also wins.
func TestLastRegistrationWins_ScopedContainer(t *testing.T) {
	t.Run("scoped last wins in scope", func(t *testing.T) {
		b := Builder()
		AddTransient[ISomething](b, func() ISomething { return &somethingTransient{} })
		AddSingleton[ISomething](b, func() ISomething { return &somethingSingleton{} })
		AddScoped[ISomething](b, func() ISomething { return &somethingScoped{} })
		c := b.Build()

		scopeFactory := Get[ScopeFactory](c)
		scope := scopeFactory.CreateScope()
		defer scope.Dispose()

		result := Get[ISomething](scope.Container())
		require.Equal(t, "scoped", result.GetName(), "scoped registered last should win in scope")

		// Scoped returns same instance within same scope
		result2 := Get[ISomething](scope.Container())
		require.Equal(t, "scoped", result2.GetName())
	})

	t.Run("transient last wins in scope", func(t *testing.T) {
		b := Builder()
		AddSingleton[ISomething](b, func() ISomething { return &somethingSingleton{} })
		AddScoped[ISomething](b, func() ISomething { return &somethingScoped{} })
		AddTransient[ISomething](b, func() ISomething { return &somethingTransient{} })
		c := b.Build()

		scopeFactory := Get[ScopeFactory](c)
		scope := scopeFactory.CreateScope()
		defer scope.Dispose()

		result := Get[ISomething](scope.Container())
		require.Equal(t, "transient", result.GetName(), "transient registered last should win in scope")
	})

	t.Run("singleton last wins in scope", func(t *testing.T) {
		b := Builder()
		AddTransient[ISomething](b, func() ISomething { return &somethingTransient{} })
		AddScoped[ISomething](b, func() ISomething { return &somethingScoped{} })
		AddSingleton[ISomething](b, func() ISomething { return &somethingSingleton{} })
		c := b.Build()

		scopeFactory := Get[ScopeFactory](c)
		scope := scopeFactory.CreateScope()
		defer scope.Dispose()

		result := Get[ISomething](scope.Container())
		require.Equal(t, "singleton", result.GetName(), "singleton registered last should win in scope")
	})
}

// TestLastRegistrationWins_TransientNewInstanceEveryTime proves transient creates new instances
func TestLastRegistrationWins_TransientNewInstanceEveryTime(t *testing.T) {
	var counter int32
	b := Builder()
	AddSingleton[ISomething](b, func() ISomething { return &somethingSingleton{} })
	AddTransient[ISomething](b, func() ISomething {
		atomic.AddInt32(&counter, 1)
		return &somethingTransient{callCount: counter}
	})
	c := b.Build()

	r1 := Get[ISomething](c)
	r2 := Get[ISomething](c)
	require.Equal(t, "transient", r1.GetName())
	require.Equal(t, "transient", r2.GetName())
	// transient creates a new instance on each call
	require.NotSame(t, r1, r2, "transient should return different instances")
}

// TestLastRegistrationWins_SingletonSameInstance proves singleton returns same instance
func TestLastRegistrationWins_SingletonSameInstance(t *testing.T) {
	b := Builder()
	AddTransient[ISomething](b, func() ISomething { return &somethingTransient{} })
	AddSingleton[ISomething](b, func() ISomething { return &somethingSingleton{} })
	c := b.Build()

	r1 := Get[ISomething](c)
	r2 := Get[ISomething](c)
	require.Equal(t, "singleton", r1.GetName())
	require.Same(t, r1, r2, "singleton should return the same instance")
}

// TestLastRegistrationWins_AllRegistrationsAvailableAsSlice proves that even though
// the last registration wins for single resolution, ALL registrations are available via slice.
func TestLastRegistrationWins_AllRegistrationsAvailableAsSlice(t *testing.T) {
	b := Builder()
	AddTransient[ISomething](b, func() ISomething { return &somethingTransient{} })
	AddSingleton[ISomething](b, func() ISomething { return &somethingSingleton{} })
	AddScoped[ISomething](b, func() ISomething { return &somethingScoped{} })
	c := b.Build()

	scopeFactory := Get[ScopeFactory](c)
	scope := scopeFactory.CreateScope()
	defer scope.Dispose()

	all := Get[[]ISomething](scope.Container())
	require.Equal(t, 3, len(all), "all three registrations should be available as a slice")
	require.Equal(t, "transient", all[0].GetName())
	require.Equal(t, "singleton", all[1].GetName())
	require.Equal(t, "scoped", all[2].GetName())
}

// TestPanicOnLifetimeConflict proves that when DetectLifetimeConflicts is true,
// building a container with the same type registered with different lifetimes panics.
func TestPanicOnLifetimeConflict(t *testing.T) {
	t.Run("panics on transient+singleton conflict", func(t *testing.T) {
		b := Builder()
		b.ConfigureOptions(func(o *Options) {
			o.DetectLifetimeConflicts = true
		})
		AddTransient[ISomething](b, func() ISomething { return &somethingTransient{} })
		AddSingleton[ISomething](b, func() ISomething { return &somethingSingleton{} })

		require.Panics(t, func() {
			b.Build()
		}, "should panic when same type registered with different lifetimes")
	})

	t.Run("panics on transient+scoped conflict", func(t *testing.T) {
		b := Builder()
		b.ConfigureOptions(func(o *Options) {
			o.DetectLifetimeConflicts = true
		})
		AddTransient[ISomething](b, func() ISomething { return &somethingTransient{} })
		AddScoped[ISomething](b, func() ISomething { return &somethingScoped{} })

		require.Panics(t, func() {
			b.Build()
		}, "should panic when same type registered with different lifetimes")
	})

	t.Run("panics on all three lifetimes", func(t *testing.T) {
		b := Builder()
		b.ConfigureOptions(func(o *Options) {
			o.DetectLifetimeConflicts = true
		})
		AddTransient[ISomething](b, func() ISomething { return &somethingTransient{} })
		AddSingleton[ISomething](b, func() ISomething { return &somethingSingleton{} })
		AddScoped[ISomething](b, func() ISomething { return &somethingScoped{} })

		require.Panics(t, func() {
			b.Build()
		}, "should panic when same type registered with different lifetimes")
	})

	t.Run("no panic when same lifetime", func(t *testing.T) {
		b := Builder()
		b.ConfigureOptions(func(o *Options) {
			o.DetectLifetimeConflicts = true
		})
		AddSingleton[ISomething](b, func() ISomething { return &somethingSingleton{} })
		AddSingleton[ISomething](b, func() ISomething { return &somethingSingleton{} })

		require.NotPanics(t, func() {
			b.Build()
		}, "should NOT panic when same type registered with same lifetime")
	})

	t.Run("no panic when detection disabled", func(t *testing.T) {
		b := Builder()
		// DetectLifetimeConflicts defaults to false
		AddTransient[ISomething](b, func() ISomething { return &somethingTransient{} })
		AddSingleton[ISomething](b, func() ISomething { return &somethingSingleton{} })

		require.NotPanics(t, func() {
			b.Build()
		}, "should NOT panic when DetectLifetimeConflicts is false")
	})
}
