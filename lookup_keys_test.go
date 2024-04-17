package di

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func AddSingletonEmployeesWithLookupKeys(b ContainerBuilder) {
	AddSingletonWithLookupKeys[*employee](b,
		func() *employee {
			return &employee{Name: "1"}
		}, []string{"1"}, map[string]interface{}{"name": "1"},
		reflect.TypeOf((*IEmployee)(nil)))
	AddSingletonWithLookupKeys[*employee](b,
		func() *employee {
			return &employee{Name: "2a"}
		}, []string{"2"}, map[string]interface{}{"name": "2a"},
		reflect.TypeOf((*IEmployee)(nil)))
	AddSingletonWithLookupKeys[*employee](b,
		func() *employee {
			return &employee{Name: "2"}
		}, []string{"2"}, map[string]interface{}{"name": "2"},
		reflect.TypeOf((*IEmployee)(nil)))
}
func AddTransientEmployeesWithLookupKeys(b ContainerBuilder) {
	AddTransientWithLookupKeys[*employee](b,
		func() *employee {
			return &employee{Name: "1"}
		}, []string{"1"},
		map[string]interface{}{"name": "1"},
		reflect.TypeOf((*IEmployee)(nil)))
	AddTransientWithLookupKeys[*employee](b,
		func() *employee {
			return &employee{Name: "2"}
		}, []string{"2"},
		map[string]interface{}{"name": "2"},
		reflect.TypeOf((*IEmployee)(nil)))
}
func AddInstanceEmployeesWithLookupKeys(b ContainerBuilder) {
	AddInstanceWithLookupKeys[*employee](b,
		&employee{Name: "1"}, []string{"1"},
		map[string]interface{}{"name": "1"},
		reflect.TypeOf((*IEmployee)(nil)))
	AddInstanceWithLookupKeys[*employee](b,
		&employee{Name: "2"}, []string{"2"},
		map[string]interface{}{"name": "2"},
		reflect.TypeOf((*IEmployee)(nil)))
}
func AddScopedHandlersWithLookupKeys(b ContainerBuilder) {
	AddScopedWithLookupKeys[*handler](b,
		func() *handler {
			return &handler{path: "1"}
		}, []string{"1"}, map[string]interface{}{"name": "1"},
		reflect.TypeOf((*IHandler)(nil)))
	AddScopedWithLookupKeys[*handler](b,
		func() *handler {
			return &handler{path: "2"}
		}, []string{"2"}, map[string]interface{}{"name": "2"},
		reflect.TypeOf((*IHandler)(nil)))
}

func TestManyWithScopeWithLookupKeys(t *testing.T) {
	b := Builder()
	// Build the container
	AddScopedHandlersWithLookupKeys(b)
	c := b.Build()
	scopeFactory := Get[ScopeFactory](c)
	scope1 := scopeFactory.CreateScope()
	ctn := scope1.Container()
	descriptors := ctn.GetDescriptors()
	require.Equal(t, 2, len(descriptors))
	for _, d := range descriptors {
		require.NotEmpty(t, d.Metadata)
		_, ok := d.Metadata["name"]
		require.True(t, ok)
		_, ok = d.Metadata["name"].(string)
		require.True(t, ok)

		fmt.Println(d)
	}
	handlers := Get[[]IHandler](ctn)
	require.Equal(t, 2, len(handlers))
	require.NotPanics(t, func() {
		h := GetByLookupKey[IHandler](c, "1")
		require.NotNil(t, h)
		require.Equal(t, "1", h.GetPath())
	})
}
func TestManyWithSingletonWithLookupKeys(t *testing.T) {
	b := Builder()
	// Build the container
	AddSingletonEmployeesWithLookupKeys(b)
	c := b.Build()
	scopeFactory := Get[ScopeFactory](c)
	scope1 := scopeFactory.CreateScope()
	employees := Get[[]IEmployee](scope1.Container())
	require.Equal(t, 3, len(employees))
	require.NotPanics(t, func() {
		h := GetByLookupKey[IEmployee](c, "1")
		require.NotNil(t, h)
		require.Equal(t, "1", h.GetName())
	})
	require.NotPanics(t, func() {
		h := GetByLookupKey[IEmployee](c, "2")
		require.NotNil(t, h)
		require.Equal(t, "2", h.GetName())
	})
}
func TestManyWithTransientWithLookupKeys(t *testing.T) {
	b := Builder()
	// Build the container
	AddTransientEmployeesWithLookupKeys(b)
	c := b.Build()
	scopeFactory := Get[ScopeFactory](c)
	scope1 := scopeFactory.CreateScope()
	employees := Get[[]IEmployee](scope1.Container())
	require.Equal(t, 2, len(employees))
	require.NotPanics(t, func() {
		h := GetByLookupKey[IEmployee](c, "1")
		require.NotNil(t, h)
		require.Equal(t, "1", h.GetName())
	})
}
func TestManyWithInstanceWithLookupKeys(t *testing.T) {
	b := Builder()
	// Build the container
	AddInstanceEmployeesWithLookupKeys(b)
	c := b.Build()
	scopeFactory := Get[ScopeFactory](c)
	scope1 := scopeFactory.CreateScope()
	employees := Get[[]IEmployee](scope1.Container())
	require.Equal(t, 2, len(employees))
	require.NotPanics(t, func() {
		h := GetByLookupKey[IEmployee](c, "1")
		require.NotNil(t, h)
		require.Equal(t, "1", h.GetName())
	})
	employeePtrs := Get[[]*employee](scope1.Container())
	require.Equal(t, 2, len(employeePtrs))
	require.NotPanics(t, func() {
		h := GetByLookupKey[*employee](c, "1")
		require.NotNil(t, h)
		require.Equal(t, "1", h.GetName())
	})
}
