# WASMan (WebAssembly Manager)

[![Go Reference](https://pkg.go.dev/badge/github.com/c0mm4nd/wasman.svg)](https://pkg.go.dev/github.com/c0mm4nd/wasman)
[![Wasm core spec](https://img.shields.io/badge/wasm_core_spec-19220%2F19220_·_0_skipped-brightgreen)](./spectest/README.md)
![CI](https://github.com/c0mm4nd/wasman/workflows/CI/badge.svg)
[![Go version](https://img.shields.io/github/go-mod/go-version/c0mm4nd/wasman)](./go.mod)
[![Release](https://img.shields.io/github/v/tag/c0mm4nd/wasman?label=release&sort=semver)](https://github.com/c0mm4nd/wasman/tags)

A WebAssembly interpreter engine for gophers: pure Go, zero dependencies,
Go 1.18+.

## Features

- **Complete core semantics** — passes the official WebAssembly core test
  suite with **zero failures and zero skips** (19k+ assertions; see
  [spectest/README.md](./spectest/README.md) for the details and how to run it)
- **Full static validation** — strict binary decoding plus spec-conformant
  type checking of every function body at load time
- **Cross-module linking** — import/export functions, tables, memories and
  globals between modules; imported functions execute in their defining
  module's state, tables dispatch correctly across modules, and mutable
  globals share storage
- **Post-MVP features** — sign-extension operators, non-trapping float→int
  (`trunc_sat`), multi-value blocks, multiple tables, bulk-memory segment
  encodings, reference-type element lists, extended constant expressions
- **Host integration** — define Go functions, globals, memories and tables
  that wasm modules import; reach into instance memory from host functions
- **Metering & safety rails** — per-instruction tolls with a budget
  (`TollStation`), call-depth limits, panic recovery, and an opt-out
  (`SkipValidation`) for trusted-but-nonconforming modules

## Usage

### As a library

```bash
go get github.com/c0mm4nd/wasman
```

```go
package main

import (
	"fmt"
	"os"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
)

func main() {
	f, _ := os.Open("module.wasm")
	defer f.Close()

	// NewModule decodes AND validates; invalid modules are rejected here
	module, err := wasman.NewModule(config.ModuleConfig{}, f)
	if err != nil {
		panic(err)
	}

	ins, err := wasman.NewInstance(module, nil)
	if err != nil {
		panic(err)
	}

	returns, _, err := ins.CallExportedFunc("fib", 20)
	if err != nil {
		panic(err)
	}
	fmt.Println(returns[0]) // 6765
}
```

Defining host functions a module can import:

```go
linker := wasman.NewLinker(config.LinkerConfig{})

// a plain Go func, importable as (import "env" "log_i32" (func (param i32)))
_ = linker.DefineFunc("env", "log_i32", func(v int32) { fmt.Println("wasm says:", v) })

// an "advanced" func that can touch the instance (memory, exported calls, tolls)
_ = linker.DefineAdvancedFunc("env", "print", func(ins *wasman.Instance) interface{} {
	return func(ptr, size uint32) {
		fmt.Println(string(ins.Memory.Value[ptr : ptr+size]))
	}
})

ins, err := linker.Instantiate(module)
```

More in the [examples folder](./examples) — including fully self-contained
examples for [cross-module linking](./examples/linking),
[validation](./examples/validation) and [gas metering](./examples/gas) that
run with just `go run`.

### As an executable

```bash
go install github.com/c0mm4nd/wasman/cmd/wasman@latest
```

```bash
$ wasman -h
Usage of ./wasman:
  -extern-files string
        external modules files
  -func string
        main func (default "main")
  -main string
        main module (default "module.wasm")
  -max-toll uint
        the maximum toll in simple toll station
```

Example: [numeric.wasm](https://github.com/C0MM4ND/minimum-wasm-rs/releases/latest)

```bash
$ wasman -main numeric.wasm -func fib 20 # calc the fibonacci number
{
  type: i32
  result: 6765
  toll: 315822
}
```

If the max toll is limited, execution aborts on overflow:

```bash
$ wasman -main numeric.wasm -max-toll 300000 -func fib 20
panic: toll overflow
```

## Validation

`wasman.NewModule` performs **full static validation** (strict binary decoding
plus spec-conformant type checking of every function body) and rejects
malformed or invalid modules with an error. Modules that older wasman versions
loaded permissively may now be refused — that is intentional.

If you must load a trusted-but-nonconforming module, set
`config.ModuleConfig{SkipValidation: true}` and be aware that invalid code can
then trap, misbehave, or (with `Recover` disabled) panic at run time. See
[examples/validation](./examples/validation).

## Configuration

`config.ModuleConfig` knobs:

| field | effect |
|---|---|
| `TollStation` | per-instruction metering with a budget; execution errors on overflow |
| `CallDepthLimit` | caps call nesting; runaway recursion traps instead of overflowing the Go stack |
| `MaxMemoryPages` | host-side hard cap on linear memory, regardless of the module's declared limits |
| `Recover` | converts VM panics into returned errors, keeping the host process alive |
| `CanonicalizeNaNs` | canonicalizes float-arithmetic NaNs for fully deterministic execution |
| `EnableJIT` | compiles function bodies to native code at instantiation (arm64/amd64); unsupported constructs fall back to the interpreter per function |
| `EnableWideInt` | exposes the optional `u128`/`u256` import namespaces: 128/256-bit add/sub/mul/div/rem/compare/shift/bitwise host operations over little-endian values in linear memory (EVM division conventions; signed variants use the `_s` suffix) |
| `SkipValidation` | skips load-time validation (trusted modules only) |

Run-time control on an `Instance`:

| API | effect |
|---|---|
| `CallExportedFuncWithContext` | binds a call to a `context.Context`: cancellation/timeout interrupts execution |
| `Interrupt` | stops the running (or next) execution from any goroutine |
| `Reset` | restores the post-instantiation snapshot (memory + own globals) for instance pooling |
| `Module.Exports` | lists exports with resolved function signatures before instantiation |

Host functions defined with `DefineFunc`/`DefineAdvancedFunc` may declare a
trailing `error` return: a non-nil error traps the calling wasm code. Traps
carry a wasm backtrace using names from the module's custom `name` section.

## Performance

wasman is **the fastest Go WebAssembly interpreter** in our cross-runtime
benchmarks — ahead of wazero (interpreter mode), wagon and life on every
workload — with ~1 allocation per exported call at steady state and the
fastest instantiation (including full validation) by 3–14×.

On arm64 and amd64, `EnableJIT` additionally compiles function bodies to
native machine code at instantiation time — a tiered, pure-Go JIT (an
optimizing compiler with register allocation and native calls, plus a
baseline tier and per-function interpreter fallback; still zero
dependencies). It runs these workloads 27–116× faster than the
interpreter, which puts wasman in the same performance class as wazero's
optimizing compiler: ahead on memory-bound code, level on arithmetic
loops, within ~10% on call-heavy recursion (arm64). The whole official
test suite passes with the JIT enabled. See
[bench/README.md](./bench/README.md) for numbers and methodology.

## Conformance

The [spectest](./spectest) harness converts the official WebAssembly core test
suite with `wast2json` and runs all of it against wasman — behavioral
assertions and rejection assertions (`assert_invalid`, `assert_malformed`,
`assert_unlinkable`, `assert_uninstantiable`) alike, including the
text-format malformed cases via the `wat` text-format reader. Current
status: **pass=19220 fail=0 skip=0**. See
[spectest/README.md](./spectest/README.md).
