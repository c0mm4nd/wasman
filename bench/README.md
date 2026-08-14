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
BenchmarkFib/wasman-8                    1_643_856 ns/op        19 B/op        1 allocs/op
BenchmarkFib/wazero-interp-8             1_392_512 ns/op   525_406 B/op   21_893 allocs/op
BenchmarkFib/wazero-compiler-8              36_645 ns/op        16 B/op        2 allocs/op
BenchmarkSum/wasman-8                    8_777_680 ns/op        11 B/op        1 allocs/op
BenchmarkSum/wazero-interp-8             3_856_769 ns/op        40 B/op        3 allocs/op
BenchmarkSum/wazero-compiler-8              26_831 ns/op        16 B/op        2 allocs/op
BenchmarkMemRW/wasman-8                 11_519_515 ns/op        12 B/op        1 allocs/op
BenchmarkMemRW/wazero-interp-8           4_939_585 ns/op        40 B/op        3 allocs/op
BenchmarkMemRW/wazero-compiler-8            73_083 ns/op        16 B/op        2 allocs/op
BenchmarkInstantiate/wasman-8                2_858 ns/op    10_828 B/op       45 allocs/op
BenchmarkInstantiate/wazero-interp-8        13_175 ns/op    16_543 B/op       91 allocs/op
BenchmarkInstantiate/wazero-compiler-8      70_616 ns/op   332_182 B/op      368 allocs/op
```

## Honest reading

- **wazero's compiler (JIT) is 40–300× faster at execution.** That is the
  expected gap between native code and any interpreter; if raw throughput is
  the priority, use a JIT.
- **Against wazero's interpreter** (the apples-to-apples comparison): wasman
  is within ~1.2× on call-heavy code and ~2.3× slower on tight loops —
  wazero pre-compiles to an internal IR while wasman dispatches the raw
  bytecode, trading peak speed for simplicity.
- **wasman is the cheapest at steady state and startup**: 1 allocation per
  exported call (the results slice) versus tens of thousands for wazero's
  interpreter on call-heavy code, and instantiation (including full
  validation) is ~4.6× faster than wazero-interp and ~25× faster than
  warming up the JIT — relevant for short-lived or pooled instances
  (see `Instance.Reset`).

## Running

```bash
go test -bench . -benchmem ./bench          # from the repository root
```

The `.wasm` fixtures are compiled from the `.wat` sources in `testdata/`
with `wat2wasm` and checked in (they are tiny).
