# Benchmarks — Go WebAssembly interpreters

Cross-runtime comparison of wasman against the other Go-implemented
WebAssembly **interpreters**:

- [wazero](https://github.com/tetratelabs/wazero) v1.12 (interpreter mode)
- [wagon](https://github.com/go-interpreter/wagon) v0.4 (archived)
- [life](https://github.com/perlin-network/life) (archived)

Scope notes: interpreters and compiler/JIT engines execute in different
performance classes, so they are tabled separately below. cgo bindings
(wasmtime-go, wasmer-go) embed prebuilt native libraries and are excluded.
This directory is its own Go module so the main wasman module keeps zero
dependencies.

## Workloads

| workload | shape |
|---|---|
| `Fib` | fib(20), recursive — 13k wasm→wasm calls (call overhead) |
| `Sum` | 100k-iteration arithmetic loop (dispatch + branching) |
| `MemRW` | write then read 64KiB of linear memory (load/store) |
| `Mandel` | f64 escape-time iteration over a 400x400 grid (float) |
| `Hash` | 1M-round i64 multiply/xor/rotate mixing |
| `Sort` | recursive quicksort of 10k i32s in linear memory |
| `Dispatch` | 1M-iteration br_table dispatch loop |
| `Indirect` | 1M call_indirect invocations through a 4-entry table |
| `Instantiate` | decode + validate + instantiate a small module |

Every run is checked for the correct result before timing.

## Results (Apple M3, Go 1.24)

| ns/op (lower is better) | **wasman** | wazero-interp | wagon | life |
|---|---|---|---|---|
| `Fib` | **974,787** | 1,787,663 | 2,296,426 | panics¹ |
| `Sum` | **3,014,618** | 3,930,498 | 5,383,776 | 6,163,361 |
| `MemRW` | **4,751,546** | 5,153,545 | 7,345,079 | 8,514,190 |
| `Instantiate` | **2,129** | 14,100 | 7,188 | 29,827 |
| allocs/op (Fib) | **1** | 21,893 | 65,673 | — |

¹ life (archived since 2019) panics executing this valid MVP module; the
harness records that as a skip.

**wasman is the fastest Go WebAssembly interpreter on every workload**:
1.1–1.8× faster than wazero's interpreter, 1.5–2.4× faster than wagon and
~2× faster than life at execution, with the cheapest steady state (~1
allocation per exported call) and the fastest instantiation (including
full validation) by 3–14×.

The wins come from load-time pre-compilation into side tables (immediates,
branch targets and block lookups decoded once), pooled call frames,
value-type labels, direct loop back-branches and an inlined jump-table
dispatch for trap-free hot opcodes.

## Compiler class: wasman JIT

`config.ModuleConfig{EnableJIT: true}` compiles function bodies to native
code at instantiation (arm64 and amd64). The pipeline is tiered: an
optimizing compiler — virtual-register IR, linear-scan register
allocation, compare/branch fusion, a native calling convention between
compiled functions — takes ~86% of the spec suite's function bodies;
a baseline template compiler covers most of the rest (notably float
code), and anything else falls back to the interpreter per function.
The whole official test suite passes with the JIT enabled.

Same workloads, same machine (Apple M3), single run:

| ns/op (lower is better) | **wasman-jit** | wazero-compiler | wasman (interp) |
|---|---|---|---|
| `Fib` | 41,187 | **37,832** | 1,120,975 |
| `Sum` | **27,112** | 26,850 | 3,030,506 |
| `MemRW` | **39,575** | 70,643 | 4,630,009 |

On an amd64 server (dual EPYC 7K62) the picture is similar: `Sum` runs
2.0x and `MemRW` 1.6x ahead of wazero-compiler, `Fib` 1.6x behind
(indirect-jump call linkage costs more on that microarchitecture).

Five kernel workloads widen the picture beyond the core loops
(`BenchmarkKernels`, same machine):

| ns/op | **wasman-jit** | wazero-compiler | note |
|---|---|---|---|
| `Sort` (recursive quicksort, memory) | **367,952** | 477,368 | 1.3x ahead |
| `Dispatch` (br_table-dense loop) | **1,513,461** | 1,536,712 | level |
| `Hash` (i64 multiply/rotate mixing) | 1,382,997 | **1,268,367** | within 9% |
| `Indirect` (call_indirect-dense) | 3,926,713 | **2,812,512** | 1.4x behind |
| `Mandel` (f64 escape-time) | 36,104,628 | **5,911,049** | 6x behind¹ |

¹ float arithmetic runs on the baseline tier today (the optimizing tier
does not allocate float registers yet); Mandel is the honest cost of
that gap and the next optimization target. `Indirect` dispatches
natively through a per-instance table mirror (bounds, null and
signature checks in generated code) and falls back to the host for
anything the mirror cannot prove.

The two engines now sit in the same performance class: wasman leads on
memory-bound code, ties on arithmetic loops and trails within ~10% on
call-heavy recursion (on arm64). wasman remains pure dependency-free Go
1.18 with full-suite conformance, metering and interruption rails; the
JIT is simply how fast that package can go when the platform allows it.

## Running

```bash
go test -bench . -benchmem ./bench          # from the repository root
```

The `.wasm` fixtures are compiled from the `.wat` sources in `testdata/`
with `wat2wasm` and checked in (they are tiny).
