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
code at instantiation (arm64 and amd64; anything outside the compiled
subset falls back to the interpreter per function — across the official
spec suite, >99% of function bodies compile). Same workloads, same
machine:

| ns/op (lower is better) | **wasman-jit** | wasman (interp) | wazero-compiler |
|---|---|---|---|
| `Fib` | 449,170 | 929,032 | **36,281** |
| `Sum` | 394,724 | 2,910,643 | **26,915** |
| `MemRW` | 417,285 | 4,574,404 | **71,001** |

wasman's JIT is a *template* JIT: each opcode expands to a fixed native
sequence against a memory-resident operand stack, and calls exit to the
host and re-enter. That removes dispatch and decode overhead entirely
(2–19× over the interpreter depending on the workload and machine — the
gap widens on hosts with slower single-thread performance) while staying
a few thousand lines of dependency-free Go. wazero's compiler is an
optimizing code generator with register allocation and native calls; it
remains 6–15× faster on these microbenchmarks and is the right choice
when raw wasm throughput is the only criterion. wasman's niche is the
combination: zero dependencies, Go 1.18, full-suite conformance,
metering/interruption rails — with a JIT that removes most of the
interpretation cost when the platform allows it.

## Running

```bash
go test -bench . -benchmem ./bench          # from the repository root
```

The `.wasm` fixtures are compiled from the `.wat` sources in `testdata/`
with `wat2wasm` and checked in (they are tiny).
