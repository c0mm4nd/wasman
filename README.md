# WASMan (WebAssembly Manager)

[![](https://godoc.org/github.com/c0mm4nd/wasman?status.svg)](http://godoc.org/github.com/c0mm4nd/wasman)
[![Go Report Card](https://goreportcard.com/badge/github.com/c0mm4nd/wasman)](https://goreportcard.com/report/github.com/c0mm4nd/wasman)
![CI](https://github.com/c0mm4nd/wasman/workflows/CI/badge.svg)

A WebAssembly interpreter engine for gophers: pure Go, zero dependencies,
Go 1.18+.

## Features

- **Complete core semantics** — passes **100% of the executable assertions**
  of the official WebAssembly core test suite (18k+ assertions; see
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
| `Recover` | converts VM panics into returned errors, keeping the host process alive |
| `SkipValidation` | skips load-time validation (trusted modules only) |

## Conformance

The [spectest](./spectest) harness converts the official WebAssembly core test
suite with `wast2json` and runs all of it against wasman — behavioral
assertions and rejection assertions (`assert_invalid`, `assert_malformed`,
`assert_unlinkable`, `assert_uninstantiable`) alike. Current status:
**pass=18655 fail=0**; the only skips are text-format cases that would need a
WAT parser. See [spectest/README.md](./spectest/README.md).
