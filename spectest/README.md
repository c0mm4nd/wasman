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
on post-MVP features. Against the current testsuite it passes ~**99.94%** of
executed behavioral assertions (`pass=15933 fail=9` at the time of writing).

Post-MVP features already implemented:

- sign-extension operators (`i32/i64.extend{8,16,32}_s`)
- non-trapping float→int (`trunc_sat`, `0xFC` prefix)
- bulk-memory data/element segment flags encoding (active / passive /
  declarative / explicit index); passive segments are parsed but not applied
- multi-table (element segments per table, `call_indirect` table index)
- the `DataCount` section (consumed and ignored)

The last handful of failures each need a distinct, still-unimplemented proposal:

- extended constant expressions — `(data (i32.add (i32.const 0) (i32.const 42)))`
  (`data`, 4 asserts)
- multi-value block/loop/if signatures — `(loop (param i64 i64) (result i64))`
  (`fac`, 2 asserts)
- reference types: typed element expression lists (`ref.func`/`ref.null` in
  element segments) (`binary`, 2 asserts)
- trapping out-of-bounds active data segments at instantiation (`data`, 1 assert)

Reaching a literal 100% also requires a module validator, since the suite's
`assert_invalid` / `assert_malformed` commands assert that a *bad* module is
rejected — wasman currently accepts and runs, so those are reported as skips.

Suites that fail to convert with `wast2json` (post-MVP syntax) are simply
skipped by `gen.sh` and reported.
