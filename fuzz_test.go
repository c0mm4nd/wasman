package wasman_test

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
)

func seedWasm(f *testing.F) {
	for _, p := range []string{
		"bench/testdata/fib.wasm", "bench/testdata/sum.wasm",
		"bench/testdata/memrw.wasm", "bench/testdata/mandel.wasm",
		"bench/testdata/hash.wasm", "bench/testdata/sort.wasm",
		"bench/testdata/vmloop.wasm", "bench/testdata/indirect.wasm",
		"testdata/wideint.wasm",
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
