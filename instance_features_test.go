package wasman

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/wasm"
)

// spinModule is (module (func (export "spin") (loop (br 0))) (func (export "ok")))
var spinModule = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	0x01, 0x04, 0x01, 0x60, 0x00, 0x00, // type () -> ()
	0x03, 0x03, 0x02, 0x00, 0x00, // funcs: spin, ok
	0x07, 0x0d, 0x02, 0x04, 0x73, 0x70, 0x69, 0x6e, 0x00, 0x00, 0x02, 0x6f, 0x6b, 0x00, 0x01, // exports
	0x0a, 0x0c, 0x02,
	0x07, 0x00, 0x03, 0x40, 0x0c, 0x00, 0x0b, 0x0b, // spin: loop br 0 end
	0x02, 0x00, 0x0b, // ok: (empty)
}

func spinInstance(t *testing.T) *Instance {
	t.Helper()
	mod, err := NewModule(config.ModuleConfig{}, bytes.NewReader(spinModule))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := NewInstance(mod, nil)
	if err != nil {
		t.Fatal(err)
	}
	return ins
}

func TestCallWithContextTimeout(t *testing.T) {
	ins := spinInstance(t)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, _, err := ins.CallExportedFuncWithContext(ctx, "spin")
	if !errors.Is(err, wasm.ErrInterrupted) {
		t.Fatalf("want ErrInterrupted, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("interrupt took too long: %v", elapsed)
	}

	// the instance stays usable after an interrupt (stale flag cleared)
	if _, _, err := ins.CallExportedFunc("ok"); err != nil {
		t.Fatalf("instance unusable after interrupt: %v", err)
	}
}

func TestInterruptFromAnotherGoroutine(t *testing.T) {
	ins := spinInstance(t)

	go func() {
		time.Sleep(20 * time.Millisecond)
		ins.Interrupt()
	}()
	_, _, err := ins.CallExportedFunc("spin")
	if !errors.Is(err, wasm.ErrInterrupted) {
		t.Fatalf("want ErrInterrupted, got %v", err)
	}
}

func TestCallWithCancelledContext(t *testing.T) {
	ins := spinInstance(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := ins.CallExportedFuncWithContext(ctx, "ok"); !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

// hostCallModule is (module (import "env" "f" (func $f (result i32)))
//
//	(func (export "run") (result i32) (call $f)))
var hostCallModule = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	0x01, 0x05, 0x01, 0x60, 0x00, 0x01, 0x7f, // type () -> i32
	0x02, 0x09, 0x01, 0x03, 0x65, 0x6e, 0x76, 0x01, 0x66, 0x00, 0x00, // import env.f
	0x03, 0x02, 0x01, 0x00, // func: run
	0x07, 0x07, 0x01, 0x03, 0x72, 0x75, 0x6e, 0x00, 0x01, // export "run" -> func 1
	0x0a, 0x06, 0x01, 0x04, 0x00, 0x10, 0x00, 0x0b, // run: call 0
}

func TestHostFuncError(t *testing.T) {
	boom := errors.New("boom")
	fail := false

	linker := NewLinker(config.LinkerConfig{})
	if err := linker.DefineFunc("env", "f", func() (int32, error) {
		if fail {
			return 0, boom
		}
		return 42, nil
	}); err != nil {
		t.Fatal(err)
	}

	mod, err := NewModule(config.ModuleConfig{}, bytes.NewReader(hostCallModule))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := linker.Instantiate(mod)
	if err != nil {
		t.Fatal(err)
	}

	// nil error: the value comes through, the error is not pushed
	rets, _, err := ins.CallExportedFunc("run")
	if err != nil || int32(rets[0]) != 42 {
		t.Fatalf("want 42, got %v (err %v)", rets, err)
	}

	// non-nil error: the wasm caller traps with the host's error
	fail = true
	if _, _, err := ins.CallExportedFunc("run"); !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}

	// the instance stays usable
	fail = false
	if rets, _, err := ins.CallExportedFunc("run"); err != nil || int32(rets[0]) != 42 {
		t.Fatalf("instance unusable after host error: %v %v", rets, err)
	}
}

// memModule is (module (memory (export "mem") 2)
//
//	(func (export "grow") (result i32) (memory.grow (i32.const 1))))
var memModule = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	0x01, 0x05, 0x01, 0x60, 0x00, 0x01, 0x7f, // type () -> i32
	0x03, 0x02, 0x01, 0x00, // func: grow
	0x05, 0x03, 0x01, 0x00, 0x02, // memory min 2
	0x07, 0x0e, 0x02, 0x03, 0x6d, 0x65, 0x6d, 0x02, 0x00, 0x04, 0x67, 0x72, 0x6f, 0x77, 0x00, 0x00, // exports
	0x0a, 0x08, 0x01, 0x06, 0x00, 0x41, 0x01, 0x40, 0x00, 0x0b, // grow: i32.const 1; memory.grow
}

