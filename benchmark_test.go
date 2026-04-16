package di

import (
	"context"
	"io"
	"strings"
	"sync/atomic"
	"testing"
)

func addServicesWithFactory(cb ContainerBuilder) {
	cb.Add(TransientFactory[io.ReadWriter](func(c Container) any { return readWriterFactory(c) }))
	cb.Add(TransientFactory[io.Reader](func(c Container) any { return strings.NewReader("") }))
	cb.Add(TransientFactory[io.Writer](func(c Container) any { return &strings.Builder{} }))
}

func addServicesWithConstructor(cb ContainerBuilder) {
	cb.Add(Transient[io.ReadWriter](
		func(r io.Reader, w io.Writer) *readWriter { return &readWriter{Reader: r, Writer: w} }))
	cb.Add(Transient[io.Reader](func() *strings.Reader { return strings.NewReader("") }))
	cb.Add(Transient[io.Writer](func() *strings.Builder { return &strings.Builder{} }))
}

func addServicesWithSingleton(cb ContainerBuilder) {
	cb.Add(Singleton[io.ReadWriter](
		func(r io.Reader, w io.Writer) *readWriter { return &readWriter{Reader: r, Writer: w} }))
	cb.Add(Singleton[io.Reader](func() *strings.Reader { return strings.NewReader("") }))
	cb.Add(Singleton[io.Writer](func() *strings.Builder { return &strings.Builder{} }))
}

func addServicesWithScoped(cb ContainerBuilder) {
	cb.Add(Scoped[io.ReadWriter](
		func(r io.Reader, w io.Writer) *readWriter { return &readWriter{Reader: r, Writer: w} }))
	cb.Add(Scoped[io.Reader](func() *strings.Reader { return strings.NewReader("") }))
	cb.Add(Scoped[io.Writer](func() *strings.Builder { return &strings.Builder{} }))
}

func buildContainer(mode string) Container {
	cb := Builder()
	cb.ConfigureOptions(func(o *Options) {
		o.ValidateScopes = true
		o.ValidateOnBuild = true
	})

	switch mode {
	case "factory":
		addServicesWithFactory(cb)
	case "constructor":
		addServicesWithConstructor(cb)
	case "singleton":
		addServicesWithSingleton(cb)
	case "scoped":
		addServicesWithScoped(cb)
	}

	c := cb.Build()
	c = Get[ScopeFactory](c).CreateScope().Container()
	return c
}

func resolve(c Container) {
	_ = Get[io.ReadWriter](c)
}

func Benchmark_Factory(b *testing.B) {
	c := buildContainer("factory")

	resolve(c)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolve(c)
	}
}

func Benchmark_Constructor(b *testing.B) {
	c := buildContainer("constructor")

	resolve(c)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolve(c)
	}
}

func Benchmark_Singleton(b *testing.B) {
	c := buildContainer("singleton")

	resolve(c)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolve(c)
	}
}

func Benchmark_Scoped(b *testing.B) {
	c := buildContainer("scoped")

	resolve(c)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolve(c)
	}
}

func buildHeavyScopedRequestBenchmarkContainer() Container {
	var created atomic.Int64
	var active atomic.Int64
	var disposed atomic.Int64
	var transientCounter atomic.Int64

	b := Builder()
	b.ConfigureOptions(func(o *Options) {
		o.ValidateScopes = true
		o.ValidateOnBuild = true
	})
	registerHeavyScopedRequestGraph(b, &created, &active, &disposed, &transientCounter)
	return b.Build()
}

func Benchmark_HeavyScopedRequestLifecycle(b *testing.B) {
	c := buildHeavyScopedRequestBenchmarkContainer()
	scopeFactory := Get[ScopeFactory](c)

	// Warm-up one request lifecycle before timing.
	warmScope := scopeFactory.CreateScope()
	_ = Get[IHeavyScopedRequest](warmScope.Container()).Handle(context.Background())
	warmScope.Dispose()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scope := scopeFactory.CreateScope()
		_ = Get[IHeavyScopedRequest](scope.Container()).Handle(context.Background())
		scope.Dispose()
	}
}
