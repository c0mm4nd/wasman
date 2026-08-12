// Package main demonstrates module validation: wasman.NewModule statically
// type-checks every function body and rejects invalid modules up front.
// config.ModuleConfig.SkipValidation opts out for trusted-but-nonconforming
// modules — at your own risk. Fully self-contained: `go run ./examples/validation`.
package main

import (
	"bytes"
	"fmt"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
)

// invalidWasm is:
//
//	(module (func (export "f") (result i32)))   ;; empty body, but promises an i32!
var invalidWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, // magic + version
	0x01, 0x05, 0x01, 0x60, 0x00, 0x01, 0x7f, // type: () -> i32
	0x03, 0x02, 0x01, 0x00, // funcs: f
	0x07, 0x05, 0x01, 0x01, 0x66, 0x00, 0x00, // export "f" -> func 0
	0x0a, 0x04, 0x01, 0x02, 0x00, 0x0b, // code: f = (empty)
}

func main() {
	// by default the module is rejected at load time with a helpful error
	_, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(invalidWasm))
	fmt.Println("validated load:", err)

	// SkipValidation loads it anyway...
	mod, err := wasman.NewModule(config.ModuleConfig{
		SkipValidation: true,
		Recover:        true, // ...so keep Recover on: invalid code may blow up at run time
	}, bytes.NewReader(invalidWasm))
	if err != nil {
		panic(err)
	}
	ins, err := wasman.NewInstance(mod, nil)
	if err != nil {
		panic(err)
	}

	// the invalid body fails at run time instead (recovered into an error)
	_, _, err = ins.CallExportedFunc("f")
	fmt.Println("unvalidated call:", err)
}