func TestMaxMemoryPages(t *testing.T) {
	// initial memory above the host cap: instantiation fails
	mod, err := NewModule(config.ModuleConfig{MaxMemoryPages: 1}, bytes.NewReader(memModule))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewInstance(mod, nil); err == nil {
		t.Fatal("want instantiation failure for min 2 > cap 1")
	}

	// growth beyond the cap returns -1 and does not allocate
	mod, err = NewModule(config.ModuleConfig{MaxMemoryPages: 2}, bytes.NewReader(memModule))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := NewInstance(mod, nil)
	if err != nil {
		t.Fatal(err)
	}
	rets, _, err := ins.CallExportedFunc("grow")
	if err != nil || int32(rets[0]) != -1 {
		t.Fatalf("grow beyond host cap: want -1, got %v (err %v)", rets, err)
	}
	if pages := len(ins.Memory.Value) / 65536; pages != 2 {
		t.Fatalf("memory grew past the cap: %d pages", pages)
	}

	// without a cap the same growth succeeds
	mod, err = NewModule(config.ModuleConfig{}, bytes.NewReader(memModule))
	if err != nil {
		t.Fatal(err)
	}
	ins, err = NewInstance(mod, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rets, _, err := ins.CallExportedFunc("grow"); err != nil || int32(rets[0]) != 2 {
		t.Fatalf("uncapped grow: want 2, got %v (err %v)", rets, err)
	}
}

func TestModuleExports(t *testing.T) {
	mod, err := NewModule(config.ModuleConfig{}, bytes.NewReader(counterModule))
	if err != nil {
		t.Fatal(err)
	}
	exps := mod.Exports()
	if len(exps) != 2 {
		t.Fatalf("want 2 exports, got %d", len(exps))
	}
	// sorted by name: "g" (global), "run" (func () -> ())
	if exps[0].Name != "g" || exps[0].Kind != segments.KindGlobal || exps[0].Type != nil {
		t.Fatalf("bad export[0]: %+v", exps[0])
	}
	if exps[1].Name != "run" || exps[1].Kind != segments.KindFunction || exps[1].Type == nil ||
		len(exps[1].Type.InputTypes) != 0 || len(exps[1].Type.ReturnTypes) != 0 {
		t.Fatalf("bad export[1]: %+v", exps[1])
	}
}

// trapModule is (module (func $boom (unreachable)) (func (export "go") (call $boom)))
// with a custom "name" section naming func 0 "boom".
var trapModule = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	0x01, 0x04, 0x01, 0x60, 0x00, 0x00, // type () -> ()
	0x03, 0x03, 0x02, 0x00, 0x00, // funcs: boom, go
	0x07, 0x06, 0x01, 0x02, 0x67, 0x6f, 0x00, 0x01, // export "go" -> func 1
	0x0a, 0x0a, 0x02,
	0x03, 0x00, 0x00, 0x0b, // boom: unreachable
	0x04, 0x00, 0x10, 0x00, 0x0b, // go: call 0
	// custom "name" section: subsection 1, func 0 -> "boom"
	0x00, 0x0e, 0x04, 0x6e, 0x61, 0x6d, 0x65, // "name"
	0x01, 0x07, 0x01, 0x00, 0x04, 0x62, 0x6f, 0x6f, 0x6d, // funcnames: 0 -> "boom"
}

func TestTrapBacktrace(t *testing.T) {
	mod, err := NewModule(config.ModuleConfig{}, bytes.NewReader(trapModule))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := NewInstance(mod, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = ins.CallExportedFunc("go")
	if err == nil {
		t.Fatal("want a trap")
	}
	msg := err.Error()
	if !strings.Contains(msg, "at boom") || !strings.Contains(msg, "at func[1]") {
		t.Fatalf("backtrace missing frames: %q", msg)
	}
	// the underlying trap survives the decoration
	if !errors.Is(err, wasm.ErrUnreachable) {
		t.Fatalf("wrapped error lost its cause: %v", err)
	}
}

func TestInstanceReset(t *testing.T) {
	mod, err := NewModule(config.ModuleConfig{}, bytes.NewReader(counterModule))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := NewInstance(mod, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, _, err := ins.CallExportedFunc("run"); err != nil {
			t.Fatal(err)
		}
	}
	if got := int32(*ins.Globals[0]); got != 3 {
		t.Fatalf("pre-reset g = %d, want 3", got)
	}

	ins.Reset()
	if got := int32(*ins.Globals[0]); got != 0 {
		t.Fatalf("post-reset g = %d, want 0", got)
	}
	// still runnable after reset
	if _, _, err := ins.CallExportedFunc("run"); err != nil {
		t.Fatal(err)
	}
	if got := int32(*ins.Globals[0]); got != 1 {
		t.Fatalf("post-reset run g = %d, want 1", got)
	}
}

func TestInstanceResetMemory(t *testing.T) {
	mod, err := NewModule(config.ModuleConfig{}, bytes.NewReader(memModule))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := NewInstance(mod, nil)
	if err != nil {
		t.Fatal(err)
	}
	ins.Memory.Value[0] = 0xAA
	if rets, _, err := ins.CallExportedFunc("grow"); err != nil || int32(rets[0]) != 2 {
		t.Fatalf("grow: %v %v", rets, err)
	}

	ins.Reset()
	if pages := len(ins.Memory.Value) / 65536; pages != 2 {
		t.Fatalf("post-reset pages = %d, want 2", pages)
	}
	if ins.Memory.Value[0] != 0 {
		t.Fatalf("post-reset memory not restored: %#x", ins.Memory.Value[0])
	}
}
