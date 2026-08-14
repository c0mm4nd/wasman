package wasman

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/c0mm4nd/wasman/config"
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
