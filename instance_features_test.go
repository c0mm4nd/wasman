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
