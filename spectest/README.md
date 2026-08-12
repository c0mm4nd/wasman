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

wasman started as a WebAssembly **MVP** interpreter and is incrementally taking
on post-MVP features. Against the current testsuite it passes ~**99.4%** of
executed behavioral assertions (`pass=15884 fail=103` at the time of writing).

Post-MVP features already implemented:

- sign-extension operators (`i32/i64.extend{8,16,32}_s`)
- non-trapping float→int (`trunc_sat`, `0xFC` prefix)

The remaining failures are post-MVP features not yet implemented, notably:

- bulk-memory / multi-memory data segments (`data`, parts of `token`)
- reference types (`select` with a type, typed `ref.null`) — blocks conversion
  of `elem`, `select`, `table`, `br_if`, `memory`, `globals`, …
- multi-value block/loop/if signatures (`fac-ssa`, parts of `if`)
- module-level validation (`assert_invalid` / `assert_malformed`, `binary*`)
- specific NaN payloads from float arithmetic (`f32`, `f64`)

Suites that fail to convert with `wast2json` (post-MVP syntax) are simply
skipped by `gen.sh` and reported.
