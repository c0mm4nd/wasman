package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/c0mm4nd/wasman"
)

var strMainModuleFile = flag.String("main", "module.wasm", "main module")

var funcName = flag.String("func", "main", "main func")
var maxToll = flag.Uint64("max-toll", 0, "cap for simple toll station")

var strExternModules = flag.String("extern-files", "", "external modules files")

var stdout = os.Stdout // for wasi

func main() {
	flag.Parse()

	externModules := strings.Split(*strExternModules, ",")
	f, err := os.Open(*strMainModuleFile)
	if err != nil {
		panic(err)
	}

	mainMod, err := wasman.NewModule(f, &wasman.ModuleConfig{
		DisableFloatPoint: false,
		TollStation:       wasman.NewSimpleTollStation(*maxToll),
	})
	if err != nil {
		panic(err)
	}

	externMods := make(map[string]*wasman.Module)
	for _, pair := range externModules {
		if pair == "" {
			continue
		}

		li := strings.Split(pair, ":")
		if len(li) != 2 {
			panic("invalid external module: should input with -extern=<name1>:<file1>,<name2>:<file2>")
		}

		f, err := os.Open(li[1])
		if err != nil {
			panic(err)
		}

		mod, err := wasman.NewModule(f, nil)

		externMods[li[0]] = mod
	}

	l := wasman.NewLinkerWithModuleMap(externMods)
	ins, err := l.Instantiate(mainMod)
	if err != nil {
		panic(err)
	}

	args := make([]uint64, 0)
	for _, strArg := range flag.Args() {
		if len(strArg) == 0 {
			continue
		}

		arg, err := strconv.ParseUint(strArg, 10, 64)
		if err != nil {
			panic(err)
		}

		args = append(args, arg)
	}

	r, ty, err := ins.CallExportedFunc(*funcName, args...)
	if err != nil {
		panic(err)
	}

	if r != nil {
		fmt.Printf("type: %v\n", ty[0])
		fmt.Printf("result: %v\n", r[0])
	}

	fmt.Printf("toll: %v\n", ins.GetToll())
}
