# Memory Profiling Playbook

This playbook is for manual diagnostics and comparison runs.

Use two modes:
- leak mode: intentionally creates long-lived background load
- steady mode: handles one DI request pipeline per HTTP request

## Prerequisites

- Go toolchain installed
- Access to this repository root

## Runtime controls

Environment variables used by `cmd/memory_profiler`:
- `MEMORY_PROFILER_MODE`: `leak` or `steady` (default: `leak`)
- `MEMORY_PROFILER_ADDR`: server bind address (default: `localhost:8989`)

## Start the profiler server

From repo root:

PowerShell (leak mode):
- `$env:MEMORY_PROFILER_MODE = "leak"`
- `go run ./cmd/memory_profiler`

PowerShell (steady mode):
- `$env:MEMORY_PROFILER_MODE = "steady"`
- `go run ./cmd/memory_profiler`

Optional custom address:
- `$env:MEMORY_PROFILER_ADDR = "localhost:8990"`

## One-command automation

Use the comparison script to run both steady and leak modes, generate traffic, capture artifacts, and write a comparison summary:

- `pwsh -File ./scripts/compare_memory_profiles.ps1`

By default, the script auto-selects an available localhost port to avoid collisions.

Common options:

- `-RequestCount 5000`
- `-Address localhost:8990`
- `-OutputDir artifacts/memory-profiles/run-01`
- `-SkipTop` (skip `go tool pprof -top` summaries)

Script output layout:

- `steady/heap.pb.gz`
- `steady/allocs.pb.gz`
- `steady/goroutine.txt`
- `steady/heap-top.txt` (unless `-SkipTop`)
- `steady/allocs-top.txt` (unless `-SkipTop`)
- `leak/heap.pb.gz`
- `leak/allocs.pb.gz`
- `leak/goroutine.txt`
- `leak/heap-top.txt` (unless `-SkipTop`)
- `leak/allocs-top.txt` (unless `-SkipTop`)
- `run-metadata.txt`
- `comparison-summary.txt` (steady-vs-leak deltas and ratio summary)

## Generate traffic

Open a second terminal and call the root endpoint repeatedly.

PowerShell:
- `1..2000 | ForEach-Object { Invoke-WebRequest -UseBasicParsing http://localhost:8989/ | Out-Null }`

Notes:
- In leak mode, each request starts additional long-lived background work.
- In steady mode, each request executes exactly one scope lifecycle.

## Capture pprof snapshots

Heap profile:
- `go tool pprof http://localhost:8989/debug/pprof/heap`

Goroutine profile:
- `go tool pprof http://localhost:8989/debug/pprof/goroutine`

Allocation profile:
- `go tool pprof http://localhost:8989/debug/pprof/allocs`

Inside pprof, useful commands:
- `top`
- `top -cum`
- `list doRequest`
- `web` (if graph rendering is available)

## Comparison workflow

1. Run steady mode and collect heap/goroutine snapshots at low and high request counts.
2. Restart in leak mode and repeat the same request counts.
3. Compare:
- goroutine count growth pattern
- retained heap growth pattern
- top cumulative call paths

Expected high-level behavior:
- steady mode should trend toward stable goroutine count after load bursts
- leak mode should show sustained growth due to intentionally spawned long-lived load loops

## Important boundaries

- This playbook is not a CI pass/fail test.
- CI leak safety is validated by deterministic tests in `fixes_test.go`.
- Use this playbook when investigating regressions or memory behavior in real workloads.
