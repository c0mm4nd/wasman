package main

import "C"
import (
	"fmt"
	"os"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
)

func FibonacciRecursion(n int) int {
	if n <= 1 {
		return n
	}
	return FibonacciRecursion(n-1) + FibonacciRecursion(n-2)
}

// Run me on root folder
// go run ./examples/hoststring
func main() {
	linker1 := wasman.NewLinker(config.LinkerConfig{})

	f, err := os.Open("examples/numeric.wasm")
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

	for i := 0; i <= 20; i++ {
		returns, _, err := ins.CallExportedFunc("fib", uint64(i))
		if err != nil {
			panic(err)
		}

		fmt.Printf("fib(%d) is %d: got %d \n", i, FibonacciRecursion(i), returns[0])

	}

	fmt.Println("mem size", len(ins.Memory.Value))
}
