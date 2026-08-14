module github.com/c0mm4nd/wasman/bench

go 1.25.0

replace github.com/c0mm4nd/wasman => ../

require (
	github.com/c0mm4nd/wasman v0.0.0-00010101000000-000000000000
	github.com/go-interpreter/wagon v0.6.0
	github.com/perlin-network/life v0.0.0-20191203030451-05c0e0f7eaea
	github.com/tetratelabs/wazero v1.12.0
)

require (
	github.com/golang/protobuf v1.2.0 // indirect
	github.com/vmihailenco/msgpack v4.0.4+incompatible // indirect
	golang.org/x/net v0.0.0-20180724234803-3673e40ba225 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.44.0 // indirect
	google.golang.org/appengine v1.6.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

replace github.com/go-interpreter/wagon => github.com/go-interpreter/wagon v0.4.0
