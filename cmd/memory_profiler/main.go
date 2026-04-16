package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
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
var profilerMode string

var stemService = (*requestService)(nil)

func (r *requestService) Ctor() (IRequest, error) {
	return &requestService{}, nil
}
func (r *requestService) Request(ctx context.Context) RequestResponse {
	return RequestResponse{StatusCode: 200, Data: []byte("Hello, World!")}
}

func doRequest() {
	scopeFactory := di.Get[di.ScopeFactory](container)
	scope := scopeFactory.CreateScope()
	defer func() {
		scope.Dispose()
	}()
	c := scope.Container()
	request := di.Get[IRequest](c)
	request.Request(context.Background())
}

func startLeakingLoad() {
	go func() {
		for {
			go doRequest()
			time.Sleep(1 * time.Millisecond)
		}
	}()
}

func handler(w http.ResponseWriter, r *http.Request) {
	if profilerMode == "leak" {
		// Intentionally starts long-lived background load for leak analysis.
		startLeakingLoad()
		fmt.Fprintln(w, "Hello, World! (leak mode)")
		return
	}

	// Steady mode executes one request pipeline per HTTP request.
	doRequest()
	fmt.Fprintln(w, "Hello, World! (steady mode)")
}

func main() {
	profilerMode = strings.ToLower(os.Getenv("MEMORY_PROFILER_MODE"))
	if profilerMode == "" {
		profilerMode = "leak"
	}
	if profilerMode != "leak" && profilerMode != "steady" {
		panic(fmt.Sprintf("invalid MEMORY_PROFILER_MODE '%s'; expected 'leak' or 'steady'", profilerMode))
	}

	addr := os.Getenv("MEMORY_PROFILER_ADDR")
	if addr == "" {
		addr = "localhost:8989"
	}

	// Create a ContainerBuilder
	b := di.Builder()
	di.AddScoped[IRequest](b, stemService.Ctor)
	// Build the container
	container = b.Build()
	http.HandleFunc("/", handler)
	fmt.Printf("memory profiler mode=%s addr=%s\n", profilerMode, addr)
	go func() {
		fmt.Println(http.ListenAndServe(addr, nil))
	}()
	select {} // Keep main goroutine alive
}
