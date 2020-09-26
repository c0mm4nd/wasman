package wasman

var (
	magic   = []byte{0x00, 0x61, 0x73, 0x6D} // aka header
	version = []byte{0x01, 0x00, 0x00, 0x00} // version 1, https://www.w3.org/TR/wasm-core-1/
)

const (
	// magic = 0x0061736D
	defaultPageSize = 65536
)
