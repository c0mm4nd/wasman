// Package main demonstrates cross-module linking: one module exports a
// function and a mutable global, another module imports and uses them.
// Both wasm binaries are hand-assembled inline, so this example is fully
// self-contained — just `go run ./examples/linking`.
package main

import (
	"bytes"
	"fmt"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
)

// mathWasm is:
//
//	(module
//	  (global $count (mut i32) (i32.const 0))
//	  (func (export "add") (param i32 i32) (result i32)
//	    (i32.add (local.get 0) (local.get 1)))
//	  (func (export "bump")
//	    (global.set $count (i32.add (global.get $count) (i32.const 1))))
//	  (export "count" (global $count)))
var mathWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, // magic + version
	0x01, 0x0a, 0x02, 0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7f, 0x60, 0x00, 0x00, // types: (i32,i32)->i32, ()->()
	0x03, 0x03, 0x02, 0x00, 0x01, // funcs: add, bump
	0x06, 0x06, 0x01, 0x7f, 0x01, 0x41, 0x00, 0x0b, // global: (mut i32) = 0
	0x07, 0x16, 0x03, // exports:
	0x03, 0x61, 0x64, 0x64, 0x00, 0x00, // "add" -> func 0
	0x04, 0x62, 0x75, 0x6d, 0x70, 0x00, 0x01, // "bump" -> func 1
	0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x03, 0x00, // "count" -> global 0
	0x0a, 0x13, 0x02, // code:
	0x07, 0x00, 0x20, 0x00, 0x20, 0x01, 0x6a, 0x0b, // add: local.get 0; local.get 1; i32.add
	0x09, 0x00, 0x23, 0x00, 0x41, 0x01, 0x6a, 0x24, 0x00, 0x0b, // bump: count = count + 1
}

// appWasm is:
//
//	(module
//	  (import "math" "add" (func $add (param i32 i32) (result i32)))
//	  (import "math" "count" (global $count (mut i32)))
//	  (func (export "calc") (param i32 i32) (result i32)
//	    (call $add (local.get 0) (local.get 1)))
//	  (func (export "read_count") (result i32) (global.get $count)))
var appWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, // magic + version
	0x01, 0x0b, 0x02, 0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7f, 0x60, 0x00, 0x01, 0x7f, // types
	0x02, 0x1a, 0x02, // imports:
	0x04, 0x6d, 0x61, 0x74, 0x68, 0x03, 0x61, 0x64, 0x64, 0x00, 0x00, // "math" "add" (func type 0)
	0x04, 0x6d, 0x61, 0x74, 0x68, 0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x03, 0x7f, 0x01, // "math" "count" (global mut i32)
	0x03, 0x03, 0x02, 0x00, 0x01, // funcs: calc, read_count
	0x07, 0x15, 0x02, // exports:
	0x04, 0x63, 0x61, 0x6c, 0x63, 0x00, 0x01, // "calc" -> func 1 (0 is the import)
	0x0a, 0x72, 0x65, 0x61, 0x64, 0x5f, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x00, 0x02, // "read_count" -> func 2
	0x0a, 0x0f, 0x02, // code:
	0x08, 0x00, 0x20, 0x00, 0x20, 0x01, 0x10, 0x00, 0x0b, // calc: call $add
	0x04, 0x00, 0x23, 0x00, 0x0b, // read_count: global.get $count
}

func main() {
	// load and instantiate the exporting module first
	mathMod, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(mathWasm))
	if err != nil {
		panic(err)
	}
	mathIns, err := wasman.NewInstance(mathMod, nil)
	if err != nil {
		panic(err)
	}

	// register it under the import name "math" and instantiate the app
	linker := wasman.NewLinkerWithModuleMap(config.LinkerConfig{}, map[string]*wasman.Module{
		"math": mathMod,
	})
	appMod, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(appWasm))
	if err != nil {
		panic(err)
	}
	appIns, err := linker.Instantiate(appMod)
	if err != nil {
		panic(err)
	}

	// a cross-module function call: app.calc runs math.add
	ret, _, err := appIns.CallExportedFunc("calc", 20, 22)
	if err != nil {
		panic(err)
	}
	fmt.Println("app.calc(20, 22) =", int32(ret[0])) // 42

	// the mutable global is SHARED: mutations made through the math instance
	// are visible when the app reads its imported global
	for i := 0; i < 3; i++ {
		if _, _, err := mathIns.CallExportedFunc("bump"); err != nil {
			panic(err)
		}
	}
	ret, _, err = appIns.CallExportedFunc("read_count")
	if err != nil {
		panic(err)
	}
	fmt.Println("app.read_count() =", int32(ret[0])) // 3
}
