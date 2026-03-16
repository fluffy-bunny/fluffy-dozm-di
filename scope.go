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
