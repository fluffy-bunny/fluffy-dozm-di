package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"time"

	di "github.com/fluffy-bunny/fluffy-dozm-di"
)

type (
	RequestResponse struct {
		StatusCode int
		Data       []byte
	}
	IRequest interface {
		Request(ctx context.Context) RequestResponse
	}
	requestService struct{}
)

var container di.Container

var stemService = (*requestService)(nil)

func (r *requestService) Ctor() (IRequest, error) {
	return &requestService{}, nil
}
func (r *requestService) Request(ctx context.Context) RequestResponse {
	return RequestResponse{StatusCode: 200, Data: []byte("Hello, World!")}
}
func leakGoroutine() {
	go func() {

		doRequest := func() {
			scopeFactory := di.Get[di.ScopeFactory](container)
			scope := scopeFactory.CreateScope()
			defer func() {
				scope.Dispose()
			}()
			c := scope.Container()
			request := di.Get[IRequest](c)
			request.Request(context.Background())

		}
		for {
			go doRequest()
			time.Sleep(1 * time.Millisecond)

		}
	}()
}

func handler(w http.ResponseWriter, r *http.Request) {
	leakGoroutine() // Start a new leaking goroutine on each request
	fmt.Fprintln(w, "Hello, World!")
}

func main() {
	// Create a ContainerBuilder
	b := di.Builder()
	di.AddScoped[IRequest](b, stemService.Ctor)
	// Build the container
	container = b.Build()
	http.HandleFunc("/", handler)
	go func() {
		fmt.Println(http.ListenAndServe("localhost:8989", nil))
	}()
	select {} // Keep main goroutine alive
}
