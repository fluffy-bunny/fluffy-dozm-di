package di

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Regression coverage for a global-singleton leak: CallSiteResolver used to be
// a package-level singleton (CallSiteResolverInstance) whose callSiteLockers
// map was keyed by CallSite pointers and never pruned. Every container built
// over the process lifetime left its root/singleton CallSites - and the
// resolved singleton instances cached on them - permanently reachable from
// that global map, even after the container was disposed and dropped.
//
// The fix gives every container its own *CallSiteResolver (see builder.go),
// so nothing outside the container graph keeps it alive once the caller lets
// go of it.

// padding keeps this well above Go's 16-byte "tiny allocator" threshold.
// Pointer-free values that small can be packed together into one block by
// the runtime, which then isn't freed until every value sharing it is
// unreachable - that would make this test flaky for reasons unrelated to
// the fix being verified.
type leakMarker struct {
	id      int
	padding [32]byte
}

// TestContainerChurn_SingletonsAreCollectibleAfterDispose builds and disposes
// many containers, each registering one singleton, and asserts every
// singleton instance becomes eligible for GC once its container is disposed
// and dereferenced. Before the fix, this failed: the resolved singleton value
// stayed reachable forever via CallSiteResolverInstance.callSiteLockers.
func TestContainerChurn_SingletonsAreCollectibleAfterDispose(t *testing.T) {
	if testing.Short() {
		t.Skip("GC-timing based collectibility test; skip in short mode")
	}

	const iterations = 200
	var collected atomic.Int64

	for i := 0; i < iterations; i++ {
		func() {
			id := i
			b := Builder()
			AddSingleton[*leakMarker](b, func() *leakMarker { return &leakMarker{id: id} })
			c := b.Build()

			marker := Get[*leakMarker](c)
			runtime.SetFinalizer(marker, func(*leakMarker) {
				collected.Add(1)
			})

			c.(Disposable).Dispose()
			// c and marker go out of scope here; nothing else should hold them.
		}()
	}

	var last int64
	for attempt := 0; attempt < 20; attempt++ {
		runtime.GC()
		last = collected.Load()
		if last >= iterations {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	require.Equal(t, int64(iterations), last,
		"expected every disposed container's singleton to become collectible; "+
			"a stray global reference (e.g. a package-level resolver lock map) would keep it alive forever")
}

// TestContainerChurn_ResolverIsPerContainer is a fast, deterministic companion
// to the GC-based test above: it proves two containers do not share resolver
// state (and therefore cannot leak into each other via a shared lock map).
func TestContainerChurn_ResolverIsPerContainer(t *testing.T) {
	b1 := Builder()
	AddSingleton[*leakMarker](b1, func() *leakMarker { return &leakMarker{id: 1} })
	c1 := b1.Build().(*container)

	b2 := Builder()
	AddSingleton[*leakMarker](b2, func() *leakMarker { return &leakMarker{id: 2} })
	c2 := b2.Build().(*container)

	require.NotNil(t, c1.resolver)
	require.NotNil(t, c2.resolver)
	require.NotSame(t, c1.resolver, c2.resolver,
		"each container must own its own CallSiteResolver so its lock map dies with it")
}
