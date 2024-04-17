package di

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type (
	MyFunc func() string
)

func AddMyFunc(b ContainerBuilder) {
	AddFunc[MyFunc](b, func() MyFunc {
		return func() string {
			return "Hello"
		}
	})
}
func AddMyFuncByName(b ContainerBuilder, name string) {
	AddFuncWithLookupKeys[MyFunc](b,
		func() MyFunc {
			return func() string {
				return "Hello: " + name
			}
		}, []string{name}, map[string]interface{}{})
}
func TestAddMyFunc(t *testing.T) {
	b := Builder()
	// Build the container
	AddMyFunc(b)
	c := b.Build()

	myFunc := Get[MyFunc](c)
	require.NotNil(t, myFunc)
	require.Equal(t, "Hello", myFunc())
}
func TestAddMyFuncByName(t *testing.T) {
	b := Builder()
	// Build the container
	AddMyFuncByName(b, "test")
	c := b.Build()

	myFunc := GetByLookupKey[MyFunc](c, "test")
	require.NotNil(t, myFunc)
	require.Equal(t, "Hello: test", myFunc())
}
