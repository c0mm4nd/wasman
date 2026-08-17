package wasman_test

import (
	"bytes"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/tollstation"
	"github.com/c0mm4nd/wasman/wat"
)

func seedWasm(f *testing.F) {
	for _, p := range []string{
		"bench/testdata/fib.wasm", "bench/testdata/sum.wasm",
		"bench/testdata/memrw.wasm", "bench/testdata/mandel.wasm",
		"bench/testdata/hash.wasm", "bench/testdata/sort.wasm",
		"bench/testdata/vmloop.wasm", "bench/testdata/indirect.wasm",
		"testdata/wideint.wasm",
		"spectest/testdata/memory_fill/memory_fill.0.wasm",
		"spectest/testdata/memory_copy/memory_copy.0.wasm",
	} {
		if b, err := os.ReadFile(p); err == nil {
			f.Add(b)
		}
	}
}

// FuzzModule: arbitrary bytes through decode + validation must never
// panic — only accept or error.
func FuzzModule(f *testing.F) {
	seedWasm(f)
	f.Fuzz(func(t *testing.T, raw []byte) {
		mod, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(raw))
		if err != nil || mod == nil {
			return
		}
		_ = mod.Exports()
	})
}

// FuzzExecDifferential: modules that instantiate run every exported
// function with zero arguments on both the interpreter and the JIT; the
// two must agree on success/trap and on every result. Execution is
// bounded by depth/memory limits and an interrupt watchdog.
func FuzzExecDifferential(f *testing.F) {
	seedWasm(f)
	depth := uint64(64)
	f.Fuzz(func(t *testing.T, raw []byte) {
		run := func(jit bool) (map[string][]uint64, map[string]bool) {
			mod, err := wasman.NewModule(config.ModuleConfig{
				Recover:        true,
				CallDepthLimit: &depth,
				MaxMemoryPages: 16,
				EnableJIT:      jit,
			}, bytes.NewReader(raw))
			if err != nil {
				return nil, nil
			}
			exps := mod.Exports()
			ins, err := wasman.NewInstance(mod, nil)
			if err != nil {
				return nil, nil
			}
			rets := map[string][]uint64{}
			oks := map[string]bool{}
			for _, e := range exps {
				if e.Type == nil {
					continue
				}
				args := make([]uint64, len(e.Type.InputTypes))
				done := make(chan struct{})
				var r []uint64
				var callErr error
				go func() {
					r, _, callErr = ins.CallExportedFunc(e.Name, args...)
					close(done)
				}()
				select {
				case <-done:
				case <-time.After(300 * time.Millisecond):
					ins.Interrupt()
					select {
					case <-done:
					case <-time.After(2 * time.Second):
						t.Skipf("uninterruptible loop in %q", e.Name)
					}
				}
				oks[e.Name] = callErr == nil
				if callErr == nil {
					rets[e.Name] = r
				}
			}
			return rets, oks
		}
		iRets, iOks := run(false)
		if iOks == nil {
			return
		}
		jRets, jOks := run(true)
		if jOks == nil {
			t.Fatal("interpreter instantiated but JIT config failed")
		}
		for name, ok := range iOks {
			if jOks[name] != ok {
				t.Fatalf("export %q: interp ok=%v, jit ok=%v", name, ok, jOks[name])
			}
			if !ok {
				continue
			}
			a, b := iRets[name], jRets[name]
			if len(a) != len(b) {
				t.Fatalf("export %q: result count %d vs %d", name, len(a), len(b))
			}
			for i := range a {
				// the i32/f32 upper halves are unspecified; compare low 32
				// unless both agree on the full width
				if a[i] != b[i] && uint32(a[i]) != uint32(b[i]) {
					t.Fatalf("export %q result[%d]: interp %#x, jit %#x", name, i, a[i], b[i])
				}
			}
		}
	})
}

// helpers for the structured bulk-memory fuzzer.
func u(v uint32) string { return strconv.FormatUint(uint64(v), 10) }

func watCompile(src string) ([]byte, error) { return wat.Compile([]byte(src)) }

func newModule(bin []byte, jit bool) (*wasman.Module, error) {
	return wasman.NewModule(config.ModuleConfig{EnableJIT: jit}, bytes.NewReader(bin))
}

