# Examples

## Self-contained (just `go run`)

These embed hand-assembled wasm binaries, so they run without any downloads:

| example | shows |
|---|---|
| [`linking`](./linking) | cross-module linking: importing another module's function and **mutable global** (mutations are shared between instances) |
| [`validation`](./validation) | `NewModule` rejecting an invalid module up front; `SkipValidation` + `Recover` turning run-time misbehavior into errors |
| [`gas`](./gas) | metering execution with a `TollStation` and aborting on budget exhaustion |

```bash
go run ./examples/linking
go run ./examples/validation
go run ./examples/gas
```

## Requiring prebuilt wasm files

These need the demo `.wasm` files from
[minimum-wasm-rs](https://github.com/C0MM4ND/minimum-wasm-rs/releases/latest)
placed in this directory (see [`downloader.go`](./downloader.go), or run
`go generate ./examples` to fetch them):

| example | shows |
|---|---|
| [`numeric`](./numeric) | loading a module and calling a pure function (`fib`) |
| [`log`](./log) | defining a host function the module imports |
| [`hoststring`](./hoststring) | passing strings host→module through linear memory (allocator + advanced host funcs) |
| [`hostbytes`](./hostbytes) | passing byte slices host→module through linear memory |

```bash
go generate ./examples   # downloads the .wasm fixtures
go run ./examples/numeric
```

All examples are run from the repository root.
