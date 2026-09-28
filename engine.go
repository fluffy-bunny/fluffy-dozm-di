package di

type ServiceAccessor func(*ContainerEngineScope) (any, error)

type ContainerEngine interface {
	RealizeService(CallSite) (ServiceAccessor, error)
}

type containerEngine struct {
	container *container
}

func (engine *containerEngine) RealizeService(callSite CallSite) (ServiceAccessor, error) {
	return func(scope *ContainerEngineScope) (any, error) {
		return engine.container.resolver.Resolve(callSite, scope)
	}, nil
}

func newContainerEngine(c *container) ContainerEngine {
	return &containerEngine{container: c}
}
