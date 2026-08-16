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

# The full WASM 1.0 core suite, plus the two bulk-memory suites wasman
# implements (memory.fill / memory.copy); the other bulk-memory ops
# (memory.init, data.drop, table.*) remain unimplemented.
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
	memory_fill memory_copy
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

# ---------------------------------------------------------------------------
# Legacy suites: several suites in the current testsuite use post-MVP syntax
# that wast2json cannot convert (reference-types text syntax etc.), so their
# coverage would be silently lost. Fetch those from a pinned pre-reference-
# types revision (2020-05-29) instead. Two documented patches are applied:
#   - elem.wast: strip (duplicate) element segment names, which modern wabt
#     rejects (a pure text-syntax issue, no semantic change)
#   - imports.wast: drop the single `assert_invalid "multiple tables"` for
#     two locally-defined tables — wasman intentionally supports multi-table
#     (the modern call_indirect suite requires it), so that MVP-era
#     restriction no longer applies
# ---------------------------------------------------------------------------
LEGACY_REF="13ca8ae7e29bdf13bcfaabfd10e415b98d3103d1"
LEGACY_BASE="https://raw.githubusercontent.com/WebAssembly/testsuite/${LEGACY_REF}"

# name[:remote-file] pairs (global.wast was later renamed globals.wast)
LEGACY=(
	align br_if br_table elem exports func globals:global imports linking
	local_tee memory memory_grow select unreached-invalid
)

if [ ${#names[@]} -eq ${#ALL[@]} ]; then # only on a full run
	for entry in "${LEGACY[@]}"; do
		name="${entry%%:*}"
		remote="${entry#*:}"
		[ "$remote" = "$entry" ] && remote="$name"
		# skip if the modern suite already converted
		[ -f "$OUT/$name/$name.json" ] && continue
		dir="$OUT/$name"
		mkdir -p "$dir"
		wast="$dir/$name.wast"
		echo "==> $name (legacy)"
		ok=0
		for attempt in 1 2 3; do
			if curl -sSf --max-time 30 "$LEGACY_BASE/$remote.wast" -o "$wast"; then ok=1; break; fi
			sleep 1
		done
		if [ "$ok" -ne 1 ]; then
			echo "    download failed for $name (skipping)" >&2
			continue
		fi
		case "$name" in
		elem)
			perl -pi -e 's/\(elem \$\w+ /(elem /g' "$wast"
			;;
		imports)
			perl -0pi -e 's/\(assert_invalid\n  \(module \(table 10 funcref\) \(table 10 funcref\)\)\n  "multiple tables"\n\)\n//' "$wast"
			;;
		esac
		if ! wast2json --no-check "$wast" -o "$dir/$name.json" 2>"$dir/gen.err"; then
			echo "    wast2json failed for $name (see $dir/gen.err)" >&2
		fi
	done
fi

echo "done -> $OUT"