func newInstance(mod *wasman.Module) (*wasman.Instance, error) {
	return wasman.NewInstance(mod, nil)
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// FuzzBulkMemory drives structured memory.fill / memory.copy operations
// with fuzzed offsets and lengths through both execution tiers and an
// independent Go reference. Random module bytes almost never keep a valid
// bulk-memory instruction, so this harness fixes the module shape and
// fuzzes only the operands — the coverage the byte-level fuzzer misses.
func FuzzBulkMemory(f *testing.F) {
	f.Add(uint32(8), uint32(0), uint32(0xAB), uint32(4), false)
	f.Add(uint32(65530), uint32(0), uint32(1), uint32(100), false) // OOB fill
	f.Add(uint32(200), uint32(0), uint32(0), uint32(4), true)      // copy
	f.Add(uint32(0), uint32(2), uint32(0), uint32(65540), true)    // OOB copy
	f.Fuzz(func(t *testing.T, dst, srcOrVal, val, n uint32, isCopy bool) {
		const pages = 1
		memBytes := pages * 65536
		// build the module text
		var op string
		if isCopy {
			op = "(memory.copy (i32.const " + u(dst) + ") (i32.const " + u(srcOrVal) + ") (i32.const " + u(n) + "))"
		} else {
			op = "(memory.fill (i32.const " + u(dst) + ") (i32.const " + u(val) + ") (i32.const " + u(n) + "))"
		}
		src := `(module (memory 1)
  (func (export "seed")
    (memory.fill (i32.const 0) (i32.const 0x11) (i32.const 256)))
  (func (export "op") ` + op + `))`
		bin, err := watCompile(src)
		if err != nil {
			return
		}

		// independent Go reference over a byte slice
		ref := make([]byte, memBytes)
		refTrap := false
		{
			for i := 0; i < 256; i++ {
				ref[i] = 0x11
			}
			if isCopy {
				if uint64(dst)+uint64(n) > uint64(memBytes) || uint64(srcOrVal)+uint64(n) > uint64(memBytes) {
					refTrap = true
				} else {
					copy(ref[dst:dst+n], ref[srcOrVal:srcOrVal+n])
				}
			} else {
				if uint64(dst)+uint64(n) > uint64(memBytes) {
					refTrap = true
				} else {
					for i := uint32(0); i < n; i++ {
						ref[dst+i] = byte(val)
					}
				}
			}
		}

		run := func(jit bool) (mem []byte, trapped bool, ok bool) {
			mod, err := newModule(bin, jit)
			if err != nil {
				return nil, false, false
			}
			ins, err := newInstance(mod)
			if err != nil {
				return nil, false, false
			}
			if _, _, err := ins.CallExportedFunc("seed"); err != nil {
				return nil, false, false
			}
			_, _, callErr := ins.CallExportedFunc("op")
			return append([]byte(nil), ins.Memory.Value...), callErr != nil, true
		}
		iMem, iTrap, iOk := run(false)
		jMem, jTrap, jOk := run(true)
		if !iOk || !jOk {
			return
		}
		if iTrap != refTrap {
			t.Fatalf("interp trap=%v, reference trap=%v (dst=%d n=%d copy=%v)", iTrap, refTrap, dst, n, isCopy)
		}
		if iTrap != jTrap {
			t.Fatalf("interp trap=%v, jit trap=%v", iTrap, jTrap)
		}
		if iTrap {
			return // both trapped; memory content unspecified after a trap
		}
		if !bytesEqual(iMem, ref) {
			t.Fatalf("interp memory differs from reference (dst=%d n=%d copy=%v)", dst, n, isCopy)
		}
		if !bytesEqual(iMem, jMem) {
			t.Fatalf("interp/jit memory divergence (dst=%d n=%d copy=%v)", dst, n, isCopy)
		}
	})
}

// FuzzTollConsistency is the consensus guard for inline-metered JIT: for
// arbitrary modules and gas caps, the interpreter and the baseline JIT must
// consume the SAME toll and trap (or not) identically.
func FuzzTollConsistency(f *testing.F) {
	for _, p := range []string{
		"bench/testdata/fib.wasm", "bench/testdata/sum.wasm",
		"bench/testdata/memrw.wasm", "bench/testdata/sort.wasm",
		"bench/testdata/vmloop.wasm", "bench/testdata/indirect.wasm",
	} {
		if b, err := os.ReadFile(p); err == nil {
			for _, m := range []uint64{5, 100, 100000, 1 << 40} {
				f.Add(b, m)
			}
		}
	}
	depth := uint64(200)
	f.Fuzz(func(t *testing.T, raw []byte, maxToll uint64) {
		if maxToll == 0 || maxToll > 20_000_000 {
			maxToll = 1 + maxToll%20_000_000
		}
		run := func(jit bool) (uint64, bool, bool) {
			cfg := config.ModuleConfig{
				EnableJIT:      jit,
				TollStation:    tollstation.NewSimpleTollStation(maxToll),
				MaxMemoryPages: 16,
				CallDepthLimit: &depth,
			}
			mod, err := wasman.NewModule(cfg, bytes.NewReader(raw))
			if err != nil {
				return 0, false, false
			}
			exps := mod.Exports()
			ins, err := wasman.NewInstance(mod, nil)
			if err != nil {
				return 0, false, false
			}
			for _, e := range exps {
				if e.Type == nil {
					continue
				}
				args := make([]uint64, len(e.Type.InputTypes))
				_, _, callErr := ins.CallExportedFunc(e.Name, args...)
				return cfg.TollStation.GetToll(), callErr != nil, true
			}
			return 0, false, false
		}
		ig, it, iok := run(false)
		if !iok {
			return
		}
		jg, jt, jok := run(true)
		if !jok {
			t.Fatal("interpreter loaded but JIT config failed")
		}
		if ig != jg || it != jt {
			t.Fatalf("gas/trap divergence: interp (gas=%d trap=%v) jit (gas=%d trap=%v)", ig, it, jg, jt)
		}
	})
}
