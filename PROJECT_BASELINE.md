# Project Baseline: fluffy-dozm-di

This document is a fast onboarding baseline for future work in this repository.

## What this project is

`fluffy-dozm-di` is a reflection-based dependency injection container for Go, based on `dozm/di` with added features.

Core capabilities:
- Generic registration and resolution helpers (`AddSingleton`, `AddScoped`, `AddTransient`, `Get`, `TryGet`).
- Lifetime handling (Singleton, Scoped, Transient).
- Constructor injection using reflected function signatures.
- Registration by lookup key with metadata.
- Multi-registration and slice resolution (`Get[[]T]`).
- Optional validation (`ValidateScopes`, `ValidateOnBuild`, `DetectLifetimeConflicts`).
- Built-in services in container (`Container`, `ScopeFactory`, `IsService`).

Primary API entry points:
- `Builder()` in `builder.go`
- `Get`, `TryGet`, `GetByLookupKey`, `TryGetByLookupKey`, `Invoke` in `di.go`

## Important feature semantics

### Resolution order
- Last registration wins for single-service resolution (`Get[T]`).
- All registrations remain available in order via slice resolution (`Get[[]T]`).

### Lifetimes
- Singleton: one instance for root container lifetime.
- Scoped: one instance per scope.
- Transient: new instance per resolution.

### Interface registration support
- Services can register implemented interfaces explicitly when adding descriptors.
- This supports selectively exposing only specific interfaces.
- New helper available for type-safe interface type extraction:
  - `ImplementedInterfaceType[T any]() reflect.Type`

Example:

```go
AddSingleton[*department](b,
    func(tt ITime) *department { return &department{Time: tt} },
    ImplementedInterfaceType[IDepartment](),
    ImplementedInterfaceType[IDepartment2](),
)
```

### Lookup key support
- `Add*WithLookupKeys` variants hash service type + key.
- `GetByLookupKey[T](c, key)` resolves keyed registrations.
- Metadata map is attached to descriptors.

### Validation switches
Configured via `ContainerBuilder.ConfigureOptions(func(*Options))`.
- `ValidateScopes`: runtime scope/lifetime safety checks.
- `ValidateOnBuild`: validates descriptor call sites while building container.
- `DetectLifetimeConflicts`: panics if same service type has mixed lifetimes.

## Leak validation strategy

The project uses a 3-layer strategy:

1. Deterministic CI tests (primary proof)
- Scope-per-request simulations create and dispose scopes repeatedly.
- Assertions focus on invariants: scoped disposables are disposed, active scoped count returns to zero, and disposed scopes reject further resolution.

2. Benchmark trend checks (regression signal)
- Request lifecycle benchmark (`create scope -> resolve request graph -> dispose scope`) is used with `-benchmem` to track allocations/op and detect regressions over time.

3. Manual pprof diagnostics (deep investigation)
- `cmd/memory_profiler` is used for manual exploratory profiling and should not be treated as deterministic CI pass/fail criteria.
- See `docs/MEMORY_PROFILING.md` for the repeatable workflow and leak-vs-steady comparison.

## Internal architecture (mental model)

1. Registrations create `Descriptor` objects.
2. `Builder.Build()` creates `container` and `CallSiteFactory`.
3. `CallSiteFactory` builds call sites for constructors/factories/constants/slices.
4. Resolver executes call sites and caches by lifetime.
5. Scope tracks scoped instances and disposables.

Key files:
- `builder.go`: registration helpers + build pipeline.
- `container.go`: resolution entry points and accessor caching.
- `callsite.go`: call site structures and circular dependency chain checks.
- `scope.go`: scope lifecycle and disposable tracking.
- `descriptor.go`: descriptor creation and service/interface type validation.

## Test coverage snapshot

Snapshot command:

```sh
go test ./... -cover
```

Latest observed result (2026-04-16):
- `github.com/fluffy-bunny/fluffy-dozm-di`: `85.6%` statements
- `cmd/memory_profiler`: `0.0%`
- `errorx`: `0.0%`
- `reflectx`: `0.0%`
- `syncx`: `0.0%`
- `util`: `0.0%`

## Behavior coverage map by test file

- `builder_test.go`
  - Builder `Contains`/`Remove` semantics.
  - Post-removal resolution behavior.

- `callsite_test.go`
  - Missing registration errors.
  - Circular dependency detection.
  - Implicit/exact slice call site behavior.

