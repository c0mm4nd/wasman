# WASMan (WebAssembly Manager)

[![](https://godoc.org/github.com/c0mm4nd/wasman?status.svg)](http://godoc.org/github.com/c0mm4nd/wasman)
[![Go Report Card](https://goreportcard.com/badge/github.com/c0mm4nd/wasman)](https://goreportcard.com/report/github.com/c0mm4nd/wasman)

Another wasm interpreter engine for gophers.

## Usage

```bash
$ ./wasman -h
Usage of ./wasman:
  -extern-files string
        external modules files
  -func string
        main func (default "main")
  -main string
        main module (default "module.wasm")
  -max-toll uint
        the maximum toll in simple toll station
```

Example:

module.wasm from https://github.com/C0MM4ND/minimum-wasm-rs/releases/latest

```bash
$ ./wasman -main module.wasm -func fib 20 # calc the fibonacci number
type: i32
result: 6765
toll: 315822
```

If we limit the max toll, it will panic when overflow.

```bash
$ ./wasman -main module.wasm -max-toll 300000 -func fib 20
panic: toll overflow

goroutine 1 [running]:
main.main()
        /home/ubuntu/Desktop/wasman/cmd/wasman/main.go:85 +0x87d
```

## go app embedding

Follow `./cmd/wasman/main.go`

Read [DOC](https://pkg.go.dev/github.com/c0mm4nd/wasman)