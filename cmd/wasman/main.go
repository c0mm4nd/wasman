package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/tollstation"
	"os"
	"strconv"
	"strings"

	"github.com/c0mm4nd/wasman"
)

var strMainModuleFile = flag.String("main", "module.wasm", "main module")

var funcName = flag.String("func", "main", "main func")
var maxToll = flag.Uint64("max-toll", 0, "the maximum toll in simple toll station")

var strExternModules = flag.String("extern-files", "", "external modules files")

var stdout = os.Stdout // for wasi

func main() {
	flag.Parse()

	externModules := strings.Split(*strExternModules, ",")
	f, err := os.Open(*strMainModuleFile)
	if err != nil {
		panic(err)
	}

	mainMod, err := wasman.NewModule(config.ModuleConfig{
		DisableFloatPoint: false,
		TollStation:       tollstation.NewSimpleTollStation(*maxToll),
	}, f)
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

		mod, err := wasman.NewModule(config.ModuleConfig{}, f)
		if err != nil {
			panic(err)
		}

		externMods[li[0]] = mod
	}

	l := wasman.NewLinkerWithModuleMap(config.LinkerConfig{}, externMods)
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

	toll := uint64(0)
	if ins.ModuleConfig.TollStation != nil {
		toll = ins.ModuleConfig.TollStation.GetToll()
	}

	result := struct {
		Type   string      `json:"type"`
		Result interface{} `json:"result"`
		Toll   uint64      `json:"toll"`
	}{
		"",
		nil,
		toll,
	}

	if r != nil {
		result.Type = ty[0].String()
		result.Result = r[0]
	}

	out, _ := json.MarshalIndent(result, "", "  ")

	fmt.Printf(string(out))
}
