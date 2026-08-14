# Benchmarks

Cross-runtime comparison of wasman against [wazero](https://github.com/tetratelabs/wazero)
(the other zero-dependency pure-Go runtime, in both its interpreter and its
compiler/JIT mode). This directory is its own Go module so the main wasman
module keeps zero dependencies.

cgo bindings (wasmtime-go, wasmer-go) are deliberately excluded: they embed
prebuilt native libraries, which is a different deployment category from a
pure-Go dependency.

## Workloads

| workload | shape |
|---|---|
| `Fib` | fib(20), recursive — 13k wasm→wasm calls (call overhead) |
| `Sum` | 100k-iteration arithmetic loop (dispatch + branching) |
| `MemRW` | write then read 64KiB of linear memory (load/store) |
| `Instantiate` | decode + validate + instantiate a small module |

## Results (Apple M3, Go 1.24, wazero v1.8.2)

```
BenchmarkFib/wasman-8                      947_240 ns/op        11 B/op        1 allocs/op
BenchmarkFib/wazero-interp-8             1_387_251 ns/op   525_411 B/op   21_893 allocs/op
BenchmarkFib/wazero-compiler-8              36_221 ns/op        16 B/op        2 allocs/op
BenchmarkSum/wasman-8                    3_021_605 ns/op         8 B/op        1 allocs/op
BenchmarkSum/wazero-interp-8             4_150_689 ns/op        40 B/op        3 allocs/op
BenchmarkSum/wazero-compiler-8              28_686 ns/op        16 B/op        2 allocs/op
BenchmarkMemRW/wasman-8                  4_659_851 ns/op         8 B/op        1 allocs/op
BenchmarkMemRW/wazero-interp-8           4_896_432 ns/op        40 B/op        3 allocs/op
BenchmarkMemRW/wazero-compiler-8            77_244 ns/op        16 B/op        2 allocs/op
BenchmarkInstantiate/wasman-8                2_021 ns/op    11_564 B/op       49 allocs/op
BenchmarkInstantiate/wazero-interp-8        13_728 ns/op    16_543 B/op       91 allocs/op
BenchmarkInstantiate/wazero-compiler-8      74_047 ns/op   332_186 B/op      368 allocs/op
```

## Honest reading

- **wasman beats wazero's interpreter on every workload**: 1.46× on
  call-heavy code, 1.37× on arithmetic loops, 1.05× on memory traffic —
  while allocating ~1 object per exported call versus thousands, and
  instantiating (including full validation) ~6.8× faster.
- The wins come from load-time pre-compilation into side tables (immediates,
  branch targets, block lookups pre-decoded once), pooled call frames,
  value-type labels, direct loop back-branches and an inlined jump-table
  dispatch for trap-free hot opcodes.
- **wazero's compiler (JIT) remains 30–160× faster at execution.** That is
  the architectural gap between native code and any interpreter, not a
  tuning gap; if raw throughput is the priority, use a JIT.

## Running

```bash
go test -bench . -benchmem ./bench          # from the repository root
```

The `.wasm` fixtures are compiled from the `.wat` sources in `testdata/`
with `wat2wasm` and checked in (they are tiny).
