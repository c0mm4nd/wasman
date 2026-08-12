#!/usr/bin/env bash
#
# gen.sh downloads the official WebAssembly 1.0 (MVP) core test suite and
# converts every .wast file into a JSON manifest + companion .wasm/.wat files
# using `wast2json` (from wabt). The output lands in ./testdata and is consumed
# by spectest_test.go.
#
# Requirements: bash, curl, and wast2json (brew install wabt / apt install wabt).
#
# Usage:  ./gen.sh            # generate the full suite
#         ./gen.sh i32 f64    # generate only the named suites
#
# note: intentionally not using `set -e` so a single flaky download or an
# unconvertible suite does not abort the whole generation run.
set -uo pipefail

# The canonical WebAssembly testsuite (current syntax accepted by modern wabt).
# wasman targets the MVP surface, so post-MVP suites simply show up as
# failures/skips in the report rather than being silently omitted.
BASE="https://raw.githubusercontent.com/WebAssembly/testsuite/main"

cd "$(dirname "$0")"
OUT="testdata"
mkdir -p "$OUT"

if ! command -v wast2json >/dev/null 2>&1; then
	echo "error: wast2json not found (install wabt: brew install wabt)" >&2
	exit 1
fi

# The full WASM 1.0 core suite.
ALL=(
	address align binary-leb128 binary block br br_if br_table break-drop
	call call_indirect comments const conversions custom data elem endianness
	exports f32 f32_bitwise f32_cmp f64 f64_bitwise f64_cmp fac float_exprs
	float_literals float_memory float_misc forward func func_ptrs globals i32
	i64 if imports inline-module int_exprs int_literals labels left-to-right
	linking load local_get local_set local_tee loop memory memory_grow
	memory_redundancy memory_size memory_trap names nop return select
	skip-stack-guard-page stack start store switch token traps type
	unreachable unreached-invalid unwind utf8-custom-section-id
	utf8-import-field utf8-import-module utf8-invalid-encoding
)

# Compile the host "spectest" module the suite imports.
if command -v wat2wasm >/dev/null 2>&1; then
	wat2wasm spectest.wat -o "$OUT/spectest.wasm"
else
	echo "warning: wat2wasm not found; spectest.wasm not (re)generated" >&2
fi

names=("$@")
if [ ${#names[@]} -eq 0 ]; then
	names=("${ALL[@]}")
fi

for name in "${names[@]}"; do
	dir="$OUT/$name"
	mkdir -p "$dir"
	wast="$dir/$name.wast"
	echo "==> $name"
	# retry a few times to ride out transient TLS/network hiccups
	ok=0
	for attempt in 1 2 3; do
		if curl -sSf --max-time 30 "$BASE/$name.wast" -o "$wast"; then ok=1; break; fi
		sleep 1
	done
	if [ "$ok" -ne 1 ]; then
		echo "    download failed for $name (skipping)" >&2
		continue
	fi
	# --enable-... left off on purpose: we want the MVP surface only.
	if ! wast2json --no-check "$wast" -o "$dir/$name.json" 2>"$dir/gen.err"; then
		echo "    wast2json failed for $name (see $dir/gen.err)" >&2
	fi
done

echo "done -> $OUT"
