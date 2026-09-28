package di

import (
	"fmt"
	"reflect"
	"sync/atomic"

	"github.com/fluffy-bunny/fluffy-dozm-di/reflectx"
	"github.com/fluffy-bunny/fluffy-dozm-di/syncx"
)

var ContainerType = reflectx.TypeOf[Container]()
var ContainerImplType = reflectx.TypeOf[container]()
var ScopeFactoryType = reflectx.TypeOf[ScopeFactory]()
var IsServiceType = reflectx.TypeOf[IsService]()

// Container options.
type Options struct {
	ValidateScopes          bool
	ValidateOnBuild         bool
	DetectLifetimeConflicts bool
}

// Get default container options.
func DefaultOptions() Options {
	return Options{}
}

// realized-accessor cache key for GetByLookupKey; avoids hashing on every call
type lookupKey struct {
	serviceType reflect.Type
	key         string
}

// Container implementation
type container struct {
	Root                      *ContainerEngineScope
	CallSiteFactory           *CallSiteFactory
	engine                    ContainerEngine
	realizedServices          *syncx.Map[reflect.Type, ServiceAccessor]
	realizedLookupKeyServices *syncx.Map[lookupKey, ServiceAccessor]
	resolver                  *CallSiteResolver

	disposed          atomic.Bool
	callSiteValidator *CallSiteValidator
}

func (c *container) GetDescriptors() []*Descriptor {
	return c.CallSiteFactory.Descriptors()
}

func (c *container) Get(serviceType reflect.Type) (any, error) {
	return c.GetWithScope(serviceType, c.Root)
}
func (c *container) GetByLookupKey(serviceType reflect.Type, key string) (any, error) {
	return c.GetWithScopeWithLookupKey(serviceType, key, c.Root)
}
func (c *container) CreateScope() Scope {
	if c.disposed.Load() {
		panic(fmt.Errorf("%v disposed", reflect.TypeOf(c).Elem()))
	}

	return newEngineScope(c, false)
}

func (c *container) GetWithScope(serviceType reflect.Type, scope *ContainerEngineScope) (result any, err error) {
	if c.disposed.Load() {
		err = fmt.Errorf("%v disposed", reflect.TypeOf(c).Elem())
		return
	}

	defer func() {
		if p := recover(); p != nil {
			if e, ok := p.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("%v", p)
			}
		}
	}()

	accessor, ok := c.realizedServices.Load(serviceType)
	if !ok {
		accessor, err = c.createServiceAccessor(serviceType)
		if err != nil {
			return
		} else {
			accessor, _ = c.realizedServices.LoadOrStore(serviceType, accessor)
		}

	}

	if c.callSiteValidator != nil {
		err := c.callSiteValidator.ValidateResolution(serviceType, scope, c.Root)
		if err != nil {
			return nil, err
		}
	}

	return accessor(scope)
}
func (c *container) GetWithScopeWithLookupKey(serviceType reflect.Type, key string, scope *ContainerEngineScope) (result any, err error) {
	if c.disposed.Load() {
		err = fmt.Errorf("%v disposed", reflect.TypeOf(c).Elem())
		return
	}

	defer func() {
		if p := recover(); p != nil {
			if e, ok := p.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("%v", p)
			}
		}
	}()
	cacheKey := lookupKey{serviceType, key}
	accessor, ok := c.realizedLookupKeyServices.Load(cacheKey)
	if !ok {
		accessor, err = c.createServiceLookupKeyAccessor(cacheKey)
		if err != nil {
			return
		} else {
			accessor, _ = c.realizedLookupKeyServices.LoadOrStore(cacheKey, accessor)
		}

	}

	if c.callSiteValidator != nil {
		err := c.callSiteValidator.ValidateLookupKeyResolution(cacheKey, scope, c.Root)
		if err != nil {
			return nil, err
		}
	}

	return accessor(scope)
}
func (c *container) validateService(d *Descriptor) error {
	callSite, err := c.CallSiteFactory.GetCallSiteByDescriptor(d, newCallSiteChain())
	if err != nil {
		return err
	}
	if c.callSiteValidator != nil {
		return c.callSiteValidator.ValidateCallSite(callSite)
	}
	return nil
}

func (c *container) Dispose() {
	c.disposed.Store(true)
	c.Root.Dispose()
}

func (c *container) IsDisposed() bool {
	return c.disposed.Load()
}

func (c *container) createEngine() ContainerEngine {
	return newContainerEngine(c)
}

func (c *container) createServiceLookupKeyAccessor(key lookupKey) (ServiceAccessor, error) {
	descriptor, ok := c.CallSiteFactory.descriptorKeyLookup[hashTypeAndString(key.serviceType, key.key)]
	if !ok || descriptor.item == nil {
		return nil, fmt.Errorf("no service of type '%v' registered for lookup key '%s'", key.serviceType, key.key)
	}
	callSite, err := c.CallSiteFactory.GetCallSiteByDescriptor(descriptor.Last(), newCallSiteChain())
	if err != nil {
		return nil, err
	}

	if c.callSiteValidator != nil {
		if err := c.callSiteValidator.ValidateLookupKeyCallSite(callSite, key); err != nil {
			return nil, err
		}
	}

	if callSite.Cache().Location == CacheLocation_Root {
		value, err := c.resolver.Resolve(callSite, c.Root)
		if err != nil {
			return nil, err
		}
		return func(scope *ContainerEngineScope) (any, error) { return value, nil }, nil
	}

	return c.engine.RealizeService(callSite)
}
func (c *container) createServiceAccessor(serviceType reflect.Type) (ServiceAccessor, error) {
	callSite, err := c.CallSiteFactory.GetCallSite(serviceType, newCallSiteChain())
	if err != nil {
		return nil, err
	}

	if c.callSiteValidator != nil {
		if err := c.callSiteValidator.ValidateCallSite(callSite); err != nil {
			return nil, err
		}
	}

	if callSite.Cache().Location == CacheLocation_Root {
		value, err := c.resolver.Resolve(callSite, c.Root)
		if err != nil {
			return nil, err
		}
		return func(scope *ContainerEngineScope) (any, error) { return value, nil }, nil
	}

	return c.engine.RealizeService(callSite)
}

func (c *container) ReplaceServiceAccessor(callSite CallSite, accessor ServiceAccessor) {
	c.realizedServices.Store(callSite.ServiceType(), accessor)
}
