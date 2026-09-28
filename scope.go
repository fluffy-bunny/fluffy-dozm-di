package di

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/fluffy-bunny/fluffy-dozm-di/errorx"
	"github.com/fluffy-bunny/fluffy-dozm-di/reflectx"
)

type ContainerEngineScope struct {
	RootContainer    *container
	IsRootScope      bool
	ResolvedServices map[ServiceCacheKey]any
	Locker           *sync.Mutex
	disposed         atomic.Bool
	disposables      []Disposable
	// signalled when a scoped service under construction is stored or its
	// construction fails; created lazily, only when two callers contend
	pendingCond *sync.Cond
}

// pendingMarker occupies a ResolvedServices slot while that scoped service is
// being constructed. It is replaced by the instance on success and removed on
// failure.
var pendingMarker any = &struct{ _ byte }{}

// acquireSlot returns the cached scoped instance for key. If there is none,
// it marks the slot pending and returns owned == true: the caller must then
// construct the service and call storeResolved or clearPending. If another
// caller is already constructing it, acquireSlot waits for that to finish.
func (s *ContainerEngineScope) acquireSlot(key ServiceCacheKey) (v any, owned bool, err error) {
	s.Locker.Lock()
	defer s.Locker.Unlock()
	for {
		if s.ResolvedServices == nil {
			return nil, false, &errorx.ObjectDisposedError{Message: reflectx.TypeOf[Container]().String()}
		}
		v, ok := s.ResolvedServices[key]
		if !ok {
			s.ResolvedServices[key] = pendingMarker
			return nil, true, nil
		}
		if v != pendingMarker {
			return v, false, nil
		}
		if s.pendingCond == nil {
			s.pendingCond = sync.NewCond(s.Locker)
		}
		s.pendingCond.Wait()
	}
}

// storeResolved captures v for disposal and caches it under key, whose slot
// the caller acquired with acquireSlot.
func (s *ContainerEngineScope) storeResolved(key ServiceCacheKey, v any) error {
	s.Locker.Lock()
	defer s.Locker.Unlock()
	if _, err := s.CaptureDisposableWithoutLock(v); err != nil {
		return err
	}
	if s.ResolvedServices == nil {
		return &errorx.ObjectDisposedError{Message: reflectx.TypeOf[Container]().String()}
	}
	s.ResolvedServices[key] = v
	s.wakePending()
	return nil
}

// clearPending releases a slot acquired with acquireSlot whose construction failed.
func (s *ContainerEngineScope) clearPending(key ServiceCacheKey) {
	s.Locker.Lock()
	defer s.Locker.Unlock()
	if s.ResolvedServices[key] == pendingMarker {
		delete(s.ResolvedServices, key)
	}
	s.wakePending()
}

func (s *ContainerEngineScope) wakePending() {
	if s.pendingCond != nil {
		s.pendingCond.Broadcast()
	}
}

func (c *ContainerEngineScope) GetDescriptors() []*Descriptor {
	return c.RootContainer.GetDescriptors()
}
func (s *ContainerEngineScope) Get(serviceType reflect.Type) (any, error) {
	if s.disposed.Load() {
		return nil, &errorx.ObjectDisposedError{Message: reflectx.TypeOf[Container]().String()}
	}

	return s.RootContainer.GetWithScope(serviceType, s)
}
func (s *ContainerEngineScope) GetByLookupKey(serviceType reflect.Type, key string) (any, error) {
	if s.disposed.Load() {
		return nil, &errorx.ObjectDisposedError{Message: reflectx.TypeOf[Container]().String()}
	}

	return s.RootContainer.GetWithScopeWithLookupKey(serviceType, key, s)
}

func (s *ContainerEngineScope) Container() Container {
	return s
}

func (s *ContainerEngineScope) CreateScope() Scope {
	return s.RootContainer.CreateScope()
}

func (s *ContainerEngineScope) Dispose() {
	disposables := s.BeginDispose()
	for i := len(disposables) - 1; i >= 0; i-- {
		disposables[i].Dispose()
	}
}

func (s *ContainerEngineScope) Disposables() []Disposable {
	return s.disposables
}

func (s *ContainerEngineScope) BeginDispose() []Disposable {
	if s.disposed.Swap(true) {
		return nil
	}

	if s.IsRootScope && !s.RootContainer.IsDisposed() {
		s.RootContainer.Dispose()
	}

	s.Locker.Lock()
	disposables := s.disposables
	s.disposables = nil
	s.ResolvedServices = nil
	s.Locker.Unlock()

	return disposables
}

func (s *ContainerEngineScope) CaptureDisposable(service any) (Disposable, error) {
	d, ok := service.(Disposable)
	if service == s || !ok {
		return d, nil
	}

	disposed := false
	s.Locker.Lock()
	if s.disposed.Load() {
		disposed = true
	} else {
		s.disposables = append(s.disposables, d)
	}
	s.Locker.Unlock()

	if disposed {
		d.Dispose()
		return d, fmt.Errorf("capture disposable service '%v', scope disposed", reflect.TypeOf(service))
	}

	return d, nil

}

func (s *ContainerEngineScope) CaptureDisposableWithoutLock(service any) (Disposable, error) {
	d, ok := service.(Disposable)
	if service == s || !ok {
		return d, nil
	}

	if s.disposed.Load() {
		d.Dispose()
		return d, fmt.Errorf("capture disposable service '%v', scope disposed", reflect.TypeOf(service))
	} else {
		s.disposables = append(s.disposables, d)
		return d, nil
	}
}

func newEngineScope(c *container, isRootScope bool) *ContainerEngineScope {
	return &ContainerEngineScope{
		RootContainer:    c,
		IsRootScope:      isRootScope,
		ResolvedServices: make(map[ServiceCacheKey]any),
		Locker:           new(sync.Mutex),
		disposables:      make([]Disposable, 0),
	}
}
