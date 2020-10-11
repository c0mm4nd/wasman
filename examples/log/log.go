package main

import (
	"fmt"
	"github.com/c0mm4nd/wasman"
	"os"
)

// Run me on root folder
// go run ./examples/log
func main() {
	linker1 := wasman.NewLinker()

	err := linker1.DefineAdvancedFunc("env", "log_message", func(ins *wasman.Instance) interface{} {
		return func(ptr uint32, l uint32) {
			message := ins.Memory[int(ptr):int(ptr+l)]

			fmt.Println(string(message))
		}
	})

	wasm, err := os.Open("examples/log.wasm")
	if err != nil {
		panic(err)
	}

	module, err := wasman.NewModule(wasm, nil)
	if err != nil {
		panic(err)
	}
	ins, err := linker1.Instantiate(module)
	if err != nil {
		panic(err)
	}

	_, _, err = ins.CallExportedFunc("do_something")
	if err != nil {
		panic(err)
	}
}
