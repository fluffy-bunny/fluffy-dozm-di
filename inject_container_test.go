package di

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type (
	ContainerInjectSingleton struct {
		Container Container
	}
	ContainerInjectTransient struct {
		Container Container
	}
	ContainerInjectScoped struct {
		Container Container
	}
)

func AddContainerInjectSingleton(b ContainerBuilder) {
	AddSingleton[*ContainerInjectSingleton](b, func(container Container) *ContainerInjectSingleton {
		return &ContainerInjectSingleton{
			Container: container,
		}
	})
}
func AddContainerInjectTransient(b ContainerBuilder) {
	AddTransient[*ContainerInjectTransient](b, func(container Container) *ContainerInjectTransient {
		return &ContainerInjectTransient{
			Container: container,
		}
	})
}
func AddContainerInjectScoped(b ContainerBuilder) {
	AddScoped[*ContainerInjectScoped](b, func(container Container) *ContainerInjectScoped {
		return &ContainerInjectScoped{
			Container: container,
		}
	})
}
func TestSingletonContainerInject(t *testing.T) {
	b := Builder()
	b.ConfigureOptions(func(o *Options) {
		o.ValidateScopes = true
		o.ValidateOnBuild = true
	})
	AddContainerInjectSingleton(b)
	AddContainerInjectTransient(b)
	AddContainerInjectScoped(b)
	c := b.Build()
	containerInjectSingleton := Get[*ContainerInjectSingleton](c)
	require.NotNil(t, containerInjectSingleton)
	rootContainer := containerInjectSingleton.Container

	require.Panics(t, func() {
		containerInjectScoped := Get[*ContainerInjectScoped](c)
		require.Nil(t, containerInjectScoped)

	})

	containerInjectTransient := Get[*ContainerInjectTransient](c)
	require.NotNil(t, containerInjectTransient)
	require.Same(t, rootContainer, containerInjectTransient.Container)

	scopeFactory := Get[ScopeFactory](c)
	scope := scopeFactory.CreateScope()
	scopedContainer := scope.Container()

	// thankfully a singleton is always fetched from the root.  This ensures that it can't create a scoped
	// object by getting access to the container.
	containerInjectSingleton = Get[*ContainerInjectSingleton](scopedContainer)
	require.NotNil(t, containerInjectSingleton)
	require.Same(t, rootContainer, containerInjectSingleton.Container)
	require.NotSame(t, scopedContainer, containerInjectSingleton.Container)

	containerInjectTransient = Get[*ContainerInjectTransient](scopedContainer)
	require.NotNil(t, containerInjectTransient)
	require.NotSame(t, rootContainer, containerInjectTransient.Container)
	require.Same(t, scopedContainer, containerInjectTransient.Container)

	containerInjectScoped := Get[*ContainerInjectScoped](scopedContainer)
	require.NotNil(t, containerInjectScoped)
	require.NotSame(t, rootContainer, containerInjectScoped.Container)
	require.Same(t, scopedContainer, containerInjectScoped.Container)

}
