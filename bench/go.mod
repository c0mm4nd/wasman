module github.com/c0mm4nd/wasman/bench

go 1.25.0

replace github.com/c0mm4nd/wasman => ../

require (
	github.com/c0mm4nd/wasman v0.0.0-00010101000000-000000000000
	github.com/tetratelabs/wazero v1.12.0
)

require golang.org/x/sys v0.44.0 // indirect
