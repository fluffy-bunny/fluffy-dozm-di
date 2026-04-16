# Result Snapshot (2026-04-16)

This snapshot captures current memory profiling comparison results and repository state at the time of generation.

## Run metadata

- generatedAtUtc: 2026-04-16T21:43:46.7959967Z
- script: scripts/compare_memory_profiles.ps1
- outputDir: artifacts/memory-profiles/snapshot-2026-04-16
- address: 127.0.0.1:59446
- requestCount: 200
- modeComparison: steady vs leak

## Comparison summary (current values)

- goroutines.steady: 4
- goroutines.leak: 204
- goroutines.delta: 200
- heapBytes.steady: 2188
- heapBytes.leak: 4191
- heapBytes.ratioLeakOverSteady: 1.915
- allocsBytes.steady: 2919
- allocsBytes.leak: 4805
- allocsBytes.ratioLeakOverSteady: 1.646

## Interpretation

- Leak mode currently shows materially higher goroutine count than steady mode for this run.
- Leak mode profile sizes are larger than steady mode for both heap and allocs captures.
- This aligns with expected behavior: leak mode intentionally starts long-lived background load while steady mode performs one request pipeline per HTTP request.

## Artifact note

- Values above were captured from a live run of `scripts/compare_memory_profiles.ps1` at snapshot time.
- Temporary profiling artifacts were generated under `artifacts/memory-profiles/snapshot-2026-04-16` and may be cleaned after capture.

## Repository state at snapshot time

`git status --short` at snapshot time:

- M PROJECT_BASELINE.md
- M README.md
- M benchmark_test.go
- M cmd/memory_profiler/main.go
- M fixes_test.go
- ?? artifacts/
- ?? docs/
- ?? scripts/
