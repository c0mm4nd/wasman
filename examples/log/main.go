package main

import (
	"C"
	"fmt"
	"os"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
)
import "unsafe"

// Run me on root folder
// go run ./examples/log
func main() {
	linker1 := wasman.NewLinker(config.LinkerConfig{})

	// cannot call host func in the host func
	err := linker1.DefineAdvancedFunc("env", "log_message", func(ins *wasman.Instance) interface{} {
		return func(ptr uint32, l uint32) {
			// need ptr & l
			messageByLen := ins.Memory.Value[int(ptr):int(ptr+l)]
			fmt.Println(string(messageByLen))

			// this method just need one ptr
			messageByCharVec := C.GoString((*C.char)(unsafe.Pointer(&ins.Memory.Value[ptr])))
			fmt.Println(messageByCharVec)
		}
	})
	if err != nil {
		panic(err)
	}

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
	ret, _, err := ins.CallExportedFunc("allocate", uint64(len(name)+1))
	if err != nil {
		panic(err)
	}

	ptr := ret[0]

	copy(ins.Memory.Value[ptr:], name)

	for range make([]byte, 100) {
		_, _, err = ins.CallExportedFunc("greet", ptr)
		if err != nil {
			panic(err)
		}
	}

	fmt.Println("mem size", len(ins.Memory.Value))

}
