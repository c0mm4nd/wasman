package main

import "C"
import (
	"fmt"
	"os"
	"unsafe"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
)

// Run me on root folder
// go run ./examples/hoststring
func main() {
	linker1 := wasman.NewLinker(config.LinkerConfig{})

	//err := linker1.DefineMemory("env", "memory", make([]byte, 10))

	err := linker1.DefineAdvancedFunc("env", "host_string", func(ins *wasman.Instance) interface{} {
		return func() uint32 {
			message := "WASMan"

			ret, _, err := ins.CallExportedFunc("allocate", uint64(len(message)+1))
			if err != nil {
				panic(err)
			}

			copy(ins.Memory.Value[ret[0]:], append([]byte(message), byte(0))) // act as a string for rust's CStr::from_ptr(ptr)

			return uint32(ret[0])
		}
	})
	if err != nil {
		panic(err)
	}

	// cannot call host func in the host func
	err = linker1.DefineAdvancedFunc("env", "log_message", func(ins *wasman.Instance) interface{} {
		return func(ptr uint32, l uint32) {
			// string way
			fmt.Println(C.GoString((*C.char)(unsafe.Pointer(&ins.Memory.Value[ptr]))))

			// bytes way
			msg := ins.Memory.Value[ptr : ptr+l]
			fmt.Println(string(msg))
		}
	})
	if err != nil {
		panic(err)
	}

	f, err := os.Open("examples/hoststring.wasm")
	if err != nil {
		panic(err)
	}

	module, err := wasman.NewModule(config.ModuleConfig{}, f)
	if err != nil {
		panic(err)
	}
	ins, err := linker1.Instantiate(module)
	if err != nil {
		panic(err)
	}

	for range make([]byte, 999) {
		_, _, err = ins.CallExportedFunc("greet")
		if err != nil {
			panic(err)
		}
	}

	fmt.Println("mem size", len(ins.Memory.Value))
}
