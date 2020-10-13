package main

import (
	"fmt"
	"os"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
)

// Run me on root folder
// go run ./examples/log
func main() {
	linker1 := wasman.NewLinker(config.LinkerConfig{})

	// cannot call host func in the host func
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

	module, err := wasman.NewModule(config.ModuleConfig{}, wasm)
	if err != nil {
		panic(err)
	}
	ins, err := linker1.Instantiate(module)
	if err != nil {
		panic(err)
	}

	name := "wasman engine"
	ptr := ins.ManuallyGrowMemory(uint32(len(name)))
	copy(ins.Memory[ptr:], name)

	_, _, err = ins.CallExportedFunc("greet", uint64(ptr))
	if err != nil {
		panic(err)
	}
}