- `container_test.go`
  - Scoped instance uniqueness across scopes.
  - Slice order and default (last) value behavior.
  - Constructor parameter injection.
  - Singleton concurrency behavior.
  - Disposal semantics for singleton/scoped/instance services.
  - `Invoke` behavior and container disposal paths.

- `conflict_test.go`
  - Last-registration-wins rules across lifetimes.
  - Root vs scope behavior when lifetimes differ.
  - Slice returns all registrations.
  - `DetectLifetimeConflicts` panic behavior.

- `many_interfaces_test.go`
  - Registering concrete types as multiple interfaces.
  - Multi-interface resolution and slice order.
  - Generic unique type registration pattern.
  - `ImplementedInterfaceType[T]()` helper behavior and integration.

- `lookup_keys_test.go`
  - Keyed registration and lookup across singleton/transient/scoped/instance.
  - Metadata presence and typed lookup behavior.

- `inject_container_test.go`
  - Injecting `Container` into singleton/transient/scoped services.
  - Scope safety expectations under `ValidateScopes`.

- `factory_test.go`
  - Factory descriptors and mixed dependencies.
  - Scoped factory lifetime behavior.

- `funcs_test.go`
  - Function-type registration helpers (`AddFunc`, `AddFuncWithLookupKeys`).

- `fixes_test.go`
  - Regression coverage for race/disposal/concurrency scenarios.
  - Lookup key not found behavior.
  - Descriptor cache/slice aliasing regression.
  - Nil/invalid constructor panic behavior.
  - `Remove`/`Contains` with implemented interface types.
  - Aggregate error unwrap behavior.
  - Very-heavy scope churn tests for request-like scoped lifecycles and deterministic non-retention assertions.

- `benchmark_test.go`
  - Benchmark scaffolding for performance baselines.
  - Request lifecycle benchmark for mixed singleton/transient/scoped resolution with scope disposal.

## Practical quick-start checklist

1. Start with `README.md` for usage intent and examples.
2. Use `builder.go` and `di.go` for public API work.
3. Use `many_interfaces_test.go`, `lookup_keys_test.go`, and `conflict_test.go` as behavior references for feature changes.
4. Run `go test ./...` after code edits.
5. Run `go test ./... -cover` when updating this baseline.
6. Run `go test ./... -run HeavyScopedRequestSimulation -count=1` when validating scoped lifecycle leak protections.
7. Run `go test ./... -bench HeavyScopedRequestLifecycle -benchmem` for allocation trend monitoring.
8. Use the manual playbook in `docs/MEMORY_PROFILING.md` when investigating memory behavior.
9. Use `scripts/compare_memory_profiles.ps1` for repeatable steady-vs-leak artifact capture and generated `comparison-summary.txt` deltas.

## Changelog highlights

### 2026-04-16
- Added very-heavy scope churn leak-safety tests (single-threaded and concurrent) in `fixes_test.go`.
- Added mixed-lifetime request lifecycle benchmark in `benchmark_test.go`.
- Added profiler mode controls (`steady`, `leak`) in `cmd/memory_profiler/main.go`.
- Added repeatable profiling playbook in `docs/MEMORY_PROFILING.md`.
- Added automation script `scripts/compare_memory_profiles.ps1` for steady-vs-leak capture.
- Added generated `comparison-summary.txt` output with goroutine deltas and profile size ratios.
- Added point-in-time status snapshot in `docs/RESULT_SNAPSHOT_2026-04-16.md`.

## Design decisions log

### Why deterministic tests are the primary leak gate
- pprof observations are environment-sensitive and not suitable as strict CI pass/fail criteria.
- Deterministic assertions on scope disposal counters give stable leak-safety guarantees in CI.

### Why manual profiling still exists
- Deterministic tests prove invariants, but pprof captures call-path and heap-shape evidence needed for deep investigations.
- The script plus playbook make this operational workflow repeatable instead of ad hoc.

### Why script output includes both raw profiles and a summary
- Raw profiles preserve full fidelity for deep debugging.
- `comparison-summary.txt` gives a fast first-pass signal (goroutine deltas and leak/steady ratios) without opening pprof immediately.

## Known coverage gaps / future hardening

- Subpackages (`errorx`, `reflectx`, `syncx`, `util`) currently show no direct test coverage in package-level report.
- `cmd/memory_profiler` is uncovered.
- If these packages become extension points, add focused unit tests to lock contracts and error semantics.
