package di

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type (
	scopedCounter struct {
		count    int
		disposed bool
	}
)

func (c *scopedCounter) Inc() int {
	c.count++
	return c.count
}
func (c *scopedCounter) Dispose() {
	c.disposed = true
}

func addScopedCounter(b ContainerBuilder) {
	AddScoped[*scopedCounter](b, func() *scopedCounter {
		return &scopedCounter{}
	})
}

func TestInvokeScoped_FreshInstancePerCall(t *testing.T) {
	b := Builder()
	addScopedCounter(b)
	c := b.Build()
	factory := Get[ScopeFactory](c)

	var firstCallCount, secondCallCount int
	err := InvokeScoped(factory, func(counter *scopedCounter) error {
		counter.Inc()
		firstCallCount = counter.Inc() // same instance within one call -> 2
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 2, firstCallCount)

	err = InvokeScoped(factory, func(counter *scopedCounter) error {
		secondCallCount = counter.Inc()
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, secondCallCount, "a new InvokeScoped call must get a fresh instance, not the previous call's")
}

func TestInvokeScoped_DisposesAfterCall(t *testing.T) {
	b := Builder()
	addScopedCounter(b)
	c := b.Build()
	factory := Get[ScopeFactory](c)

	var resolved *scopedCounter
	err := InvokeScoped(factory, func(counter *scopedCounter) error {
		resolved = counter
		require.False(t, resolved.disposed, "must not be disposed while fn is still running")
		return nil
	})
	require.NoError(t, err)
	require.True(t, resolved.disposed, "the scope must be disposed once fn returns")
}

func TestInvokeScoped_DisposesEvenWhenFnErrors(t *testing.T) {
	b := Builder()
	addScopedCounter(b)
	c := b.Build()
	factory := Get[ScopeFactory](c)

	wantErr := errors.New("boom")
	var resolved *scopedCounter
	err := InvokeScoped(factory, func(counter *scopedCounter) error {
		resolved = counter
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	require.True(t, resolved.disposed, "the scope must be disposed even when fn returns an error")
}

func TestInvokeScoped_DisposesEvenWhenFnPanics(t *testing.T) {
	b := Builder()
	addScopedCounter(b)
	c := b.Build()
	factory := Get[ScopeFactory](c)

	var resolved *scopedCounter
	func() {
		defer func() { _ = recover() }()
		_ = InvokeScoped(factory, func(counter *scopedCounter) error {
			resolved = counter
			panic("boom")
		})
	}()
	require.True(t, resolved.disposed, "the scope must still be disposed if fn panics")
}

func TestInvokeScoped_ResolutionFailureReturnsErrorWithoutCallingFn(t *testing.T) {
	b := Builder()
	// scopedCounter deliberately never registered.
	c := b.Build()
	factory := Get[ScopeFactory](c)

	called := false
	err := InvokeScoped(factory, func(counter *scopedCounter) error {
		called = true
		return nil
	})
	require.Error(t, err)
	require.False(t, called, "fn must not run when the scoped resolution fails")
}

func TestInvokeScopedResult_ReturnsValue(t *testing.T) {
	b := Builder()
	addScopedCounter(b)
	c := b.Build()
	factory := Get[ScopeFactory](c)

	result, err := InvokeScopedResult(factory, func(counter *scopedCounter) (int, error) {
		return counter.Inc(), nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, result)
}

func TestInvokeScopedResult_DisposesAfterCall(t *testing.T) {
	b := Builder()
	addScopedCounter(b)
	c := b.Build()
	factory := Get[ScopeFactory](c)

	var resolved *scopedCounter
	_, err := InvokeScopedResult(factory, func(counter *scopedCounter) (int, error) {
		resolved = counter
		return counter.Inc(), nil
	})
	require.NoError(t, err)
	require.True(t, resolved.disposed)
}
