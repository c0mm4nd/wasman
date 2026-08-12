// Package main demonstrates metering execution with a TollStation: every
// executed instruction is charged, and execution aborts once the configured
// budget is exhausted. Fully self-contained: `go run ./examples/gas`.
package main

import (
	"bytes"
	"fmt"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/tollstation"
)

// burnWasm is:
//
//	(module
//	  (func (export "burn") (param i32)          ;; spin the loop $n times
//	    (block (loop
//	      (br_if 1 (i32.eqz (local.get 0)))
//	      (local.set 0 (i32.sub (local.get 0) (i32.const 1)))
//	      (br 0)))))
var burnWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, // magic + version
	0x01, 0x05, 0x01, 0x60, 0x01, 0x7f, 0x00, // type: (i32) -> ()
	0x03, 0x02, 0x01, 0x00, // funcs: burn
	0x07, 0x08, 0x01, 0x04, 0x62, 0x75, 0x72, 0x6e, 0x00, 0x00, // export "burn"
	0x0a, 0x18, 0x01, 0x16, // code, 1 body of 22 bytes
	0x00,       // no locals
	0x02, 0x40, // block
	0x03, 0x40, // loop
	0x20, 0x00, 0x45, 0x0d, 0x01, // br_if 1 (i32.eqz (local.get 0))
	0x20, 0x00, 0x41, 0x01, 0x6b, 0x21, 0x00, // local.set 0 (i32.sub (local.get 0) 1)
	0x0c, 0x00, // br 0
	0x0b, 0x0b, 0x0b, // end end end
}

func run(budget uint64, n uint64) {
	ts := tollstation.NewSimpleTollStation(budget) // 0 = unlimited
	mod, err := wasman.NewModule(config.ModuleConfig{TollStation: ts}, bytes.NewReader(burnWasm))
	if err != nil {
		panic(err)
	}
	ins, err := wasman.NewInstance(mod, nil)
	if err != nil {
		panic(err)
	}

	_, _, err = ins.CallExportedFunc("burn", n)
	if err != nil {
		fmt.Printf("burn(%d) with budget %d: %v (toll spent: %d)\n", n, budget, err, ts.GetToll())
		return
	}
	fmt.Printf("burn(%d) completed, toll spent: %d\n", n, ts.GetToll())
}

func main() {
	run(0, 10000)    // unlimited budget: runs to completion, reports the cost
	run(1000, 10000) // tight budget: aborts with a toll overflow error
}
