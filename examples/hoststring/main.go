package main

import (
	"fmt"
	"github.com/c0mm4nd/wasman/utils"
	"os"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
)

// Run me on root folder
// go run ./examples/hoststring
func main() {
	linker1 := wasman.NewLinker(config.LinkerConfig{})

	err := linker1.DefineAdvancedFunc("env", "host_string", func(ins *wasman.Instance) interface{} {
		return func() uint32 {
			message := "WASMan"

			ptr := ins.Memory.Grow(utils.CalcPageSize(len(message), config.DefaultPageSize))
			copy(ins.Memory.Value[ptr:], message)

			return ptr
		}
	})
	if err != nil {
		panic(err)
	}

	// cannot call host func in the host func
	err = linker1.DefineAdvancedFunc("env", "log_message", func(ins *wasman.Instance) interface{} {
		return func(ptr uint32, l uint32) {
			message := ins.Memory.Value[int(ptr):int(ptr+l)]

			fmt.Println(string(message))
		}
	})
	if err != nil {
		panic(err)
	}

	wasm, err := os.Open("examples/hoststring.wasm")
	if err != nil {
		panic(err)
	}

	module, err := wasman.NewModule(config.ModuleConfig{}, wasm)
	if err != nil {
		panic(err)
	}
	ins, err := linker1.Instantiate(module)
	if err != nil {
		panic(err)
	}

	_, _, err = ins.CallExportedFunc("greet")
	if err != nil {
		panic(err)
	}
}
