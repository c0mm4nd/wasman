# spectest — official WebAssembly core suite runner

This package runs the official [WebAssembly core test suite](https://github.com/WebAssembly/testsuite)
against wasman.

## How it works

1. `gen.sh` downloads the `.wast` source files and converts each one into a
   JSON manifest plus companion `.wasm` binaries using
   [`wast2json`](https://github.com/WebAssembly/wabt) (from wabt). It also
   compiles `spectest.wat` — the host module the suite imports — into
   `testdata/spectest.wasm`.
2. `spectest_test.go` reads every manifest and drives wasman: instantiating
   modules, invoking exported functions, reading globals, and checking results
   / traps / instantiation errors.

The generated `testdata/` is **not** committed. If it is absent the test skips
itself, so CI without wabt stays green.

## Usage

```bash
# one-time: install wabt, then generate the suite
brew install wabt        # or: apt-get install wabt
./gen.sh                 # or ./gen.sh i32 f64  to generate specific suites

# run (report-only; see the per-suite + TOTAL summary)
go test ./spectest -v

# strict mode: behavioral mismatches fail the test (regression gating)
SPECTEST_STRICT=1 go test ./spectest
```

## Command coverage

| command                                   | handling                              |
|-------------------------------------------|---------------------------------------|
| `module` / `register` / `action` / `get`  | executed                              |
| `assert_return`                           | executed, result values compared      |
| `assert_trap` / `assert_exhaustion`       | executed, a trap is required          |
| `assert_unlinkable` / `assert_uninstantiable` | executed, an error is required    |
| `assert_invalid` / `assert_malformed`     | skipped — wasman has no validator     |

NaN results are compared per spec (`nan:canonical` / `nan:arithmetic`). Each
invoke runs under a watchdog so a VM infinite-loop is reported as a failure
instead of hanging the test binary.

## Current status

wasman started as a WebAssembly **MVP** interpreter and now passes **100%** of
the executed behavioral assertions in the core suite (`pass=15936 fail=0` at
the time of writing).

Post-MVP features implemented along the way:

- sign-extension operators (`i32/i64.extend{8,16,32}_s`)
- non-trapping float→int (`trunc_sat`, `0xFC` prefix)
- bulk-memory data/element segment flags encoding (active / passive /
  declarative / explicit index); passive segments are parsed but not applied
- reference-type element expression lists (`ref.func` / `ref.null`)
- multi-table (element segments per table, `call_indirect` table index)
- multi-value block/loop/if signatures (type-index block types + branch arity)
- extended constant expressions (`i32/i64` add/sub/mul over constants)
- the `DataCount` section (consumed and ignored)

The remaining ~1900 commands are **skipped**, not failed: they are
`assert_invalid` (type-invalid modules) and `assert_malformed` (malformed
binaries) commands that assert a *bad* module is **rejected**. wasman has no
validation pass — it decodes and runs — so it cannot (yet) satisfy them. Making
those green requires a full WebAssembly validator (type checking, control-flow
checking), strict binary-decoding, and UTF-8 validation of name strings — a
sizeable subsystem tracked separately.

Suites that fail to convert with `wast2json` (post-MVP syntax) are simply
skipped by `gen.sh` and reported.
